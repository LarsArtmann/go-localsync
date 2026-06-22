package cqrs

import (
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/codec/v3"
	"github.com/larsartmann/go-cqrs-lite/event/v3"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// toDataItem converts a provider.Item to a data model Item.
// This is the boundary between the provider DTO and the domain entity.
func toDataItem(p *provider.Item) *model.Item {
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

// dataItemFromPayload reconstructs a data.Item from an event payload.
func dataItemFromPayload(payload ItemSyncedPayload) (*model.Item, error) {
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

// dataItemToPayload serializes a data.Item into an event payload.
func dataItemToPayload(item *model.Item, rawJSON []byte) ItemSyncedPayload {
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

	item, err := dataItemFromPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("reconstruct item from payload: %w", err)
	}

	return item, nil
}
