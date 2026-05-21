package cqrs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/storage"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
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

	mustNoError(t, rm.Upsert(ctx, item))

	got, err := rm.Get(ctx, "github", "123")
	mustNoError(t, err)

	if got == nil {
		t.Fatal("Get returned nil")
	}

	if got.ID.String() != item.ID.String() {
		t.Errorf("ID = %q, want %q (ItemID not preserved)", got.ID.String(), item.ID.String())
	}

	assertExternalID(t, got, "123")

	assertItemType(t, got, "PushEvent")
}

func TestTursoReadModel_Get_NotFound(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", "nonexistent")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("Get: got %v, want ErrNotFound", err)
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
	mustNoError(t, err)

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

	pushType := types.NewEventTypeID("PushEvent")
	items, err := rm.List(ctx, ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 PushEvent, got %d", len(items))
	}

	assertItemType(t, items[0], "PushEvent")
}

func TestTursoReadModel_Count(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo"))

	count, err := rm.Count(ctx, ItemFilter{})
	mustNoError(t, err)

	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	pushType := types.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, ItemFilter{Type: &pushType})
	mustNoError(t, err)

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
	mustNoError(t, err)

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
	mustNoError(t, err)

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
	assertItemType(t, got, "IssueEvent")

	count, _ := rm.Count(ctx, ItemFilter{})
	if count != 1 {
		t.Errorf("Count = %d, want 1 (upsert should overwrite, not duplicate)", count)
	}
}

func TestTursoReadModel_List_FilterByActorLogin(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "2", "PushEvent", "bob", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "3", "IssueEvent", "alice", "org/repo"))

	actor := types.NewActorID("alice")
	items, err := rm.List(ctx, ItemFilter{ActorLogin: &actor})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items for alice, got %d", len(items))
	}

	count, err := rm.Count(ctx, ItemFilter{ActorLogin: &actor})
	mustNoError(t, err)
	if count != 2 {
		t.Errorf("expected count=2 for alice, got %d", count)
	}
}

func TestTursoReadModel_List_FilterByRepoName(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo-a"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "2", "PushEvent", "bob", "org/repo-b"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "3", "IssueEvent", "charlie", "org/repo-a"))

	repo := types.NewRepoID("org/repo-a")
	items, err := rm.List(ctx, ItemFilter{RepoName: &repo})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items for org/repo-a, got %d", len(items))
	}
}

func TestTursoReadModel_List_FilterBySource(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "gitlab", "2", "PushEvent", "bob", "org/repo"))

	source := types.NewProviderID("github")
	items, err := rm.List(ctx, ItemFilter{Source: &source})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 item for github source, got %d", len(items))
	}
	if items[0].Source.Get() != "github" {
		t.Errorf("Source = %q, want github", items[0].Source.Get())
	}
}

func TestTursoReadModel_List_FilterBySince(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	oldItem := tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo")
	oldItem.CreatedAt = time.Now().Add(-48 * time.Hour).Truncate(time.Microsecond)
	oldItem.UpdatedAt = oldItem.CreatedAt

	newItem := tursoTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo")
	newItem.CreatedAt = time.Now().Truncate(time.Microsecond)
	newItem.UpdatedAt = newItem.CreatedAt

	_ = rm.Upsert(ctx, oldItem)
	_ = rm.Upsert(ctx, newItem)

	since := time.Now().Add(-24 * time.Hour)
	items, err := rm.List(ctx, ItemFilter{Since: &since})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 item after Since cutoff, got %d", len(items))
	}
	assertExternalID(t, items[0], "2")

	count, err := rm.Count(ctx, ItemFilter{Since: &since})
	mustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after Since cutoff, got %d", count)
	}
}

func TestTursoReadModel_List_Pagination(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = rm.Upsert(
			ctx,
			tursoTestItem(t, "github", string(rune('A'+i)), "PushEvent", "alice", "org/repo"),
		)
	}

	items, err := rm.List(ctx, ItemFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items with Limit=2, got %d", len(items))
	}

	items, err = rm.List(ctx, ItemFilter{Limit: 2, Offset: 2})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items with Limit=2 Offset=2, got %d", len(items))
	}

	items, err = rm.List(ctx, ItemFilter{Limit: 2, Offset: 4})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 item with Limit=2 Offset=4, got %d", len(items))
	}
}

func TestTursoReadModel_List_FilterByTypeAndActorLogin(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "2", "PushEvent", "bob", "org/repo"))
	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "3", "IssueEvent", "alice", "org/repo"))

	pushType := types.NewEventTypeID("PushEvent")
	actor := types.NewActorID("alice")
	items, err := rm.List(ctx, ItemFilter{Type: &pushType, ActorLogin: &actor})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 PushEvent by alice, got %d", len(items))
	}
	assertExternalID(t, items[0], "1")
}

func TestTursoReadModel_List_ZeroResults(t *testing.T) {
	t.Parallel()

	rm := newTursoTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, tursoTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))

	pushType := types.NewEventTypeID("NonExistentType")
	items, err := rm.List(ctx, ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for non-existent type, got %d", len(items))
	}
}
