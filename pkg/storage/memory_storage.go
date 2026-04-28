package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// MemoryStorage implements Storage using an in-memory map.
type MemoryStorage struct {
	mu    sync.RWMutex
	items map[string]*provider.Item
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		mu:    sync.RWMutex{},
		items: make(map[string]*provider.Item),
	}
}

// Upsert inserts or updates an item.
func (s *MemoryStorage) Upsert(_ context.Context, item *provider.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[item.ID.Get()] = item

	return nil
}

// UpsertBatch inserts or updates multiple items atomically.
func (s *MemoryStorage) UpsertBatch(_ context.Context, items []*provider.Item) error {
	if len(items) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range items {
		s.items[item.ID.Get()] = item
	}

	return nil
}

// GetByID retrieves a single item by its source ID.
func (s *MemoryStorage) GetByID(_ context.Context, id types.ItemID) (*provider.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[id.Get()]
	if !ok {
		return nil, pkgerrors.ErrNotFound
	}

	return item, nil
}

// GetLatest returns the most recently created item.
func (s *MemoryStorage) GetLatest(_ context.Context) (*provider.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.items) == 0 {
		return nil, pkgerrors.ErrNotFound
	}

	var latest *provider.Item

	for _, item := range s.items {
		if latest == nil || item.CreatedAt.After(latest.CreatedAt) {
			latest = item
		}
	}

	return latest, nil
}

// GetItems retrieves items with pagination.
func (s *MemoryStorage) GetItems(_ context.Context, limit, offset int) ([]*provider.Item, error) {
	return s.filterItems(nil, limit, offset), nil
}

// GetItemsByType retrieves items filtered by type.
func (s *MemoryStorage) GetItemsByType(
	_ context.Context,
	itemType string,
	limit, offset int,
) ([]*provider.Item, error) {
	return s.filterItems(func(item *provider.Item) bool {
		return item.Type.Get() == itemType
	}, limit, offset), nil
}

// GetItemsByActor retrieves items filtered by actor login.
func (s *MemoryStorage) GetItemsByActor(
	_ context.Context,
	actorLogin string,
	limit, offset int,
) ([]*provider.Item, error) {
	return s.filterItems(func(item *provider.Item) bool {
		return item.ActorLogin.Get() == actorLogin
	}, limit, offset), nil
}

// GetItemsByRepo retrieves items filtered by repository name.
func (s *MemoryStorage) GetItemsByRepo(
	_ context.Context,
	repoName string,
	limit, offset int,
) ([]*provider.Item, error) {
	return s.filterItems(func(item *provider.Item) bool {
		return item.RepoName.Get() == repoName
	}, limit, offset), nil
}

// GetItemsBySource retrieves items filtered by source provider.
func (s *MemoryStorage) GetItemsBySource(
	_ context.Context,
	source string,
	limit, offset int,
) ([]*provider.Item, error) {
	return s.filterItems(func(item *provider.Item) bool {
		return item.Source.Get() == source
	}, limit, offset), nil
}

// GetItemsSince retrieves items created after the given timestamp.
func (s *MemoryStorage) GetItemsSince(
	_ context.Context,
	since time.Time,
) ([]*provider.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*provider.Item

	for _, item := range s.items {
		if item.CreatedAt.After(since) {
			result = append(result, item)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

// BatchGetByIDs retrieves multiple items by their source IDs.
func (s *MemoryStorage) BatchGetByIDs(
	_ context.Context,
	ids []types.ItemID,
) ([]*provider.Item, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*provider.Item

	for _, id := range ids {
		if item, ok := s.items[id.Get()]; ok {
			result = append(result, item)
		}
	}

	return result, nil
}

// Delete removes an item by its source ID.
func (s *MemoryStorage) Delete(_ context.Context, id types.ItemID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, id.Get())

	return nil
}

// DeleteAll removes all items.
func (s *MemoryStorage) DeleteAll(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.items)

	return nil
}

// Count returns the total number of items.
func (s *MemoryStorage) Count(_ context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return int64(len(s.items)), nil
}

// CountByType returns the number of items of a specific type.
func (s *MemoryStorage) CountByType(_ context.Context, itemType string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := int64(0)

	for _, item := range s.items {
		if item.Type.Get() == itemType {
			count++
		}
	}

	return count, nil
}

// GetTypes returns all unique item types.
func (s *MemoryStorage) GetTypes(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	typeSet := make(map[string]bool)

	for _, item := range s.items {
		typeSet[item.Type.Get()] = true
	}

	result := make([]string, 0, len(typeSet))

	for t := range typeSet {
		result = append(result, t)
	}

	sort.Strings(result)

	return result, nil
}

// Close is a no-op for in-memory storage.
func (s *MemoryStorage) Close() error {
	return nil
}

// itemFilter is a predicate for filtering items.
type itemFilter func(*provider.Item) bool

// filterItems returns filtered and paginated items sorted by CreatedAt descending.
func (s *MemoryStorage) filterItems(filter itemFilter, limit, offset int) []*provider.Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*provider.Item

	for _, item := range s.items {
		if filter != nil && !filter(item) {
			continue
		}

		filtered = append(filtered, item)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	if offset >= len(filtered) {
		return nil
	}

	filtered = filtered[offset:]

	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	return filtered
}

// Ensure MemoryStorage implements Storage.
var _ Storage = (*MemoryStorage)(nil)
