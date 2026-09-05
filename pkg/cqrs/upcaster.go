package cqrs

import (
	"github.com/larsartmann/go-codec"
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
// Legacy schema versions of ItemSynced payloads (ADR-0007): V1 and V2 carry
// actor/repo as top-level fields.
const (
	legacySchemaV1 = event.SchemaVersion(1)
	legacySchemaV2 = event.SchemaVersion(2)
	currentSchemaV = event.SchemaVersion(3)
)

func newLegacyUpcasters() []schema.Upcaster {
	return []schema.Upcaster{
		schema.NewUpcaster(EventItemSynced, legacySchemaV1, upcastItemSyncedToV3),
		schema.NewUpcaster(EventItemSynced, legacySchemaV2, upcastItemSyncedToV3),
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
	payload.SchemaVersion = int(currentSchemaV)

	// Re-encode with the same codec family the event was created with
	// (NewEvents defaults to CBOR) and keep the encoding stamp consistent,
	// so downstream DecodePayloadAuto keeps working.
	raw, err := codec.CBORCodec{}.Encode(payload)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: encode payload")
	}

	upcasted, err := event.ReconstructEventWithAdoptedPayload(
		evt.ID(), evt.Type(), evt.StreamType(), evt.StreamID(),
		int(evt.Version()), int(currentSchemaV), raw, evt.Metadata(), evt.OccurredAt(),
		codec.EncodingCBOR, "localsync.upcast_item_synced",
	)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: reconstruct event")
	}

	return upcasted, nil
}
