package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/scenario/v4"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// Projection scenario specs (scenario.GivenProjection) extend the decider
// convention to the read side: feed events through the REAL projector against
// a real memory read model, assert no error, then assert the projected state.
// The DSL proves the Handle contract; the follow-up List/Get assertions prove
// what the projection actually did — neither alone would pin the behavior.

// projectedItem fetches one item from the read model, failing the test on error.
func projectedItem(t *testing.T, rm ReadModel, sourceID string, includeTombstoned bool) *model.Item {
	t.Helper()

	items, err := rm.List(context.Background(), model.ItemFilter{IncludeTombstoned: includeTombstoned})
	testutil.MustNoError(t, err)

	for _, item := range items {
		if item.SourceID.Get() == sourceID {
			return item
		}
	}

	return nil
}

// mustScenarioEvent builds one event for the projection specs at the given version.
func mustScenarioEvent(
	t *testing.T, sourceID string, version event.Version, eventType event.Type, payload any,
) event.Event {
	t.Helper()

	evts, err := event.NewEvents(
		MustStreamID("github", id.NewSourceID(sourceID)),
		aggregateType,
		version,
		[]event.Type{eventType},
		[]any{payload},
	)
	testutil.MustNoError(t, err)

	return evts[0]
}

func syncedPayloadFor(item *model.Item) ItemSyncedPayload {
	return ItemSyncedPayload{
		ItemID:      item.ID.String(),
		Source:      item.Source.Get(),
		SourceID:    item.SourceID.Get(),
		Type:        item.Type.Get(),
		Attributes:  item.Attributes,
		ContentHash: item.ContentHash.String(),
		RawJSON:     []byte(`{}`),
		CreatedAt:   item.CreatedAt.UnixNano(),
		UpdatedAt:   item.UpdatedAt.UnixNano(),
	}
}

// TestProjectionScenario_SyncUpsertsIntoReadModel: a synced event lands as a
// live, queryable row with all payload fields intact.
func TestProjectionScenario_SyncUpsertsIntoReadModel(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)
	item := testDataItem("proj-1", "PushEvent")
	item.UpdatedAt = time.Now().UTC()

	scenario.GivenProjection(t, proj,
		mustScenarioEvent(t, "proj-1", 1, EventItemSynced, syncedPayloadFor(item)),
	).ThenNoError()

	got := projectedItem(t, rm, "proj-1", false)
	if got == nil {
		t.Fatal("synced event must project to a live row")
	}
	if got.Type.Get() != "PushEvent" || got.Source.Get() != "github" {
		t.Errorf("projected item lost identity fields: %+v", got)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("projected item must carry UpdatedAt from the payload")
	}
}

// TestProjectionScenario_TombstoneHidesButKeeps: synced → tombstoned leaves
// the row hidden from the default view, visible under IncludeTombstoned with
// the typed reason preserved (ADR-0005: tombstone over delete).
func TestProjectionScenario_TombstoneHidesButKeeps(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)
	item := testDataItem("proj-2", "PushEvent")

	scenario.GivenProjection(t, proj,
		mustScenarioEvent(t, "proj-2", 1, EventItemSynced, syncedPayloadFor(item)),
		mustScenarioEvent(t, "proj-2", 2, EventItemTombstoned, ItemTombstonedPayload{
			Source:       "github",
			SourceID:     "proj-2",
			Reason:       string(model.ReasonUserHidden),
			TombstonedAt: time.Now().UTC().UnixNano(),
		}),
	).ThenNoError()

	if projectedItem(t, rm, "proj-2", false) != nil {
		t.Error("tombstoned item must be hidden from the default view")
	}

	kept := projectedItem(t, rm, "proj-2", true)
	if kept == nil {
		t.Fatal("tombstoned item must remain in the read model under IncludeTombstoned")
	}
	if kept.Tombstone.Reason != model.ReasonUserHidden {
		t.Errorf("tombstone reason = %q, want %q", kept.Tombstone.Reason, model.ReasonUserHidden)
	}
}

