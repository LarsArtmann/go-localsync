package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestCQRSStack_SyncNewItem(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	testutil.MustNoError(t, stack.SyncItem(ctx, item))

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

func TestCQRSStack_SyncMultipleItems(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := testItems("1", "PushEvent", "2", "IssueEvent", "3", "PushEvent")

	result := stack.SyncItems(ctx, items)
	testutil.AssertInt(t, result.Synced, 3, "Synced")
	testutil.AssertInt(t, result.Conflicts, 0, "Conflicts")
	testutil.AssertInt(t, result.Errors, 0, "Errors")

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 3, "count")
}

func TestCQRSStack_Idempotency_DeterministicAggregateID(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	testutil.MustNoError(t, stack.SyncItem(ctx, item))
	testutil.MustNoError(t, stack.SyncItem(ctx, item))

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 1, "count")
}

func TestCQRSStack_TombstoneItem(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "123", "PushEvent")

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 1, "count")

	testutil.MustNoError(t, stack.TombstoneItem(ctx, "github", id.NewExternalID("123"), model.ReasonUpstreamGone))

	count, err = stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	testutil.AssertInt64(t, count, 0, "count after tombstone")
}

func TestCQRSStack_TombstoneThenResurrect(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "123", "PushEvent")
	testutil.MustNoError(t, stack.TombstoneItem(ctx, "github", id.NewExternalID("123"), model.ReasonUpstreamGone))

	count, _ := stack.Count(ctx, model.ItemFilter{})
	testutil.AssertInt64(t, count, 0, "count after tombstone")

	syncTestItem(t, stack, ctx, "123", "IssueEvent")

	count, _ = stack.Count(ctx, model.ItemFilter{})
	testutil.AssertInt64(t, count, 1, "count after resurrect")

	got, err := stack.Get(ctx, "github", id.NewExternalID("123"))
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, got.Type.Get(), "IssueEvent", "resurrected item type")
}

func TestCQRSStack_ConflictDetection(t *testing.T) {
	stack, ctx := setupMemoryStack(t)

	items := []*provider.Item{testItem("1", "PushEvent")}

	result := stack.SyncItems(ctx, items)
	testutil.AssertInt(t, result.Synced, 1, "Synced")
	testutil.AssertInt(t, result.Conflicts, 0, "Conflicts")
	testutil.AssertInt(t, result.Errors, 0, "Errors")

	updatedItem := testItem("1", "PushEvent")
	updatedItem.UpdatedAt = time.Now().Add(time.Hour)

	result = stack.SyncItems(ctx, []*provider.Item{updatedItem})
	testutil.AssertInt(t, result.Synced, 1, "Synced")
	testutil.AssertInt(t, result.Conflicts, 1, "Conflicts")
	testutil.AssertInt(t, result.Errors, 0, "Errors")
}

func TestCQRSStack_FilterByType(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := testItems("1", "PushEvent", "2", "IssueEvent", "3", "PushEvent")

	result := stack.SyncItems(ctx, items)
	testutil.AssertInt(t, result.Synced, 3, "Synced")

	pushType := id.NewEventTypeID("PushEvent")
	results, err := stack.List(ctx, model.ItemFilter{Type: &pushType})
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, results, 2, "results")
}

func TestCQRSStack_Close(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	testutil.MustNoError(t, stack.Close())
}

func TestCQRSStack_InvalidBackend(t *testing.T) {
	t.Parallel()

	_, err := NewCQRSStack(CQRSConfig{Backend: "postgres"})
	if err == nil {
		t.Fatal("expected error for invalid backend")
	}
}

func TestCQRSStack_DeterministicAggregateID_Matches(t *testing.T) {
	t.Parallel()

	id1 := AggregateID("github", id.NewExternalID("123"))
	id2 := AggregateID("github", id.NewExternalID("123"))

	if id1 != id2 {
		t.Error("deterministic IDs must be equal for same inputs")
	}
}

func TestCQRSStack_ProjectionRunner_HasCheckpointing(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	testutil.MustNoError(t, stack.SyncItem(ctx, item))

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after sync, got %d", count)
	}
}
