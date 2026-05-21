package cqrs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/memory"
	"github.com/larsartmann/go-cqrs-lite/middleware"
	cqrsprojection "github.com/larsartmann/go-cqrs-lite/projection"
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
	Store        event.Store
	Bus          event.Bus
	Repo         *decider.Repository[SyncItemState]
	ReadModel    ReadModel
	syncDB       *cqrsstorage.TursoSyncDB
	outbox       event.Outbox
	db           *sql.DB
	cancelOutbox context.CancelFunc
	cancelRunner context.CancelFunc
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
		cancelRunner = startProjectionRunner(sr, checkpointStore, proj)
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

	return &CQRSStack{
		Store:        sr.store,
		Bus:          sr.bus,
		Repo:         repo,
		ReadModel:    rm,
		syncDB:       sr.syncDB,
		outbox:       sr.outbox,
		db:           sr.db,
		cancelOutbox: startOutboxPoller(sr.outbox, sr.bus),
		cancelRunner: cancelRunner,
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

	corrID := id.NewCorrelationID()
	syncOpts := []event.Option{event.WithCorrelationID(corrID)}

	for _, item := range items {
		aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

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

	if s.cancelOutbox != nil {
		s.cancelOutbox()
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

func startOutboxPoller(outbox event.Outbox, bus event.Bus) context.CancelFunc {
	if outbox == nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				entries, err := outbox.PollPending(ctx, 100)
				if err != nil {
					continue
				}

				for _, entry := range entries {
					for _, evt := range entry.Events {
						_ = bus.Publish(ctx, evt)
					}

					_ = outbox.Ack(ctx, []event.OutboxID{entry.ID})
				}
			}
		}
	}()

	return cancel
}

func startProjectionRunner(
	sr storeResult,
	checkpointStore event.CheckpointStore,
	proj event.Projection,
) context.CancelFunc {
	runner, err := cqrsprojection.NewRunner(
		sr.loader, sr.bus, checkpointStore,
		cqrsprojection.WithRetry(3, 100*time.Millisecond),
	)
	if err != nil {
		return nil
	}

	if regErr := runner.Register(proj); regErr != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = runner.Run(ctx)
	}()

	return cancel
}

func startInMemoryRunner(
	bus event.Bus,
	checkpointStore event.CheckpointStore,
	proj event.Projection,
) error {
	runner, err := event.NewInMemoryRunner(checkpointStore)
	if err != nil {
		return fmt.Errorf("create in-memory projection runner: %w", err)
	}

	if regErr := runner.Register(proj); regErr != nil {
		return fmt.Errorf("register projector: %w", regErr)
	}

	if subErr := bus.SubscribeAll(runner.Handle); subErr != nil {
		return fmt.Errorf("subscribe projection runner: %w", subErr)
	}

	return nil
}

var errTursoRequiresDB = errors.New("turso backend requires database connection")

type storeResult struct {
	store  event.Store
	bus    event.Bus
	syncDB *cqrsstorage.TursoSyncDB
	outbox event.Outbox
	db     *sql.DB
	loader event.GlobalLoader
}

func createStoreAndBus(cfg CQRSConfig) (storeResult, error) {
	switch cfg.Backend {
	case backendMemory, "":
		return storeResult{
			store:  cqrsmemory.NewMemoryStore(),
			bus:    cqrsmemory.NewMemoryBus(),
			syncDB: nil,
			outbox: nil,
			db:     nil,
			loader: nil,
		}, nil
	case backendTurso:
		return createTursoStore(cfg)
	default:
		return storeResult{}, fmt.Errorf(
			"unknown backend: %s: %w",
			cfg.Backend,
			pkgerrors.ErrUnknownBackend,
		)
	}
}

func createTursoStore(cfg CQRSConfig) (storeResult, error) {
	if cfg.RemoteURL != "" {
		return createTursoRemoteStore(cfg)
	}

	return createTursoLocalStore(cfg)
}

