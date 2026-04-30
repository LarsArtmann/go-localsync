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

// testItem creates a consistent test item for use across multiple tests.
func testItem() *provider.Item {
	now := time.Now()

	return &provider.Item{
		ID:         types.NewItemID("12345"),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID("PushEvent"),
		ActorLogin: types.NewActorID("testuser"),
		RepoName:   types.NewRepoID("test/repo"),
		CreatedAt:  now,
		UpdatedAt:  now,
		RawJSON:    json.RawMessage(`{"id":"12345","type":"PushEvent"}`),
	}
}

// assertItemCount verifies the item count matches expected value.
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

func TestSQLiteStorage(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewSQLiteStorage(db)
	ctx := context.Background()

	t.Run("Count initially returns 0", func(t *testing.T) {
		count, err := store.Count(ctx)
		if err != nil {
			t.Fatalf("Count failed: %v", err)
		}

		if count != 0 {
			t.Errorf("Expected 0 items, got %d", count)
		}
	})

	t.Run("GetLatest returns ErrNotFound for empty database", func(t *testing.T) {
		item, err := store.GetLatest(ctx)
		if err == nil {
			t.Fatal("Expected error, got nil")
		}

		if !errors.Is(err, pkgerrors.ErrNotFound) {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}

		if item != nil {
			t.Errorf("Expected nil item, got %+v", item)
		}
	})

	t.Run("Upsert inserts new item", func(t *testing.T) {
		err := store.Upsert(ctx, testItem())
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}

		assertItemCount(t, store, ctx, 1)
	})

	t.Run("Upsert is idempotent", func(t *testing.T) {
		err := store.Upsert(ctx, testItem())
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}

		assertItemCount(t, store, ctx, 1, "idempotent")
	})

	t.Run("GetLatest returns the latest item", func(t *testing.T) {
		item, err := store.GetLatest(ctx)
		if err != nil {
			t.Fatalf("GetLatest failed: %v", err)
		}

		if item == nil {
			t.Fatal("Expected item, got nil")
		}

		if item.ID.Get() != "12345" {
			t.Errorf("Expected ID 12345, got %s", item.ID.Get())
		}
	})

	t.Run("GetItems returns items", func(t *testing.T) {
		items, err := store.GetItems(ctx, 10, 0)
		if err != nil {
			t.Fatalf("GetItems failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("Expected 1 item, got %d", len(items))
		}
	})

	t.Run("GetItemsByType filters by type", func(t *testing.T) {
		items, err := store.GetItemsByType(ctx, "PushEvent", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsByType failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("Expected 1 PushEvent, got %d", len(items))
		}

		items, err = store.GetItemsByType(ctx, "PullRequestEvent", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsByType failed: %v", err)
		}

		if len(items) != 0 {
			t.Errorf("Expected 0 PullRequestEvent, got %d", len(items))
		}
	})

	t.Run("GetTypes returns distinct types", func(t *testing.T) {
		types, err := store.GetTypes(ctx)
		if err != nil {
			t.Fatalf("GetTypes failed: %v", err)
		}

		if len(types) != 1 || types[0] != "PushEvent" {
			t.Errorf("Expected [PushEvent], got %v", types)
		}
	})

	t.Run("CountByType counts by type", func(t *testing.T) {
		count, err := store.CountByType(ctx, "PushEvent")
		if err != nil {
			t.Fatalf("CountByType failed: %v", err)
		}

		if count != 1 {
			t.Errorf("Expected 1 PushEvent, got %d", count)
		}
	})

	t.Run("GetByID returns item when found", func(t *testing.T) {
		item, err := store.GetByID(ctx, types.NewItemID("12345"))
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		if item == nil {
			t.Fatal("Expected item, got nil")
		}

		if item.ID.Get() != "12345" {
			t.Errorf("Expected ID 12345, got %s", item.ID.Get())
		}
	})

	t.Run("GetByID returns ErrNotFound when not found", func(t *testing.T) {
		item, err := store.GetByID(ctx, types.NewItemID("nonexistent"))
		require.Error(t, err)
		assert.True(t, errors.Is(err, pkgerrors.ErrNotFound))
		assert.Nil(t, item)
	})

	t.Run("GetItemsByActor filters by actor login", func(t *testing.T) {
		items, err := store.GetItemsByActor(ctx, "testuser", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsByActor failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("Expected 1 item for testuser, got %d", len(items))
		}

		items, err = store.GetItemsByActor(ctx, "otheruser", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsByActor failed: %v", err)
		}

		if len(items) != 0 {
			t.Errorf("Expected 0 items for otheruser, got %d", len(items))
		}
	})

	t.Run("GetItemsByRepo filters by repo name", func(t *testing.T) {
		items, err := store.GetItemsByRepo(ctx, "test/repo", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsByRepo failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("Expected 1 item for test/repo, got %d", len(items))
		}

		items, err = store.GetItemsByRepo(ctx, "other/repo", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsByRepo failed: %v", err)
		}

		if len(items) != 0 {
			t.Errorf("Expected 0 items for other/repo, got %d", len(items))
		}
	})

	t.Run("GetItemsBySource filters by source", func(t *testing.T) {
		items, err := store.GetItemsBySource(ctx, "github", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsBySource failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("Expected 1 item for github, got %d", len(items))
		}

		items, err = store.GetItemsBySource(ctx, "gitlab", 10, 0)
		if err != nil {
			t.Fatalf("GetItemsBySource failed: %v", err)
		}

		if len(items) != 0 {
			t.Errorf("Expected 0 items for gitlab, got %d", len(items))
		}
	})

	t.Run("GetItemsSince returns items after timestamp", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		items, err := store.GetItemsSince(ctx, past)
		if err != nil {
			t.Fatalf("GetItemsSince failed: %v", err)
		}

		if len(items) != 1 {
			t.Errorf("Expected 1 item since past, got %d", len(items))
		}

		future := time.Now().Add(1 * time.Hour)
		items, err = store.GetItemsSince(ctx, future)
		if err != nil {
			t.Fatalf("GetItemsSince failed: %v", err)
		}

		if len(items) != 0 {
			t.Errorf("Expected 0 items since future, got %d", len(items))
		}
	})

	t.Run("Delete removes item", func(t *testing.T) {
		err := store.Delete(ctx, types.NewItemID("12345"))
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		assertItemCount(t, store, ctx, 0)

		item, err := store.GetByID(ctx, types.NewItemID("12345"))
		require.Error(t, err)
		assert.True(t, errors.Is(err, pkgerrors.ErrNotFound))
		assert.Nil(t, item)
	})

	t.Run("DeleteAll removes all items", func(t *testing.T) {
		err := store.Upsert(ctx, testItem())
		if err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}

		assertItemCount(t, store, ctx, 1)

		err = store.DeleteAll(ctx)
		if err != nil {
			t.Fatalf("DeleteAll failed: %v", err)
		}

		assertItemCount(t, store, ctx, 0)
	})
}

