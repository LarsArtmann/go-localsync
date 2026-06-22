package cqrs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func assertLen(t *testing.T, rm *MemoryReadModel, want int) {
	t.Helper()

	testutil.AssertInt(t, rm.Len(), want, "Len")
}

func assertNotFound(t *testing.T, err error, got *model.Item) {
	t.Helper()

	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil item")
	}
}

func upsertTestItem(
	t *testing.T,
	rm ReadModel,
	ctx context.Context,
	source, extID, eventType, actor, repo string,
) {
	t.Helper()

	testutil.MustNoError(t, rm.Upsert(ctx, &model.Item{
		ExternalID: id.NewExternalID(extID),
		Source:     id.NewProviderID(source),
		Type:       id.NewEventTypeID(eventType),
		ActorLogin: id.NewActorLogin(actor),
		RepoName:   id.NewRepoID(repo),
		CreatedAt:  time.Now(),
	}))
}

func TestMemoryReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &model.Item{
		ExternalID: id.NewExternalID("123"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
	}

	testutil.MustNoError(t, rm.Upsert(ctx, item))

	got, err := rm.Get(ctx, "github", id.NewExternalID("123"))
	testutil.MustNoError(t, err)
	if got == nil {
		t.Fatal("expected non-nil item")
	}
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")
}

func TestMemoryReadModel_GetNotFound(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", id.NewExternalID("nonexistent"))
	assertNotFound(t, err, got)
}

func TestMemoryReadModel_Delete(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &model.Item{
		ExternalID: id.NewExternalID("123"),
		Source:     id.NewProviderID("github"),
	}

	testutil.MustNoError(t, rm.Upsert(ctx, item))
	testutil.MustNoError(t, rm.Delete(ctx, "github", id.NewExternalID("123")))

	got, err := rm.Get(ctx, "github", id.NewExternalID("123"))
	assertNotFound(t, err, got)
}

func TestMemoryReadModel_ListWithFilters(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	upsertTestItem(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo1")
	upsertTestItem(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo2")
	upsertTestItem(t, rm, ctx, "gitlab", "3", "PushEvent", "alice", "org/repo3")

	pushTypeFilter := id.NewEventTypeID("PushEvent")
	items, err := rm.List(ctx, model.ItemFilter{Type: &pushTypeFilter})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items")

	sourceFilter := id.NewProviderID("github")
	items, err = rm.List(ctx, model.ItemFilter{Source: &sourceFilter})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items")

	actorFilter := id.NewActorLogin("alice")
	items, err = rm.List(ctx, model.ItemFilter{ActorLogin: &actorFilter})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items")

	items, err = rm.List(ctx, model.ItemFilter{Limit: 2})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, items, 2, "items")

	items, err = rm.List(ctx, model.ItemFilter{Offset: 10})
	testutil.MustNoError(t, err)
	if items != nil {
		t.Errorf("expected nil for out-of-range offset, got %d items", len(items))
	}
}

func TestMemoryReadModel_Count(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	upsertTestItem(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	upsertTestItem(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")

	count, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 2, "count")

	pushTypeFilter := id.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, model.ItemFilter{Type: &pushTypeFilter})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 1, "count")
}

func TestMemoryReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	upsertTestItem(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	upsertTestItem(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")
	upsertTestItem(t, rm, ctx, "github", "3", "PushEvent", "charlie", "org/repo2")

	result, err := rm.GetTypes(ctx)
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, result, 2, "types")
	if result[0] != "IssueEvent" || result[1] != "PushEvent" {
		t.Errorf("expected [IssueEvent, PushEvent], got %v", result)
	}
}

func TestProjector_ItemSynced(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)

	payload := testSyncedPayload("123", "PushEvent")

	evt := mustNewTestEvent(EventItemSynced, payload)

	testutil.MustNoError(t, proj.Handle(context.Background(), evt))

	assertLen(t, rm, 1)

	got, err := rm.Get(context.Background(), "github", id.NewExternalID("123"))
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")
}

func TestProjector_ItemDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	upsertTestItem(t, rm, ctx, "github", "123", "PushEvent", "alice", "org/repo")

	proj := newProjector(rm)

	evt := mustNewTestEvent(EventItemDeleted, ItemDeletedPayload{Source: "github", SourceID: "123"})

	testutil.MustNoError(t, proj.Handle(ctx, evt))

	assertLen(t, rm, 0)
}

func TestProjector_ItemConflictFound_NoStateChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	upsertTestItem(t, rm, ctx, "github", "123", "PushEvent", "alice", "org/repo")

	proj := newProjector(rm)

	evt := mustNewTestEvent(EventItemConflictFound, ItemConflictFoundPayload{
		Source: "github", SourceID: "123", Winner: "remote",
	})

	testutil.MustNoError(t, proj.Handle(ctx, evt))

	assertLen(t, rm, 1)
}

func TestProjector_ItemSynced_InvalidItemID(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)

	payload := ItemSyncedPayload{
		ItemID:   "not-a-valid-ulid",
		Source:   "github",
		SourceID: "123",
		Type:     "PushEvent",
	}

	evt := mustNewTestEvent(EventItemSynced, payload)

	err := proj.Handle(context.Background(), evt)
	if err == nil {
		t.Error("expected error for invalid ItemID")
	}
}

func TestProjector_ItemSynced_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)

	payload := ItemSyncedPayload{
		Source:   "github",
		SourceID: "",
		Type:     "PushEvent",
	}

	evt := mustNewTestEvent(EventItemSynced, payload)

	err := proj.Handle(context.Background(), evt)
	if err == nil {
		t.Error("expected error for missing SourceID")
	}
}

func TestReadModel_Integration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()
	proj := newProjector(rm)

	item := testItem("123", "PushEvent")

	decide := decideSync(toDataItem(item), nil, nil)
	events, err := decide(InitialState, 0)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 1)

	for _, evt := range events {
		testutil.MustNoError(t, proj.Handle(ctx, evt))
	}

	got, err := rm.Get(ctx, "github", id.NewExternalID("123"))
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")
	if got.ActorLogin.Get() != "testuser" {
		t.Errorf("expected ActorLogin=testuser, got %s", got.ActorLogin.Get())
	}

	count, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}
