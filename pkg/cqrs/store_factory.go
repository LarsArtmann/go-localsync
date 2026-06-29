package cqrs

import (
	"context"
	"database/sql"

	"github.com/larsartmann/go-cqrs-lite/event/v3"
	"github.com/larsartmann/go-cqrs-lite/snapshot/v3"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/storage/memory/v3"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage/v3"
	cqrswatermill "github.com/larsartmann/go-cqrs-lite/watermill/v3"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

type storeResult struct {
	store   event.Store
	bus     event.Bus
	db      *sql.DB
	journal event.SeekableJournal
	cpStore event.CheckpointStore
}

func createStoreAndBus(ctx context.Context, cfg CQRSConfig) (storeResult, error) {
	switch cfg.Backend {
	case backendMemory, "":
		memStore := cqrsmemory.NewMemoryStore()

		return storeResult{
			store:   memStore,
			bus:     cqrswatermill.NewEventBus(),
			db:      nil,
			journal: memStore,
			cpStore: cqrsmemory.NewMemoryCheckpointStore(),
		}, nil
	case backendSQLite:
		return createSQLiteStore(ctx, cfg)
	default:
		return storeResult{}, pkgerrors.Wrapf(
			pkgerrors.ErrUnknownBackend,
			"unknown backend: %s",
			cfg.Backend,
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
		return storeResult{}, pkgerrors.Wrap(err, "open sqlite database")
	}

	if err := cqrsstorage.SQLiteInitSchema(ctx, db); err != nil {
		_ = db.Close()

		return storeResult{}, pkgerrors.Wrap(err, "init schema")
	}

	store, storeErr := cqrsstorage.NewSQLiteEventStore(db)
	if storeErr != nil {
		_ = db.Close()

		return storeResult{}, pkgerrors.Wrap(storeErr, "create event store")
	}

	if walErr := cqrsstorage.SQLiteEnableWAL(ctx, db); walErr != nil {
		_ = db.Close()

		return storeResult{}, pkgerrors.Wrap(walErr, "enable WAL mode")
	}

	cpStore, cpErr := cqrsstorage.NewSQLiteCheckpointStore(db)
	if cpErr != nil {
		_ = db.Close()

		return storeResult{}, pkgerrors.Wrap(cpErr, "create checkpoint store")
	}

	configureSQLitePool(dbPath, db)

	return storeResult{
		store:   store,
		bus:     cqrswatermill.NewEventBus(),
		db:      db,
		journal: store,
		cpStore: cpStore,
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

		return nil, pkgerrors.ErrDBNil
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
