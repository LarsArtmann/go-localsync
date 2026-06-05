// Package repo provides generic repository abstractions over the read model.
//
// The repository pattern decouples the API and sync packages from the
// concrete ReadModel implementation (memory vs SQLite).
package repo

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/query"
)

// Reader is a generic read-only repository for type T.
type Reader[T any] interface {
	// Get retrieves a single entity by its composite key.
	Get(ctx context.Context, key model.Key) (T, error)

	// List returns a paginated list of entities matching the query.
	List(ctx context.Context, q query.Query[T]) (query.Page[T], error)

	// Count returns the total number of entities matching the query.
	Count(ctx context.Context, q query.Query[T]) (int64, error)
}

// Writer is a generic write-only repository for type T.
type Writer[T any] interface {
	// Upsert inserts or updates an entity.
	Upsert(ctx context.Context, entity T) error

	// Delete removes an entity by its composite key.
	Delete(ctx context.Context, key model.Key) error
}

// ReadWriter combines read and write operations.
type ReadWriter[T any] interface {
	Reader[T]
	Writer[T]
}

// Closer adds lifecycle management.
type Closer interface {
	Close() error
}

// Repository is the full repository interface: read + write + close.
type Repository[T any] interface {
	ReadWriter[T]
	Closer
}

// ItemRepository is a Repository specialized for *model.ItemView.
// It adds domain-specific read methods.
type ItemRepository interface {
	Repository[*model.ItemView]

	// GetTypes returns all distinct event types.
	GetTypes(ctx context.Context) ([]string, error)

	// GetStats returns aggregated statistics.
	GetStats(ctx context.Context) (model.StatsView, error)
}

// Observable is a generic decorator that adds metrics/logging to any Reader.
// Use it to wrap repositories without changing their interface.
type Observable[T any] struct {
	base Reader[T]
}

// NewObservable wraps a Reader with observability.
func NewObservable[T any](base Reader[T]) *Observable[T] {
	return &Observable[T]{base: base}
}

// Get delegates to the base reader.
func (o *Observable[T]) Get(ctx context.Context, key model.Key) (T, error) {
	return o.base.Get(ctx, key)
}

// List delegates to the base reader.
func (o *Observable[T]) List(ctx context.Context, q query.Query[T]) (query.Page[T], error) {
	return o.base.List(ctx, q)
}

// Count delegates to the base reader.
func (o *Observable[T]) Count(ctx context.Context, q query.Query[T]) (int64, error) {
	return o.base.Count(ctx, q)
}
