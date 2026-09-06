package cqrs

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// baseFromClause is the shared "FROM sync_items WHERE 1=1" suffix used by every
// read-model query that opts into the appendFilterArgs filter mechanism. The
// trailing "WHERE 1=1" lets appendFilterArgs unconditionally emit "AND ..." for
// each active filter without branching on the first clause.
const baseFromClause = "FROM sync_items WHERE 1=1"

func buildListQuery(filter model.ItemFilter) (string, []any, error) {
	query := `SELECT item_id, source, source_id, type, attributes, created_at, updated_at, tombstoned, tombstone_reason, tombstoned_at, content_hash, schema_version
		` + baseFromClause

	args, err := appendFilterArgs(&query, filter)
	if err != nil {
		return "", nil, err
	}

	// The item_id tiebreaker makes the ordering TOTAL: rows sharing a
	// created_at (batch imports, truncated timestamps) otherwise have
	// unspecified relative order, and OFFSET pagination across ties can
	// duplicate or skip rows between pages. Deterministic walks are a
	// contract — pinned by TestSQLiteReadModel_CursorWalk_RealOrdering.
	query += " ORDER BY created_at DESC, item_id ASC"

	if filter.Limit > 0 {
		query += " LIMIT ?"

		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		query += " OFFSET ?"

		args = append(args, filter.Offset)
	}

	return query, args, nil
}

// validateAttributeKey enforces that an attribute filter key is a safe JSON-path
// identifier before it is interpolated into the SQL string. SQLite's
// json_extract path cannot use a ? placeholder, so the key is concatenated
// directly; this guard rejects any key containing characters outside
// [A-Za-z0-9_] or not starting with a letter/underscore, closing a latent
// SQL-injection vector if filter.Attributes is ever populated from untrusted
// input. Values are parameterized separately and need no validation.
func validateAttributeKey(key string) error {
	if key == "" {
		return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "attribute filter key is empty")
	}

	for i, r := range key {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			// always allowed
		case i > 0 && r >= '0' && r <= '9':
			// digits allowed after the first rune
		default:
			return pkgerrors.WithDetail(
				pkgerrors.ErrInvalidInput,
				"attribute filter key has invalid character (allowed: ASCII letters, digits, underscore; first char must be a letter or underscore)",
			)
		}
	}

	return nil
}

// buildFilteredQuery assembles a "SELECT <selectClause> " + baseFromClause
// query and applies the filter conditions via appendFilterArgs. It is the
// single shared prologue for every read-model query that streams rows out of
// sync_items; the only differences between callers are the SELECT projection
// and any trailing clauses (ORDER BY / GROUP BY / LIMIT). Keeping the prologue
// in one place makes it impossible for a new query to forget to append filters
// or to skip the 1=1 anchor that lets appendFilterArgs emit "AND ..." freely.
func buildFilteredQuery(selectClause string, filter model.ItemFilter) (string, []any, error) {
	query := selectClause + " " + baseFromClause

	args, err := appendFilterArgs(&query, filter)
	if err != nil {
		return "", nil, err
	}

	return query, args, nil
}

func appendFilterArgs(query *string, filter model.ItemFilter) ([]any, error) {
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
		if err := validateAttributeKey(key); err != nil {
			return nil, err
		}

		*query += " AND json_extract(attributes, '$." + key + "') = ?"

		args = append(args, value)
	}

	return args, nil
}
