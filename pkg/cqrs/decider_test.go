package cqrs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateID_Deterministic(t *testing.T) {
	t.Parallel()

	a := AggregateID("github", "123")
	b := AggregateID("github", "123")

	assert.Equal(t, a, b, "same inputs must produce same AggregateID")
}

func TestAggregateID_DifferentInputs(t *testing.T) {
	t.Parallel()

	a := AggregateID("github", "123")
	b := AggregateID("github", "456")

	assert.NotEqual(t, a, b, "different inputs must produce different AggregateIDs")
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
	require.NoError(t, err)

	require.NotNil(t, state.Item)
	assert.Equal(t, "github", state.Item.Source.Get())
	assert.Equal(t, "123", state.Item.ExternalID.Get())
	assert.Equal(t, "PushEvent", state.Item.Type.Get())
	assert.False(t, state.Deleted)
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
	require.NoError(t, err)

	assert.Equal(t, "IssueEvent", state.Item.Type.Get())
}

func TestDecideSync_Fold_PreservesItemID(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")
	item.ID = types.NewItemID()
	originalID := item.ID.String()

	events, err := DecideSync(item)(InitialState, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)

	var payload ItemSyncedPayload
	require.NoError(t, json.Unmarshal(events[0].Payload(), &payload))
	assert.Equal(t, originalID, payload.ItemID, "ItemID must be serialized into the event payload")

	state, err := Fold(InitialState, events[0])
	require.NoError(t, err)
	require.NotNil(t, state.Item)
	assert.Equal(t, originalID, state.Item.ID.String(), "ItemID must survive Fold round-trip")
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
	require.NoError(t, err)

	assert.True(t, state.Deleted)
	assert.NotNil(t, state.Item, "deleted state still holds the item for potential resurrection")
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
	require.NoError(t, err)

	assert.Equal(t, "PushEvent", state.Item.Type.Get(), "conflict event does not change state")
}

func TestFold_UnknownEventType(t *testing.T) {
	t.Parallel()

	evt := mustNewTestEvent(event.Type("unknown"), map[string]string{"test": "data"})

	_, err := Fold(InitialState, evt)
	assert.Error(t, err)
}

func TestDecideSync_NewItem(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")

	events, err := DecideSync(item)(InitialState, 0)
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, EventItemSynced, events[0].Type())
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
	require.NoError(t, err)

	assert.Nil(t, events, "unchanged item produces no events")
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
	require.NoError(t, err)

	assert.Len(t, events, 2)
	assert.Equal(t, EventItemConflictFound, events[0].Type())
	assert.Equal(t, EventItemSynced, events[1].Type())
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
	require.NoError(t, err)
	require.Len(t, events, 2)

	var conflictPayload ItemConflictFoundPayload
	require.NoError(t, json.Unmarshal(events[0].Payload(), &conflictPayload))

	assert.NotEqual(t, conflictPayload.LocalUpdatedAt, conflictPayload.RemoteUpdatedAt,
		"LocalUpdatedAt and RemoteUpdatedAt must differ in conflict")
	assert.Equal(t, localTime.UnixNano(), conflictPayload.LocalUpdatedAt,
		"LocalUpdatedAt must come from existing state")
	assert.Equal(t, remoteTime.UnixNano(), conflictPayload.RemoteUpdatedAt,
		"RemoteUpdatedAt must come from incoming item")
}

func TestDecideSync_ResurrectDeletedItem(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")

	state := SyncItemState{
		Item:    &provider.Item{ExternalID: types.NewExternalID("123")},
		Deleted: true,
	}

	events, err := DecideSync(item)(state, 2)
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, EventItemSynced, events[0].Type())
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
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, EventItemDeleted, events[0].Type())
}

func TestDecideDelete_AlreadyDeleted(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Item:    &provider.Item{ExternalID: types.NewExternalID("123")},
		Deleted: true,
	}

	events, err := DecideDelete("github", "123")(state, 1)
	require.NoError(t, err)

	assert.Nil(t, events)
}

func TestDecideDelete_NewItem(t *testing.T) {
	t.Parallel()

	events, err := DecideDelete("github", "123")(InitialState, 0)
	require.NoError(t, err)

	assert.Nil(t, events)
}

func TestSyncItemState_IsNew(t *testing.T) {
	t.Parallel()

	assert.True(t, InitialState.IsNew())

	existing := SyncItemState{Item: &provider.Item{}}
	assert.False(t, existing.IsNew())
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
