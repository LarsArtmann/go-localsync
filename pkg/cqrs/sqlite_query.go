package cqrs

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

func buildListQuery(filter model.ItemFilter) (string, []any) {
	query := `SELECT item_id, source, source_id, type, attributes, created_at, updated_at, tombstoned, tombstone_reason, tombstoned_at, content_hash, schema_version
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

	if !filter.IncludeTombstoned {
		*query += " AND tombstoned = 0"
	}

	if filter.Type != nil {
		*query += " AND type = ?"

		args = append(args, filter.Type.Get())
	}

	if filter.Source != nil {
		*query += " AND source = ?"

		args = append(args, filter.Source.Get())
	}

	if filter.Since != nil {
		*query += " AND created_at >= ?"

		args = append(args, filter.Since.Format(time.RFC3339Nano))
	}

	for key, value := range filter.Attributes {
		*query += " AND json_extract(attributes, '$." + key + "') = ?"

		args = append(args, value)
	}

	return args
}
