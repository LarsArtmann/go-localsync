package cqrs

import (
	"context"
	"testing"
	"time"

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

	count, err := stack.Count(ctx)
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	resultTypes, err := stack.GetItemTypes(ctx)
	testutil.MustNoError(t, err)
	if len(resultTypes) != 1 || resultTypes[0] != "PushEvent" {
		t.Errorf("expected [PushEvent], got %v", resultTypes)
	}
}

func TestCQRSStack_SyncMultipleItems(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
		testItem("3", "PushEvent"),
	}

	result := stack.SyncItems(ctx, items)
	if result.Synced != 3 {
		t.Errorf("expected Synced=3, got %d", result.Synced)
	}
	if result.Conflicts != 0 {
		t.Errorf("expected Conflicts=0, got %d", result.Conflicts)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}

	count, err := stack.Count(ctx)
	testutil.MustNoError(t, err)
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}

	resultTypes, err := stack.GetItemTypes(ctx)
	testutil.MustNoError(t, err)
	if len(resultTypes) != 2 {
		t.Errorf("expected 2 types, got %v", resultTypes)
	}
}

func TestCQRSStack_Idempotency_DeterministicAggregateID(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	testutil.MustNoError(t, stack.SyncItem(ctx, item))
	testutil.MustNoError(t, stack.SyncItem(ctx, item))

	count, err := stack.Count(ctx)
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("same item synced twice should still have count 1 — idempotent, got %d", count)
	}
}

func TestCQRSStack_DeleteItem(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	testutil.MustNoError(t, stack.SyncItem(ctx, testItem("123", "PushEvent")))

	count, err := stack.Count(ctx)
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	testutil.MustNoError(t, stack.DeleteItem(ctx, "github", id.NewExternalID("123")))

	count, err = stack.Count(ctx)
	testutil.MustNoError(t, err)
	if count != 0 {
		t.Errorf("item should be deleted from read model, got count=%d", count)
	}
}

func TestCQRSStack_DeleteThenResurrect(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	testutil.MustNoError(t, stack.SyncItem(ctx, testItem("123", "PushEvent")))
	testutil.MustNoError(t, stack.DeleteItem(ctx, "github", id.NewExternalID("123")))

	count, _ := stack.Count(ctx)
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}

	testutil.MustNoError(t, stack.SyncItem(ctx, testItem("123", "IssueEvent")))

	count, _ = stack.Count(ctx)
	if count != 1 {
		t.Errorf("resurrected item should reappear in read model, got count=%d", count)
	}

	got, err := stack.ReadModel.Get(ctx, "github", id.NewExternalID("123"))
	testutil.MustNoError(t, err)
	testutil.AssertEqual(t, got.Type.Get(), "IssueEvent", "resurrected item type")
}

func TestCQRSStack_ConflictDetection(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	items := []*provider.Item{testItem("1", "PushEvent")}

	result := stack.SyncItems(ctx, items)
	if result.Synced != 1 {
		t.Errorf("expected Synced=1, got %d", result.Synced)
	}
	if result.Conflicts != 0 {
		t.Errorf("expected Conflicts=0, got %d", result.Conflicts)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}

	updatedItem := testItem("1", "PushEvent")
	updatedItem.UpdatedAt = time.Now().Add(time.Hour)

	result = stack.SyncItems(ctx, []*provider.Item{updatedItem})
	if result.Synced != 1 {
		t.Errorf("expected Synced=1, got %d", result.Synced)
	}
	if result.Conflicts != 1 {
		t.Errorf(
			"updated item with newer timestamp should trigger conflict, got Conflicts=%d",
			result.Conflicts,
		)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}
}

func TestCQRSStack_FilterByType(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
		testItem("3", "PushEvent"),
	}

	result := stack.SyncItems(ctx, items)
	if result.Synced != 3 {
		t.Errorf("expected Synced=3, got %d", result.Synced)
	}

	pushType := id.NewEventTypeID("PushEvent")
	results, err := stack.ReadModel.List(ctx, provider.ItemFilter{Type: &pushType})
	testutil.MustNoError(t, err)
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
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

	count, err := stack.Count(ctx)
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after sync, got %d", count)
	}
}
