package cqrs

import (
	"context"
	"database/sql"
	"fmt"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/memory"
	"github.com/larsartmann/go-cqrs-lite/middleware"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
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
	Store     event.Store
	Bus       event.Bus
	Repo      *decider.Repository[SyncItemState]
	ReadModel ReadModel
	Runner    *event.InMemoryRunner
	syncDB    *cqrsstorage.TursoSyncDB
}

func NewCQRSStack(cfg CQRSConfig) (*CQRSStack, error) {
	store, bus, syncDB, err := createStoreAndBus(cfg)
	if err != nil {
		return nil, err
	}

	rm, err := createReadModel(cfg, syncDB)
	if err != nil {
		return nil, err
	}

	proj := NewProjector(rm)

	runner, err := event.NewInMemoryRunner(cqrsmemory.NewCheckpointStore())
	if err != nil {
		return nil, fmt.Errorf("create projection runner: %w", err)
	}

	if err := runner.Register(proj); err != nil {
		return nil, fmt.Errorf("register projector: %w", err)
	}

	err = bus.SubscribeAll(runner.Handle)
	if err != nil {
		return nil, fmt.Errorf("subscribe projection runner: %w", err)
	}

	if mwErr := bus.Use(
		middleware.EventLogging(&charmLogAdapter{logger: log.Default()}),
	); mwErr != nil {
		return nil, fmt.Errorf("wire event logging middleware: %w", mwErr)
	}

	deciderSpec := decider.Decider[SyncItemState]{
		Initial: InitialState,
		Fold:    Fold,
	}

	snapshotStore := cqrsmemory.NewMemorySnapshotStore()

	snapshotStrategy, stratErr := event.EveryNEvents(10)
	if stratErr != nil {
		return nil, fmt.Errorf("create snapshot strategy: %w", stratErr)
	}

	repo, err := decider.NewRepository[SyncItemState](
		store, bus, deciderSpec,
		decider.WithSnapshotStore[SyncItemState](snapshotStore),
		decider.WithCodec[SyncItemState](event.JSONCodec{}),
		decider.WithSnapshotStrategy[SyncItemState](snapshotStrategy),
	)
	if err != nil {
		return nil, fmt.Errorf("create decider repository: %w", err)
	}

	return &CQRSStack{
		Store:     store,
		Bus:       bus,
		Repo:      repo,
		ReadModel: rm,
		Runner:    runner,
		syncDB:    syncDB,
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
	aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

	return s.Repo.Execute(ctx, aggID, aggregateType, DecideSync(item))
}

func (s *CQRSStack) DeleteItem(ctx context.Context, source, sourceID string) error {
	aggID := AggregateID(source, sourceID)

	return s.Repo.Execute(ctx, aggID, aggregateType, DecideDelete(source, sourceID))
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

	for _, item := range items {
		aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

		var (
			eventCount int
			wasNew     bool
		)

		countingDecide := func(state SyncItemState, ver event.Version) ([]event.Event, error) {
			wasNew = state.IsNew()

			events, err := DecideSync(item)(state, ver)
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
		default:
			// ActionUnchanged: no counters
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
	err := s.ReadModel.Close()
	if err != nil {
		return err
	}

	return s.Store.Close()
}

//nolint:ireturn
func createStoreAndBus(cfg CQRSConfig) (event.Store, event.Bus, *cqrsstorage.TursoSyncDB, error) {
	switch cfg.Backend {
	case backendMemory, "":
		return cqrsmemory.NewMemoryStore(), cqrsmemory.NewMemoryBus(), nil, nil
	case backendTurso:
		return createTursoStore(cfg)
	default:
		return nil, nil, nil, fmt.Errorf(
			"unknown backend: %s: %w",
			cfg.Backend,
			pkgerrors.ErrUnknownBackend,
		)
	}
}

//nolint:ireturn
func createTursoStore(cfg CQRSConfig) (event.Store, event.Bus, *cqrsstorage.TursoSyncDB, error) {
	if cfg.RemoteURL != "" {
		return createTursoRemoteStore(cfg)
	}

	return createTursoLocalStore(cfg)
}

type dbExecContext interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

//nolint:ireturn
func createTursoRemoteStore(
	cfg CQRSConfig,
) (event.Store, event.Bus, *cqrsstorage.TursoSyncDB, error) {
	ctx := context.Background()

	syncDB, err := cqrsstorage.OpenTursoSync(ctx, cfg.DBPath, cfg.RemoteURL, cfg.AuthToken)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open turso sync database: %w", err)
	}

	if err := initSchema(ctx, syncDB); err != nil {
		_ = syncDB.Close()

		return nil, nil, nil, err
	}

	store, err := cqrsstorage.NewSQLiteEventStore(syncDB.DB)
	if err != nil {
		_ = syncDB.Close()

		return nil, nil, nil, fmt.Errorf("create turso event store: %w", err)
	}

	return store, cqrsmemory.NewMemoryBus(), syncDB, nil
}

//nolint:ireturn
func createTursoLocalStore(
	cfg CQRSConfig,
) (event.Store, event.Bus, *cqrsstorage.TursoSyncDB, error) {
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = dbPathInMemory
	}

	db, err := cqrsstorage.OpenTurso(dbPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open turso database: %w", err)
	}

	ctx := context.Background()

	if err := initSchema(ctx, db); err != nil {
		_ = db.Close()

		return nil, nil, nil, err
	}

	store, err := cqrsstorage.NewSQLiteEventStore(db)
	if err != nil {
		_ = db.Close()

		return nil, nil, nil, fmt.Errorf("create turso event store: %w", err)
	}

	return store, cqrsmemory.NewMemoryBus(), nil, nil
}

func initSchema(ctx context.Context, db dbExecContext) error {
	_, err := db.ExecContext(ctx, cqrsstorage.SQLiteSchema())
	if err != nil {
		return fmt.Errorf("create event store schema: %w", err)
	}

	return nil
}

//nolint:ireturn
func createReadModel(cfg CQRSConfig, syncDB *cqrsstorage.TursoSyncDB) (ReadModel, error) {
	if cfg.Backend == backendTurso {
		if syncDB != nil {
			return NewTursoReadModel(syncDB.DB)
		}

		dbPath := cfg.DBPath
		if dbPath == "" {
			dbPath = dbPathInMemory
		}

		readDB, err := cqrsstorage.OpenTurso(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open turso read model db: %w", err)
		}

		return NewTursoReadModel(readDB)
	}

	return NewMemoryReadModel(), nil
}
