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

func createStoreAndBus(ctx context.Context, cfg CQRSConfig) (storeResult, error) {
	switch cfg.Backend {
	case backendMemory, "":
		return storeResult{
			store:  cqrsmemory.NewMemoryStore(),
			bus:    cqrsmemory.NewMemoryBus(),
			db:     nil,
			loader: nil,
		}, nil
	case backendSQLite:
		return createSQLiteStore(ctx, cfg)
	default:
		return storeResult{}, fmt.Errorf(
			"unknown backend: %s: %w",
			cfg.Backend,
			pkgerrors.ErrUnknownBackend,
		)
	}
}

func createSQLiteStore(ctx context.Context, cfg CQRSConfig) (storeResult, error) {
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

	if walErr := cqrsstorage.SQLiteEnableWAL(ctx, db); walErr != nil {
		_ = db.Close()

		return storeResult{}, fmt.Errorf("enable WAL mode: %w", walErr)
	}

	configureSQLitePool(dbPath, db)

	return storeResult{
		store:  store,
		bus:    cqrsmemory.NewMemoryBus(),
		db:     db,
		loader: store,
	}, nil
}

// configureSQLitePool sets connection pool size and PRAGMAs based on whether
// the database is in-memory or file-backed.
//
// In-memory databases MUST use a single connection (each connection gets its
// own private database). File-backed databases use WAL mode with multiple
// reader connections for concurrent read access.
func configureSQLitePool(_ string, db *sql.DB) {
	// SQLite with database/sql requires a single connection to ensure
	// PRAGMAs (like busy_timeout) apply consistently. Multiple connections
	// would each need their own PRAGMA setup, and in-memory databases
	// require exactly 1 connection to share state.
	db.SetMaxOpenConns(1)
}

//nolint:ireturn
func createReadModel(ctx context.Context, cfg CQRSConfig, sr storeResult) (ReadModel, error) {
	if cfg.Backend == backendSQLite {
		if sr.db != nil {
			return newSQLiteReadModel(ctx, sr.db)
		}

		return nil, errSQLiteRequiresDB
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
