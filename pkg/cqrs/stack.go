package cqrs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/codec/v3"
	"github.com/larsartmann/go-cqrs-lite/command/v3"
	"github.com/larsartmann/go-cqrs-lite/decider/v3"
	"github.com/larsartmann/go-cqrs-lite/event/v3"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v3"
	"github.com/larsartmann/go-cqrs-lite/middleware/v3"
	"github.com/larsartmann/go-cqrs-lite/snapshot/v3"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// newSlogLogger creates an *slog.Logger backed by charm.land/log/v2.
// Required by middleware.EventLogging which expects *slog.Logger.
func newSlogLogger() *slog.Logger {
	return slog.New(log.Default())
}

const (
	backendMemory  = "memory"
	backendSQLite  = "sqlite"
	dbPathInMemory = ":memory:"
)

// CQRSConfig configures the CQRS stack's storage backend and conflict resolution.
type CQRSConfig struct {
	Backend          string
	DBPath           string
	ConflictResolver crdt.ConflictResolver[*model.Item]
}

// CQRSStack wires together the event store, bus, decider repository, read model,
// command/query dispatchers, and projection runner.
//
// ReadModel is embedded so the read-side methods (List, Count, CountByType, Get, Upsert, Tombstone)
// are promoted onto *CQRSStack. This lets *CQRSStack satisfy both the
// internal ReadModel contract and the external sync.SyncStore contract
// without duplicate wrapper methods.
type CQRSStack struct {
	event.Store
	event.Bus
	ReadModel

	Repo              *decider.Repository[SyncItemState]
	CommandDispatcher *command.Dispatcher
	conflictResolver  crdt.ConflictResolver[*model.Item]
	db                *sql.DB
	cancelRunner      context.CancelFunc
	drainDone         <-chan struct{}
}

var _ synclib.SyncStore = (*CQRSStack)(nil)

// NewCQRSStack creates a fully wired CQRS stack based on the given config.
func NewCQRSStack(cfg CQRSConfig) (*CQRSStack, error) {
	ctx := context.Background()

	sr, err := createStoreAndBus(ctx, cfg)
	if err != nil {
		return nil, err
	}

	rm, err := createReadModel(ctx, cfg, sr)
	if err != nil {
		return nil, err
	}

	proj := newProjector(rm)

	if err := sr.bus.Use(
		middleware.EventLogging(newSlogLogger()),
	); err != nil {
		return nil, fmt.Errorf("wire event logging middleware: %w", err)
	}

	cancelRunner, drainDone, err := startProjectionRunner(sr, proj)
	if err != nil {
		return nil, fmt.Errorf("start projection runner: %w", err)
	}

	deciderSpec := decider.Decider[SyncItemState]{
		Initial: InitialState,
		Apply:   fold,
	}

	snapshotStore, stratStoreErr := createSnapshotStore(cfg, sr.db)
	if stratStoreErr != nil {
		return nil, stratStoreErr
	}

	snapshotStrategy, stratErr := snapshot.EveryNEvents(10)
	if stratErr != nil {
		return nil, fmt.Errorf("create snapshot strategy: %w", stratErr)
	}

	repo, err := decider.NewRepository(
		sr.store, sr.bus, deciderSpec,
		decider.WithSnapshotStore[SyncItemState](snapshotStore),
		decider.WithCodec[SyncItemState](codec.JSONCodec{}),
		decider.WithSnapshotStrategy[SyncItemState](snapshotStrategy),
	)
	if err != nil {
		return nil, fmt.Errorf("create decider repository: %w", err)
	}

	commandDispatcher, err := wireCommandDispatcher(repo, cfg.ConflictResolver)
	if err != nil {
		return nil, fmt.Errorf("wire command dispatcher: %w", err)
	}

	return &CQRSStack{
		Store:             sr.store,
		Bus:               sr.bus,
		Repo:              repo,
		ReadModel:         rm,
		CommandDispatcher: commandDispatcher,
		conflictResolver:  cfg.ConflictResolver,
		db:                sr.db,
		cancelRunner:      cancelRunner,
		drainDone:         drainDone,
	}, nil
}

