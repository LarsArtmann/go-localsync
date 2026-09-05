package cqrs

import (
	"encoding/json"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/schema/v4"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// newLegacyUpcasters returns the schema-evolution pipeline for ItemSynced
// events persisted before schema V3 (ADR-0007 de-githubification): V1 and V2
// payloads carry actor/repo as top-level fields; V3 folds them into the
// Attributes map. Applied at the store read boundary via
// event.DecorateStore + schema.UpcastSourceTransform so every consumer of
// stored events (fold, projection, journal replay, export) sees V3 only.
func newLegacyUpcasters() []schema.Upcaster {
	return []schema.Upcaster{
		schema.NewUpcaster(EventItemSynced, 1, upcastItemSyncedToV3),
		schema.NewUpcaster(EventItemSynced, 2, upcastItemSyncedToV3),
	}
}

// upcastItemSyncedToV3 rewrites a pre-V3 ItemSynced payload: legacy
// actor/repo fields become Attributes entries and the schema version bumps
// to 3. Already-V3 payloads (Attributes present) pass through untouched.
// The event identity (ID, stream, version, metadata, occurredAt) is
// preserved — only payload and schema version change.
func upcastItemSyncedToV3(evt event.Event) (event.Event, error) {
	payload, err := event.DecodePayloadAuto[ItemSyncedPayload](evt)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: decode payload")
	}

	if payload.Attributes != nil {
		return evt, nil
	}

	payload.Attributes = upcastLegacyAttributes(payload)
	payload.SchemaVersion = 3

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: encode payload")
	}

	upcasted, err := event.ReconstructEventWithAdoptedPayload(
		evt.ID(), evt.Type(), evt.StreamType(), evt.StreamID(),
		int(evt.Version()), 3, raw, evt.Metadata(), evt.OccurredAt(),
		evt.Encoding(), "localsync.upcast_item_synced",
	)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: reconstruct event")
	}

	return upcasted, nil
}
