package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func TestSQLiteReadModel_List_FilterByActorLogin(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "2", "PushEvent", "bob", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "3", "IssueEvent", "alice", "org/repo"))

	actor := id.NewActorID("alice")
	items, err := rm.List(ctx, provider.ItemFilter{ActorLogin: &actor})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items for alice, got %d", len(items))
	}

	count, err := rm.Count(ctx, provider.ItemFilter{ActorLogin: &actor})
	mustNoError(t, err)
	if count != 2 {
		t.Errorf("expected count=2 for alice, got %d", count)
	}
}

func TestSQLiteReadModel_List_FilterByRepoName(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo-a"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "2", "PushEvent", "bob", "org/repo-b"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "3", "IssueEvent", "charlie", "org/repo-a"))

	repo := id.NewRepoID("org/repo-a")
	items, err := rm.List(ctx, provider.ItemFilter{RepoName: &repo})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items for org/repo-a, got %d", len(items))
	}
}

func TestSQLiteReadModel_List_FilterBySource(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "gitlab", "2", "PushEvent", "bob", "org/repo"))

	source := id.NewProviderID("github")
	items, err := rm.List(ctx, provider.ItemFilter{Source: &source})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 item for github source, got %d", len(items))
	}
	if items[0].Source.Get() != "github" {
		t.Errorf("Source = %q, want github", items[0].Source.Get())
	}
}

func TestSQLiteReadModel_List_FilterBySince(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	oldItem := sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo")
	oldItem.CreatedAt = time.Now().Add(-48 * time.Hour).Truncate(time.Microsecond)
	oldItem.UpdatedAt = oldItem.CreatedAt

	newItem := sqliteTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo")
	newItem.CreatedAt = time.Now().Truncate(time.Microsecond)
	newItem.UpdatedAt = newItem.CreatedAt

	_ = rm.Upsert(ctx, oldItem)
	_ = rm.Upsert(ctx, newItem)

	since := time.Now().Add(-24 * time.Hour)
	items, err := rm.List(ctx, provider.ItemFilter{Since: &since})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 item after Since cutoff, got %d", len(items))
	}
	assertExternalID(t, items[0], "2")

	count, err := rm.Count(ctx, provider.ItemFilter{Since: &since})
	mustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after Since cutoff, got %d", count)
	}
}

func TestSQLiteReadModel_List_Pagination(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	for i := range 5 {
		_ = rm.Upsert(
			ctx,
			sqliteTestItem(t, "github", string(rune('A'+i)), "PushEvent", "alice", "org/repo"),
		)
	}

	items, err := rm.List(ctx, provider.ItemFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items with Limit=2, got %d", len(items))
	}

	items, err = rm.List(ctx, provider.ItemFilter{Limit: 2, Offset: 2})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items with Limit=2 Offset=2, got %d", len(items))
	}

	items, err = rm.List(ctx, provider.ItemFilter{Limit: 2, Offset: 4})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 item with Limit=2 Offset=4, got %d", len(items))
	}
}

func TestSQLiteReadModel_List_FilterByTypeAndActorLogin(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "2", "PushEvent", "bob", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "3", "IssueEvent", "alice", "org/repo"))

	pushType := id.NewEventTypeID("PushEvent")
	actor := id.NewActorID("alice")
	items, err := rm.List(ctx, provider.ItemFilter{Type: &pushType, ActorLogin: &actor})
	mustNoError(t, err)
	if len(items) != 1 {
		t.Errorf("expected 1 PushEvent by alice, got %d", len(items))
	}
	assertExternalID(t, items[0], "1")
}

func TestSQLiteReadModel_List_ZeroResults(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))

	pushType := id.NewEventTypeID("NonExistentType")
	items, err := rm.List(ctx, provider.ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for non-existent type, got %d", len(items))
	}
}
