package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/internal/database"
)

func TestSQLiteStorage(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	store := NewSQLiteStorage(db)
	ctx := context.Background()

	t.Run("CountEvents initially returns 0", func(t *testing.T) {
		count, err := store.CountEvents(ctx)
		if err != nil {
			t.Fatalf("CountEvents failed: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 events, got %d", count)
		}
	})

	t.Run("GetLatestEvent returns nil for empty database", func(t *testing.T) {
		event, err := store.GetLatestEvent(ctx)
		if err != nil {
			t.Fatalf("GetLatestEvent failed: %v", err)
		}
		if event != nil {
			t.Errorf("Expected nil event, got %+v", event)
		}
	})

	t.Run("UpsertEvent inserts new event", func(t *testing.T) {
		event := &Event{
			GithubID:   "12345",
			Type:       "PushEvent",
			ActorLogin: "testuser",
			RepoName:   "test/repo",
			CreatedAt:  time.Now(),
			RawJSON:    json.RawMessage(`{"id":"12345","type":"PushEvent"}`),
		}

		if err := store.UpsertEvent(ctx, event); err != nil {
			t.Fatalf("UpsertEvent failed: %v", err)
		}

		count, err := store.CountEvents(ctx)
		if err != nil {
			t.Fatalf("CountEvents failed: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 event, got %d", count)
		}
	})

	t.Run("UpsertEvent is idempotent", func(t *testing.T) {
		event := &Event{
			GithubID:   "12345",
			Type:       "PushEvent",
			ActorLogin: "testuser",
			RepoName:   "test/repo",
			CreatedAt:  time.Now(),
			RawJSON:    json.RawMessage(`{"id":"12345","type":"PushEvent"}`),
		}

		if err := store.UpsertEvent(ctx, event); err != nil {
			t.Fatalf("UpsertEvent failed: %v", err)
		}

		count, err := store.CountEvents(ctx)
		if err != nil {
			t.Fatalf("CountEvents failed: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 event (idempotent), got %d", count)
		}
	})

	t.Run("GetLatestEvent returns the latest event", func(t *testing.T) {
		event, err := store.GetLatestEvent(ctx)
		if err != nil {
			t.Fatalf("GetLatestEvent failed: %v", err)
		}
		if event == nil {
			t.Fatal("Expected event, got nil")
		}
		if event.GithubID != "12345" {
			t.Errorf("Expected GithubID 12345, got %s", event.GithubID)
		}
	})

	t.Run("GetEvents returns events", func(t *testing.T) {
		events, err := store.GetEvents(ctx, 10, 0)
		if err != nil {
			t.Fatalf("GetEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
	})

	t.Run("GetEventsByType filters by type", func(t *testing.T) {
		events, err := store.GetEventsByType(ctx, "PushEvent", 10, 0)
		if err != nil {
			t.Fatalf("GetEventsByType failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 PushEvent, got %d", len(events))
		}

		events, err = store.GetEventsByType(ctx, "PullRequestEvent", 10, 0)
		if err != nil {
			t.Fatalf("GetEventsByType failed: %v", err)
		}
		if len(events) != 0 {
			t.Errorf("Expected 0 PullRequestEvent, got %d", len(events))
		}
	})

	t.Run("GetEventTypes returns distinct types", func(t *testing.T) {
		types, err := store.GetEventTypes(ctx)
		if err != nil {
			t.Fatalf("GetEventTypes failed: %v", err)
		}
		if len(types) != 1 || types[0] != "PushEvent" {
			t.Errorf("Expected [PushEvent], got %v", types)
		}
	})

	t.Run("CountEventsByType counts by type", func(t *testing.T) {
		count, err := store.CountEventsByType(ctx, "PushEvent")
		if err != nil {
			t.Fatalf("CountEventsByType failed: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 PushEvent, got %d", count)
		}
	})
}