// SyncItem dispatches a SyncItemCommand for a single item.
func (s *CQRSStack) SyncItem(ctx context.Context, item *provider.Item) error {
	aggID := AggregateID(item.Source.Get(), item.ExternalID)

	return s.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
		Item:         toDataItem(item),
		RawJSON:      item.RawJSON,
		Options:      nil,
		outcome:      nil,
	})
}

// TombstoneItem dispatches a TombstoneItemCommand for the given source/externalID
// and reason, hiding the item from the default read model while preserving its
// history. A later sync resurrects it automatically.
func (s *CQRSStack) TombstoneItem(
	ctx context.Context,
	source string,
	sourceID id.ExternalID,
	reason model.TombstoneReason,
) error {
	aggID := AggregateID(source, sourceID)

	return s.CommandDispatcher.Dispatch(ctx, &TombstoneItemCommand{
		BasicCommand: mustNewCommand(commandTypeTombstone, aggID),
		Source:       source,
		SourceID:     sourceID,
		Reason:       reason,
	})
}

// Reconcile tombstones live items for source that are absent from seen,
// detecting upstream deletions. It returns the number of items tombstoned.
//
// Only call this after a COMPLETE fetch (every item the provider currently
// holds), since any live item not present in seen is assumed gone upstream and
// will be tombstoned with ReasonUpstreamGone.
func (s *CQRSStack) Reconcile(ctx context.Context, source string, seen []model.Key) (int, error) {
	src := id.NewProviderID(source)

	live, err := s.List(ctx, model.ItemFilter{Source: &src})
	if err != nil {
		return 0, fmt.Errorf("reconcile: list live items for %s: %w", source, err)
	}

	seenSet := make(map[string]struct{}, len(seen))

	for _, k := range seen {
		seenSet[itemKey(k.Source.Get(), k.ExternalID)] = struct{}{}
	}

	var tombstoned int

	for _, item := range live {
		if ctx.Err() != nil {
			return tombstoned, ctx.Err()
		}

		if _, ok := seenSet[itemKey(item.Source.Get(), item.ExternalID)]; ok {
			continue
		}

		if err := s.TombstoneItem(ctx, source, item.ExternalID, model.ReasonUpstreamGone); err != nil {
			return tombstoned, fmt.Errorf("reconcile: tombstone %s/%s: %w", source, item.ExternalID, err)
		}

		tombstoned++
	}

	return tombstoned, nil
}

// SyncItems syncs a batch of items, returning a summary with per-item results.
func (s *CQRSStack) SyncItems(
	ctx context.Context,
	items []*provider.Item,
) *synclib.SyncSummary {
	summary := &synclib.SyncSummary{
		Results:   make([]synclib.ItemSyncResult, 0, len(items)),
		Synced:    0,
		Conflicts: 0,
		Errors:    0,
	}

	corrID := cqrsid.NewCorrelationID()

	for _, item := range items {
		if ctx.Err() != nil {
			break
		}

		aggID := AggregateID(item.Source.Get(), item.ExternalID)
		dataItem := toDataItem(item)

		var outcome SyncOutcome

		syncOpts := []event.Option{event.WithCorrelationID(corrID)}

		cmd := &SyncItemCommand{
			BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
			Item:         dataItem,
			RawJSON:      item.RawJSON,
			Options:      syncOpts,
			outcome:      &outcome,
		}

		err := s.CommandDispatcher.Dispatch(ctx, cmd)
		if err != nil {
			err = fmt.Errorf("sync %s/%s: %w", item.Source.Get(), item.ExternalID.Get(), err)
		}

		action := classifyAction(err, outcome)

		if err != nil {
			err = fmt.Errorf("eventCount=%d, conflict=%v: %w", outcome.EventCount, outcome.ConflictDetected, err)
		}

		result := synclib.ItemSyncResult{
			SourceID: item.ExternalID,
			Action:   action,
			Error:    err,
		}

		switch result.Action {
		case synclib.ActionError:
			summary.Errors++
		case synclib.ActionCreated, synclib.ActionUpdated, synclib.ActionConflictRemote, synclib.ActionConflictLocal:
			summary.Synced++

			if result.Action == synclib.ActionConflictRemote || result.Action == synclib.ActionConflictLocal {
				summary.Conflicts++
			}
		case synclib.ActionUnchanged, synclib.ActionTombstoned:
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}
