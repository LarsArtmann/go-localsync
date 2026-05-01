package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/internal/database"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testItem() *provider.Item {
	now := time.Now()

	return &provider.Item{
		ID:         types.NewItemID(),
		ExternalID: types.NewExternalID("12345"),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID("PushEvent"),
		ActorLogin: types.NewActorID("testuser"),
		RepoName:   types.NewRepoID("test/repo"),
		CreatedAt:  now,
		UpdatedAt:  now,
		RawJSON:    json.RawMessage(`{"id":"12345","type":"PushEvent"}`),
	}
}

func assertItemCount(
	t *testing.T,
	store Storage,
	ctx context.Context,
	expected int64,
	msgAndArgs ...any,
) {
	t.Helper()

	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != expected {
		if len(msgAndArgs) > 0 {
			t.Errorf("%v (expected %d items, got %d)", msgAndArgs[0], expected, count)
		} else {
			t.Errorf("Expected %d items, got %d", expected, count)
		}
	}
}

func newTestSQLiteStore(t *testing.T) (*SQLiteStorage, context.Context) {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return NewSQLiteStorage(db), context.Background()
}

func mustUpsert(t *testing.T, store Storage, ctx context.Context, item *provider.Item) {
	t.Helper()

	err := store.Upsert(ctx, item)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
}

func TestSQLiteStorage_EmptyStore(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)

	count, err := store.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	_, err = store.GetLatest(ctx)
	assert.True(t, errors.Is(err, pkgerrors.ErrNotFound))
}

func TestSQLiteStorage_UpsertAndRead(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)

	mustUpsert(t, store, ctx, testItem())
	assertItemCount(t, store, ctx, 1)

	item, err := store.GetLatest(ctx)
	require.NoError(t, err)
	assert.Equal(t, "12345", item.ExternalID.Get())

	items, err := store.GetItems(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestSQLiteStorage_UpsertIdempotent(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)

	mustUpsert(t, store, ctx, testItem())
	mustUpsert(t, store, ctx, testItem())
	assertItemCount(t, store, ctx, 1, "idempotent")
}

