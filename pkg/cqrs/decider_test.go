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

func TestFold_ItemSynced(t *testing.T) {
	t.Parallel()

	state := InitialSyncItemState
	payload := ItemSyncedPayload{
		Source:    "github",
		SourceID:  "123",
		Type:      "PushEvent",
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	evt := mustNewTestEvent(EventItemSynced, payload)

	newState, err := fold(state, evt)
	require.NoError(t, err)

	assert.Equal(t, "github", newState.Source)
	assert.Equal(t, "123", newState.SourceID)
	assert.Equal(t, "PushEvent", newState.Type)
	assert.False(t, newState.Deleted)
}

func TestFold_ItemSyncedOverwritesState(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Source:   "github",
		SourceID: "123",
		Type:     "PushEvent",
	}

	updatedPayload := ItemSyncedPayload{
		Source:    "github",
		SourceID:  "123",
		Type:      "IssueEvent",
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	evt := mustNewTestEvent(EventItemSynced, updatedPayload)

	newState, err := fold(state, evt)
	require.NoError(t, err)

	assert.Equal(t, "IssueEvent", newState.Type)
}

func TestFold_ItemDeleted(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Source:   "github",
		SourceID: "123",
	}

	payload := ItemDeletedPayload{Source: "github", SourceID: "123"}
	evt := mustNewTestEvent(EventItemDeleted, payload)

	newState, err := fold(state, evt)
	require.NoError(t, err)

	assert.True(t, newState.Deleted)
}

func TestFold_ItemConflictFound(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Source:   "github",
		SourceID: "123",
		Type:     "PushEvent",
	}

	payload := ItemConflictFoundPayload{
		Source:          "github",
		SourceID:        "123",
		LocalUpdatedAt:  100,
		RemoteUpdatedAt: 200,
		Winner:          "remote",
	}

	evt := mustNewTestEvent(EventItemConflictFound, payload)

	newState, err := fold(state, evt)
	require.NoError(t, err)

	assert.Equal(t, "PushEvent", newState.Type)
}

func TestFold_UnknownEventType(t *testing.T) {
	t.Parallel()

	state := InitialSyncItemState
	evt := mustNewTestEvent(event.Type("unknown"), map[string]string{"test": "data"})

	_, err := fold(state, evt)
	assert.Error(t, err)
}

func TestDecideSync_NewItem(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")
	decide := DecideSync(item)

	events, err := decide(InitialSyncItemState, 0)
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
		Source:       "github",
		SourceID:     "123",
		Type:         "PushEvent",
		ActorLogin:   "testuser",
		RepoName:     "owner/repo",
		UpdatedAt:    now,
	}

	decide := DecideSync(item)
	events, err := decide(state, 1)
	require.NoError(t, err)

	assert.Nil(t, events)
}

func TestDecideSync_ConflictResolution(t *testing.T) {
	t.Parallel()

	item := testItem("123", "PushEvent")
	item.UpdatedAt = time.Now().Add(time.Hour)

	state := SyncItemState{
		Source:    "github",
		SourceID:  "123",
		Type:      "PushEvent",
		UpdatedAt: time.Now(),
	}

	decide := DecideSync(item)
	events, err := decide(state, 1)
	require.NoError(t, err)

	assert.Len(t, events, 2)
	assert.Equal(t, EventItemConflictFound, events[0].Type())
	assert.Equal(t, EventItemSynced, events[1].Type())
}

func TestDecideDelete_ActiveItem(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Source:   "github",
		SourceID: "123",
	}

	decide := DecideDelete()
	events, err := decide(state, 1)
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, EventItemDeleted, events[0].Type())
}

func TestDecideDelete_AlreadyDeleted(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Source:   "github",
		SourceID: "123",
		Deleted:  true,
	}

	decide := DecideDelete()
	events, err := decide(state, 1)
	require.NoError(t, err)

	assert.Nil(t, events)
}

func TestDecideDelete_NewItem(t *testing.T) {
	t.Parallel()

	decide := DecideDelete()
	events, err := decide(InitialSyncItemState, 0)
	require.NoError(t, err)

	assert.Nil(t, events)
}

func TestSyncItemState_IsNew(t *testing.T) {
	t.Parallel()

	assert.True(t, InitialSyncItemState.IsNew())

	existing := SyncItemState{SourceID: "123"}
	assert.False(t, existing.IsNew())
}

func TestSyncItemState_ToItem(t *testing.T) {
	t.Parallel()

	state := SyncItemState{
		Source:     "github",
		SourceID:   "123",
		Type:       "PushEvent",
		ActorLogin: "testuser",
		CreatedAt:  time.Now(),
	}

	item := state.ToItem()
	assert.Equal(t, "123", item.ExternalID.Get())
	assert.Equal(t, "github", item.Source.Get())
	assert.Equal(t, "PushEvent", item.Type.Get())
	assert.Equal(t, "testuser", item.ActorLogin.Get())
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
