package cqrs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func TestAggregateID_Deterministic(t *testing.T) {
	t.Parallel()

	aggID := AggregateID("github", id.NewExternalID("123"))

	if aggID != AggregateID("github", id.NewExternalID("123")) {
		t.Error("same inputs must produce same AggregateID")
	}
}

func TestAggregateID_DifferentInputs(t *testing.T) {
	t.Parallel()

	if AggregateID(
		"github",
		id.NewExternalID("123"),
	) == AggregateID(
		"github",
		id.NewExternalID("456"),
	) {
		t.Error("different inputs must produce different AggregateIDs")
	}
}

func TestFold_ItemSynced(t *testing.T) {
	t.Parallel()

	payload := testSyncedPayload("123", "PushEvent")

	evt := mustNewTestEvent(EventItemSynced, payload)

	state, err := Fold(InitialState, evt)
	mustNoError(t, err)
	if state.Item == nil {
		t.Fatal("expected non-nil Item")
	}
	assertEqual(t, state.Item.Source.Get(), "github", "Source")
	assertExternalID(t, state.Item, "123")
	assertItemType(t, state.Item, "PushEvent")
	if state.Deleted {
		t.Error("expected Deleted=false")
	}
}

func TestFold_ItemSyncedOverwritesState(t *testing.T) {
	t.Parallel()

	existing := testActiveState("123", "PushEvent")

	updatedPayload := testSyncedPayload("123", "IssueEvent")

	evt := mustNewTestEvent(EventItemSynced, updatedPayload)

	state, err := Fold(existing, evt)
	mustNoError(t, err)

	assertItemType(t, state.Item, "IssueEvent")
}

func TestDecideSync_Fold_PreservesItemID(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")
	item.ID = id.NewItemID()
	originalID := item.ID.String()

	events, err := DecideSync(item, nil)(InitialState, 0)
	mustNoError(t, err)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	var payload ItemSyncedPayload
	if err := json.Unmarshal(events[0].Payload(), &payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.ItemID != originalID {
		t.Errorf("expected ItemID=%s, got %s", originalID, payload.ItemID)
	}

	state, err := Fold(InitialState, events[0])
	mustNoError(t, err)
	if state.Item == nil {
		t.Fatal("expected non-nil Item")
	}
	if state.Item.ID.String() != originalID {
		t.Errorf("expected ID=%s, got %s", originalID, state.Item.ID.String())
	}
}

func TestFold_ItemDeleted(t *testing.T) {
	t.Parallel()

	existing := testActiveState("123", "")

	evt := mustNewTestEvent(EventItemDeleted, ItemDeletedPayload{Source: "github", SourceID: "123"})

	state, err := Fold(existing, evt)
	mustNoError(t, err)

	if !state.Deleted {
		t.Error("expected Deleted=true")
	}
	if state.Item == nil {
		t.Error("deleted state still holds the item for potential resurrection")
	}
}

func TestFold_ItemConflictFound(t *testing.T) {
	t.Parallel()

	existing := testActiveState("123", "PushEvent")

	evt := mustNewTestEvent(EventItemConflictFound, ItemConflictFoundPayload{
		Source: "github", SourceID: "123", Winner: "remote",
	})

	state, err := Fold(existing, evt)
	mustNoError(t, err)

	assertItemType(t, state.Item, "PushEvent")
}

func TestFold_UnknownEventType(t *testing.T) {
	t.Parallel()

	evt := mustNewTestEvent(event.Type("unknown"), map[string]string{"test": "data"})

	_, err := Fold(InitialState, evt)
	if err == nil {
		t.Fatal("expected error for unknown event type")
	}
}

func TestDecideSync_NewItem(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")

	events, err := DecideSync(item, nil)(InitialState, 0)
	mustNoError(t, err)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	assertEventType(t, events[0], EventItemSynced)
}

func TestDecideSync_UnchangedItem(t *testing.T) {
	t.Parallel()

	now := time.Now()
	item := testItem("123", "PushEvent")
	item.UpdatedAt = now

	state := SyncItemState{
		Item: &provider.Item{
			ExternalID: id.NewExternalID("123"),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			ActorLogin: id.NewActorID("testuser"),
			RepoName:   id.NewRepoID("owner/repo"),
			UpdatedAt:  now,
		},
	}

	events, err := DecideSync(item, nil)(state, 1)
	mustNoError(t, err)
	if events != nil {
		t.Errorf("unchanged item produces no events, got %d", len(events))
	}
}

func TestDecideSync_ConflictResolution(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")
	item.UpdatedAt = time.Now().Add(time.Hour)

	state := SyncItemState{
		Item: &provider.Item{
			ExternalID: id.NewExternalID("123"),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			UpdatedAt:  time.Now(),
		},
	}

	events, err := DecideSync(item, nil)(state, 1)
	mustNoError(t, err)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	assertEventType(t, events[0], EventItemConflictFound)
	assertEventType(t, events[1], EventItemSynced)
}

func TestDecideSync_ConflictTimestamps(t *testing.T) {
	t.Parallel()

	localTime := time.Now().Truncate(time.Millisecond)
	remoteTime := localTime.Add(2 * time.Hour)

	item := testItem("123", "PushEvent")
	item.UpdatedAt = remoteTime

	state := SyncItemState{
		Item: &provider.Item{
			ExternalID: id.NewExternalID("123"),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			UpdatedAt:  localTime,
		},
	}

	events, err := DecideSync(item, nil)(state, 1)
	mustNoError(t, err)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	var conflictPayload ItemConflictFoundPayload
	if err := json.Unmarshal(events[0].Payload(), &conflictPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conflictPayload.LocalUpdatedAt == conflictPayload.RemoteUpdatedAt {
		t.Error("LocalUpdatedAt and RemoteUpdatedAt must differ in conflict")
	}
	if conflictPayload.LocalUpdatedAt != localTime.UnixNano() {
		t.Errorf("LocalUpdatedAt must come from existing state")
	}
	if conflictPayload.RemoteUpdatedAt != remoteTime.UnixNano() {
		t.Errorf("RemoteUpdatedAt must come from incoming item")
	}
}

func TestDecideSync_ResurrectDeletedItem(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")

	state := testDeletedState("123")

	events, err := DecideSync(item, nil)(state, 2)
	mustNoError(t, err)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	assertEventType(t, events[0], EventItemSynced)
}

func TestDecideDelete_ActiveItem(t *testing.T) {
	t.Parallel()

	state := testActiveState("123", "")

	events, err := DecideDelete("github", id.NewExternalID("123"))(state, 1)
	mustNoError(t, err)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	assertEventType(t, events[0], EventItemDeleted)
}

func TestDecideDelete_AlreadyDeleted(t *testing.T) {
	t.Parallel()

	state := testDeletedState("123")

	events, err := DecideDelete("github", id.NewExternalID("123"))(state, 1)
	mustNoError(t, err)
	if events != nil {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestDecideDelete_NewItem(t *testing.T) {
	t.Parallel()

	events, err := DecideDelete("github", id.NewExternalID("123"))(InitialState, 0)
	mustNoError(t, err)
	if events != nil {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestSyncItemState_IsNew(t *testing.T) {
	t.Parallel()

	if !InitialState.IsNew() {
		t.Error("expected initial state to be new")
	}

	existing := SyncItemState{Item: &provider.Item{}}
	if existing.IsNew() {
		t.Error("expected existing state to not be new")
	}
}
