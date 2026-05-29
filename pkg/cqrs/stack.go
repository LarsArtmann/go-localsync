package cqrs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/core/command"
	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	cqrsid "github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-cqrs-lite/core/query"
	"github.com/larsartmann/go-cqrs-lite/middleware"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage"
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
// command/query dispatchers, outbox publisher, and projection runner.
type CQRSStack struct {
	Store             event.Store
	Bus               event.Bus
	Repo              *decider.Repository[SyncItemState]
	ReadModel         ReadModel
	CommandDispatcher *command.Dispatcher
	QueryDispatcher   *query.Dispatcher
	conflictResolver  crdt.ConflictResolver[*provider.Item]
	syncDB            *cqrsstorage.TursoSyncDB
	outbox            event.Outbox
	outboxPublisher   *event.OutboxPublisher
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

	var cancelRunner context.CancelFunc

	if err := sr.bus.Use(
		middleware.EventLogging(newSlogLogger()),
	); err != nil {
		return nil, fmt.Errorf("wire event logging middleware: %w", err)
	}

	if sr.loader != nil {
		cancelRunner, err = startProjectionRunner(sr, checkpointStore, proj)
		if err != nil {
			return nil, fmt.Errorf("start projection runner: %w", err)
		}
	} else {
		if err := startInMemoryRunner(sr.bus, checkpointStore, proj); err != nil {
			return nil, err
		}
	}

	deciderSpec := decider.Decider[SyncItemState]{
		Initial: InitialState,
		Fold:    Fold,
	}

	snapshotStore, stratStoreErr := createSnapshotStore(cfg, sr.db)
	if stratStoreErr != nil {
		return nil, stratStoreErr
	}

	snapshotStrategy, stratErr := event.EveryNEvents(10)
	if stratErr != nil {
		return nil, fmt.Errorf("create snapshot strategy: %w", stratErr)
	}

	var repoOpts []decider.RepositoryOption[SyncItemState]

	repoOpts = append(
		repoOpts,
		decider.WithSnapshotStore[SyncItemState](snapshotStore),
		decider.WithCodec[SyncItemState](event.JSONCodec{}),
		decider.WithSnapshotStrategy[SyncItemState](snapshotStrategy),
	)

	if sr.outbox != nil {
		repoOpts = append(repoOpts, decider.WithOutbox[SyncItemState](sr.outbox))
	}

	repo, err := decider.NewRepository[SyncItemState](
		sr.store, sr.bus, deciderSpec, repoOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("create decider repository: %w", err)
	}

	outboxPublisher, err := startOutboxPublisher(sr.outbox, sr.bus)
	if err != nil {
		return nil, fmt.Errorf("start outbox publisher: %w", err)
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
		syncDB:            sr.syncDB,
		outbox:            sr.outbox,
		outboxPublisher:   outboxPublisher,
		db:                sr.db,
		cancelRunner:      cancelRunner,
	}, nil
}

// Push syncs local changes to the remote Turso database.
func (s *CQRSStack) Push(ctx context.Context) error {
	if s.syncDB == nil {
		return nil
	}

	return fmt.Errorf("push: %w", s.syncDB.Push(ctx))
}

// Pull syncs remote changes from the Turso database.
func (s *CQRSStack) Pull(ctx context.Context) (bool, error) {
	if s.syncDB == nil {
		return false, nil
	}

	ok, err := s.syncDB.Pull(ctx)
	if err != nil {
		return false, fmt.Errorf("pull: %w", err)
	}

	return ok, nil
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

func classifyAction(err error, eventCount int, wasNew bool, conflictWinner string) synclib.SyncAction {
	if err != nil {
		return synclib.ActionError
	}

	if eventCount > 1 {
		if conflictWinner == conflictWinnerLocal {
			return synclib.ActionConflictLocal
		}

		return synclib.ActionConflictRemote
	}

	if eventCount == 1 && wasNew {
		return synclib.ActionCreated
	}

	if eventCount == 1 {
		return synclib.ActionUpdated
	}

	return synclib.ActionUnchanged
}

// Count returns the total number of items in the read model.
func (s *CQRSStack) Count(ctx context.Context) (int64, error) {
	return s.ReadModel.Count(ctx, provider.ItemFilter{
		Type:       nil,
		ActorLogin: nil,
		RepoName:   nil,
		Source:     nil,
		Since:      nil,
		Limit:      0,
		Offset:     0,
	})
}

// GetTypes returns all distinct item types in the read model.
func (s *CQRSStack) GetTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

// ListItems delegates to the read model's List method.
func (s *CQRSStack) ListItems(
	ctx context.Context,
	filter provider.ItemFilter,
) ([]*provider.Item, error) {
	return s.ReadModel.List(ctx, filter)
}

// CountItems delegates to the read model's Count method.
func (s *CQRSStack) CountItems(
	ctx context.Context,
	filter provider.ItemFilter,
) (int64, error) {
	return s.ReadModel.Count(ctx, filter)
}

// GetItemTypes delegates to the read model's GetTypes method.
func (s *CQRSStack) GetItemTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

// Close gracefully shuts down all components: dispatchers, outbox publisher,
// projection runner, read model, outbox, and event store.
func (s *CQRSStack) Close() error {
	var errs []error

	if s.CommandDispatcher != nil {
		if err := s.CommandDispatcher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.QueryDispatcher != nil {
		if err := s.QueryDispatcher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.outboxPublisher != nil {
		if err := s.outboxPublisher.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.cancelRunner != nil {
		s.cancelRunner()
	}

	if err := s.ReadModel.Close(); err != nil {
		errs = append(errs, err)
	}

	if s.outbox != nil {
		if err := s.outbox.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := s.Store.Close(); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}