func TestSQLiteStorage_GetItemsByType(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	items, err := store.GetItemsByType(ctx, "PushEvent", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	items, err = store.GetItemsByType(ctx, "PullRequestEvent", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestSQLiteStorage_GetTypes(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	typeList, err := store.GetTypes(ctx)
	require.NoError(t, err)
	require.Len(t, typeList, 1)
	assert.Equal(t, "PushEvent", typeList[0])
}

func TestSQLiteStorage_CountByType(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	count, err := store.CountByType(ctx, "PushEvent")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestSQLiteStorage_GetByExternalID(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	item, err := store.GetByExternalID(ctx, types.NewExternalID("12345"))
	require.NoError(t, err)
	assert.Equal(t, "12345", item.ExternalID.Get())

	_, err = store.GetByExternalID(ctx, types.NewExternalID("nonexistent"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, pkgerrors.ErrNotFound))
}

func TestSQLiteStorage_GetItemsByActor(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	items, err := store.GetItemsByActor(ctx, "testuser", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	items, err = store.GetItemsByActor(ctx, "otheruser", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestSQLiteStorage_GetItemsByRepo(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	items, err := store.GetItemsByRepo(ctx, "test/repo", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	items, err = store.GetItemsByRepo(ctx, "other/repo", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestSQLiteStorage_GetItemsBySource(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	items, err := store.GetItemsBySource(ctx, "github", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	items, err = store.GetItemsBySource(ctx, "gitlab", 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestSQLiteStorage_GetItemsSince(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	past := time.Now().Add(-1 * time.Hour)
	items, err := store.GetItemsSince(ctx, past)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	future := time.Now().Add(1 * time.Hour)
	items, err = store.GetItemsSince(ctx, future)
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

func TestSQLiteStorage_DeleteByExternalID(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())

	err := store.DeleteByExternalID(ctx, types.NewExternalID("12345"))
	require.NoError(t, err)
	assertItemCount(t, store, ctx, 0)

	_, err = store.GetByExternalID(ctx, types.NewExternalID("12345"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, pkgerrors.ErrNotFound))
}

func TestSQLiteStorage_DeleteAll(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	mustUpsert(t, store, ctx, testItem())
	assertItemCount(t, store, ctx, 1)

	err := store.DeleteAll(ctx)
	require.NoError(t, err)
	assertItemCount(t, store, ctx, 0)
}

func TestSQLiteStorage_UpsertBatch(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	now := time.Now()

	items := []*provider.Item{
		{
			ID: types.NewItemID(), ExternalID: types.NewExternalID("batch-1"),
			Source: types.NewProviderID("github"),
			Type:   types.NewEventTypeID("PushEvent"), ActorLogin: types.NewActorID("user1"),
			RepoName: types.NewRepoID("repo1"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-1"}`),
		},
		{
			ID: types.NewItemID(), ExternalID: types.NewExternalID("batch-2"),
			Source: types.NewProviderID("github"),
			Type:   types.NewEventTypeID("IssuesEvent"), ActorLogin: types.NewActorID("user2"),
			RepoName: types.NewRepoID("repo2"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-2"}`),
		},
	}

	err := store.UpsertBatch(ctx, items)
	require.NoError(t, err)
	assertItemCount(t, store, ctx, 2)

	item, err := store.GetByExternalID(ctx, types.NewExternalID("batch-1"))
	require.NoError(t, err)
	assert.Equal(t, "batch-1", item.ExternalID.Get())

	err = store.UpsertBatch(ctx, items)
	require.NoError(t, err)
	assertItemCount(t, store, ctx, 2, "idempotent")

	err = store.UpsertBatch(ctx, []*provider.Item{})
	require.NoError(t, err, "empty slice should be no-op")
}

func TestSQLiteStorage_BatchGetByExternalIDs(t *testing.T) {
	store, ctx := newTestSQLiteStore(t)
	now := time.Now()

	items := []*provider.Item{
		{
			ID: types.NewItemID(), ExternalID: types.NewExternalID("batch-a"),
			Source: types.NewProviderID("github"),
			Type:   types.NewEventTypeID("PushEvent"), ActorLogin: types.NewActorID("user1"),
			RepoName: types.NewRepoID("repo1"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-a"}`),
		},
		{
			ID: types.NewItemID(), ExternalID: types.NewExternalID("batch-b"),
			Source: types.NewProviderID("github"),
			Type:   types.NewEventTypeID("IssuesEvent"), ActorLogin: types.NewActorID("user2"),
			RepoName: types.NewRepoID("repo2"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-b"}`),
		},
		{
			ID: types.NewItemID(), ExternalID: types.NewExternalID("batch-c"),
			Source: types.NewProviderID("github"),
			Type:   types.NewEventTypeID("WatchEvent"), ActorLogin: types.NewActorID("user3"),
			RepoName: types.NewRepoID("repo3"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-c"}`),
		},
	}

	err := store.UpsertBatch(ctx, items)
	require.NoError(t, err)

	t.Run("returns all existing items", func(t *testing.T) {
		ids := []types.ExternalID{types.NewExternalID("batch-a"), types.NewExternalID("batch-c")}
		result, err := store.BatchGetByExternalIDs(ctx, ids)
		require.NoError(t, err)
		require.Len(t, result, 2)

		gotIDs := make(map[string]bool)
		for _, item := range result {
			gotIDs[item.ExternalID.Get()] = true
		}

		assert.True(t, gotIDs["batch-a"])
		assert.True(t, gotIDs["batch-c"])
	})

	t.Run("omits non-existent IDs", func(t *testing.T) {
		ids := []types.ExternalID{
			types.NewExternalID("batch-a"),
			types.NewExternalID("nonexistent"),
		}
		result, err := store.BatchGetByExternalIDs(ctx, ids)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "batch-a", result[0].ExternalID.Get())
	})

	t.Run("returns empty for empty input", func(t *testing.T) {
		result, err := store.BatchGetByExternalIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns empty when no IDs match", func(t *testing.T) {
		ids := []types.ExternalID{types.NewExternalID("nope1"), types.NewExternalID("nope2")}
		result, err := store.BatchGetByExternalIDs(ctx, ids)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestSQLiteStorage_Close(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	store := NewSQLiteStorage(db)

	err = store.Close()
	require.NoError(t, err)
}
