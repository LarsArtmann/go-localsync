package cqrs

import (
	"context"
	"database/sql"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-cqrs-lite/snapshot/v4"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/storage/memory/v4"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage/v4"
	cqrswatermill "github.com/larsartmann/go-cqrs-lite/watermill/v4"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

type storeResult struct {
	store   event.Store
	bus     event.Bus
	db      *sql.DB
	journal event.SeekableJournal
	cpStore event.CheckpointStore
	dlq     projectionhost.DeadLetterStore
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
			// Ephemeral DLQ to mirror the ephemeral store (C017 discipline: DLQ
			// lifetime matches event-store lifetime; the sqlite branch below wires
			// the persistent SQLite DLQ — this is the memory-backend branch only).
			//cqrs-lint:ignore(C017) memory backend pairs with the ephemeral memory store by design; the sqlite branch persists
			dlq: projectionhost.NewMemoryDeadLetterStore(),
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
		closeLogged("sqlite db (schema init failed)", db)

		return storeResult{}, pkgerrors.Wrap(err, "init schema")
	}

	store, storeErr := cqrsstorage.NewSQLiteEventStore(db)
	if storeErr != nil {
		closeLogged("sqlite db (event store init failed)", db)

		return storeResult{}, pkgerrors.Wrap(storeErr, "create event store")
	}

	if walErr := cqrsstorage.SQLiteEnableWAL(ctx, db); walErr != nil {
		closeLogged("sqlite db (WAL enable failed)", db)

		return storeResult{}, pkgerrors.Wrap(walErr, "enable WAL mode")
	}

	cpStore, cpErr := cqrsstorage.NewSQLiteCheckpointStore(db)
	if cpErr != nil {
		closeLogged("sqlite db (checkpoint store init failed)", db)

		return storeResult{}, pkgerrors.Wrap(cpErr, "create checkpoint store")
	}

	// The DLQ shares the SQLite file so captured poison events survive restarts
	// (C017): catch-up no longer re-processes and re-captures the same event
	// after every crash. The memory backend keeps its in-memory DLQ.
	dlq, dlqErr := projectionhost.NewSQLiteDeadLetterStore(ctx, db)
	if dlqErr != nil {
		closeLogged("sqlite db (dead-letter store init failed)", db)

		return storeResult{}, pkgerrors.Wrap(dlqErr, "create dead-letter store")
	}

	configureSQLitePool(dbPath, db)

	return storeResult{
		store:   store,
		bus:     cqrswatermill.NewEventBus(),
		db:      db,
		journal: store,
		cpStore: cpStore,
		dlq:     dlq,
	}, nil
}

// configureSQLitePool sets connection pool size and PRAGMAs based on whether
// configureSQLitePool constrains the connection pool for the SQLite driver.
//
// SQLite via database/sql requires a single connection so PRAGMAs (busy_timeout,
// journal_mode) apply consistently to all statements, and so an in-memory
// database (:memory:) shares its state across queries. A second connection would
// get a fresh private database for :memory: and would need its own PRAGMA setup,
// so MaxOpenConns is pinned to 1 for both in-memory and file-backed databases.
func configureSQLitePool(dbPath string, db *sql.DB) {
	_ = dbPath

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
