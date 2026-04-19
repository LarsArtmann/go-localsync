package testhelpers

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// NewStorageItem creates a test item with all fields populated.
// This is a convenience wrapper around NewTestItem for tests that need
// specific actor and repo values (e.g., storage BDD tests).
func NewStorageItem(id, eventType, actor, repo string, createdAt time.Time) *provider.Item {
	item := NewTestItem(id, eventType, createdAt)
	item.Source = types.NewProviderID("github")
	item.ActorLogin = types.NewActorID(actor)
	item.RepoName = types.NewRepoID(repo)
	item.RawJSON = json.RawMessage(`{"id":"` + id + `","type":"` + eventType + `"}`)

	return item
}
