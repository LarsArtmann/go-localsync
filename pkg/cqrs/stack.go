package cqrs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/codec/v2"
	"github.com/larsartmann/go-cqrs-lite/command/v2"
	"github.com/larsartmann/go-cqrs-lite/decider/v2"
	"github.com/larsartmann/go-cqrs-lite/event/v2"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v2"
	"github.com/larsartmann/go-cqrs-lite/middleware/v2"
	"github.com/larsartmann/go-cqrs-lite/query/v2"
	"github.com/larsartmann/go-cqrs-lite/snapshot/v2"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// newSlogLogger creates an slog.Logger backed by charm.land/log/v2.
// Used by middleware.EventLogging to bridge charm's structured logger
// to the slog interface expected by go-cqrs-lite middleware.
func newSlogLogger() *slog.Logger {
	return slog.New(log.Default())
}

const (
	backendMemory  = "memory"
	backendTurso   = "turso"
	dbPathInMemory = ":memory:"
)

// CQRSConfig configures the CQRS stack's storage backend and conflict resolution.
type CQRSConfig struct {
	Backend          string
	DBPath           string
	RemoteURL        string
	AuthToken        string
	ConflictResolver crdt.ConflictResolver[*provider.Item]
}

// CQRSStack wires together the event store, bus, decider repository, read model,
// command/query dispatchers, and projection runner.
type CQRSStack struct {
	Store             event.Store
	Bus               event.Bus
	Repo              *decider.Repository[SyncItemState]
	ReadModel         ReadModel
	CommandDispatcher *command.Dispatcher
	QueryDispatcher   *query.Dispatcher
	conflictResolver  crdt.ConflictResolver[*provider.Item]
	db                *sql.DB
	cancelRunner      context.CancelFunc
}

// NewCQRSStack creates a fully wired CQRS stack based on the given config.
func NewCQRSStack(cfg CQRSConfig) (*CQRSStack, error) {
	sr, err := createStoreAndBus(cfg)
	if err != nil {
		return nil, err
	}

	rm, err := createReadModel(cfg, sr)
	if err != nil {
		return nil, err
	}

	proj := NewProjector(rm)

	checkpointStore, cpErr := createCheckpointStore(cfg, sr.db)
	if cpErr != nil {
		return nil, cpErr
	}

	if err := sr.bus.Use(
		middleware.EventLogging(newSlogLogger()),
	); err != nil {
		return nil, fmt.Errorf("wire event logging middleware: %w", err)
	}

	cancelRunner, err := startProjectionRunner(sr, checkpointStore, proj)
	if err != nil {
		return nil, fmt.Errorf("start projection runner: %w", err)
	}

	deciderSpec := decider.Decider[SyncItemState]{
		Initial: InitialState,
		Fold:    Fold,
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
		BasicCommand: *command.MustNew(commandTypeSyncItem, aggID),
		Item:         item,
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
		BasicCommand: *command.MustNew(commandTypeDeleteItem, aggID),
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
	syncOpts := []event.Option{event.WithCorrelationID(corrID)}

	for _, item := range items {
		aggID := AggregateID(item.Source.Get(), item.ExternalID)

		var (
			eventCount     int
			wasNew         bool
			conflictWinner string
		)

		countingDecide := func(state SyncItemState, ver event.Version) ([]event.Event, error) {
			wasNew = state.IsNew()

			events, err := DecideSync(item, s.conflictResolver, syncOpts...)(state, ver)
			if err != nil {
				return nil, fmt.Errorf(
					"decide sync for %s/%s: %w",
					item.Source.Get(),
					item.ExternalID.Get(),
					err,
				)
			}

			eventCount = len(events)

			if eventCount > 1 {
				var cp ItemConflictFoundPayload

				_ = json.Unmarshal(events[0].Payload(), &cp)
				conflictWinner = cp.Winner
			}

			return events, nil
		}

		err := s.Repo.Execute(ctx, aggID, aggregateType, countingDecide)
		if err != nil {
			err = fmt.Errorf("sync %s/%s (events=%d, winner=%q): %w",
				item.Source.Get(), item.ExternalID.Get(), eventCount, conflictWinner, err)
		}

		result := synclib.ItemSyncResult{
			SourceID: item.ExternalID.Get(),
			Action:   classifyAction(err, eventCount, wasNew, conflictWinner),
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
			// No counters
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}
