package cqrs

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
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

// TestUpcaster_V1PayloadFoldsAttributes exercises the upcaster directly:
// legacy fields fold into Attributes; the event schema version is PRESERVED
// (the registry chain owns version bumps).
func TestUpcaster_V1PayloadFoldsAttributes(t *testing.T) {
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

	// The registry owns the version bump; a direct call only rewrites the
	// payload (Attributes folded) and PRESERVES the original schema version.
	if upcasted.SchemaVersion() != 1 {
		t.Errorf(
			"direct upcast must preserve the event schema version (registry bumps it), got %d",
			upcasted.SchemaVersion(),
		)
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

// TestUpcaster_ChainSemantics_V1ToFoldedV3 pins WHY the registry's
// V1→V2→V3 chain applies the same upcaster function twice without corrupting
// the payload: the fold runs exactly once (the second pass sees Attributes
// already present and only re-encodes), legacy fields stay for round-trip
// safety, and identity is preserved through every step. A library change to
// the chaining contract must break THIS test, not production data.
func TestUpcaster_ChainSemantics_V1ToFoldedV3(t *testing.T) {
	t.Parallel()

	transform := schema.UpcastSourceTransform(newLegacyUpcasters()...)

	aggID := AggregateID("github", id.NewExternalID("up-chain"))

	raw, err := event.NewEvents(aggID, aggregateType, event.Version(7),
		[]event.Type{EventItemSynced}, []any{legacyV1Payload()}, event.WithSchemaVersion(1))
	testutil.MustNoError(t, err)

	upcasted, err := transform(raw)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, upcasted, 1)

	got := upcasted[0]

	if got.SchemaVersion() != 3 {
		t.Fatalf("raw V1 event must reach V3 through the chain, got %d", got.SchemaVersion())
	}

	if got.ID() != raw[0].ID() || got.Version() != raw[0].Version() {
		t.Error("chain must preserve event identity (ID, stream version)")
	}

	payload, err := event.DecodePayloadAuto[ItemSyncedPayload](got)
	testutil.MustNoError(t, err)

	testutil.AssertEqual(t, payload.Attributes["actor_login"], "octocat", "folded actor_login")
	testutil.AssertEqual(t, payload.Attributes["repo_name"], "octo/hello", "folded repo_name")
	testutil.AssertEqual(t, payload.ActorLogin, "octocat", "legacy field retained")

	// Idempotent at the read boundary: re-transforming the already-V3 result
	// changes nothing (same pointer, no second fold).
	again, err := transform([]event.Event{got})
	testutil.MustNoError(t, err)

	if again[0] != got {
		t.Error("re-transforming a V3 event must be a pass-through")
	}

	reread, err := event.DecodePayloadAuto[ItemSyncedPayload](again[0])
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, reread.Attributes["actor_login"], "octocat", "no double fold")
}

// TestUpcaster_LegacyVersionWithAttributes_RebuildsPrivateEvent pins the
// concurrency fix: an event carrying a legacy version stamp (1/2) but already
// folded Attributes must come back as a FRESH event, because the registry
// stamps the returned event's schema version in place and the memory backend
// serves stored pointers to concurrent readers. Handing the stored pointer
// back was the exact shape of the 2026-09-05 data race.
func TestUpcaster_LegacyVersionWithAttributes_RebuildsPrivateEvent(t *testing.T) {
	t.Parallel()

	transform := schema.UpcastSourceTransform(newLegacyUpcasters()...)

	aggID := AggregateID("github", id.NewExternalID("up-anomaly"))

	anomalous := legacyV1Payload()
	anomalous.Attributes = map[string]string{"actor_login": "octocat"}

	raw, err := event.NewEvents(aggID, aggregateType, event.Version(0),
		[]event.Type{EventItemSynced}, []any{anomalous}, event.WithSchemaVersion(1))
	testutil.MustNoError(t, err)

	upcasted, err := transform(raw)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, upcasted, 1)

	if upcasted[0] == raw[0] {
		t.Error("legacy-versioned event with Attributes must be rebuilt, not passed through: " +
			"the registry's in-place version stamp would mutate the stored event")
	}

	if upcasted[0].SchemaVersion() != 3 {
		t.Errorf("anomalous legacy event must reach V3, got %d", upcasted[0].SchemaVersion())
	}
}

