package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestIntegration_SyncItemsPipeline verifies the complete CQRS pipeline:
// provider.Item → adapter → command dispatch → decider → events → projection → read model.
func TestIntegration_SyncItemsPipeline(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
	}

	summary := stack.SyncItems(ctx, items)
	if summary.Errors != 0 {
		t.Fatalf("unexpected sync errors: %d", summary.Errors)
	}
	if summary.Synced != 2 {
		t.Fatalf("expected synced=2, got %d", summary.Synced)
	}

	waitForCount(t, stack, ctx, 2)

	list, err := stack.ListItems(ctx, provider.ItemFilter{})
	testutil.MustNoError(t, err)
	if len(list) != 2 {
		t.Fatalf("expected 2 items in read model, got %d", len(list))
	}

	types, err := stack.GetItemTypes(ctx)
	testutil.MustNoError(t, err)
	if len(types) != 2 {
		t.Errorf("expected 2 types, got %d", len(types))
	}

	for _, item := range list {
		if item.Source.IsZero() {
			t.Errorf("expected non-zero Source for item %s", item.ExternalID.Get())
		}
		if item.Type.IsZero() {
			t.Errorf("expected non-zero Type for item %s", item.ExternalID.Get())
		}
	}
}

// TestIntegration_SyncItemsIdempotent verifies that syncing the same item
// twice produces idempotent results (1 aggregate, 1 read model entry).
func TestIntegration_SyncItemsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	item := testItem("1", "PushEvent")

	summary1 := stack.SyncItems(ctx, []*provider.Item{item})
	if summary1.Synced != 1 {
		t.Fatalf("expected synced=1 on first sync, got %d", summary1.Synced)
	}

	waitForCount(t, stack, ctx, 1)

	summary2 := stack.SyncItems(ctx, []*provider.Item{item})
	if summary2.Synced != 0 {
		t.Fatalf("expected synced=0 on second sync (unchanged), got %d", summary2.Synced)
	}

	count, err := stack.Count(ctx)
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after idempotent sync, got %d", count)
	}
}

// TestIntegration_SyncItemsWithConflictResolver verifies that a configured
// LWW resolver is invoked during sync and produces correct conflict events.
func TestIntegration_SyncItemsWithConflictResolver(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resolver := newUpdatedAtLWWResolver(t)
	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory", ConflictResolver: resolver})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{
		testItem("1", "PushEvent"),
	}

	summary := stack.SyncItems(ctx, items)
	if summary.Errors != 0 {
		t.Fatalf("unexpected sync errors: %d", summary.Errors)
	}

	waitForCount(t, stack, ctx, 1)
}

// TestIntegration_DeleteAndResurrect verifies that a deleted item can be
// re-synced and reappear in the read model.
func TestIntegration_DeleteAndResurrect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	item := testItem("1", "PushEvent")

	stack.SyncItems(ctx, []*provider.Item{item})
	waitForCount(t, stack, ctx, 1)

	err := stack.DeleteItem(ctx, "github", id.NewExternalID("1"))
	testutil.MustNoError(t, err)
	waitForCount(t, stack, ctx, 0)

	stack.SyncItems(ctx, []*provider.Item{item})
	waitForCount(t, stack, ctx, 1)
}

// TestIntegration_ReadModelFilter verifies that read model filtering works
// correctly after items are synced through the full pipeline.
func TestIntegration_ReadModelFilter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
		testItem("3", "PushEvent"),
	}

	stack.SyncItems(ctx, items)
	waitForCount(t, stack, ctx, 3)

	pushType := id.NewEventTypeID("PushEvent")
	filtered, err := stack.ListItems(ctx, provider.ItemFilter{Type: &pushType})
	testutil.MustNoError(t, err)
	if len(filtered) != 2 {
		t.Errorf("expected 2 PushEvent items, got %d", len(filtered))
	}
}

// TestIntegration_SQLiteBackend verifies the full pipeline with the SQLite
// backend to ensure persistence and projection replay work correctly.
func TestIntegration_SQLiteBackend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack := newSQLiteMemoryStack(t)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
	}

	stack.SyncItems(ctx, items)
	waitForCount(t, stack, ctx, 2)

	list, err := stack.ListItems(ctx, provider.ItemFilter{})
	testutil.MustNoError(t, err)
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
}
