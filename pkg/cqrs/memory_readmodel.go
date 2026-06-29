package cqrs

import (
	"context"
	"sort"
	"sync"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// MemoryReadModel is a concurrent-safe in-memory implementation of ReadModel.
type MemoryReadModel struct {
	mu    sync.RWMutex
	items map[string]*model.Item
}

// NewMemoryReadModel creates a new empty MemoryReadModel.
func NewMemoryReadModel() *MemoryReadModel {
	return &MemoryReadModel{
		mu:    sync.RWMutex{},
		items: make(map[string]*model.Item),
	}
}

func (m *MemoryReadModel) Get(
	_ context.Context,
	source string,
	sourceID id.ExternalID,
) (*model.Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.items[itemKey(source, sourceID)]
	if !ok {
		return nil, pkgerrors.ErrNotFound
	}

	return item, nil
}

func (m *MemoryReadModel) List(_ context.Context, filter model.ItemFilter) ([]*model.Item, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*model.Item, 0, len(m.items))

	for _, item := range m.items {
		if matchesFilter(item, filter) {
			all = append(all, item)
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})

	return paginate(all, filter.Limit, filter.Offset), nil
}

func (m *MemoryReadModel) Count(_ context.Context, filter model.ItemFilter) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var count int64

	for _, item := range m.items {
		if matchesFilter(item, filter) {
			count++
		}
	}

	return count, nil
}

func (m *MemoryReadModel) CountByType(_ context.Context, filter model.ItemFilter) (map[string]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int64)

	for _, item := range m.items {
		if matchesFilter(item, filter) {
			counts[item.Type.Get()]++
		}
	}

	return counts, nil
}

func (m *MemoryReadModel) Upsert(_ context.Context, item *model.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[itemKey(item.Source.Get(), item.ExternalID)] = item

	return nil
}

func (m *MemoryReadModel) Tombstone(
	_ context.Context,
	source string,
	sourceID id.ExternalID,
	tombstone model.Tombstone,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := itemKey(source, sourceID)

	item, ok := m.items[key]
	if !ok {
		return nil // idempotent: nothing to tombstone
	}

	tombstoned := *item
	tombstoned.Tombstone = tombstone
	m.items[key] = &tombstoned

	return nil
}

func (m *MemoryReadModel) Close() error { return nil }

// Len returns the number of items in the read model.
func (m *MemoryReadModel) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.items)
}

func matchesFilter(item *model.Item, filter model.ItemFilter) bool {
	if !filter.IncludeTombstoned && item.IsTombstoned() {
		return false
	}

	if filter.Type != nil && item.Type.Get() != filter.Type.Get() {
		return false
	}

	if filter.ActorLogin != nil && item.ActorLogin.Get() != filter.ActorLogin.Get() {
		return false
	}

	if filter.RepoName != nil && item.RepoName.Get() != filter.RepoName.Get() {
		return false
	}

	if filter.Source != nil && item.Source.Get() != filter.Source.Get() {
		return false
	}

	if filter.Since != nil && item.CreatedAt.Before(*filter.Since) {
		return false
	}

	return true
}

func paginate(items []*model.Item, limit, offset int) []*model.Item {
	if offset >= len(items) {
		return nil
	}

	items = items[offset:]

	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	return items
}
