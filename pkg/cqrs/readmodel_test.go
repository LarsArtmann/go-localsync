package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	state := &itemState{
		Source:   "github",
		SourceID: "123",
		Type:     "PushEvent",
	}

	err := rm.Upsert(ctx, state)
	require.NoError(t, err)

	got, err := rm.Get(ctx, "github", "123")
	require.NoError(t, err)

	assert.Equal(t, "PushEvent", got.Type)
}

func TestMemoryReadModel_GetNotFound(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", "nonexistent")
	require.NoError(t, err)

	assert.Nil(t, got)
}

func TestMemoryReadModel_Delete(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	state := &itemState{Source: "github", SourceID: "123"}
	require.NoError(t, rm.Upsert(ctx, state))

	require.NoError(t, rm.Delete(ctx, "github", "123"))

	got, err := rm.Get(ctx, "github", "123")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemoryReadModel_ListWithFilters(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	pushType := "PushEvent"
	issueType := "IssueEvent"
	github := "github"

	require.NoError(t, rm.Upsert(ctx, &itemState{
		Source: "github", SourceID: "1", Type: pushType,
		ActorLogin: "alice", RepoName: "org/repo1", CreatedAt: time.Now().Add(-2 * time.Hour),
	}))
	require.NoError(t, rm.Upsert(ctx, &itemState{
		Source: "github", SourceID: "2", Type: issueType,
		ActorLogin: "bob", RepoName: "org/repo2", CreatedAt: time.Now().Add(-1 * time.Hour),
	}))
	require.NoError(t, rm.Upsert(ctx, &itemState{
		Source: "gitlab", SourceID: "3", Type: pushType,
		ActorLogin: "alice", RepoName: "org/repo3", CreatedAt: time.Now(),
	}))

	// Filter by type
	items, err := rm.List(ctx, ItemFilter{Type: &pushType})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Filter by source
	items, err = rm.List(ctx, ItemFilter{Source: &github})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Filter by actor
	actor := "alice"
	items, err = rm.List(ctx, ItemFilter{ActorLogin: &actor})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Pagination
	items, err = rm.List(ctx, ItemFilter{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	items, err = rm.List(ctx, ItemFilter{Offset: 10})
	require.NoError(t, err)
	assert.Nil(t, items)
}

func TestMemoryReadModel_Count(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	require.NoError(t, rm.Upsert(ctx, &itemState{Source: "github", SourceID: "1", Type: "PushEvent"}))
	require.NoError(t, rm.Upsert(ctx, &itemState{Source: "github", SourceID: "2", Type: "IssueEvent"}))

	count, err := rm.Count(ctx, ItemFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	pushType := "PushEvent"
	count, err = rm.Count(ctx, ItemFilter{Type: &pushType})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestMemoryReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	require.NoError(t, rm.Upsert(ctx, &itemState{Source: "github", SourceID: "1", Type: "PushEvent"}))
	require.NoError(t, rm.Upsert(ctx, &itemState{Source: "github", SourceID: "2", Type: "IssueEvent"}))
	require.NoError(t, rm.Upsert(ctx, &itemState{Source: "github", SourceID: "3", Type: "PushEvent"}))

	types, err := rm.GetTypes(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"IssueEvent", "PushEvent"}, types)
}

func TestProjector_ItemSynced(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := NewProjector(rm)

	payload := ItemSyncedPayload{
		Source:    "github",
		SourceID:  "123",
		Type:      "PushEvent",
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	evt := mustNewTestEvent(EventItemSynced, payload)

	err := proj.HandleEvent(context.Background(), evt)
	require.NoError(t, err)

	assert.Equal(t, 1, rm.Len())

	got, err := rm.Get(context.Background(), "github", "123")
	require.NoError(t, err)
	assert.Equal(t, "PushEvent", got.Type)
}

func TestProjector_ItemDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()
	require.NoError(t, rm.Upsert(ctx, &itemState{Source: "github", SourceID: "123", Type: "PushEvent"}))

	proj := NewProjector(rm)

	payload := ItemDeletedPayload{Source: "github", SourceID: "123"}
	evt := mustNewTestEvent(EventItemDeleted, payload)

	err := proj.HandleEvent(ctx, evt)
	require.NoError(t, err)

	assert.Equal(t, 0, rm.Len())
}

func TestProjector_ItemConflictFound_NoStateChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()
	require.NoError(t, rm.Upsert(ctx, &itemState{Source: "github", SourceID: "123", Type: "PushEvent"}))

	proj := NewProjector(rm)

	payload := ItemConflictFoundPayload{Source: "github", SourceID: "123", Winner: "remote"}
	evt := mustNewTestEvent(EventItemConflictFound, payload)

	err := proj.HandleEvent(ctx, evt)
	require.NoError(t, err)

	assert.Equal(t, 1, rm.Len())
}

func TestReadModel_Integration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()
	proj := NewProjector(rm)

	now := time.Now()
	item := testItem("123", "PushEvent")
	item.UpdatedAt = now

	// Simulate: sync a new item → events → project → query
	decide := DecideSync(item)
	events, err := decide(InitialSyncItemState, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)

	for _, evt := range events {
		err = proj.HandleEvent(ctx, evt)
		require.NoError(t, err)
	}

	got, err := rm.Get(ctx, "github", "123")
	require.NoError(t, err)
	assert.Equal(t, "PushEvent", got.Type)
	assert.Equal(t, "testuser", got.ActorLogin)

	count, err := rm.Count(ctx, ItemFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
