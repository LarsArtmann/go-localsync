package testhelpers

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
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


