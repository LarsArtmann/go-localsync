package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestSQLiteReadModel_List_FilterByActorLogin(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "2", "PushEvent", "bob", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "3", "IssueEvent", "alice", "org/repo")

	actor := id.NewActorLogin("alice")
	items, err := rm.List(ctx, model.ItemFilter{ActorLogin: &actor})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items for alice")

	count, err := rm.Count(ctx, model.ItemFilter{ActorLogin: &actor})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 2, "count for alice")
}

func TestSQLiteReadModel_List_FilterByRepoName(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo-a")
	sqliteSeed(t, rm, ctx, "github", "2", "PushEvent", "bob", "org/repo-b")
	sqliteSeed(t, rm, ctx, "github", "3", "IssueEvent", "charlie", "org/repo-a")

	repo := id.NewRepoID("org/repo-a")
	items, err := rm.List(ctx, model.ItemFilter{RepoName: &repo})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items for org/repo-a")
}

func TestSQLiteReadModel_List_FilterBySource(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "gitlab", "2", "PushEvent", "bob", "org/repo")

	source := id.NewProviderID("github")
	items, err := rm.List(ctx, model.ItemFilter{Source: &source})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 1, "items for github source")
	testutil.AssertEqual(t, items[0].Source.Get(), "github", "Source")
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

	testutil.MustNoError(t, rm.Upsert(ctx, oldItem))
	testutil.MustNoError(t, rm.Upsert(ctx, newItem))

	since := time.Now().Add(-24 * time.Hour)
	items, err := rm.List(ctx, model.ItemFilter{Since: &since})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 1, "items after Since cutoff")
	testutil.AssertEqual(t, items[0].ExternalID.Get(), "2", "ExternalID")

	count, err := rm.Count(ctx, model.ItemFilter{Since: &since})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 1, "count after Since cutoff")
}

func TestSQLiteReadModel_List_Pagination(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	for i := range 5 {
		sqliteSeed(t, rm, ctx, "github", string(rune('A'+i)), "PushEvent", "alice", "org/repo")
	}

	items, err := rm.List(ctx, model.ItemFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	testutil.AssertLen(t, items, 2, "items with Limit=2")

	items, err = rm.List(ctx, model.ItemFilter{Limit: 2, Offset: 2})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items with Limit=2 Offset=2")

	items, err = rm.List(ctx, model.ItemFilter{Limit: 2, Offset: 4})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 1, "items with Limit=2 Offset=4")
}

func TestSQLiteReadModel_List_FilterByTypeAndActorLogin(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "2", "PushEvent", "bob", "org/repo")
	sqliteSeed(t, rm, ctx, "github", "3", "IssueEvent", "alice", "org/repo")

	pushType := id.NewEventTypeID("PushEvent")
	actor := id.NewActorLogin("alice")
	items, err := rm.List(ctx, model.ItemFilter{Type: &pushType, ActorLogin: &actor})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 1, "PushEvent by alice")
	testutil.AssertEqual(t, items[0].ExternalID.Get(), "1", "ExternalID")
}

func TestSQLiteReadModel_List_ZeroResults(t *testing.T) {
	t.Parallel()

	rm := newSQLiteTestDB(t)
	ctx := context.Background()

	sqliteSeed(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")

	pushType := id.NewEventTypeID("NonExistentType")
	items, err := rm.List(ctx, model.ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	testutil.AssertLen(t, items, 0, "items for non-existent type")
}
