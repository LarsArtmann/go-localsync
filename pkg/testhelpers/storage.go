package testhelpers

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func NewStorageItem(id, eventType, actor, repo string, createdAt time.Time) *provider.Item {
	return &provider.Item{
		ID:             types.NewItemID(),
		ExternalID:     types.NewExternalID(id),
		Source:         types.NewProviderID("github"),
		Type:           types.NewEventTypeID(eventType),
		ActorLogin:     types.NewActorID(actor),
		ActorAvatarURL: "https://avatar.example.com/" + actor,
		RepoName:       types.NewRepoID(repo),
		RepoURL:        "https://github.com/" + repo,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
		RawJSON:        json.RawMessage(`{"id":"` + id + `","type":"` + eventType + `"}`),
	}
}
