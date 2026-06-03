package cqrs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/event/v2"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/memory/v2"
	"github.com/larsartmann/go-cqrs-lite/snapshot/v2"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

var errSQLiteRequiresDB = errors.New("sqlite backend requires database connection")

type storeResult struct {
	store  event.Store
	bus    event.Bus
	db     *sql.DB
	loader event.Journal
}

func createStoreAndBus(cfg CQRSConfig) (storeResult, error) {
	switch cfg.Backend {
	case backendMemory, "":
		return storeResult{
			store:  cqrsmemory.NewMemoryStore(),
			bus:    cqrsmemory.NewMemoryBus(),
			db:     nil,
			loader: nil,
		}, nil
	case backendSQLite:
		return createSQLiteStore(cfg)
	default:
		return storeResult{}, fmt.Errorf(
			"unknown backend: %s: %w",
			cfg.Backend,
			pkgerrors.ErrUnknownBackend,
		)
	}
}

func createSQLiteStore(cfg CQRSConfig) (storeResult, error) {
	ctx := context.Background()

	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = dbPathInMemory
	}

	db, err := cqrsstorage.OpenSQLite(dbPath)
	if err != nil {
		return storeResult{}, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := cqrsstorage.SQLiteInitSchema(ctx, db); err != nil {
		_ = db.Close()

		return storeResult{}, fmt.Errorf("init schema: %w", err)
	}

	store, storeErr := cqrsstorage.NewSQLiteEventStore(db)
	if storeErr != nil {
		_ = db.Close()

		return storeResult{}, fmt.Errorf("create event store: %w", storeErr)
	}

	cqrsstorage.ConfigureTursoPool(db)

	return storeResult{
		store:  store,
		bus:    cqrsmemory.NewMemoryBus(),
		db:     db,
		loader: store,
	}, nil
}

//nolint:ireturn
func createReadModel(cfg CQRSConfig, sr storeResult) (ReadModel, error) {
	if cfg.Backend == backendSQLite {
		if sr.db != nil {
			return NewSQLiteReadModel(sr.db)
		}

		return nil, fmt.Errorf("%w", errSQLiteRequiresDB)
	}

	return NewMemoryReadModel(), nil
}

//nolint:ireturn
func createSnapshotStore(
	cfg CQRSConfig,
	db *sql.DB,
) (snapshot.SnapshotStore, error) {
	if cfg.Backend != backendSQLite || db == nil {
		return cqrsmemory.NewMemorySnapshotStore(), nil
	}

	return cqrsstorage.NewSQLiteSnapshotStore(db)
}

//nolint:ireturn
func createCheckpointStore(
	cfg CQRSConfig,
	db *sql.DB,
) (event.CheckpointStore, error) {
	if cfg.Backend != backendSQLite || db == nil {
		return cqrsmemory.NewMemoryCheckpointStore(), nil
	}

	return cqrsstorage.NewSQLiteCheckpointStore(db)
}
