package cqrs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

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
	PRIMARY KEY (source, source_id)
)`

const syncItemsIndexes = `
CREATE INDEX IF NOT EXISTS idx_sync_items_type ON sync_items(type);
CREATE INDEX IF NOT EXISTS idx_sync_items_created_at ON sync_items(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_items_actor ON sync_items(actor_login);
CREATE INDEX IF NOT EXISTS idx_sync_items_repo_name ON sync_items(repo_name);
CREATE INDEX IF NOT EXISTS idx_sync_items_type_created ON sync_items(type, created_at DESC)`

// SQLiteReadModel is a SQLite-backed implementation of ReadModel.
type SQLiteReadModel struct {
	db *sql.DB
}

// NewSQLiteReadModel creates a SQLiteReadModel, initializing the schema.
func NewSQLiteReadModel(db *sql.DB) (*SQLiteReadModel, error) {
	if db == nil {
		return nil, fmt.Errorf("sqlite read model: %w", pkgerrors.ErrDBNil)
	}

	ctx := context.Background()

	_, err := db.ExecContext(ctx, syncItemsDDL)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("create sync_items table: %v", err))
	}

	_, err = db.ExecContext(ctx, syncItemsIndexes)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("create sync_items indexes: %v", err))
	}

	return &SQLiteReadModel{db: db}, nil
}

func (m *SQLiteReadModel) Get(
	ctx context.Context,
	source string,
	sourceID id.ExternalID,
) (*model.Item, error) {
	query := `SELECT item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at
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

func (m *SQLiteReadModel) GetTypes(ctx context.Context) ([]string, error) {
	query := "SELECT DISTINCT type FROM sync_items ORDER BY type"

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
	query := `INSERT OR REPLACE INTO sync_items
		(item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

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

func (m *SQLiteReadModel) Delete(
	ctx context.Context,
	source string,
	sourceID id.ExternalID,
) error {
	_, err := m.db.ExecContext(
		ctx,
		"DELETE FROM sync_items WHERE source = ? AND source_id = ?",
		source,
		sourceID.Get(),
	)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("delete item %s/%s: %v", source, sourceID, err))
	}

	return nil
}

func (m *SQLiteReadModel) Close() error {
	return m.db.Close()
}

func buildListQuery(filter model.ItemFilter) (string, []any) {
	query := `SELECT item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at
		FROM sync_items WHERE 1=1`

	args := appendFilterArgs(&query, filter)

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"

		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"

		args = append(args, filter.Offset)
	}

	return query, args
}

func appendFilterArgs(query *string, filter model.ItemFilter) []any {
	var args []any

	if filter.Type != nil {
		*query += " AND type = ?"

		args = append(args, filter.Type.Get())
	}

	if filter.ActorLogin != nil {
		*query += " AND actor_login = ?"

		args = append(args, filter.ActorLogin.Get())
	}

	if filter.RepoName != nil {
		*query += " AND repo_name = ?"

		args = append(args, filter.RepoName.Get())
	}

	if filter.Source != nil {
		*query += " AND source = ?"

		args = append(args, filter.Source.Get())
	}

	if filter.Since != nil {
		*query += " AND created_at > ?"

		args = append(args, filter.Since.Format(time.RFC3339Nano))
	}

	return args
}

type scannedItem struct {
	itemIDStr, source, sourceID, eventType, actorLogin, actorAvatarURL, repoName, repoURL string
	createdAt, updatedAt                                                                  time.Time
}

//nolint:exhaustruct // SchemaVersion not stored in read model schema
func (si *scannedItem) toItem() (*model.Item, error) {
	itemID, err := parseItemID(si.itemIDStr)
	if err != nil {
		return nil, fmt.Errorf("parse item ID from row: %w", err)
	}

	return &model.Item{
		ID:             itemID,
		ExternalID:     id.NewExternalID(si.sourceID),
		Source:         id.NewProviderID(si.source),
		Type:           id.NewEventTypeID(si.eventType),
		ActorLogin:     id.NewActorID(si.actorLogin),
		ActorAvatarURL: si.actorAvatarURL,
		RepoName:       id.NewRepoID(si.repoName),
		RepoURL:        si.repoURL,
		CreatedAt:      si.createdAt,
		UpdatedAt:      si.updatedAt,
	}, nil
}

func newScannedItem() *scannedItem {
	return &scannedItem{
		itemIDStr:      "",
		source:         "",
		sourceID:       "",
		eventType:      "",
		actorLogin:     "",
		actorAvatarURL: "",
		repoName:       "",
		repoURL:        "",
		createdAt:      time.Time{},
		updatedAt:      time.Time{},
	}
}

func scanItem(row *sql.Row) (*model.Item, error) {
	si := newScannedItem()

	err := row.Scan(&si.itemIDStr, &si.source, &si.sourceID, &si.eventType, &si.actorLogin,
		&si.actorAvatarURL, &si.repoName, &si.repoURL, &si.createdAt, &si.updatedAt)
	if err != nil {
		return nil, err
	}

	return si.toItem()
}

func scanItems(rows *sql.Rows) ([]*model.Item, error) {
	var items []*model.Item

	for rows.Next() {
		si := newScannedItem()

		err := rows.Scan(
			&si.itemIDStr,
			&si.source,
			&si.sourceID,
			&si.eventType,
			&si.actorLogin,
			&si.actorAvatarURL,
			&si.repoName,
			&si.repoURL,
			&si.createdAt,
			&si.updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}

		item, err := si.toItem()
		if err != nil {
			return nil, fmt.Errorf("convert row to item: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}

	return items, nil
}

// Ensure SQLiteReadModel implements ReadModel.
var _ ReadModel = (*SQLiteReadModel)(nil)
