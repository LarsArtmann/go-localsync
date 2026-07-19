package cqrs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

const syncItemsDDL = `CREATE TABLE IF NOT EXISTS sync_items (
	item_id TEXT NOT NULL DEFAULT '',
	source TEXT NOT NULL,
	source_id TEXT NOT NULL,
	type TEXT NOT NULL,
	attributes TEXT NOT NULL DEFAULT '{}',
	content_hash TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	tombstoned INTEGER NOT NULL DEFAULT 0,
	tombstone_reason TEXT NOT NULL DEFAULT '',
	tombstoned_at DATETIME,
	schema_version INTEGER NOT NULL DEFAULT 1,
	PRIMARY KEY (source, source_id)
)`

const syncItemsIndexes = `
CREATE INDEX IF NOT EXISTS idx_sync_items_type ON sync_items(type);
CREATE INDEX IF NOT EXISTS idx_sync_items_type_created ON sync_items(type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_items_created_at ON sync_items(created_at DESC);`

// wrapDBErr wraps a database driver error with the Database sentinel while
// preserving the original error chain. Callers can use errors.Is(result,
// pkgerrors.ErrDatabase) for classification AND errors.As/Is on the root
// cause (e.g. sql.ErrConnDone, constraint-violation errors). Without this,
// fmt.Sprintf("…: %v", err) severs the chain and root-cause diagnosis is
// impossible from caller code.
func wrapDBErr(original error, detail string) error {
	return fmt.Errorf("%s: %w: %w", detail, original, pkgerrors.ErrDatabase)
}

// syncItemsMigrations adds tombstone columns to databases created before they
// existed. SQLite lacks IF NOT EXISTS for ADD COLUMN, so duplicate-column errors
// (already-migrated DBs) are tolerated.
const syncItemsMigrations = `
ALTER TABLE sync_items ADD COLUMN tombstoned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sync_items ADD COLUMN tombstone_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_items ADD COLUMN tombstoned_at DATETIME;
ALTER TABLE sync_items ADD COLUMN content_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_items ADD COLUMN schema_version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE sync_items ADD COLUMN attributes TEXT NOT NULL DEFAULT '{}';`

func migrateSyncItems(ctx context.Context, db *sql.DB) error {
	for stmt := range strings.SplitSeq(syncItemsMigrations, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return wrapDBErr(err, "migrate sync_items")
			}
		}
	}

	return nil
}

// SQLiteReadModel is a SQLite-backed implementation of ReadModel.
type SQLiteReadModel struct {
	db *sql.DB
}

// newSQLiteReadModel creates a SQLiteReadModel, initializing the schema.
func newSQLiteReadModel(ctx context.Context, db *sql.DB) (*SQLiteReadModel, error) {
	if db == nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDBNil, "sqlite read model")
	}

	if _, err := db.ExecContext(ctx, syncItemsDDL); err != nil {
		return nil, wrapDBErr(err, "create sync_items table")
	}

	if _, err := db.ExecContext(ctx, syncItemsIndexes); err != nil {
		return nil, wrapDBErr(err, "create sync_items indexes")
	}

	if err := migrateSyncItems(ctx, db); err != nil {
		return nil, err
	}

	return &SQLiteReadModel{db: db}, nil
}

func (m *SQLiteReadModel) Get(
	ctx context.Context,
	source string,
	sourceID id.ExternalID,
) (*model.Item, error) {
	query := `SELECT item_id, source, source_id, type, attributes, created_at, updated_at, tombstoned, tombstone_reason, tombstoned_at, content_hash, schema_version
		FROM sync_items WHERE source = ? AND source_id = ?`

	row := m.db.QueryRowContext(ctx, query, source, sourceID.Get())

	item, err := scanItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, wrapDBErr(err, fmt.Sprintf("get item %s/%s", source, sourceID))
	}

	return item, nil
}

func (m *SQLiteReadModel) List(ctx context.Context, filter model.ItemFilter) ([]*model.Item, error) {
	query, args, err := buildListQuery(filter)
	if err != nil {
		return nil, err
	}

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBErr(err, "list items")
	}

	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (m *SQLiteReadModel) Count(ctx context.Context, filter model.ItemFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM sync_items WHERE 1=1"
	args, err := appendFilterArgs(&query, filter)
	if err != nil {
		return 0, err
	}

	var count int64

	err = m.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, wrapDBErr(err, "count items")
	}

	return count, nil
}

func (m *SQLiteReadModel) CountByType(ctx context.Context, filter model.ItemFilter) (map[string]int64, error) {
	query := "SELECT type, COUNT(*) FROM sync_items WHERE 1=1"
	args, err := appendFilterArgs(&query, filter)
	if err != nil {
		return nil, err
	}
	query += " GROUP BY type"

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapDBErr(err, "count by type")
	}

	defer func() { _ = rows.Close() }()

	counts := make(map[string]int64)

	for rows.Next() {
		var itemType string

		var count int64

		if err := rows.Scan(&itemType, &count); err != nil {
			return nil, wrapDBErr(err, "scan count by type")
		}

		counts[itemType] = count
	}

	if err := rows.Err(); err != nil {
		return nil, wrapDBErr(err, "iterate count by type")
	}

	return counts, nil
}

func (m *SQLiteReadModel) Upsert(ctx context.Context, item *model.Item) error {
	// Tombstone columns reset to defaults on upsert: a sync event always writes a
	// live item, so re-syncing a previously-tombstoned item resurrects it.
	attrsJSON, err := json.Marshal(item.Attributes)
	if err != nil {
		return wrapDBErr(err, "marshal attributes for upsert")
	}

	query := `INSERT OR REPLACE INTO sync_items
		(item_id, source, source_id, type, attributes, content_hash, created_at, updated_at, tombstoned, tombstone_reason, tombstoned_at, schema_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, '', NULL, ?)`

	_, err = m.db.ExecContext(
		ctx, query,
		item.ID.String(), item.Source.Get(), item.ExternalID.Get(), item.Type.Get(),
		string(attrsJSON), item.ContentHash, item.CreatedAt, item.UpdatedAt, item.SchemaVersion.Int(),
	)
	if err != nil {
		return wrapDBErr(err, "upsert item")
	}

	return nil
}

func (m *SQLiteReadModel) Tombstone(
	ctx context.Context,
	source string,
	sourceID id.ExternalID,
	tombstone model.Tombstone,
) error {
	var at any
	if !tombstone.At.IsZero() {
		at = tombstone.At
	}

	_, err := m.db.ExecContext(
		ctx,
		"UPDATE sync_items SET tombstoned = 1, tombstone_reason = ?, tombstoned_at = ? WHERE source = ? AND source_id = ?",
		string(tombstone.Reason),
		at,
		source,
		sourceID.Get(),
	)
	if err != nil {
		return wrapDBErr(err, fmt.Sprintf("tombstone item %s/%s", source, sourceID))
	}

	return nil
}

func (m *SQLiteReadModel) Close() error {
	return m.db.Close()
}

// Ensure SQLiteReadModel implements ReadModel.
var _ ReadModel = (*SQLiteReadModel)(nil)
