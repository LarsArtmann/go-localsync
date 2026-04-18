package testhelpers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// NewStorageItem creates a test item with sensible defaults.
func NewStorageItem(id, eventType, actor, repo string, createdAt time.Time) *provider.Item {
	return &provider.Item{
		ID:         types.NewItemID(id),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID(eventType),
		ActorLogin: types.NewActorID(actor),
		RepoName:   types.NewRepoID(repo),
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
		RawJSON:    json.RawMessage(`{"id":"` + id + `","type":"` + eventType + `"}`),
	}
}

// StorageItem represents a set of items to insert into storage.
type StorageItemSet struct {
	Items []*provider.Item
}

// AddItem adds an item to the set.
func (s *StorageItemSet) AddItem(
	id, eventType, actor, repo string,
	createdAt time.Time,
) *StorageItemSet {
	s.Items = append(s.Items, NewStorageItem(id, eventType, actor, repo, createdAt))

	return s
}

// UpsertAll inserts all items into the storage.
func (s *StorageItemSet) UpsertAll(ctx context.Context, store storage.Storage) error {
	return store.UpsertBatch(ctx, s.Items)
}

// NewStorageItemSet creates a new StorageItemSet with the given items.
func NewStorageItemSet(items ...*provider.Item) *StorageItemSet {
	return &StorageItemSet{Items: items}
}
