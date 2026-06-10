package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

// Criterion is a generic predicate on type T.
// Implementations provide both in-memory matching and SQL generation.
type Criterion[T any] interface {
	Match(value T) bool
	ToSQL() (clause string, args []any)
}

// CriterionFunc is a Criterion implemented as a closure.
// Use it for quick ad-hoc criteria without defining a named type.
type CriterionFunc[T any] struct {
	matchFn func(T) bool
	sqlFn   func() (string, []any)
}

// Match implements Criterion.
func (c CriterionFunc[T]) Match(value T) bool {
	return c.matchFn(value)
}

// ToSQL implements Criterion.
func (c CriterionFunc[T]) ToSQL() (string, []any) {
	return c.sqlFn()
}

// NewCriterion creates a Criterion from match and SQL functions.
func NewCriterion[T any](
	match func(T) bool,
	sql func() (string, []any),
) Criterion[T] {
	return CriterionFunc[T]{matchFn: match, sqlFn: sql}
}

// ---------------------------------------------------------------------------
// Criteria constructors — these work with any type that exposes the
// relevant accessor. We use type parameters with interface constraints
// so the same criterion factory works for Item, ItemView, ProviderItem, etc.
// ---------------------------------------------------------------------------

// HasSource matches items with the given source.
func HasSource[T interface{ GetSource() id.ProviderID }](source id.ProviderID) Criterion[T] {
	return NewCriterion(
		func(v T) bool { return v.GetSource() == source },
		func() (string, []any) { return "source = ?", []any{source.Get()} },
	)
}

// HasType matches items with the given event type.
func HasType[T interface{ GetType() id.EventTypeID }](eventType id.EventTypeID) Criterion[T] {
	return NewCriterion(
		func(v T) bool { return v.GetType() == eventType },
		func() (string, []any) { return "type = ?", []any{eventType.Get()} },
	)
}

// HasActor matches items with the given actor login.
func HasActor[T interface{ GetActorLogin() id.ActorID }](actor id.ActorID) Criterion[T] {
	return NewCriterion(
		func(v T) bool { return v.GetActorLogin() == actor },
		func() (string, []any) { return "actor_login = ?", []any{actor.Get()} },
	)
}

// HasRepo matches items with the given repo name.
func HasRepo[T interface{ GetRepoName() id.RepoID }](repo id.RepoID) Criterion[T] {
	return NewCriterion(
		func(v T) bool { return v.GetRepoName() == repo },
		func() (string, []any) { return "repo_name = ?", []any{repo.Get()} },
	)
}

// CreatedAfter matches items created after the given time.
func CreatedAfter[T interface{ GetCreatedAt() time.Time }](t time.Time) Criterion[T] {
	return NewCriterion(
		func(v T) bool { return v.GetCreatedAt().After(t) },
		func() (string, []any) {
			return "created_at > ?", []any{t.Format(time.RFC3339Nano)}
		},
	)
}

// ---------------------------------------------------------------------------
// Logical combinators — And, Or, Not — fully generic over T.
// ---------------------------------------------------------------------------

// joinSQL flattens sub-criteria into a single SQL clause joined by sep.
// Returns emptySentinel if criteria is empty.
func joinSQL[T any](criteria []Criterion[T], sep, emptySentinel string) (string, []any) {
	if len(criteria) == 0 {
		return emptySentinel, nil
	}

	clauses := make([]string, 0, len(criteria))

	var args []any

	for _, c := range criteria {
		clause, cargs := c.ToSQL()
		clauses = append(clauses, fmt.Sprintf("(%s)", clause))
		args = append(args, cargs...)
	}

	return strings.Join(clauses, sep), args
}

// And returns a criterion that matches when ALL sub-criteria match.
func And[T any](criteria ...Criterion[T]) Criterion[T] {
	return NewCriterion(
		func(v T) bool {
			for _, c := range criteria {
				if !c.Match(v) {
					return false
				}
			}

			return true
		},
		func() (string, []any) {
			return joinSQL(criteria, " AND ", "1=1")
		},
	)
}

// Or returns a criterion that matches when ANY sub-criterion matches.
func Or[T any](criteria ...Criterion[T]) Criterion[T] {
	return NewCriterion(
		func(v T) bool {
			for _, c := range criteria {
				if c.Match(v) {
					return true
				}
			}

			return false
		},
		func() (string, []any) {
			return joinSQL(criteria, " OR ", "1=0")
		},
	)
}

// Not returns a criterion that inverts the given criterion.
func Not[T any](c Criterion[T]) Criterion[T] {
	return NewCriterion(
		func(v T) bool { return !c.Match(v) },
		func() (string, []any) {
			clause, args := c.ToSQL()

			return fmt.Sprintf("NOT (%s)", clause), args
		},
	)
}