func TestSQLiteStorage_UpsertBatch(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	store := NewSQLiteStorage(db)
	ctx := context.Background()

	now := time.Now()
	items := []*provider.Item{
		{
			ID:         types.NewItemID("batch-1"),
			Source:     types.NewProviderID("github"),
			Type:       types.NewEventTypeID("PushEvent"),
			ActorLogin: types.NewActorID("user1"),
			RepoName:   types.NewRepoID("repo1"),
			CreatedAt:  now,
			UpdatedAt:  now,
			RawJSON:    json.RawMessage(`{"id":"batch-1"}`),
		},
		{
			ID:         types.NewItemID("batch-2"),
			Source:     types.NewProviderID("github"),
			Type:       types.NewEventTypeID("IssuesEvent"),
			ActorLogin: types.NewActorID("user2"),
			RepoName:   types.NewRepoID("repo2"),
			CreatedAt:  now,
			UpdatedAt:  now,
			RawJSON:    json.RawMessage(`{"id":"batch-2"}`),
		},
	}

	t.Run("inserts multiple items", func(t *testing.T) {
		err := store.UpsertBatch(ctx, items)
		if err != nil {
			t.Fatalf("UpsertBatch failed: %v", err)
		}

		assertItemCount(t, store, ctx, 2)

		item, err := store.GetByID(ctx, types.NewItemID("batch-1"))
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}

		if item == nil || item.ID.Get() != "batch-1" {
			t.Error("Expected batch-1 to exist")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		err := store.UpsertBatch(ctx, items)
		if err != nil {
			t.Fatalf("UpsertBatch failed: %v", err)
		}

		assertItemCount(t, store, ctx, 2)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		err := store.UpsertBatch(ctx, []*provider.Item{})
		if err != nil {
			t.Fatalf("UpsertBatch with empty slice failed: %v", err)
		}
	})

	t.Run("adds items after previous batch", func(t *testing.T) {
		badItems := []*provider.Item{
			{
				ID:         types.NewItemID("batch-ok"),
				Source:     types.NewProviderID("github"),
				Type:       types.NewEventTypeID("PushEvent"),
				ActorLogin: types.NewActorID("user1"),
				RepoName:   types.NewRepoID("repo1"),
				CreatedAt:  now,
				UpdatedAt:  now,
				RawJSON:    json.RawMessage(`{"id":"batch-ok"}`),
			},
		}

		err := store.UpsertBatch(ctx, badItems)
		if err != nil {
			t.Fatalf("UpsertBatch failed: %v", err)
		}

		assertItemCount(t, store, ctx, 3)
	})
}

