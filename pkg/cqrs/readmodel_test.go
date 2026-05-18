package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func TestMemoryReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &provider.Item{
		ExternalID: types.NewExternalID("123"),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID("PushEvent"),
	}

	if err := rm.Upsert(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := rm.Get(ctx, "github", "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil item")
	}
	if got.Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", got.Type.Get())
	}
}

func TestMemoryReadModel_GetNotFound(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	got, err := rm.Get(ctx, "github", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	if err := rm.Upsert(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rm.Delete(ctx, "github", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := rm.Get(ctx, "github", "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestMemoryReadModel_ListWithFilters(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	pushType := "PushEvent"
	issueType := "IssueEvent"

	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("1"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID(pushType), ActorLogin: types.NewActorID("alice"),
		RepoName: types.NewRepoID("org/repo1"), CreatedAt: time.Now().Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("2"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID(issueType), ActorLogin: types.NewActorID("bob"),
		RepoName: types.NewRepoID("org/repo2"), CreatedAt: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("3"), Source: types.NewProviderID("gitlab"),
		Type: types.NewEventTypeID(pushType), ActorLogin: types.NewActorID("alice"),
		RepoName: types.NewRepoID("org/repo3"), CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pushTypeFilter := types.NewEventTypeID(pushType)
	items, err := rm.List(ctx, ItemFilter{Type: &pushTypeFilter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	sourceFilter := types.NewProviderID("github")
	items, err = rm.List(ctx, ItemFilter{Source: &sourceFilter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	actorFilter := types.NewActorID("alice")
	items, err = rm.List(ctx, ItemFilter{ActorLogin: &actorFilter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	items, err = rm.List(ctx, ItemFilter{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	items, err = rm.List(ctx, ItemFilter{Offset: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil for out-of-range offset, got %d items", len(items))
	}
}

func TestMemoryReadModel_Count(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("1"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("2"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("IssueEvent"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, err := rm.Count(ctx, ItemFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}

	pushTypeFilter := types.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, ItemFilter{Type: &pushTypeFilter})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

func TestMemoryReadModel_GetTypes(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("1"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("2"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("IssueEvent"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("3"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := rm.GetTypes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	payload := ItemSyncedPayload{
		Source:    "github",
		SourceID:  "123",
		Type:      "PushEvent",
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}

	evt := mustNewTestEvent(EventItemSynced, payload)

	if err := proj.HandleEvent(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rm.Len() != 1 {
		t.Errorf("expected Len=1, got %d", rm.Len())
	}

	got, err := rm.Get(context.Background(), "github", "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", got.Type.Get())
	}
}

func TestProjector_ItemDeleted(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("123"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	proj := NewProjector(rm)

	evt := mustNewTestEvent(EventItemDeleted, ItemDeletedPayload{Source: "github", SourceID: "123"})

	if err := proj.HandleEvent(ctx, evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rm.Len() != 0 {
		t.Errorf("expected Len=0, got %d", rm.Len())
	}
}

func TestProjector_ItemConflictFound_NoStateChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	if err := rm.Upsert(ctx, &provider.Item{
		ExternalID: types.NewExternalID("123"), Source: types.NewProviderID("github"),
		Type: types.NewEventTypeID("PushEvent"),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	proj := NewProjector(rm)

	evt := mustNewTestEvent(EventItemConflictFound, ItemConflictFoundPayload{
		Source: "github", SourceID: "123", Winner: "remote",
	})

	if err := proj.HandleEvent(ctx, evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	for _, evt := range events {
		if err := proj.HandleEvent(ctx, evt); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	got, err := rm.Get(ctx, "github", "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", got.Type.Get())
	}
	if got.ActorLogin.Get() != "testuser" {
		t.Errorf("expected ActorLogin=testuser, got %s", got.ActorLogin.Get())
	}

	count, err := rm.Count(ctx, ItemFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}
