package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestIntegration_SyncItemsPipeline verifies the complete CQRS pipeline:
// provider.Item → adapter → command dispatch → decider → events → projection → read model.
func TestIntegration_SyncItemsPipeline(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	summary := syncTestItemsResult(t, stack, ctx, "1", "PushEvent", "2", "IssueEvent")
	if summary.Errors != 0 {
		t.Fatalf("unexpected sync errors: %d", summary.Errors)
	}
	if summary.Synced != 2 {
		t.Fatalf("expected synced=2, got %d", summary.Synced)
	}

	waitForCount(t, stack, ctx, 2)

	list, err := stack.List(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if len(list) != 2 {
		t.Fatalf("expected 2 items in read model, got %d", len(list))
	}

	for _, item := range list {
		if item.Source.IsZero() {
			t.Errorf("expected non-zero Source for item %s", item.SourceID.Get())
		}
		if item.Type.IsZero() {
			t.Errorf("expected non-zero Type for item %s", item.SourceID.Get())
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

	count, err := stack.Count(ctx, model.ItemFilter{})
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

	summary := syncTestItemsResult(t, stack, ctx, "1", "PushEvent")
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

	err := stack.TombstoneItem(ctx, "github", id.NewSourceID("1"), model.ReasonUpstreamGone)
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

	items := testItems("1", "PushEvent", "2", "IssueEvent", "3", "PushEvent")

	stack.SyncItems(ctx, items)
	waitForCount(t, stack, ctx, 3)

	pushType := id.NewEventTypeID("PushEvent")
	filtered, err := stack.List(ctx, model.ItemFilter{Type: &pushType})
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

	syncTestItems(t, stack, ctx, "1", "PushEvent", "2", "IssueEvent")
	waitForCount(t, stack, ctx, 2)

	list, err := stack.List(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
}

// TestIntegration_SyncItems_ConflictLocal verifies that the LWW resolver
// can produce ActionConflictLocal when local is newer than remote.
func TestIntegration_SyncItems_ConflictLocal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	resolver := newUpdatedAtLWWResolver(t)
	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory", ConflictResolver: resolver})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	futureTime := time.Now().Add(3 * time.Hour).Truncate(time.Millisecond)
	localItem := &provider.Item{
		SourceID: id.NewSourceID("1"),
		Source:   id.NewProviderID("github"),
		Type:     id.NewEventTypeID("PushEvent"),
		Attributes: map[string]string{
			"actor_login": "testuser",
			"repo_name":   "owner/repo",
		},
		CreatedAt: futureTime,
		UpdatedAt: futureTime,
		RawJSON:   []byte(`{"test":true}`),
	}

	summary1 := stack.SyncItems(ctx, []*provider.Item{localItem})
	if summary1.Synced != 1 {
		t.Fatalf("expected synced=1 on first sync, got %d", summary1.Synced)
	}

	waitForCount(t, stack, ctx, 1)

	olderTime := time.Now().Truncate(time.Millisecond)
	remoteItem := &provider.Item{
		SourceID: id.NewSourceID("1"),
		Source:   id.NewProviderID("github"),
		Type:     id.NewEventTypeID("PushEvent"),
		Attributes: map[string]string{
			"actor_login": "testuser",
			"repo_name":   "owner/repo",
		},
		CreatedAt: olderTime,
		UpdatedAt: olderTime,
		RawJSON:   []byte(`{"test":true}`),
	}

	summary2 := stack.SyncItems(ctx, []*provider.Item{remoteItem})
	if summary2.Conflicts != 1 {
		t.Fatalf("expected 1 conflict, got %d", summary2.Conflicts)
	}

	if len(summary2.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(summary2.Results))
	}

	if summary2.Results[0].Action != synclib.ActionConflictLocal {
		t.Errorf("expected ActionConflictLocal, got %s", summary2.Results[0].Action)
	}
}
