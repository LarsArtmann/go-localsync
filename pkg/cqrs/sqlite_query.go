package cqrs

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

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
