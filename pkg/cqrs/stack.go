package cqrs

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/memory"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage"
	"github.com/larsartmann/go-localsync/pkg/provider"
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

	err = bus.SubscribeAll(proj.HandleEvent)
	if err != nil {
		return nil, fmt.Errorf("subscribe projector: %w", err)
	}

	d := decider.Decider[SyncItemState]{
		Initial: InitialState,
		Fold:    Fold,
	}

	repo, err := decider.NewRepository[SyncItemState](store, bus, d)
	if err != nil {
		return nil, fmt.Errorf("create decider repository: %w", err)
	}

	return &CQRSStack{
		Store:     store,
		Bus:       bus,
		Repo:      repo,
		ReadModel: rm,
		syncDB:    syncDB,
	}, nil
}

func (s *CQRSStack) Push(ctx context.Context) error {
	if s.syncDB == nil {
		return nil
	}

	return s.syncDB.Push(ctx)
}

func (s *CQRSStack) Pull(ctx context.Context) (bool, error) {
	if s.syncDB == nil {
		return false, nil
	}

	return s.syncDB.Pull(ctx)
}

func (s *CQRSStack) SyncItem(ctx context.Context, item *provider.Item) error {
	aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

	return s.Repo.Execute(ctx, aggID, aggregateType, DecideSync(item))
}

func (s *CQRSStack) DeleteItem(ctx context.Context, source, sourceID string) error {
	aggID := AggregateID(source, sourceID)

	return s.Repo.Execute(ctx, aggID, aggregateType, DecideDelete(source, sourceID))
}

//nolint:nonamedreturns
func (s *CQRSStack) SyncItems(
	ctx context.Context,
	items []*provider.Item,
) (synced, conflicts, errs int) {
	for _, item := range items {
		aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

		_, beforeVer, _ := s.Repo.Load(ctx, aggID, aggregateType)

		err := s.Repo.Execute(ctx, aggID, aggregateType, DecideSync(item))
		if err != nil {
			errs++

			continue
		}

		_, afterVer, _ := s.Repo.Load(ctx, aggID, aggregateType)

		delta := int(afterVer) - int(beforeVer)
		if delta > 0 {
			synced++

			if delta > 1 {
				conflicts += delta - 1
			}
		}
	}

	return synced, conflicts, errs
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

func createStoreAndBus(cfg CQRSConfig) (event.Store, event.Bus, *cqrsstorage.TursoSyncDB, error) {
	switch cfg.Backend {
	case "memory", "":
		return cqrsmemory.NewMemoryStore(), cqrsmemory.NewMemoryBus(), nil, nil
	case "turso":
		return createTursoStore(cfg)
	default:
		return nil, nil, nil, fmt.Errorf("unknown backend: %s", cfg.Backend)
	}
}

func createTursoStore(cfg CQRSConfig) (event.Store, event.Bus, *cqrsstorage.TursoSyncDB, error) {
	if cfg.RemoteURL != "" {
		ctx := context.Background()
		syncDB, err := cqrsstorage.OpenTursoSync(ctx, cfg.DBPath, cfg.RemoteURL, cfg.AuthToken)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open turso sync database: %w", err)
		}

		if _, err := syncDB.Exec(cqrsstorage.SQLiteSchema()); err != nil {
			_ = syncDB.Close()
			return nil, nil, nil, fmt.Errorf("create event store schema: %w", err)
		}

		store, err := cqrsstorage.NewSQLiteEventStore(syncDB.DB)
		if err != nil {
			_ = syncDB.Close()
			return nil, nil, nil, fmt.Errorf("create turso event store: %w", err)
		}

		return store, cqrsmemory.NewMemoryBus(), syncDB, nil
	}

	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = ":memory:"
	}

	db, err := cqrsstorage.OpenTurso(dbPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open turso database: %w", err)
	}

	if _, err := db.Exec(cqrsstorage.SQLiteSchema()); err != nil {
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("create event store schema: %w", err)
	}

	store, err := cqrsstorage.NewSQLiteEventStore(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, nil, fmt.Errorf("create turso event store: %w", err)
	}

	return store, cqrsmemory.NewMemoryBus(), nil, nil
}

func createReadModel(cfg CQRSConfig, syncDB *cqrsstorage.TursoSyncDB) (ReadModel, error) {
	if cfg.Backend == "turso" {
		if syncDB != nil {
			return NewTursoReadModel(syncDB.DB)
		}

		dbPath := cfg.DBPath
		if dbPath == "" {
			dbPath = ":memory:"
		}

		readDB, err := cqrsstorage.OpenTurso(dbPath)
		if err != nil {
			return nil, fmt.Errorf("open turso read model db: %w", err)
		}

		return NewTursoReadModel(readDB)
	}

	return NewMemoryReadModel(), nil
}
