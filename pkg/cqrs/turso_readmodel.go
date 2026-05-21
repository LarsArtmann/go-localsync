package cqrs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
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
	raw_json TEXT NOT NULL DEFAULT '{}',
	PRIMARY KEY (source, source_id)
)`

const syncItemsIndexes = `
CREATE INDEX IF NOT EXISTS idx_sync_items_type ON sync_items(type);
CREATE INDEX IF NOT EXISTS idx_sync_items_created_at ON sync_items(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sync_items_actor ON sync_items(actor_login)`

type TursoReadModel struct {
	db *sql.DB
}

func NewTursoReadModel(db *sql.DB) (*TursoReadModel, error) {
	if db == nil {
		return nil, fmt.Errorf("turso read model: %w", pkgerrors.ErrDBNil)
	}

	ctx := context.Background()

	_, err := db.ExecContext(ctx, syncItemsDDL)
	if err != nil {
		return nil, fmt.Errorf("create sync_items table: %w", err)
	}

	_, err = db.ExecContext(ctx, syncItemsIndexes)
	if err != nil {
		return nil, fmt.Errorf("create sync_items indexes: %w", err)
	}

	return &TursoReadModel{db: db}, nil
}

func (m *TursoReadModel) Get(ctx context.Context, source, sourceID string) (*provider.Item, error) {
	query := `SELECT item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at, raw_json
		FROM sync_items WHERE source = ? AND source_id = ?`

	row := m.db.QueryRowContext(ctx, query, source, sourceID)

	item, err := scanItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, fmt.Errorf("get item %s/%s: %w", source, sourceID, err)
	}

	return item, nil
}

func (m *TursoReadModel) List(ctx context.Context, filter ItemFilter) ([]*provider.Item, error) {
	query, args := buildListQuery(filter)

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list items with filter %+v: %w", filter, err)
	}

	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (m *TursoReadModel) Count(ctx context.Context, filter ItemFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM sync_items WHERE 1=1"
	args := appendFilterArgs(&query, filter)

	var count int64

	err := m.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return count, fmt.Errorf("count items with filter %+v: %w", filter, err)
	}

	return count, nil
}

func (m *TursoReadModel) GetTypes(ctx context.Context) ([]string, error) {
	query := "SELECT DISTINCT type FROM sync_items ORDER BY type"

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get types: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var types []string

	for rows.Next() {
		var t string

		err := rows.Scan(&t)
		if err != nil {
			return nil, fmt.Errorf("scan type: %w", err)
		}

		types = append(types, t)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("iterate types: %w", err)
	}

	if types == nil {
		types = []string{}
	}

	return types, nil
}

func (m *TursoReadModel) Upsert(ctx context.Context, item *provider.Item) error {
	query := `INSERT OR REPLACE INTO sync_items
		(item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.db.ExecContext(
		ctx, query,
		item.ID.String(), item.Source.Get(), item.ExternalID.Get(), item.Type.Get(),
		item.ActorLogin.Get(), item.ActorAvatarURL, item.RepoName.Get(),
		item.RepoURL, item.CreatedAt, item.UpdatedAt, item.RawJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert item: %w", err)
	}

	return nil
}

func (m *TursoReadModel) Delete(ctx context.Context, source, sourceID string) error {
	_, err := m.db.ExecContext(
		ctx,
		"DELETE FROM sync_items WHERE source = ? AND source_id = ?",
		source,
		sourceID,
	)
	if err != nil {
		return fmt.Errorf("delete item %s/%s: %w", source, sourceID, err)
	}

	return nil
}

func (m *TursoReadModel) Close() error {
	return m.db.Close()
}

func buildListQuery(filter ItemFilter) (string, []any) {
	query := `SELECT item_id, source, source_id, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at, raw_json
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

func appendFilterArgs(query *string, filter ItemFilter) []any {
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
	rawJSON                                                                               []byte
	createdAt, updatedAt                                                                  time.Time
}

func (si *scannedItem) toItem() *provider.Item {
	return &provider.Item{
		ID:             parseItemID(si.itemIDStr),
		ExternalID:     types.NewExternalID(si.sourceID),
		Source:         types.NewProviderID(si.source),
		Type:           types.NewEventTypeID(si.eventType),
		ActorLogin:     types.NewActorID(si.actorLogin),
		ActorAvatarURL: si.actorAvatarURL,
		RepoName:       types.NewRepoID(si.repoName),
		RepoURL:        si.repoURL,
		CreatedAt:      si.createdAt,
		UpdatedAt:      si.updatedAt,
		RawJSON:        si.rawJSON,
	}
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
		rawJSON:        nil,
		createdAt:      time.Time{},
		updatedAt:      time.Time{},
	}
}

func scanItem(row *sql.Row) (*provider.Item, error) {
	si := newScannedItem()

	err := row.Scan(&si.itemIDStr, &si.source, &si.sourceID, &si.eventType, &si.actorLogin,
		&si.actorAvatarURL, &si.repoName, &si.repoURL, &si.createdAt, &si.updatedAt, &si.rawJSON)
	if err != nil {
		return nil, err
	}

	return si.toItem(), nil
}

func scanItems(rows *sql.Rows) ([]*provider.Item, error) {
	var items []*provider.Item

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
			&si.rawJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}

		items = append(items, si.toItem())
	}

	return items, nil
}

// Ensure TursoReadModel implements ReadModel.
var _ ReadModel = (*TursoReadModel)(nil)