func TestSQLiteStorage_BatchGetByIDs(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	defer func() { _ = db.Close() }()

	store := NewSQLiteStorage(db)
	ctx := context.Background()
	now := time.Now()

	items := []*provider.Item{
		{
			ID: types.NewItemID("batch-a"), Source: types.NewProviderID("github"),
			Type: types.NewEventTypeID("PushEvent"), ActorLogin: types.NewActorID("user1"),
			RepoName: types.NewRepoID("repo1"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-a"}`),
		},
		{
			ID: types.NewItemID("batch-b"), Source: types.NewProviderID("github"),
			Type: types.NewEventTypeID("IssuesEvent"), ActorLogin: types.NewActorID("user2"),
			RepoName: types.NewRepoID("repo2"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-b"}`),
		},
		{
			ID: types.NewItemID("batch-c"), Source: types.NewProviderID("github"),
			Type: types.NewEventTypeID("WatchEvent"), ActorLogin: types.NewActorID("user3"),
			RepoName: types.NewRepoID("repo3"), CreatedAt: now, UpdatedAt: now,
			RawJSON: json.RawMessage(`{"id":"batch-c"}`),
		},
	}

	err = store.UpsertBatch(ctx, items)
	if err != nil {
		t.Fatalf("UpsertBatch failed: %v", err)
	}

	t.Run("returns all existing items", func(t *testing.T) {
		ids := []types.ItemID{types.NewItemID("batch-a"), types.NewItemID("batch-c")}
		result, err := store.BatchGetByIDs(ctx, ids)
		require.NoError(t, err)
		require.Len(t, result, 2)

		gotIDs := make(map[string]bool)
		for _, item := range result {
			gotIDs[item.ID.Get()] = true
		}

		assert.True(t, gotIDs["batch-a"])
		assert.True(t, gotIDs["batch-c"])
	})

	t.Run("omits non-existent IDs", func(t *testing.T) {
		ids := []types.ItemID{types.NewItemID("batch-a"), types.NewItemID("nonexistent")}
		result, err := store.BatchGetByIDs(ctx, ids)
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "batch-a", result[0].ID.Get())
	})

	t.Run("returns empty for empty input", func(t *testing.T) {
		result, err := store.BatchGetByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns empty when no IDs match", func(t *testing.T) {
		ids := []types.ItemID{types.NewItemID("nope1"), types.NewItemID("nope2")}
		result, err := store.BatchGetByIDs(ctx, ids)
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
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