// TestUpcaster_ConcurrentReadsDuringSync is the standing regression for the
// upcaster data race: legacy events in the memory store are replayed
// (Load → upcast transform) by several goroutines WHILE a live writer appends
// fresh V3 events — the same overlap that turned the registry's in-place
// version stamp into a data race on 2026-09-05. Each legacy stream's FIRST
// load is the write window (the registry stamps the returned event's schema
// version in place), so the test seeds many legacy streams with a barrier
// start and shifted visit orders to collide readers inside those windows.
// Run with -race; any in-place mutation of a stored pointer fails the run.
func TestUpcaster_ConcurrentReadsDuringSync(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	sr, err := createStoreAndBus(ctx, CQRSConfig{Backend: backendMemory})
	testutil.MustNoError(t, err)
	defer closeStoreResult(t, sr)

	const legacyStreams = 100

	refs := make([]cqrsid.StreamRef, 0, legacyStreams)

	for i := range legacyStreams {
		sid := fmt.Sprintf("up-race-%d", i)

		legacy := legacyV1Payload()
		legacy.SourceID = sid
		// Anomalous shape (Attributes folded, legacy version stamp): with the
		// old pass-through this event's FIRST load hands the STORED pointer
		// to the registry for in-place stamping — the write window.
		legacy.Attributes = map[string]string{"actor_login": "octocat"}

		aggID := AggregateID("github", id.NewExternalID(sid))
		ref := cqrsid.NewStreamRef(aggregateType, aggID)

		rawLegacy, err := event.NewEvents(aggID, aggregateType, event.Version(0),
			[]event.Type{EventItemSynced}, []any{legacy}, event.WithSchemaVersion(1))
		testutil.MustNoError(t, err)

		testutil.MustNoError(t, sr.store.Save(ctx, ref, rawLegacy, event.Version(0)))
		refs = append(refs, ref)
	}

	const readers = 4

	start := make(chan struct{})
	var wg sync.WaitGroup

	for r := range readers {
		wg.Go(func() {
			<-start

			// Shifted start offset: readers discover each stream's first-load
			// write window at different times, maximizing overlap.
			for round := range 30 {
				for i := range legacyStreams {
					ref := refs[(i+r+round)%legacyStreams]

					loaded, err := sr.store.Load(ctx, ref)
					if err != nil {
						t.Errorf("concurrent load: %v", err)

						return
					}

					if len(loaded) != 1 || loaded[0].SchemaVersion() != 3 {
						t.Errorf("concurrent load must serve exactly one V3 event, got %d events", len(loaded))

						return
					}
				}
			}
		})
	}

	// Live writer overlapping the replay readers: fresh V3 events appended
	// while others are mid-transform (the sync-vs-replay overlap).
	for i := range 25 {
		sid := fmt.Sprintf("up-race-w%d", i)
		writerAgg := AggregateID("github", id.NewExternalID(sid))
		writerRef := cqrsid.NewStreamRef(aggregateType, writerAgg)

		fresh, err := event.NewEvents(writerAgg, aggregateType, event.Version(0),
			[]event.Type{EventItemSynced},
			[]any{ItemSyncedPayload{
				Source: "github", SourceID: sid, Type: "PushEvent",
				Attributes: map[string]string{"actor_login": "fresh"},
			}},
			event.WithSchemaVersion(3))
		testutil.MustNoError(t, err)

		testutil.MustNoError(t, sr.store.Save(ctx, writerRef, fresh, event.Version(0)))
	}

	close(start)
	wg.Wait()
}
