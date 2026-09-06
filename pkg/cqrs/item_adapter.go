package cqrs

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
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
		ID:            p.ID,
		ExternalID:    p.ExternalID,
		Source:        p.Source,
		Type:          p.Type,
		Attributes:    p.Attributes,
		ContentHash:   id.NewContentHash(hashRawJSON(p.RawJSON)),
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
		SchemaVersion: schema.CurrentVersion(),
	}
}

func hashRawJSON(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}

	h := sha256.Sum256(raw)

	return hex.EncodeToString(h[:])
}

// dataItemFromPayload reconstructs a data.Item from an event payload.
// For schema V1/V2 events (no Attributes), legacy fields are upcasted
// into the Attributes map so consumers always see a uniform shape.
func dataItemFromPayload(payload ItemSyncedPayload) (*model.Item, error) {
	itemID, err := parseItemID(payload.ItemID)
	if err != nil {
		return nil, err
	}

	schemaVer := schema.Version(payload.SchemaVersion)
	if schemaVer == 0 {
		schemaVer = schema.V1
	}

	attrs := payload.Attributes
	if attrs == nil {
		attrs = upcastLegacyAttributes(payload)
	}

	item := &model.Item{
		ID:            itemID,
		ExternalID:    id.NewExternalID(payload.SourceID),
		Source:        id.NewProviderID(payload.Source),
		Type:          id.NewEventTypeID(payload.Type),
		Attributes:    attrs,
		ContentHash:   id.NewContentHash(payload.ContentHash),
		CreatedAt:     fromUnixNano(payload.CreatedAt),
		UpdatedAt:     fromUnixNano(payload.UpdatedAt),
		SchemaVersion: schemaVer,
	}

	if err := item.Validate(); err != nil {
		return nil, pkgerrors.Wrap(err, "invalid item from payload")
	}

	return item, nil
}

// upcastLegacyAttributes reconstructs the Attributes map from V1/V2 event
// payload fields. Only non-empty legacy fields are included, matching the
// omitempty semantics of the payload struct.
func upcastLegacyAttributes(payload ItemSyncedPayload) map[string]string {
	attrs := map[string]string{}

	if payload.ActorLogin != "" {
		attrs[model.AttrActorLogin] = payload.ActorLogin
	}

	if payload.ActorAvatarURL != "" {
		attrs[model.AttrActorAvatarURL] = payload.ActorAvatarURL
	}

	if payload.RepoName != "" {
		attrs[model.AttrRepoName] = payload.RepoName
	}

	if payload.RepoURL != "" {
		attrs[model.AttrRepoURL] = payload.RepoURL
	}

	return attrs
}

// dataItemToPayload serializes a data.Item into an event payload.
func dataItemToPayload(item *model.Item, rawJSON []byte) ItemSyncedPayload {
	if item == nil {
		return ItemSyncedPayload{}
	}

	return ItemSyncedPayload{
		ItemID:        item.ID.String(),
		Source:        item.Source.Get(),
		SourceID:      item.ExternalID.Get(),
		Type:          item.Type.Get(),
		Attributes:    item.Attributes,
		ContentHash:   item.ContentHash.String(),
		CreatedAt:     unixNano(item.CreatedAt),
		UpdatedAt:     unixNano(item.UpdatedAt),
		RawJSON:       rawJSON,
		SchemaVersion: item.SchemaVersion.Int(),
	}
}

// decodeItemFromEvent decodes an ItemSynced event payload and reconstructs
// the domain model Item. Shared between decider (fold) and projection (handle).
func decodeItemFromEvent(evt event.Event) (*model.Item, error) {
	payload, err := event.DecodePayloadAuto[ItemSyncedPayload](evt)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "decode ItemSyncedPayload for event %s", evt.ID())
	}

	item, err := dataItemFromPayload(payload)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "reconstruct item from payload")
	}

	return item, nil
}
