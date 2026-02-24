package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/internal/database"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// testItem creates a consistent test item for use across multiple tests.
func testItem() *provider.Item {
	return &provider.Item{
		ID:         "12345",
		Source:     "github",
		Type:       "PushEvent",
		ActorLogin: "testuser",
		RepoName:   "test/repo",
		CreatedAt:  time.Now(),
		RawJSON:    json.RawMessage(`{"id":"12345","type":"PushEvent"}`),
	}
}

// assertItemCount verifies the item count matches expected value.
func assertItemCount(t *testing.T, store Storage, ctx context.Context, expected int64, msgAndArgs ...interface{}) {
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
	defer db.Close()

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

	t.Run("GetLatest returns nil for empty database", func(t *testing.T) {
		item, err := store.GetLatest(ctx)
		if err != nil {
			t.Fatalf("GetLatest failed: %v", err)
		}
		if item != nil {
			t.Errorf("Expected nil item, got %+v", item)
		}
	})

	t.Run("Upsert inserts new item", func(t *testing.T) {
		if err := store.Upsert(ctx, testItem()); err != nil {
			t.Fatalf("Upsert failed: %v", err)
		}
		assertItemCount(t, store, ctx, 1)
	})

	t.Run("Upsert is idempotent", func(t *testing.T) {
		if err := store.Upsert(ctx, testItem()); err != nil {
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
		if item.ID != "12345" {
			t.Errorf("Expected ID 12345, got %s", item.ID)
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
}
