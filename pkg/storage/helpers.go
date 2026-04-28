package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/larsartmann/go-localsync/internal/db"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// batchGetByIDs is a shared implementation for SQL-backed storage backends.
// It builds a dynamic IN clause with placeholders and scans results into items.
//
//nolint:funlen // Scanning 12 DB fields requires lines; extraction would not improve readability.
func batchGetByIDs(
	ctx context.Context,
	dbc *sql.DB,
	ids []types.ItemID,
) ([]*provider.Item, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))

	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = types.NewSourceItemID(id.Get())
	}

	query := fmt.Sprintf(
		`SELECT id, source_id, source, type, actor_login, actor_avatar_url, repo_name, repo_url, created_at, updated_at, raw_json, synced_at FROM events WHERE source_id IN (%s)`,
		strings.Join(placeholders, ", "),
	)

	rows, err := dbc.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: batch get by IDs: %w", pkgerrors.ErrDatabase, err)
	}

	defer func() {
		_ = rows.Close()
	}()

	var items []*provider.Item

	for rows.Next() {
		var e db.Events

		err := rows.Scan(
			&e.ID,
			&e.SourceID,
			&e.Source,
			&e.Type,
			&e.ActorLogin,
			&e.ActorAvatarUrl,
			&e.RepoName,
			&e.RepoUrl,
			&e.CreatedAt,
			&e.UpdatedAt,
			&e.RawJson,
			&e.SyncedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("%w: scan in batch get by IDs: %w", pkgerrors.ErrDatabase, err)
		}

		items = append(items, toItem(&e))
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("%w: iterate batch get by IDs: %w", pkgerrors.ErrDatabase, err)
	}

	return items, nil
}
