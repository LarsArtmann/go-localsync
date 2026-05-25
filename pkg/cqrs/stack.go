package cqrs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"charm.land/log/v2"
	"log/slog"
	"github.com/larsartmann/go-cqrs-lite/core/command"
	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-cqrs-lite/core/query"
	"github.com/larsartmann/go-cqrs-lite/middleware"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func newSlogLogger() *slog.Logger {
	return slog.New(log.Default())
}

const (
	backendMemory  = "memory"
	backendTurso   = "turso"
	dbPathInMemory = ":memory:"
)

type CQRSConfig struct {
	Backend   string
	DBPath    string
	RemoteURL string
	AuthToken string
}

type CQRSStack struct {
	Store             event.Store
	Bus               event.Bus
	Repo              *decider.Repository[SyncItemState]
	ReadModel         ReadModel
	CommandDispatcher *command.Dispatcher
	QueryDispatcher   *query.Dispatcher
	syncDB            *cqrsstorage.TursoSyncDB
	outbox            event.Outbox
	outboxPublisher   *event.OutboxPublisher
	db                *sql.DB
	cancelRunner      context.CancelFunc
}

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

	commandDispatcher, err := wireCommandDispatcher(repo)
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
		syncDB:            sr.syncDB,
		outbox:            sr.outbox,
		outboxPublisher:   outboxPublisher,
		db:                sr.db,
		cancelRunner:      cancelRunner,
	}, nil
}

func (s *CQRSStack) Push(ctx context.Context) error {
	if s.syncDB == nil {
		return nil
	}

	return fmt.Errorf("push: %w", s.syncDB.Push(ctx))
}

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

func (s *CQRSStack) SyncItem(ctx context.Context, item *provider.Item) error {
	aggID := AggregateID(item.Source.Get(), item.ExternalID)

	return s.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: *command.MustNew(commandTypeSyncItem, aggID),
		Item:         item,
	})
}

func (s *CQRSStack) DeleteItem(
	ctx context.Context,
	source string,
	sourceID types.ExternalID,
) error {
	aggID := AggregateID(source, sourceID)

	return s.CommandDispatcher.Dispatch(ctx, &DeleteItemCommand{
		BasicCommand: *command.MustNew(commandTypeDeleteItem, aggID),
		Source:       source,
		SourceID:     sourceID,
	})
}

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

	corrID := id.NewCorrelationID()
	syncOpts := []event.Option{event.WithCorrelationID(corrID)}

	for _, item := range items {
		aggID := AggregateID(item.Source.Get(), item.ExternalID)

		var (
			eventCount int
			wasNew     bool
		)

		countingDecide := func(state SyncItemState, ver event.Version) ([]event.Event, error) {
			wasNew = state.IsNew()

			events, err := DecideSync(item, syncOpts...)(state, ver)
			if err != nil {
				return nil, fmt.Errorf(
					"decide sync for %s/%s: %w",
					item.Source.Get(),
					item.ExternalID.Get(),
					err,
				)
			}

			eventCount = len(events)

			return events, nil
		}

		err := s.Repo.Execute(ctx, aggID, aggregateType, countingDecide)

		result := synclib.ItemSyncResult{
			SourceID: item.ExternalID.Get(),
			Action:   classifyAction(err, eventCount, wasNew),
			Error:    err,
		}

		switch result.Action {
		case synclib.ActionError:
			summary.Errors++
		case synclib.ActionCreated, synclib.ActionUpdated, synclib.ActionConflictRemote:
			summary.Synced++

			if result.Action == synclib.ActionConflictRemote {
				summary.Conflicts++
			}
		case synclib.ActionUnchanged:
			// No counters
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}

func classifyAction(err error, eventCount int, wasNew bool) synclib.SyncAction {
	if err != nil {
		return synclib.ActionError
	}

	if eventCount > 1 {
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

func (s *CQRSStack) Count(ctx context.Context) (int64, error) {
	return s.ReadModel.Count(ctx, ItemFilter{
		Type:       nil,
		ActorLogin: nil,
		RepoName:   nil,
		Source:     nil,
		Since:      nil,
		Limit:      0,
		Offset:     0,
	})
}

func (s *CQRSStack) GetTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

func (s *CQRSStack) ListItems(
	ctx context.Context,
	filter synclib.ItemFilter,
) ([]*provider.Item, error) {
	return s.ReadModel.List(ctx, toItemFilter(filter))
}

func (s *CQRSStack) CountItems(
	ctx context.Context,
	filter synclib.ItemFilter,
) (int64, error) {
	return s.ReadModel.Count(ctx, toItemFilter(filter))
}

func (s *CQRSStack) GetItemTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

func toItemFilter(filter synclib.ItemFilter) ItemFilter {
	return ItemFilter{
		Type:       filter.Type,
		ActorLogin: filter.ActorLogin,
		RepoName:   filter.RepoName,
		Source:     filter.Source,
		Since:      filter.Since,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	}
}

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