func createTursoRemoteStore(
	cfg CQRSConfig,
) (storeResult, error) {
	ctx := context.Background()

	syncDB, err := cqrsstorage.OpenTursoSync(ctx, cfg.DBPath, cfg.RemoteURL, cfg.AuthToken)
	if err != nil {
		return storeResult{}, fmt.Errorf("open turso sync database: %w", err)
	}

	if err := cqrsstorage.SQLiteInitSchema(ctx, syncDB.DB); err != nil {
		_ = syncDB.Close()

		return storeResult{}, fmt.Errorf("init turso schema: %w", err)
	}

	store, err := cqrsstorage.NewSQLiteEventStore(syncDB.DB)
	if err != nil {
		_ = syncDB.Close()

		return storeResult{}, fmt.Errorf("create turso event store: %w", err)
	}

	outbox, outboxErr := cqrsstorage.NewSQLiteOutbox(syncDB.DB)
	if outboxErr != nil {
		_ = syncDB.Close()

		return storeResult{}, fmt.Errorf("create turso outbox: %w", outboxErr)
	}

	cqrsstorage.ConfigureTursoPool(syncDB.DB)

	txStore, txErr := cqrsstorage.NewSQLTransactionalStore(store, outbox)
	if txErr != nil {
		_ = syncDB.Close()

		return storeResult{}, fmt.Errorf("create turso transactional store: %w", txErr)
	}

	return storeResult{
		store:  txStore,
		bus:    cqrsmemory.NewMemoryBus(),
		syncDB: syncDB,
		outbox: outbox,
		db:     syncDB.DB,
		loader: store,
	}, nil
}

func createTursoLocalStore(
	cfg CQRSConfig,
) (storeResult, error) {
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = dbPathInMemory
	}

	db, err := cqrsstorage.OpenTurso(dbPath)
	if err != nil {
		return storeResult{}, fmt.Errorf("open turso database: %w", err)
	}

	ctx := context.Background()

	if err := cqrsstorage.SQLiteInitSchema(ctx, db); err != nil {
		_ = db.Close()

		return storeResult{}, fmt.Errorf("init turso schema: %w", err)
	}

	store, storeErr := cqrsstorage.NewSQLiteEventStore(db)
	if storeErr != nil {
		_ = db.Close()

		return storeResult{}, fmt.Errorf("create turso event store: %w", storeErr)
	}

	outbox, outboxErr := cqrsstorage.NewSQLiteOutbox(db)
	if outboxErr != nil {
		_ = db.Close()

		return storeResult{}, fmt.Errorf("create turso outbox: %w", outboxErr)
	}

	cqrsstorage.ConfigureTursoPool(db)

	txStore, txErr := cqrsstorage.NewSQLTransactionalStore(store, outbox)
	if txErr != nil {
		_ = db.Close()

		return storeResult{}, fmt.Errorf("create turso transactional store: %w", txErr)
	}

	return storeResult{
		store:  txStore,
		bus:    cqrsmemory.NewMemoryBus(),
		syncDB: nil,
		outbox: outbox,
		db:     db,
		loader: store,
	}, nil
}

//nolint:ireturn
func createReadModel(cfg CQRSConfig, sr storeResult) (ReadModel, error) {
	if cfg.Backend == backendTurso {
		if sr.db != nil {
			return NewTursoReadModel(sr.db)
		}

		return nil, fmt.Errorf("%w", errTursoRequiresDB)
	}

	return NewMemoryReadModel(), nil
}

//nolint:ireturn
func createSnapshotStore(
	cfg CQRSConfig,
	db *sql.DB,
) (event.SnapshotStore, error) {
	if cfg.Backend != backendTurso || db == nil {
		return cqrsmemory.NewMemorySnapshotStore(), nil
	}

	return cqrsstorage.NewSQLiteSnapshotStore(db)
}

//nolint:ireturn
func createCheckpointStore(
	cfg CQRSConfig,
	db *sql.DB,
) (event.CheckpointStore, error) {
	if cfg.Backend != backendTurso || db == nil {
		return cqrsmemory.NewCheckpointStore(), nil
	}

	return cqrsstorage.NewSQLiteCheckpointStore(db)
}
