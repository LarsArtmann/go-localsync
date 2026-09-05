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
//
// WHY the registry's V1→V2→V3 chain applies this function twice, safely:
// both registered upcasters share this function, and it rebuilds each event
// at its ORIGINAL schema version, so the registry's chain can stamp V1→V2
// (first pass: legacy fields folded) and then V2→V3 (second pass: Attributes
// already present, payload unchanged). The fold is therefore applied exactly
// once; the second pass is a pure re-encode. This is pinned by
// TestUpcaster_ChainSemantics_V1ToFoldedV3.
//
// The rebuild is also a CONCURRENCY requirement, not just a chaining detail:
// the registry stamps the returned event's schema version in place (an
// event.Option mutating *ImmutableEvent), and the memory backend serves
// stored event pointers to concurrent readers (its read path clones the
// slice, not the elements). Every event this function returns to the
// registry is therefore a fresh private copy — the in-place stamp can never
// land on a stored event. The 2026-09-05 data race lived exactly here.
func upcastItemSyncedToV3(evt event.Event) (event.Event, error) {
	payload, err := event.DecodePayloadAuto[ItemSyncedPayload](evt)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: decode payload")
	}

	// True V3 events take the pass-through fast path. The registry never
	// routes them here (no upcaster has source version 3), so returning the
	// shared pointer is safe — nothing downstream mutates it. Anything still
	// carrying a legacy version stamp (1/2) is rebuilt, Attributes folded or
	// not: the registry will stamp it in place, and a stored pointer must
	// never be handed back for that.
	if payload.Attributes != nil && evt.SchemaVersion() == currentSchemaV {
		return evt, nil
	}

	if payload.Attributes == nil {
		payload.Attributes = upcastLegacyAttributes(payload)
	}

	payload.SchemaVersion = int(currentSchemaV)

	// Re-encode with the same codec family the event was created with
	// (NewEvents defaults to CBOR) and keep the encoding stamp consistent,
	// so downstream DecodePayloadAuto keeps working.
	raw, err := codec.CBORCodec{}.Encode(payload)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: encode payload")
	}

	// The REGISTRY owns the schema-version bump (V1 -> V2 -> V3 chaining);
	// this upcaster rebuilds the event at its ORIGINAL version so the chain
	// can advance it step by step (see the WHY comment above).
	upcasted, err := event.ReconstructEventWithAdoptedPayload(
		evt.ID(), evt.Type(), evt.StreamType(), evt.StreamID(),
		int(evt.Version()), int(evt.SchemaVersion()), raw, evt.Metadata(), evt.OccurredAt(),
		codec.EncodingCBOR, "localsync.upcast_item_synced",
	)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "upcast item.synced: reconstruct event")
	}

	return upcasted, nil
}
