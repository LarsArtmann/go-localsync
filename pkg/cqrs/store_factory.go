package cqrs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/event"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/memory"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

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
