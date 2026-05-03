package cqrs

import (
	"context"
	"sort"
	"sync"
)

// MemoryReadModel is an in-memory implementation of ReadModel for testing.
type MemoryReadModel struct {
	mu    sync.RWMutex
	items map[string]*itemState // key: "source:sourceID"
}

// NewMemoryReadModel creates a new empty MemoryReadModel.
func NewMemoryReadModel() *MemoryReadModel {
	return &MemoryReadModel{
		items: make(map[string]*itemState),
	}
}

// Get retrieves a single item by source and sourceID.
func (m *MemoryReadModel) Get(_ context.Context, source, sourceID string) (*itemState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.items[source+":"+sourceID], nil
}

// List retrieves items matching the filter with pagination.
func (m *MemoryReadModel) List(_ context.Context, filter ItemFilter) ([]*itemState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*itemState, 0, len(m.items))
	for _, item := range m.items {
		if item.matchesFilter(filter) {
			all = append(all, item)
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	return paginate(all, filter.Limit, filter.Offset), nil
}

// Count returns the number of items matching the filter.
func (m *MemoryReadModel) Count(_ context.Context, filter ItemFilter) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := int64(0)

	for _, item := range m.items {
		if item.matchesFilter(filter) {
			count++
		}
	}

	return count, nil
}

// GetTypes returns all unique item types.
func (m *MemoryReadModel) GetTypes(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]struct{})

	for _, item := range m.items {
		seen[item.Type] = struct{}{}
	}

	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}

	sort.Strings(types)

	return types, nil
}

// Upsert inserts or updates an item in the read model.
func (m *MemoryReadModel) Upsert(_ context.Context, state *itemState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[state.key()] = state

	return nil
}

// Delete removes an item from the read model.
func (m *MemoryReadModel) Delete(_ context.Context, source, sourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, source+":"+sourceID)

	return nil
}

// Close is a no-op for the in-memory implementation.
func (m *MemoryReadModel) Close() error { return nil }

// Len returns the number of items (for testing).
func (m *MemoryReadModel) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.items)
}

func paginate(items []*itemState, limit, offset int) []*itemState {
	if offset >= len(items) {
		return nil
	}

	items = items[offset:]

	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	return items
}
