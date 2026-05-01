package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMemoryItem(id, eventType, actor, repo string, createdAt time.Time) *provider.Item {
	return &provider.Item{
		ID:         types.NewItemID(),
		ExternalID: types.NewExternalID(id),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID(eventType),
		ActorLogin: types.NewActorID(actor),
		RepoName:   types.NewRepoID(repo),
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
		RawJSON:    json.RawMessage(`{"id":"` + id + `"}`),
	}
}

func TestMemoryStorage_UpsertAndGetByExternalID(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	item := testMemoryItem("1", "PushEvent", "alice", "repo1", time.Now())

	err := store.Upsert(ctx, item)
	require.NoError(t, err)

	got, err := store.GetByExternalID(ctx, types.NewExternalID("1"))
	require.NoError(t, err)
	assert.Equal(t, "1", got.ExternalID.Get())
}

func TestMemoryStorage_GetByExternalIDNotFound(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	_, err := store.GetByExternalID(ctx, types.NewExternalID("nonexistent"))
	assert.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestMemoryStorage_UpsertIsIdempotent(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	item := testMemoryItem("1", "PushEvent", "alice", "repo1", time.Now())

	require.NoError(t, store.Upsert(ctx, item))
	require.NoError(t, store.Upsert(ctx, item))

	count, _ := store.Count(ctx)
	assert.Equal(t, int64(1), count)
}

func TestMemoryStorage_UpsertBatch(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	items := []*provider.Item{
		testMemoryItem("b1", "PushEvent", "alice", "repo1", now),
		testMemoryItem("b2", "IssuesEvent", "bob", "repo2", now),
	}

	err := store.UpsertBatch(ctx, items)
	require.NoError(t, err)

	count, _ := store.Count(ctx)
	assert.Equal(t, int64(2), count)
}

func TestMemoryStorage_UpsertBatchEmpty(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	err := store.UpsertBatch(ctx, []*provider.Item{})
	require.NoError(t, err)
}

func TestMemoryStorage_GetLatest(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(
		t,
		store.Upsert(ctx, testMemoryItem("old", "PushEvent", "a", "r", now.Add(-1*time.Hour))),
	)
	require.NoError(t, store.Upsert(ctx, testMemoryItem("new", "PushEvent", "a", "r", now)))

	latest, err := store.GetLatest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "new", latest.ExternalID.Get())
}

func TestMemoryStorage_GetLatestEmpty(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	_, err := store.GetLatest(ctx)
	assert.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestMemoryStorage_GetItems(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	for i := range 5 {
		require.NoError(
			t,
			store.Upsert(
				ctx,
				testMemoryItem(
					string(rune('A'+i)),
					"PushEvent",
					"user",
					"repo",
					now.Add(time.Duration(i)*time.Minute),
				),
			),
		)
	}

	items, err := store.GetItems(ctx, 3, 0)
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestMemoryStorage_GetItemsByType(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "IssuesEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("3", "PushEvent", "a", "r", now)))

	items, err := store.GetItemsByType(ctx, "PushEvent", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestMemoryStorage_GetItemsByActor(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "alice", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "PushEvent", "bob", "r", now)))

	items, err := store.GetItemsByActor(ctx, "alice", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "alice", items[0].ActorLogin.Get())
}

func TestMemoryStorage_GetItemsByRepo(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(
		t,
		store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "owner/repo-a", now)),
	)
	require.NoError(
		t,
		store.Upsert(ctx, testMemoryItem("2", "PushEvent", "a", "owner/repo-b", now)),
	)

	items, err := store.GetItemsByRepo(ctx, "owner/repo-a", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestMemoryStorage_GetItemsBySource(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))

	items, err := store.GetItemsBySource(ctx, "github", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestMemoryStorage_GetItemsSince(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(
		t,
		store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now.Add(-2*time.Hour))),
	)
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "PushEvent", "a", "r", now)))

	items, err := store.GetItemsSince(ctx, now.Add(-1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestMemoryStorage_BatchGetByExternalIDs(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "PushEvent", "a", "r", now)))

	items, err := store.BatchGetByExternalIDs(
		ctx,
		[]types.ExternalID{types.NewExternalID("1"), types.NewExternalID("2")},
	)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestMemoryStorage_BatchGetByExternalIDsMissing(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))

	items, err := store.BatchGetByExternalIDs(
		ctx,
		[]types.ExternalID{types.NewExternalID("1"), types.NewExternalID("nonexistent")},
	)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestMemoryStorage_DeleteByExternalID(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))

	err := store.DeleteByExternalID(ctx, types.NewExternalID("1"))
	require.NoError(t, err)

	_, err = store.GetByExternalID(ctx, types.NewExternalID("1"))
	assert.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestMemoryStorage_DeleteNotFound(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()

	err := store.DeleteByExternalID(ctx, types.NewExternalID("nonexistent"))
	assert.NoError(t, err)
}

func TestMemoryStorage_DeleteAll(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "PushEvent", "a", "r", now)))

	err := store.DeleteAll(ctx)
	require.NoError(t, err)

	count, _ := store.Count(ctx)
	assert.Equal(t, int64(0), count)
}

func TestMemoryStorage_Count(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "IssuesEvent", "a", "r", now)))

	count, err := store.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestMemoryStorage_CountByType(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "PushEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("3", "IssuesEvent", "a", "r", now)))

	count, err := store.CountByType(ctx, "PushEvent")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestMemoryStorage_GetTypes(t *testing.T) {
	store := NewMemoryStorage()
	ctx := context.Background()
	now := time.Now()

	require.NoError(t, store.Upsert(ctx, testMemoryItem("1", "PushEvent", "a", "r", now)))
	require.NoError(t, store.Upsert(ctx, testMemoryItem("2", "IssuesEvent", "a", "r", now)))

	types, err := store.GetTypes(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"PushEvent", "IssuesEvent"}, types)
}

func TestMemoryStorage_Close(t *testing.T) {
	store := NewMemoryStorage()
	assert.NoError(t, store.Close())
}
