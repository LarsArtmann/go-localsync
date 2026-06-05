package cqrs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/storage/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
	_ "modernc.org/sqlite"
)

func newSQLiteTestDB(t *testing.T) *SQLiteReadModel {
	t.Helper()

	db, err := storage.OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	rm, err := NewSQLiteReadModel(db)
	if err != nil {
		t.Fatalf("NewSQLiteReadModel: %v", err)
	}

	return rm
}

func sqliteTestItem(t *testing.T, source, extID, eventType, actor, repo string) *model.Item {
	t.Helper()

	return &model.Item{
		ID:             id.NewItemID(),
		ExternalID:     id.NewExternalID(extID),
		Source:         id.NewProviderID(source),
		Type:           id.NewEventTypeID(eventType),
		ActorLogin:     id.NewActorID(actor),
		ActorAvatarURL: "https://avatar.example.com/" + actor,
		RepoName:       id.NewRepoID(repo),
		RepoURL:        "https://github.com/" + repo,
		CreatedAt:      time.Now().Truncate(time.Microsecond),
		UpdatedAt:      time.Now().Truncate(time.Microsecond),
	}
}

func TestSQLiteReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	item := sqliteTestItem(t, "github", "123", "PushEvent", "alice", "org/repo")

	testutil.MustNoError(t, rm.Upsert(ctx, item))

	got, err := rm.Get(ctx, "github", id.NewExternalID("123"))
	testutil.MustNoError(t, err)

	if got == nil {
		t.Fatal("Get returned nil")
	}

	if got.ID.String() != item.ID.String() {
		t.Errorf("ID = %q, want %q (ItemID not preserved)", got.ID.String(), item.ID.String())
	}

	testutil.AssertEqual(t, got.ExternalID.Get(), "123", "ExternalID")

	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")
}

func TestSQLiteReadModel_Get_NotFound(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", id.NewExternalID("nonexistent"))
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

func TestSQLiteReadModel_List(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	item1 := sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo")
	item2 := sqliteTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo")

	_ = rm.Upsert(ctx, item1)
	_ = rm.Upsert(ctx, item2)

	items, err := rm.List(ctx, provider.ItemFilter{})
	testutil.MustNoError(t, err)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestSQLiteReadModel_List_FilterByType(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo"))

	pushType := id.NewEventTypeID("PushEvent")
	items, err := rm.List(ctx, provider.ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 PushEvent, got %d", len(items))
	}

	testutil.AssertEqual(t, items[0].Type.Get(), "PushEvent", "Type")
}

func TestSQLiteReadModel_Count(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo"))

	count, err := rm.Count(ctx, provider.ItemFilter{})
	testutil.MustNoError(t, err)

	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	pushType := id.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, provider.ItemFilter{Type: &pushType})
	testutil.MustNoError(t, err)

	if count != 1 {
		t.Errorf("filtered Count = %d, want 1", count)
	}
}

func TestSQLiteReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "2", "IssueEvent", "bob", "org/repo"))
	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "3", "PushEvent", "charlie", "org/repo2"))

	types, err := rm.GetTypes(ctx)
	testutil.MustNoError(t, err)

	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d: %v", len(types), types)
	}
}

func TestSQLiteReadModel_Delete(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	_ = rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))

	err := rm.Delete(ctx, "github", id.NewExternalID("1"))
	testutil.MustNoError(t, err)

	got, _ := rm.Get(ctx, "github", id.NewExternalID("1"))
	if got != nil {
		t.Fatal("item should be nil after delete")
	}
}

func TestSQLiteReadModel_Upsert_Idempotent(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	item1 := sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo")
	_ = rm.Upsert(ctx, item1)

	item2 := sqliteTestItem(t, "github", "1", "IssueEvent", "bob", "org/repo")
	_ = rm.Upsert(ctx, item2)

	got, _ := rm.Get(ctx, "github", id.NewExternalID("1"))
	testutil.AssertEqual(t, got.Type.Get(), "IssueEvent", "Type")

	count, _ := rm.Count(ctx, provider.ItemFilter{})
	if count != 1 {
		t.Errorf("Count = %d, want 1 (upsert should overwrite, not duplicate)", count)
	}
}
