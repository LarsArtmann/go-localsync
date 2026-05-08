package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &provider.Item{
		ExternalID: types.NewExternalID("123"),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID("PushEvent"),
	}

	err := rm.Upsert(ctx, item)
	require.NoError(t, err)

	got, err := rm.Get(ctx, "github", "123")
	require.NoError(t, err)

	require.NotNil(t, got)
	assert.Equal(t, "PushEvent", got.Type.Get())
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

	item := &provider.Item{
		ExternalID: types.NewExternalID("123"),
		Source:     types.NewProviderID("github"),
	}

	require.NoError(t, rm.Upsert(ctx, item))
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

	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("1"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID(pushType), ActorLogin: types.NewActorID("alice"),
		RepoName: types.NewRepoID("org/repo1"), CreatedAt: time.Now().Add(-2 * time.Hour),
	}))
	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("2"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID(issueType), ActorLogin: types.NewActorID("bob"),
		RepoName: types.NewRepoID("org/repo2"), CreatedAt: time.Now().Add(-time.Hour),
	}))
	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("3"), Source: types.NewProviderID("gitlab"),
		Type: types.NewEventTypeID(pushType), ActorLogin: types.NewActorID("alice"),
		RepoName: types.NewRepoID("org/repo3"), CreatedAt: time.Now(),
	}))

	pushTypeFilter := types.NewEventTypeID(pushType)
	items, err := rm.List(ctx, ItemFilter{Type: &pushTypeFilter})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	sourceFilter := types.NewProviderID(github)
	items, err = rm.List(ctx, ItemFilter{Source: &sourceFilter})
	require.NoError(t, err)
	assert.Len(t, items, 2)

	actorFilter := types.NewActorID("alice")
	items, err = rm.List(ctx, ItemFilter{ActorLogin: &actorFilter})
	require.NoError(t, err)
	assert.Len(t, items, 2)

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

	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("1"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}))
	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("2"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("IssueEvent"),
	}))

	count, err := rm.Count(ctx, ItemFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	pushTypeFilter := types.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, ItemFilter{Type: &pushTypeFilter})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestMemoryReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("1"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}))
	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("2"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("IssueEvent"),
	}))
	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("3"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}))

	result, err := rm.GetTypes(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"IssueEvent", "PushEvent"}, result)
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
	assert.Equal(t, "PushEvent", got.Type.Get())
}

func TestProjector_ItemDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("123"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}))

	proj := NewProjector(rm)

	evt := mustNewTestEvent(EventItemDeleted, ItemDeletedPayload{Source: "github", SourceID: "123"})

	err := proj.HandleEvent(ctx, evt)
	require.NoError(t, err)

	assert.Equal(t, 0, rm.Len())
}

func TestProjector_ItemConflictFound_NoStateChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	require.NoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("123"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}))

	proj := NewProjector(rm)

	evt := mustNewTestEvent(EventItemConflictFound, ItemConflictFoundPayload{
		Source: "github", SourceID: "123", Winner: "remote",
	})

	err := proj.HandleEvent(ctx, evt)
	require.NoError(t, err)

	assert.Equal(t, 1, rm.Len())
}

func TestReadModel_Integration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()
	proj := NewProjector(rm)

	item := testItem("123", "PushEvent")

	decide := DecideSync(item)
	events, err := decide(InitialState, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)

	for _, evt := range events {
		err = proj.HandleEvent(ctx, evt)
		require.NoError(t, err)
	}

	got, err := rm.Get(ctx, "github", "123")
	require.NoError(t, err)
	assert.Equal(t, "PushEvent", got.Type.Get())
	assert.Equal(t, "testuser", got.ActorLogin.Get())

	count, err := rm.Count(ctx, ItemFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
