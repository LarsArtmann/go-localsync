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
	"github.com/larsartmann/go-cqrs-lite/query/v3"
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
// ReadModel is embedded so the read-side methods (List, Count, GetTypes, Get, Upsert, Delete)
// are promoted onto *CQRSStack. This lets *CQRSStack satisfy both the
// internal ReadModel contract and the external sync.SyncStore contract
// without duplicate wrapper methods.
type CQRSStack struct {
	event.Store
	event.Bus
	ReadModel

	Repo              *decider.Repository[SyncItemState]
	CommandDispatcher *command.Dispatcher
	QueryDispatcher   *query.Dispatcher
	conflictResolver  crdt.ConflictResolver[*model.Item]
	db                *sql.DB
	cancelRunner      context.CancelFunc
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

	cancelRunner, err := startProjectionRunner(sr, proj)
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

	repo, err := decider.NewRepository[SyncItemState](
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

	queryDispatcher, err := wireQueryDispatcher(rm)
	if err != nil {
		return nil, fmt.Errorf("wire query dispatcher: %w", err)
	}

	return &CQRSStack{
		Store:             sr.store,
		Bus:               sr.bus,
		Repo:              repo,
		ReadModel:         rm,
		CommandDispatcher: commandDispatcher,
		QueryDispatcher:   queryDispatcher,
		conflictResolver:  cfg.ConflictResolver,
		db:                sr.db,
		cancelRunner:      cancelRunner,
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
	})
}

// DeleteItem dispatches a DeleteItemCommand for the given source/externalID.
func (s *CQRSStack) DeleteItem(
	ctx context.Context,
	source string,
	sourceID id.ExternalID,
) error {
	aggID := AggregateID(source, sourceID)

	return s.CommandDispatcher.Dispatch(ctx, &DeleteItemCommand{
		BasicCommand: mustNewCommand(commandTypeDeleteItem, aggID),
		Source:       source,
		SourceID:     sourceID,
	})
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
		}

		itemCtx := contextWithSyncOutcome(ctx, &outcome)

		err := s.CommandDispatcher.Dispatch(itemCtx, cmd)
		if err != nil {
			err = fmt.Errorf("sync %s/%s: %w", item.Source.Get(), item.ExternalID.Get(), err)
		}

		action := classifyAction(err, outcome.EventCount, outcome.WasNew, outcome.ConflictWinner)

		if err != nil {
			err = fmt.Errorf("eventCount=%d, conflictWinner=%s: %w", outcome.EventCount, outcome.ConflictWinner, err)
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
		case synclib.ActionUnchanged:
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}
