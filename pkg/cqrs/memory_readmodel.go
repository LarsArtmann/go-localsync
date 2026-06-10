package cqrs

import (
	"context"
	"sort"
	"sync"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
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

func (m *MemoryReadModel) List(_ context.Context, filter provider.ItemFilter) ([]*model.Item, error) {
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

func (m *MemoryReadModel) Count(_ context.Context, filter provider.ItemFilter) (int64, error) {
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

func (m *MemoryReadModel) GetTypes(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[string]struct{})

	for _, item := range m.items {
		seen[item.Type.Get()] = struct{}{}
	}

	types := make([]string, 0, len(seen))

	for t := range seen {
		types = append(types, t)
	}

	sort.Strings(types)

	return types, nil
}

func (m *MemoryReadModel) Upsert(_ context.Context, item *model.Item) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items[itemKey(item.Source.Get(), item.ExternalID)] = item

	return nil
}

func (m *MemoryReadModel) Delete(
	_ context.Context,
	source string,
	sourceID id.ExternalID,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, itemKey(source, sourceID))

	return nil
}

func (m *MemoryReadModel) Close() error { return nil }

// Len returns the number of items in the read model.
func (m *MemoryReadModel) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.items)
}

func matchesFilter(item *model.Item, filter provider.ItemFilter) bool {
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
