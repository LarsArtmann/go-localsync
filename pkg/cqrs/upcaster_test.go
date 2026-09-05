package cqrs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/schema/v4"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// legacyV1Payload builds an ItemSynced payload in the pre-V3 shape: actor and
// repo as top-level fields, no Attributes map.
func legacyV1Payload() ItemSyncedPayload {
	return ItemSyncedPayload{
		ItemID:         "legacy-id",
		Source:         "github",
		SourceID:       "up-1",
		Type:           "PushEvent",
		ActorLogin:     "octocat",
		ActorAvatarURL: "https://avatars.example/u/1",
		RepoName:       "octo/hello",
		RepoURL:        "https://github.com/octo/hello",
		CreatedAt:      1,
		UpdatedAt:      2,
		SchemaVersion:  1,
	}
}

// TestUpcaster_V1PayloadBecomesV3 exercises the upcaster directly: legacy
// fields fold into Attributes, schema version stamps 3, identity is kept.
func TestUpcaster_V1PayloadBecomesV3(t *testing.T) {
	t.Parallel()

	aggID := AggregateID("github", id.NewExternalID("up-1"))

	evts, err := event.NewEvents(aggID, aggregateType, event.Version(0),
		[]event.Type{EventItemSynced},
		[]any{legacyV1Payload()},
		event.WithSchemaVersion(1),
	)
	testutil.MustNoError(t, err)

	upcasted, err := upcastItemSyncedToV3(evts[0])
	testutil.MustNoError(t, err)

	if upcasted.SchemaVersion() != 3 {
		t.Errorf("schema version: want 3, got %d", upcasted.SchemaVersion())
	}

	if upcasted.ID() != evts[0].ID() || upcasted.Version() != evts[0].Version() {
		t.Error("upcast must preserve event identity")
	}

	payload, err := event.DecodePayloadAuto[ItemSyncedPayload](upcasted)
	testutil.MustNoError(t, err)

	if payload.Attributes["actor_login"] != "octocat" ||
		payload.Attributes["repo_name"] != "octo/hello" ||
		payload.Attributes["repo_url"] != "https://github.com/octo/hello" {
		t.Errorf("legacy fields not folded into Attributes: %v", payload.Attributes)
	}

	if payload.ActorLogin != "octocat" {
		t.Error("legacy fields must remain in the payload for round-trip safety")
	}
}

// TestUpcaster_AlreadyV3PassesThrough: current-schema events are returned
// untouched (same pointer) — no pointless re-encoding on the hot path.
func TestUpcaster_AlreadyV3PassesThrough(t *testing.T) {
	t.Parallel()

	aggID := AggregateID("github", id.NewExternalID("up-2"))

	evts, err := event.NewEvents(aggID, aggregateType, event.Version(0),
		[]event.Type{EventItemSynced},
		[]any{ItemSyncedPayload{
			Source: "github", SourceID: "up-2", Type: "PushEvent",
			Attributes:    map[string]string{"actor_login": "x"},
			SchemaVersion: 3,
		}},
		event.WithSchemaVersion(3),
	)
	testutil.MustNoError(t, err)

	upcasted, err := upcastItemSyncedToV3(evts[0])
	testutil.MustNoError(t, err)

	if upcasted != evts[0] {
		t.Error("V3 events must pass through without rebuilding")
	}
}

// TestUpcaster_RegistryAppliesByVersion pins the registry wiring: the
// transform pipeline routes (EventItemSynced, version 1) and (…, version 2)
// to the upcaster and leaves other versions/types alone.
func TestUpcaster_RegistryAppliesByVersion(t *testing.T) {
	t.Parallel()

	transform := schema.UpcastSourceTransform(newLegacyUpcasters()...)

	aggID := AggregateID("github", id.NewExternalID("up-3"))

	v1, err := event.NewEvents(aggID, aggregateType, event.Version(0),
		[]event.Type{EventItemSynced}, []any{legacyV1Payload()}, event.WithSchemaVersion(1))
	testutil.MustNoError(t, err)

	got, err := transform(v1)
	testutil.MustNoError(t, err)

	if got[0].SchemaVersion() != 3 {
		t.Errorf("registry did not upcast V1: schema version %d", got[0].SchemaVersion())
	}

	v3, err := event.NewEvents(aggID, aggregateType, event.Version(0),
		[]event.Type{EventItemTombstoned},
		[]any{ItemTombstonedPayload{Source: "github", SourceID: "up-3", Reason: "user_hidden"}},
		event.WithSchemaVersion(1))
	testutil.MustNoError(t, err)

	gotOther, err := transform(v3)
	testutil.MustNoError(t, err)

	if gotOther[0] != v3[0] {
		t.Error("non-matching (type, version) pairs must not be touched")
	}
}

// TestUpcaster_StoreReadBoundaryUpcasts proves the end-to-end contract: a
// legacy-shaped event saved raw into the SQLite store is served as V3 through
// the decorated store (this is exactly the restart-with-old-data scenario).
func TestUpcaster_StoreReadBoundaryUpcasts(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "upcast.db")
	ctx := context.Background()

	sr, err := createStoreAndBus(ctx, CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
	testutil.MustNoError(t, err)
	defer closeStoreResult(t, sr)

	aggID := AggregateID("github", id.NewExternalID("up-4"))
	ref := cqrsid.NewStreamRef(aggregateType, aggID)

	// Save a legacy event BYPASSING the decorated store (raw store semantics):
	// write through the journal-free path so the upcast cannot happen on write.
	raw := sr.store
	legacy := legacyV1Payload()
	legacy.SourceID = "up-4"

	evts, err := event.NewEvents(aggID, aggregateType, event.Version(0),
		[]event.Type{EventItemSynced}, []any{legacy}, event.WithSchemaVersion(1))
	testutil.MustNoError(t, err)

	testutil.MustNoError(t, raw.Save(ctx, ref, evts, event.Version(0)))

	// Read through the decorated store: the payload must be V3.
	loaded, err := sr.store.Load(ctx, ref)
	testutil.MustNoError(t, err)

	if len(loaded) != 1 {
		t.Fatalf("expected 1 stored event, got %d", len(loaded))
	}

	if loaded[0].SchemaVersion() != 3 {
		t.Errorf("store read must serve V3, got schema version %d", loaded[0].SchemaVersion())
	}
}
