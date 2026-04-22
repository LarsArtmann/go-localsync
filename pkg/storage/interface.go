// Package storage defines the interface for storing and retrieving sync items.
package storage

import (
	"context"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// Storage defines the interface for storing and retrieving sync items.
//
// # Interface Contracts
//
// All implementations must satisfy the compliance test suite in compliance_test.go.
// Key behavioral contracts:
//
//   - Upsert is idempotent: inserting the same ID twice overwrites the previous value.
//   - UpsertBatch is atomic: on error, no items from the batch are persisted.
//   - UpsertBatch with an empty or nil slice returns nil (no-op).
//   - Delete is idempotent: deleting a non-existent item returns nil.
//   - GetByID returns (nil, ErrNotFound) for missing items.
//   - GetLatest returns (nil, ErrNotFound) when storage is empty.
//   - BatchGetByIDs silently omits missing IDs (no error).
//   - All pagination methods use limit/offset with items ordered by CreatedAt descending.
//   - All implementations must be safe for concurrent use.
//
//nolint:interfacebloat // storage interfaces naturally have many CRUD methods
type Storage interface {
	// Upsert inserts or updates an item. ID is used as the unique key.
	// Idempotent: calling twice with the same ID overwrites the previous value.
	Upsert(ctx context.Context, item *provider.Item) error
	// UpsertBatch inserts or updates multiple items in a single transaction.
	// Atomic: on error, no items are persisted. Empty/nil slice is a no-op.
	UpsertBatch(ctx context.Context, items []*provider.Item) error
	// GetByID retrieves a single item by its source ID.
	// Returns nil and ErrNotFound if not found.
	GetByID(ctx context.Context, id types.ItemID) (*provider.Item, error)
	// GetLatest returns the most recently created item, or ErrNotFound if empty.
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
	// BatchGetByIDs retrieves multiple items by their source IDs.
	// Returns items that exist; missing IDs are silently omitted.
	BatchGetByIDs(ctx context.Context, ids []types.ItemID) ([]*provider.Item, error)
	// Delete removes an item by its source ID.
	// It is idempotent: deleting a non-existent item returns nil.
	Delete(ctx context.Context, id types.ItemID) error
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
