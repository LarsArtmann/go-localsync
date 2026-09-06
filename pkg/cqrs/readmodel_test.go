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

	testutil.AssertEqual(t, rm.Len(), want, "Len")
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
		SourceID: id.NewSourceID(extID),
		Source:     id.NewProviderID(source),
		Type:       id.NewEventTypeID(eventType),
		Attributes: map[string]string{
			"actor_login": actor,
			"repo_name":   repo,
		},
		CreatedAt: time.Now(),
	}))
}

func TestMemoryReadModel_UpsertAndGet(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &model.Item{
		SourceID: id.NewSourceID("123"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
	}

	testutil.MustNoError(t, rm.Upsert(ctx, item))

	got, err := rm.Get(ctx, "github", id.NewSourceID("123"))
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

	got, err := rm.Get(ctx, "github", id.NewSourceID("nonexistent"))
	assertNotFound(t, err, got)
}

func TestMemoryReadModel_Tombstone(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := &model.Item{
		SourceID: id.NewSourceID("123"),
		Source:     id.NewProviderID("github"),
	}

	testutil.MustNoError(t, rm.Upsert(ctx, item))
	testutil.MustNoError(
		t,
		rm.Tombstone(ctx, "github", id.NewSourceID("123"), model.NewTombstone(model.ReasonUserHidden)),
	)

	// Get returns the item itself (now tombstoned), since it is a direct key lookup.
	got, err := rm.Get(ctx, "github", id.NewSourceID("123"))
	testutil.MustNoError(t, err)
	if !got.IsTombstoned() {
		t.Error("expected tombstoned item")
	}

	// Default (live) view excludes tombstoned items.
	live, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if live != 0 {
		t.Errorf("expected live count=0, got %d", live)
	}

	// IncludeTombstoned reveals it again.
	withTomb, err := rm.Count(ctx, model.ItemFilter{IncludeTombstoned: true})
	testutil.MustNoError(t, err)
	if withTomb != 1 {
		t.Errorf("expected count=1 including tombstoned, got %d", withTomb)
	}
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

	items, err = rm.List(ctx, model.ItemFilter{Attributes: map[string]string{"actor_login": "alice"}})
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
	testutil.AssertEqual(t, count, 2, "count")

	pushTypeFilter := id.NewEventTypeID("PushEvent")
	count, err = rm.Count(ctx, model.ItemFilter{Type: &pushTypeFilter})
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, count, 1, "count")
}

func TestProjector_ItemSynced(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	proj := newProjector(rm)

	payload := testSyncedPayload("123", "PushEvent")

	evt := mustNewTestEvent(EventItemSynced, payload)

	testutil.MustNoError(t, proj.Handle(context.Background(), evt))

	assertLen(t, rm, 1)

	got, err := rm.Get(context.Background(), "github", id.NewSourceID("123"))
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")
}

func TestProjector_ItemTombstoned(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()

	upsertTestItem(t, rm, ctx, "github", "123", "PushEvent", "alice", "org/repo")

	proj := newProjector(rm)

	evt := mustNewTestEvent(EventItemTombstoned, ItemTombstonedPayload{
		Source:       "github",
		SourceID:     "123",
		Reason:       string(model.ReasonUpstreamGone),
		TombstonedAt: time.Now().UnixNano(),
	})

	testutil.MustNoError(t, proj.Handle(ctx, evt))

	// Tombstoned item is hidden from the live view but still present.
	live, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if live != 0 {
		t.Errorf("expected live count=0, got %d", live)
	}

	withTomb, err := rm.Count(ctx, model.ItemFilter{IncludeTombstoned: true})
	testutil.MustNoError(t, err)
	if withTomb != 1 {
		t.Errorf("expected count=1 including tombstoned, got %d", withTomb)
	}
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

	got, err := rm.Get(ctx, "github", id.NewSourceID("123"))
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type")
	if got.Attributes["actor_login"] != "testuser" {
		t.Errorf("expected actor_login=testuser, got %s", got.Attributes["actor_login"])
	}

	count, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}
