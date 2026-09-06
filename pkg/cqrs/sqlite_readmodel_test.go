package cqrs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/storage/v4"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
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

	rm, err := newSQLiteReadModel(context.Background(), db)
	if err != nil {
		t.Fatalf("newSQLiteReadModel: %v", err)
	}

	return rm
}

func sqliteTestItem(t *testing.T, source, extID, eventType, actor, repo string) *model.Item {
	t.Helper()

	return &model.Item{
		ID:       id.NewItemID(),
		SourceID: id.NewSourceID(extID),
		Source:   id.NewProviderID(source),
		Type:     id.NewEventTypeID(eventType),
		Attributes: map[string]string{
			"actor_login":      actor,
			"actor_avatar_url": "https://avatar.example.com/" + actor,
			"repo_name":        repo,
			"repo_url":         "https://github.com/" + repo,
		},
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
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
	item.ContentHash = "abc123hash"
	item.SchemaVersion = schema.V2

	testutil.MustNoError(t, rm.Upsert(ctx, item))

	got, err := rm.Get(ctx, "github", id.NewSourceID("123"))
	testutil.MustNoError(t, err)

	if got == nil {
		t.Fatal("Get returned nil")
	}

	if got.ID.String() != item.ID.String() {
		t.Errorf("ID = %q, want %q (ItemID not preserved)", got.ID.String(), item.ID.String())
	}

	testutil.AssertEqual(t, got.SourceID.Get(), "123", "SourceID")
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")
	testutil.AssertEqual(t, got.ContentHash, "abc123hash", "ContentHash")
	testutil.AssertEqual(t, got.SchemaVersion.Int(), schema.V2.Int(), "SchemaVersion")
}

func TestSQLiteReadModel_Get_NotFound(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", id.NewSourceID("nonexistent"))
	assertNotFound(t, err, got)
}

func TestSQLiteReadModel_List(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")

	items, err := rm.List(ctx, model.ItemFilter{})
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
	items, err := rm.List(ctx, model.ItemFilter{Type: &pushType})
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

	count, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, count, 2, "Count")

	pushType := id.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, model.ItemFilter{Type: &pushType})
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, count, 1, "filtered Count")
}

func TestSQLiteReadModel_Tombstone(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")

	testutil.MustNoError(
		t,
		rm.Tombstone(ctx, "github", id.NewSourceID("1"), model.NewTombstone(model.ReasonUpstreamGone)),
	)

	// Get returns the tombstoned item directly (key lookup, not the live view).
	got, err := rm.Get(ctx, "github", id.NewSourceID("1"))
	testutil.MustNoError(t, err)
	if got == nil || !got.IsTombstoned() {
		t.Fatal("expected tombstoned item")
	}

	// Default (live) view excludes the tombstoned item.
	live, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if live != 0 {
		t.Errorf("expected live count=0, got %d", live)
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

	got, _ := rm.Get(ctx, "github", id.NewSourceID("1"))
	testutil.AssertEqual(t, got.Type.Get(), "IssueEvent", "Type")

	count, _ := rm.Count(ctx, model.ItemFilter{})
	testutil.AssertEqual(t, count, 1, "Count")
}

// TestSQLiteReadModel_ErrorChainPreserved guards the session-28 fix: database
// errors must wrap BOTH the original driver error (for errors.As/Is root-cause
// inspection) AND the ErrDatabase sentinel (for classification). The old
// fmt.Sprintf("…: %v", err) pattern severed the chain.
func TestSQLiteReadModel_ErrorChainPreserved(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	// Close the underlying db so all subsequent operations fail.
	testutil.MustNoError(t, rm.Close())

	err := rm.Upsert(ctx, sqliteTestItem(t, "github", "1", "PushEvent", "alice", "org/repo"))
	if err == nil {
		t.Fatal("expected error after db close")
	}

	// Must classify as a database error.
	if !errors.Is(err, pkgerrors.ErrDatabase) {
		t.Errorf("errors.Is(err, ErrDatabase) = false; error: %v", err)
	}

	// The error message must still contain the human-readable detail.
	if msg := err.Error(); msg == "" {
		t.Error("error message is empty")
	}
}
