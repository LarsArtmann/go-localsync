package query

import (
	"slices"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

// Order defines a sort order for type T.
// Less returns true if a should come before b.
// FieldName is the SQL column name for ORDER BY generation.
type Order[T any] struct {
	Less      func(a, b T) bool
	FieldName string
}

// Query is an immutable, type-safe query specification.
// Build it with QueryBuilder — once built, it is read-only.
type Query[T any] struct {
	Criteria []Criterion[T]
	OrderBy  []Order[T]
	Limit    int
	Offset   int
}

// Match tests whether a value satisfies all criteria in the query.
func (q Query[T]) Match(value T) bool {
	for _, c := range q.Criteria {
		if !c.Match(value) {
			return false
		}
	}

	return true
}

// Sort sorts a slice according to the query's OrderBy clauses.
func (q Query[T]) Sort(items []T) {
	if len(q.OrderBy) == 0 {
		return
	}

	slices.SortFunc(items, func(a, b T) int {
		for _, order := range q.OrderBy {
			if order.Less(a, b) {
				return -1
			}

			if order.Less(b, a) {
				return 1
			}
		}

		return 0
	})
}

// QueryBuilder is the mutable builder for Query[T].
// Use it to construct queries fluently, then call Build() to freeze.
type QueryBuilder[T any] struct {
	criteria []Criterion[T]
	orderBy  []Order[T]
	limit    int
	offset   int
}

// NewBuilder starts a new query builder.
func NewBuilder[T any]() QueryBuilder[T] {
	return QueryBuilder[T]{}
}

// Where adds a criterion. The builder is returned for chaining.
func (qb QueryBuilder[T]) Where(c Criterion[T]) QueryBuilder[T] {
	qb.criteria = append(qb.criteria, c)

	return qb
}

// OrderBy adds a sort order. Multiple orders are applied lexicographically.
func (qb QueryBuilder[T]) OrderBy(o Order[T]) QueryBuilder[T] {
	qb.orderBy = append(qb.orderBy, o)

	return qb
}

// Limit sets the max number of results.
func (qb QueryBuilder[T]) Limit(n int) QueryBuilder[T] {
	qb.limit = n

	return qb
}

// Offset sets the skip count.
func (qb QueryBuilder[T]) Offset(n int) QueryBuilder[T] {
	qb.offset = n

	return qb
}

// Build freezes the builder into an immutable Query.
func (qb QueryBuilder[T]) Build() Query[T] {
	return Query[T]{
		Criteria: append([]Criterion[T](nil), qb.criteria...),
		OrderBy:  append([]Order[T](nil), qb.orderBy...),
		Limit:    qb.limit,
		Offset:   qb.offset,
	}
}

// ---------------------------------------------------------------------------
// Common sort orders — generic over any type with a time accessor.
// ---------------------------------------------------------------------------

// ByCreatedAtDesc sorts by created_at descending.
func ByCreatedAtDesc[T interface{ GetCreatedAt() time.Time }]() Order[T] {
	return Order[T]{
		Less:      func(a, b T) bool { return a.GetCreatedAt().After(b.GetCreatedAt()) },
		FieldName: "created_at DESC",
	}
}

// ByCreatedAtAsc sorts by created_at ascending.
func ByCreatedAtAsc[T interface{ GetCreatedAt() time.Time }]() Order[T] {
	return Order[T]{
		Less:      func(a, b T) bool { return a.GetCreatedAt().Before(b.GetCreatedAt()) },
		FieldName: "created_at ASC",
	}
}

// ByUpdatedAtDesc sorts by updated_at descending.
func ByUpdatedAtDesc[T interface{ GetUpdatedAt() time.Time }]() Order[T] {
	return Order[T]{
		Less:      func(a, b T) bool { return a.GetUpdatedAt().After(b.GetUpdatedAt()) },
		FieldName: "updated_at DESC",
	}
}

// ByTypeAsc sorts by type ascending (lexicographic).
func ByTypeAsc[T interface{ GetType() id.EventTypeID }]() Order[T] {
	return Order[T]{
		Less:      func(a, b T) bool { return a.GetType().Get() < b.GetType().Get() },
		FieldName: "type ASC",
	}
}
