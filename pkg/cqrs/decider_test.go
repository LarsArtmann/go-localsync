package cqrs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func TestAggregateID_Deterministic(t *testing.T) {
	t.Parallel()

	a := AggregateID("github", "123")
	b := AggregateID("github", "123")

	if a != b {
		t.Error("same inputs must produce same AggregateID")
	}
}

func TestAggregateID_DifferentInputs(t *testing.T) {
	t.Parallel()

	a := AggregateID("github", "123")
	b := AggregateID("github", "456")

	if a == b {
		t.Error("different inputs must produce different AggregateIDs")
	}
}

func TestFold_ItemSynced(t *testing.T) {
	t.Parallel()

	payload := ItemSyncedPayload{
		Source:    "github",
		SourceID:  "123",
		Type:      "PushEvent",
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	evt := mustNewTestEvent(EventItemSynced, payload)

	state, err := Fold(InitialState, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Item == nil {
		t.Fatal("expected non-nil Item")
	}
	if state.Item.Source.Get() != "github" {
		t.Errorf("expected Source=github, got %s", state.Item.Source.Get())
	}
	if state.Item.ExternalID.Get() != "123" {
		t.Errorf("expected ExternalID=123, got %s", state.Item.ExternalID.Get())
	}
	if state.Item.Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", state.Item.Type.Get())
	}
	if state.Deleted {
		t.Error("expected Deleted=false")
	}
}

func TestFold_ItemSyncedOverwritesState(t *testing.T) {
	t.Parallel()

	existing := SyncItemState{
		Item: &provider.Item{
			ExternalID: types.NewExternalID("123"),
			Source:     types.NewProviderID("github"),
			Type:       types.NewEventTypeID("PushEvent"),
		},
	}

	updatedPayload := ItemSyncedPayload{
		Source:    "github",
		SourceID:  "123",
		Type:      "IssueEvent",
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	evt := mustNewTestEvent(EventItemSynced, updatedPayload)

	state, err := Fold(existing, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Item.Type.Get() != "IssueEvent" {
		t.Errorf("expected Type=IssueEvent, got %s", state.Item.Type.Get())
	}
}

func TestDecideSync_Fold_PreservesItemID(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")
	item.ID = types.NewItemID()
	originalID := item.ID.String()

	events, err := DecideSync(item)(InitialState, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Item == nil {
		t.Fatal("expected non-nil Item")
	}
	if state.Item.ID.String() != originalID {
		t.Errorf("expected ID=%s, got %s", originalID, state.Item.ID.String())
	}
}

func TestFold_ItemDeleted(t *testing.T) {
	t.Parallel()

	existing := SyncItemState{
		Item: &provider.Item{
			ExternalID: types.NewExternalID("123"),
			Source:     types.NewProviderID("github"),
		},
	}

	evt := mustNewTestEvent(EventItemDeleted, ItemDeletedPayload{Source: "github", SourceID: "123"})

	state, err := Fold(existing, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !state.Deleted {
		t.Error("expected Deleted=true")
	}
	if state.Item == nil {
		t.Error("deleted state still holds the item for potential resurrection")
	}
}

func TestFold_ItemConflictFound(t *testing.T) {
	t.Parallel()

	existing := SyncItemState{
		Item: &provider.Item{
			ExternalID: types.NewExternalID("123"),
			Source:     types.NewProviderID("github"),
			Type:       types.NewEventTypeID("PushEvent"),
		},
	}

	evt := mustNewTestEvent(EventItemConflictFound, ItemConflictFoundPayload{
		Source: "github", SourceID: "123", Winner: "remote",
	})

	state, err := Fold(existing, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Item.Type.Get() != "PushEvent" {
		t.Errorf("conflict event should not change state, got Type=%s", state.Item.Type.Get())
	}
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

	events, err := DecideSync(item)(InitialState, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type() != EventItemSynced {
		t.Errorf("expected type=%s, got %s", EventItemSynced, events[0].Type())
	}
}

func TestDecideSync_UnchangedItem(t *testing.T) {
	t.Parallel()

	now := time.Now()
	item := testItem("123", "PushEvent")
	item.UpdatedAt = now

	state := SyncItemState{
		Item: &provider.Item{
			ExternalID: types.NewExternalID("123"),
			Source:     types.NewProviderID("github"),
			Type:       types.NewEventTypeID("PushEvent"),
			ActorLogin: types.NewActorID("testuser"),
			RepoName:   types.NewRepoID("owner/repo"),
			UpdatedAt:  now,
		},
	}

	events, err := DecideSync(item)(state, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
			ExternalID: types.NewExternalID("123"),
			Source:     types.NewProviderID("github"),
			Type:       types.NewEventTypeID("PushEvent"),
			UpdatedAt:  time.Now(),
		},
	}

	events, err := DecideSync(item)(state, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type() != EventItemConflictFound {
		t.Errorf("expected type=%s, got %s", EventItemConflictFound, events[0].Type())
	}
	if events[1].Type() != EventItemSynced {
		t.Errorf("expected type=%s, got %s", EventItemSynced, events[1].Type())
	}
}

func TestDecideSync_ConflictTimestamps(t *testing.T) {
	t.Parallel()

	localTime := time.Now().Truncate(time.Millisecond)
	remoteTime := localTime.Add(2 * time.Hour)

	item := testItem("123", "PushEvent")
	item.UpdatedAt = remoteTime

	state := SyncItemState{
		Item: &provider.Item{
			ExternalID: types.NewExternalID("123"),
			Source:     types.NewProviderID("github"),
			Type:       types.NewEventTypeID("PushEvent"),
			UpdatedAt:  localTime,
		},
	}

	events, err := DecideSync(item)(state, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	state := SyncItemState{
		Item:    &provider.Item{ExternalID: types.NewExternalID("123")},
		Deleted: true,
	}

	events, err := DecideSync(item)(state, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type() != EventItemSynced {
		t.Errorf("expected type=%s, got %s", EventItemSynced, events[0].Type())
	}
}

func TestDecideDelete_ActiveItem(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Item: &provider.Item{
			ExternalID: types.NewExternalID("123"),
			Source:     types.NewProviderID("github"),
		},
	}

	events, err := DecideDelete("github", "123")(state, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type() != EventItemDeleted {
		t.Errorf("expected type=%s, got %s", EventItemDeleted, events[0].Type())
	}
}

func TestDecideDelete_AlreadyDeleted(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Item:    &provider.Item{ExternalID: types.NewExternalID("123")},
		Deleted: true,
	}

	events, err := DecideDelete("github", "123")(state, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestDecideDelete_NewItem(t *testing.T) {
	t.Parallel()

	events, err := DecideDelete("github", "123")(InitialState, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func mustNewTestEvent(eventType event.Type, payload any) *event.Core {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	aggID := id.NewAggregateID()

	evt, err := event.NewEvent(eventType, aggID, aggregateType, 1, data)
	if err != nil {
		panic(err)
	}

	return evt
}

func testItem(sourceID, itemType string) *provider.Item {
	return &provider.Item{
		ExternalID: types.NewExternalID(sourceID),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID(itemType),
		ActorLogin: types.NewActorID("testuser"),
		RepoName:   types.NewRepoID("owner/repo"),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RawJSON:    json.RawMessage(`{"test":true}`),
	}
}
