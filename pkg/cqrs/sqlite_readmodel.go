package cqrs

import (
	"context"
	"database/sql"
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
	actor_login TEXT NOT NULL DEFAULT '',
	actor_avatar_url TEXT NOT NULL DEFAULT '',
	repo_name TEXT NOT NULL DEFAULT '',
	repo_url TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	tombstoned INTEGER NOT NULL DEFAULT 0,
	tombstone_reason TEXT NOT NULL DEFAULT '',
	tombstoned_at DATETIME,
	PRIMARY KEY (source, source_id)
)`

const syncItemsIndexes = `
CREATE INDEX IF NOT EXISTS idx_sync_items_type ON sync_items(type);
CREATE INDEX IF NOT EXISTS idx_sync_items_created_at ON sync_items(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_items_actor ON sync_items(actor_login);
CREATE INDEX IF NOT EXISTS idx_sync_items_repo_name ON sync_items(repo_name);
CREATE INDEX IF NOT EXISTS idx_sync_items_type_created ON sync_items(type, created_at DESC)`

// syncItemsMigrations adds tombstone columns to databases created before they
// existed. SQLite lacks IF NOT EXISTS for ADD COLUMN, so duplicate-column errors
// (already-migrated DBs) are tolerated.
const syncItemsMigrations = `
ALTER TABLE sync_items ADD COLUMN tombstoned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sync_items ADD COLUMN tombstone_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE sync_items ADD COLUMN tombstoned_at DATETIME;`

func migrateSyncItems(ctx context.Context, db *sql.DB) error {
	for stmt := range strings.SplitSeq(syncItemsMigrations, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("migrate sync_items: %v", err))
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
		return nil, fmt.Errorf("sqlite read model: %w", pkgerrors.ErrDBNil)
	}

	if _, err := db.ExecContext(ctx, syncItemsDDL); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("create sync_items table: %v", err))
	}

	if _, err := db.ExecContext(ctx, syncItemsIndexes); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("create sync_items indexes: %v", err))
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
	query := `SELECT item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at, tombstoned, tombstone_reason, tombstoned_at
		FROM sync_items WHERE source = ? AND source_id = ?`

	row := m.db.QueryRowContext(ctx, query, source, sourceID.Get())

	item, err := scanItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("get item %s/%s: %v", source, sourceID, err))
	}

	return item, nil
}

func (m *SQLiteReadModel) List(ctx context.Context, filter model.ItemFilter) ([]*model.Item, error) {
	query, args := buildListQuery(filter)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("list items: %v", err))
	}

	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (m *SQLiteReadModel) Count(ctx context.Context, filter model.ItemFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM sync_items WHERE 1=1"
	args := appendFilterArgs(&query, filter)

	var count int64

	err := m.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return count, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("count items (count=%d): %v", count, err))
	}

	return count, nil
}

func (m *SQLiteReadModel) CountByType(ctx context.Context, filter model.ItemFilter) (map[string]int64, error) {
	query := "SELECT type, COUNT(*) FROM sync_items WHERE 1=1"
	args := appendFilterArgs(&query, filter)
	query += " GROUP BY type"

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("count by type: %v", err))
	}

	defer func() { _ = rows.Close() }()

	counts := make(map[string]int64)

	for rows.Next() {
		var itemType string

		var count int64

		if err := rows.Scan(&itemType, &count); err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("scan count by type: %v", err))
		}

		counts[itemType] = count
	}

	if err := rows.Err(); err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("iterate count by type: %v", err))
	}

	return counts, nil
}

func (m *SQLiteReadModel) GetTypes(ctx context.Context) ([]string, error) {
	query := "SELECT DISTINCT type FROM sync_items WHERE tombstoned = 0 ORDER BY type"

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("get types: %v", err))
	}

	defer func() { _ = rows.Close() }()

	var types []string

	for rows.Next() {
		var t string

		err := rows.Scan(&t)
		if err != nil {
			return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("scan type: %v", err))
		}

		types = append(types, t)
	}

	err = rows.Err()
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("iterate types: %v", err))
	}

	if types == nil {
		types = []string{}
	}

	return types, nil
}

func (m *SQLiteReadModel) Upsert(ctx context.Context, item *model.Item) error {
	// Tombstone columns reset to defaults on upsert: a sync event always writes a
	// live item, so re-syncing a previously-tombstoned item resurrects it.
	query := `INSERT OR REPLACE INTO sync_items
		(item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at, tombstoned, tombstone_reason, tombstoned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '', NULL)`

	_, err := m.db.ExecContext(
		ctx, query,
		item.ID.String(), item.Source.Get(), item.ExternalID.Get(), item.Type.Get(),
		item.ActorLogin.Get(), item.ActorAvatarURL, item.RepoName.Get(),
		item.RepoURL, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("upsert item: %v", err))
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
		return pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("tombstone item %s/%s: %v", source, sourceID, err))
	}

	return nil
}

func (m *SQLiteReadModel) Close() error {
	return m.db.Close()
}

// Ensure SQLiteReadModel implements ReadModel.
var _ ReadModel = (*SQLiteReadModel)(nil)
