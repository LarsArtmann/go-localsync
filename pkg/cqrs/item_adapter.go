package cqrs

import (
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/codec/v2"
	"github.com/larsartmann/go-cqrs-lite/event/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// ToDataItem converts a provider.Item to a data model Item.
// This is the boundary between the provider DTO and the domain entity.
func ToDataItem(p *provider.Item) *model.Item {
	if p == nil {
		return nil
	}

	return &model.Item{
		ID:             p.ID,
		ExternalID:     p.ExternalID,
		Source:         p.Source,
		Type:           p.Type,
		ActorLogin:     p.ActorLogin,
		ActorAvatarURL: p.ActorAvatarURL,
		RepoName:       p.RepoName,
		RepoURL:        p.RepoURL,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		SchemaVersion:  schema.CurrentVersion(),
	}
}

// FromDataItem converts a data model Item back to a provider.Item.
// Used for backward compatibility during the migration.
func FromDataItem(item *model.Item, rawJSON []byte) *provider.Item {
	if item == nil {
		return nil
	}

	return &provider.Item{
		ID:             item.ID,
		ExternalID:     item.ExternalID,
		Source:         item.Source,
		Type:           item.Type,
		ActorLogin:     item.ActorLogin,
		ActorAvatarURL: item.ActorAvatarURL,
		RepoName:       item.RepoName,
		RepoURL:        item.RepoURL,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		RawJSON:        rawJSON,
	}
}

// DataItemFromPayload reconstructs a data.Item from an event payload.
func DataItemFromPayload(payload ItemSyncedPayload) (*model.Item, error) {
	itemID, err := parseItemID(payload.ItemID)
	if err != nil {
		return nil, err
	}

	schemaVer := schema.Version(payload.SchemaVersion)
	if schemaVer == 0 {
		schemaVer = schema.V1
	}

	item := &model.Item{
		ID:             itemID,
		ExternalID:     id.NewExternalID(payload.SourceID),
		Source:         id.NewProviderID(payload.Source),
		Type:           id.NewEventTypeID(payload.Type),
		ActorLogin:     id.NewActorID(payload.ActorLogin),
		ActorAvatarURL: payload.ActorAvatarURL,
		RepoName:       id.NewRepoID(payload.RepoName),
		RepoURL:        payload.RepoURL,
		CreatedAt:      fromUnixNano(payload.CreatedAt),
		UpdatedAt:      fromUnixNano(payload.UpdatedAt),
		SchemaVersion:  schemaVer,
	}

	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("invalid item from payload: %w", err)
	}

	return item, nil
}

// DataItemToPayload serializes a data.Item into an event payload.
func DataItemToPayload(item *model.Item, rawJSON []byte) ItemSyncedPayload {
	if item == nil {
		//nolint:exhaustruct // zero payload for nil item
		return ItemSyncedPayload{}
	}

	return ItemSyncedPayload{
		ItemID:         item.ID.String(),
		Source:         item.Source.Get(),
		SourceID:       item.ExternalID.Get(),
		Type:           item.Type.Get(),
		ActorLogin:     item.ActorLogin.Get(),
		ActorAvatarURL: item.ActorAvatarURL,
		RepoName:       item.RepoName.Get(),
		RepoURL:        item.RepoURL,
		CreatedAt:      unixNano(item.CreatedAt),
		UpdatedAt:      unixNano(item.UpdatedAt),
		RawJSON:        rawJSON,
		SchemaVersion:  item.SchemaVersion.Int(),
	}
}

// decodeItemFromEvent decodes an ItemSynced event payload and reconstructs
// the domain model Item. Shared between decider (fold) and projection (handle).
func decodeItemFromEvent(evt event.Event) (*model.Item, error) {
	payload, err := event.DecodePayload[ItemSyncedPayload](evt, codec.JSONCodec{})
	if err != nil {
		return nil, fmt.Errorf("decode ItemSyncedPayload for event %s: %w", evt.ID(), err)
	}

	item, err := DataItemFromPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("reconstruct item from payload: %w", err)
	}

	return item, nil
}
