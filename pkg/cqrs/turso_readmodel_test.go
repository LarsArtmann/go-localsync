package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/storage"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func newTursoTestDB(t *testing.T) *TursoReadModel {
	t.Helper()

	db, err := storage.OpenTurso(":memory:")
	if err != nil {
		t.Fatalf("open turso: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	rm, err := NewTursoReadModel(db)
	if err != nil {
		t.Fatalf("NewTursoReadModel: %v", err)
	}

	return rm
}

func tursoTestItem(t *testing.T, source, extID, eventType, actor, repo string) *provider.Item {
	t.Helper()

	return &provider.Item{
		ID:             types.NewItemID(),
		ExternalID:     types.NewExternalID(extID),
		Source:         types.NewProviderID(source),
		Type:           types.NewEventTypeID(eventType),
		ActorLogin:     types.NewActorID(actor),
		ActorAvatarURL: "https://avatar.example.com/" + actor,
		RepoName:       types.NewRepoID(repo),
		RepoURL:        "https://github.com/" + repo,
		CreatedAt:      time.Now().Truncate(time.Microsecond),
		UpdatedAt:      time.Now().Truncate(time.Microsecond),
		RawJSON:        []byte(`{"test":true}`),
	}
}

func TestTursoReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	item := tursoTestItem(t, "github", "123", "PushEvent", "alice", "org/repo")

	err := rm.Upsert(ctx, item)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := rm.Get(ctx, "github", "123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got == nil {
		t.Fatal("Get returned nil")
	}

	if got.ID.String() != item.ID.String() {
		t.Errorf("ID = %q, want %q (ItemID not preserved)", got.ID.String(), item.ID.String())
	}

	if got.ExternalID.Get() != "123" {
		t.Errorf("ExternalID = %q, want 123", got.ExternalID.Get())
	}

	if got.Type.Get() != "PushEvent" {
		t.Errorf("Type = %q, want PushEvent", got.Type.Get())
	}
}

func TestTursoReadModel_Get_NotFound(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got != nil {
		t.Fatal("expected nil for missing item")
	}
}

func TestTursoReadModel_List(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	item1 := tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo")
	item2 := tursoTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo")

	_ = rm.Upsert(ctx, item1)
	_ = rm.Upsert(ctx, item2)

	items, err := rm.List(ctx, ItemFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestTursoReadModel_List_FilterByType(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo"))

	pushType := "PushEvent"
	items, err := rm.List(ctx, ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 PushEvent, got %d", len(items))
	}

	if items[0].Type.Get() != "PushEvent" {
		t.Errorf("Type = %q, want PushEvent", items[0].Type.Get())
	}
}

func TestTursoReadModel_Count(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo"))

	count, err := rm.Count(ctx, ItemFilter{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}

	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	pushType := "PushEvent"
	count, err = rm.Count(ctx, ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("Count filtered: %v", err)
	}

	if count != 1 {
		t.Errorf("filtered Count = %d, want 1", count)
	}
}

func TestTursoReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "3", "PushEvent", "charlie", "org/repo2"))

	types, err := rm.GetTypes(ctx)
	if err != nil {
		t.Fatalf("GetTypes: %v", err)
	}

	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d: %v", len(types), types)
	}
}

func TestTursoReadModel_Delete(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))

	err := rm.Delete(ctx, "github", "1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _ := rm.Get(ctx, "github", "1")
	if got != nil {
		t.Fatal("item should be nil after delete")
	}
}

func TestTursoReadModel_Upsert_Idempotent(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	item1 := tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo")
	_ = rm.Upsert(ctx, item1)

	item2 := tursoTestItem(t, "github", "1", "IssueEvent", "bob", "org/repo")
	_ = rm.Upsert(ctx, item2)

	got, _ := rm.Get(ctx, "github", "1")
	if got.Type.Get() != "IssueEvent" {
		t.Errorf("after second upsert, Type = %q, want IssueEvent", got.Type.Get())
	}

	count, _ := rm.Count(ctx, ItemFilter{})
	if count != 1 {
		t.Errorf("Count = %d, want 1 (upsert should overwrite, not duplicate)", count)
	}
}