// TestProjectionScenario_StaleReplayCannotResurrect: the mutex-guarded
// version gate (ADR-0006 live+replay serialization) drops a stale replayed
// event whose version is not newer than the last applied one — even a synced
// event after a tombstone must not resurrect the row.
func TestProjectionScenario_StaleReplayCannotResurrect(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)
	item := testDataItem("proj-3", "PushEvent")

	scenario.GivenProjection(t, proj,
		mustScenarioEvent(t, "proj-3", 1, EventItemSynced, syncedPayloadFor(item)),
		mustScenarioEvent(t, "proj-3", 2, EventItemTombstoned, ItemTombstonedPayload{
			Source:       "github",
			SourceID:     "proj-3",
			Reason:       string(model.ReasonUpstreamGone),
			TombstonedAt: time.Now().UTC().UnixNano(),
		}),
		// Stale replay of the ORIGINAL synced event (version 1 <= 2): dropped.
		mustScenarioEvent(t, "proj-3", 1, EventItemSynced, syncedPayloadFor(item)),
	).ThenNoError()

	if projectedItem(t, rm, "proj-3", false) != nil {
		t.Error("stale replayed synced event must not resurrect the tombstoned row")
	}

	if kept := projectedItem(t, rm, "proj-3", true); kept == nil {
		t.Fatal("the tombstoned row itself must survive the stale replay")
	}
}

// TestProjectionScenario_NewerSyncResurrects: a synced event with a HIGHER
// version than the tombstone resurrects the item (the only path back to live).
func TestProjectionScenario_NewerSyncResurrects(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)
	item := testDataItem("proj-4", "PushEvent")

	scenario.GivenProjection(t, proj,
		mustScenarioEvent(t, "proj-4", 1, EventItemSynced, syncedPayloadFor(item)),
		mustScenarioEvent(t, "proj-4", 2, EventItemTombstoned, ItemTombstonedPayload{
			Source:       "github",
			SourceID:     "proj-4",
			Reason:       string(model.ReasonUserHidden),
			TombstonedAt: time.Now().UTC().UnixNano(),
		}),
		mustScenarioEvent(t, "proj-4", 3, EventItemSynced, syncedPayloadFor(item)),
	).ThenNoError()

	if got := projectedItem(t, rm, "proj-4", false); got == nil {
		t.Fatal("a newer synced event must resurrect the tombstoned item")
	}
}

// TestProjectionScenario_ConlictFoundIsMetadataOnly: the conflict event is a
// no-op on the read model — it must neither error nor mutate rows.
func TestProjectionScenario_ConflictFoundIsMetadataOnly(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)
	item := testDataItem("proj-5", "PushEvent")

	scenario.GivenProjection(t, proj,
		mustScenarioEvent(t, "proj-5", 1, EventItemSynced, syncedPayloadFor(item)),
		mustScenarioEvent(t, "proj-5", 2, EventItemConflictFound, ItemConflictFoundPayload{
			Source:          "github",
			SourceID:        "proj-5",
			Winner:          "remote",
			LocalUpdatedAt:  time.Now().Add(-time.Hour).Unix(),
			RemoteUpdatedAt: time.Now().Unix(),
		}),
	).ThenNoError()

	got := projectedItem(t, rm, "proj-5", false)
	if got == nil {
		t.Fatal("conflict event must not remove the live row")
	}
}

// TestProjectionScenario_TombstoneForUnknownItemIsIdempotent: a tombstone
// with no prior row is a silent no-op — reconciliation replays may tombstone
// items the local store never saw, and re-delivery must stay idempotent.
func TestProjectionScenario_TombstoneForUnknownItemIsIdempotent(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)

	sc := scenario.GivenProjection(t, proj,
		mustScenarioEvent(t, "proj-ghost", 1, EventItemTombstoned, ItemTombstonedPayload{
			Source:       "github",
			SourceID:     "proj-ghost",
			Reason:       string(model.ReasonUpstreamGone),
			TombstonedAt: time.Now().UTC().UnixNano(),
		}),
	)
	sc.ThenNoError()

	if got := projectedItem(t, rm, "proj-ghost", true); got != nil {
		t.Error("tombstoning an unseen item must not invent a row")
	}
}
