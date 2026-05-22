package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func upsertTestItem(
	t *testing.T,
	rm ReadModel,
	ctx context.Context,
	source, extID, eventType, actor, repo string,
) {
	t.Helper()

	mustNoError(t, rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID(extID),
		Source:     types.NewProviderID(source),
		Type:       types.NewEventTypeID(eventType),
		ActorLogin: types.NewActorID(actor),
		RepoName:   types.NewRepoID(repo),
		CreatedAt:  time.Now(),
	}))
}

func TestMemoryReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &provider.Item{
		ExternalID: types.NewExternalID("123"),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID("PushEvent"),
	}

	mustNoError(t, rm.Upsert(ctx, item))

	got, err := rm.Get(ctx, "github", types.NewExternalID("123"))
	mustNoError(t, err)
	if got == nil {
		t.Fatal("expected non-nil item")
	}
	assertItemType(t, got, "PushEvent")
}

func TestMemoryReadModel_GetNotFound(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", types.NewExternalID("nonexistent"))
	mustNoError(t, err)
	if got != nil {
		t.Error("expected nil for nonexistent item")
	}
}

func TestMemoryReadModel_Delete(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &provider.Item{
		ExternalID: types.NewExternalID("123"),
		Source:     types.NewProviderID("github"),
	}

	mustNoError(t, rm.Upsert(ctx, item))
	mustNoError(t, rm.Delete(ctx, "github", types.NewExternalID("123")))

	got, err := rm.Get(ctx, "github", types.NewExternalID("123"))
	mustNoError(t, err)
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestMemoryReadModel_ListWithFilters(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	upsertTestItem(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo1")
	upsertTestItem(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo2")
	upsertTestItem(t, rm, ctx, "gitlab", "3", "PushEvent", "alice", "org/repo3")

	pushTypeFilter := types.NewEventTypeID("PushEvent")
	items, err := rm.List(ctx, ItemFilter{Type: &pushTypeFilter})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	sourceFilter := types.NewProviderID("github")
	items, err = rm.List(ctx, ItemFilter{Source: &sourceFilter})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	actorFilter := types.NewActorID("alice")
	items, err = rm.List(ctx, ItemFilter{ActorLogin: &actorFilter})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	items, err = rm.List(ctx, ItemFilter{Limit: 2})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	items, err = rm.List(ctx, ItemFilter{Offset: 10})
	mustNoError(t, err)
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

	count, err := rm.Count(ctx, ItemFilter{})
	mustNoError(t, err)
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}

	pushTypeFilter := types.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, ItemFilter{Type: &pushTypeFilter})
	mustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

func TestMemoryReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	upsertTestItem(t, rm, ctx, "github", "1", "PushEvent", "alice", "org/repo")
	upsertTestItem(t, rm, ctx, "github", "2", "IssueEvent", "bob", "org/repo")
	upsertTestItem(t, rm, ctx, "github", "3", "PushEvent", "charlie", "org/repo2")

	result, err := rm.GetTypes(ctx)
	mustNoError(t, err)
	if len(result) != 2 {
		t.Fatalf("expected 2 types, got %d", len(result))
	}
	if result[0] != "IssueEvent" || result[1] != "PushEvent" {
		t.Errorf("expected [IssueEvent, PushEvent], got %v", result)
	}
}

func TestProjector_ItemSynced(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := NewProjector(rm)

	payload := testSyncedPayload("123", "PushEvent")

	evt := mustNewTestEvent(EventItemSynced, payload)

	mustNoError(t, proj.Handle(context.Background(), evt))

	if rm.Len() != 1 {
		t.Errorf("expected Len=1, got %d", rm.Len())
	}

	got, err := rm.Get(context.Background(), "github", types.NewExternalID("123"))
	mustNoError(t, err)
	assertItemType(t, got, "PushEvent")
}

func TestProjector_ItemDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	upsertTestItem(t, rm, ctx, "github", "123", "PushEvent", "alice", "org/repo")

	proj := NewProjector(rm)

	evt := mustNewTestEvent(EventItemDeleted, ItemDeletedPayload{Source: "github", SourceID: "123"})

	mustNoError(t, proj.Handle(ctx, evt))

	if rm.Len() != 0 {
		t.Errorf("expected Len=0, got %d", rm.Len())
	}
}

func TestProjector_ItemConflictFound_NoStateChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	upsertTestItem(t, rm, ctx, "github", "123", "PushEvent", "alice", "org/repo")

	proj := NewProjector(rm)

	evt := mustNewTestEvent(EventItemConflictFound, ItemConflictFoundPayload{
		Source: "github", SourceID: "123", Winner: "remote",
	})

	mustNoError(t, proj.Handle(ctx, evt))

	if rm.Len() != 1 {
		t.Errorf("expected Len=1, got %d", rm.Len())
	}
}

func TestReadModel_Integration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()
	proj := NewProjector(rm)

	item := testItem("123", "PushEvent")

	decide := DecideSync(item)
	events, err := decide(InitialState, 0)
	mustNoError(t, err)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	for _, evt := range events {
		mustNoError(t, proj.Handle(ctx, evt))
	}

	got, err := rm.Get(ctx, "github", types.NewExternalID("123"))
	mustNoError(t, err)
	if got.Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", got.Type.Get())
	}
	if got.ActorLogin.Get() != "testuser" {
		t.Errorf("expected ActorLogin=testuser, got %s", got.ActorLogin.Get())
	}

	count, err := rm.Count(ctx, ItemFilter{})
	mustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}
