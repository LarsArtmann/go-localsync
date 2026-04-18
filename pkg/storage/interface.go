// Package storage defines the interface for storing and retrieving sync items.
package storage

import (
	"context"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
)

// Storage defines the interface for storing and retrieving sync items.
//
//nolint:interfacebloat // storage interfaces naturally have many CRUD methods
type Storage interface {
	// Upsert inserts or updates an item. ID is used as the unique key.
	Upsert(ctx context.Context, item *provider.Item) error
	// GetByID retrieves a single item by its source ID. Returns nil if not found.
	GetByID(ctx context.Context, id string) (*provider.Item, error)
	// GetLatest returns the most recently created item, or nil if empty.
	GetLatest(ctx context.Context) (*provider.Item, error)
	// GetItems retrieves items with pagination.
	GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error)
	// GetItemsByType retrieves items filtered by type.
	GetItemsByType(
		ctx context.Context,
		itemType string,
		limit, offset int,
	) ([]*provider.Item, error)
	// GetItemsByActor retrieves items filtered by actor login.
	GetItemsByActor(
		ctx context.Context,
		actorLogin string,
		limit, offset int,
	) ([]*provider.Item, error)
	// GetItemsByRepo retrieves items filtered by repository name.
	GetItemsByRepo(
		ctx context.Context,
		repoName string,
		limit, offset int,
	) ([]*provider.Item, error)
	// GetItemsBySource retrieves items filtered by source provider.
	GetItemsBySource(
		ctx context.Context,
		source string,
		limit, offset int,
	) ([]*provider.Item, error)
	// GetItemsSince retrieves items created after the given timestamp.
	GetItemsSince(ctx context.Context, since time.Time) ([]*provider.Item, error)
	// Delete removes an item by its source ID.
	Delete(ctx context.Context, id string) error
	// DeleteAll removes all items.
	DeleteAll(ctx context.Context) error
	// Count returns the total number of items.
	Count(ctx context.Context) (int64, error)
	// CountByType returns the number of items of a specific type.
	CountByType(ctx context.Context, itemType string) (int64, error)
	// GetTypes returns all unique item types.
	GetTypes(ctx context.Context) ([]string, error)
	// Close releases resources.
	Close() error
}
