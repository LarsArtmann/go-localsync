package cqrs

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// SyncItemState is the aggregate state for a single sync item, reconstructed from events.
type SyncItemState struct {
	Source         string
	SourceID       string
	Type           string
	ActorLogin     string
	ActorAvatarURL string
	RepoName       string
	RepoURL        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RawJSON        json.RawMessage
	Deleted        bool
}

// InitialSyncItemState is the zero state for a new SyncItem aggregate.
var InitialSyncItemState = SyncItemState{}

// IsNew returns true if this state has never been synced (no events applied).
func (s SyncItemState) IsNew() bool {
	return s.SourceID == ""
}

// ToItem converts the aggregate state back to a provider.Item.
func (s SyncItemState) ToItem() *provider.Item {
	return &provider.Item{
		ID:             types.NewItemID(),
		ExternalID:     types.NewExternalID(s.SourceID),
		Source:         types.NewProviderID(s.Source),
		Type:           types.NewEventTypeID(s.Type),
		ActorLogin:     types.NewActorID(s.ActorLogin),
		ActorAvatarURL: s.ActorAvatarURL,
		RepoName:       types.NewRepoID(s.RepoName),
		RepoURL:        s.RepoURL,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		RawJSON:        s.RawJSON,
	}
}

// itemToPayload converts a provider.Item to an ItemSyncedPayload for event creation.
func itemToPayload(item *provider.Item) ItemSyncedPayload {
	return ItemSyncedPayload{
		Source:         item.Source.Get(),
		SourceID:       item.ExternalID.Get(),
		Type:           item.Type.Get(),
		ActorLogin:     item.ActorLogin.Get(),
		ActorAvatarURL: item.ActorAvatarURL,
		RepoName:       item.RepoName.Get(),
		RepoURL:        item.RepoURL,
		CreatedAt:      unixNano(item.CreatedAt),
		UpdatedAt:      unixNano(item.UpdatedAt),
		RawJSON:        item.RawJSON,
	}
}

// hasChanged returns true if the remote item differs from the current state.
func (s SyncItemState) hasChanged(remote *provider.Item) bool {
	return s.UpdatedAt != remote.UpdatedAt ||
		s.Type != remote.Type.Get() ||
		s.ActorLogin != remote.ActorLogin.Get() ||
		s.RepoName != remote.RepoName.Get()
}
