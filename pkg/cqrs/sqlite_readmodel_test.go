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

func sqliteAssertNotFound(t *testing.T, err error, got *model.Item) {
	t.Helper()

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

// sqliteSeed inserts a test item and fails on error.
// Reduces the "_ = rm.Upsert(ctx, sqliteTestItem(...))" fixture pattern
// to a single line per call site.
func sqliteSeed(t *testing.T, rm *SQLiteReadModel, ctx context.Context, source, extID, eventType, actor, repo string) {
	t.Helper()

	testutil.MustNoError(t, rm.Upsert(ctx, sqliteTestItem(t, source, extID, eventType, actor, repo)))
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
	sqliteAssertNotFound(t, err, got)
}

func TestSQLiteReadModel_List(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")

	items, err := rm.List(ctx, provider.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items")
}

func TestSQLiteReadModel_List_FilterByType(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")

	pushType := id.NewEventTypeID("PushEvent")
	items, err := rm.List(ctx, provider.ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	testutil.AssertLen(t, items, 1, "PushEvent items")
	testutil.AssertEqual(t, items[0].Type.Get(), "PushEvent", "Type")
}

func TestSQLiteReadModel_Count(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")

	count, err := rm.Count(ctx, provider.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 2, "Count")

	pushType := id.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, provider.ItemFilter{Type: &pushType})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 1, "filtered Count")
}

func TestSQLiteReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "3", "PushEvent", "charlie", "org/repo2")

	types, err := rm.GetTypes(ctx)
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, types, 2, "types")
}

func TestSQLiteReadModel_Delete(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")

	testutil.MustNoError(t, rm.Delete(ctx, "github", id.NewExternalID("1")))

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
	testutil.MustNoError(t, rm.Upsert(ctx, item1))

	item2 := sqliteTestItem(t, "github", "1", "IssueEvent", "bob", "org/repo")
	testutil.MustNoError(t, rm.Upsert(ctx, item2))

	got, _ := rm.Get(ctx, "github", id.NewExternalID("1"))
	testutil.AssertEqual(t, got.Type.Get(), "IssueEvent", "Type")

	count, _ := rm.Count(ctx, provider.ItemFilter{})
	testutil.AssertInt64(t, count, 1, "Count")
}
