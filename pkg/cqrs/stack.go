package cqrs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/core/command"
	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-cqrs-lite/core/query"
	"github.com/larsartmann/go-cqrs-lite/middleware"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

type charmLogAdapter struct {
	logger *log.Logger
}

func (a *charmLogAdapter) Info(msg string, keyvals ...any) {
	a.logger.Info(msg, keyvals...)
}

func (a *charmLogAdapter) Error(msg string, keyvals ...any) {
	a.logger.Error(msg, keyvals...)
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

	if mwErr := sr.bus.Use(
		middleware.EventLogging(&charmLogAdapter{logger: log.Default()}),
	); mwErr != nil {
		return nil, fmt.Errorf("wire event logging middleware: %w", mwErr)
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
		Core: *command.MustNew(commandTypeSyncItem, aggID),
		Item: item,
	})
}

func (s *CQRSStack) DeleteItem(ctx context.Context, source string, sourceID types.ExternalID) error {
	aggID := AggregateID(source, sourceID)

	return s.CommandDispatcher.Dispatch(ctx, &DeleteItemCommand{
		Core:     *command.MustNew(commandTypeDeleteItem, aggID),
		Source:   source,
		SourceID: sourceID,
	})
}

type SyncAction string

const (
	ActionCreated        SyncAction = "created"
	ActionUpdated        SyncAction = "updated"
	ActionConflictRemote SyncAction = "conflict_remote"
	ActionUnchanged      SyncAction = "unchanged"
	ActionError          SyncAction = "error"
)

type ItemSyncResult struct {
	SourceID string
	Action   SyncAction
	Error    error
}

type SyncSummary struct {
	Synced    int
	Conflicts int
	Errors    int
	Results   []ItemSyncResult
}

func (s *CQRSStack) SyncItems(
	ctx context.Context,
	items []*provider.Item,
) *SyncSummary {
	summary := &SyncSummary{
		Results:   make([]ItemSyncResult, 0, len(items)),
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

		result := ItemSyncResult{
			SourceID: item.ExternalID.Get(),
			Action:   classifyAction(err, eventCount, wasNew),
			Error:    err,
		}

		switch result.Action {
		case ActionError:
			summary.Errors++
		case ActionCreated, ActionUpdated, ActionConflictRemote:
			summary.Synced++

			if result.Action == ActionConflictRemote {
				summary.Conflicts++
			}
		case ActionUnchanged:
			// No counters
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}

func classifyAction(err error, eventCount int, wasNew bool) SyncAction {
	if err != nil {
		return ActionError
	}

	if eventCount > 1 {
		return ActionConflictRemote
	}

	if eventCount == 1 && wasNew {
		return ActionCreated
	}

	if eventCount == 1 {
		return ActionUpdated
	}

	return ActionUnchanged
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
