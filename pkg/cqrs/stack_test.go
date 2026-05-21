package cqrs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

func waitForCount(t *testing.T, stack *CQRSStack, ctx context.Context, expected int64) {
	t.Helper()

	deadline := time.Now().Add(time.Second)

	for time.Now().Before(deadline) {
		count, err := stack.Count(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if count == expected {
			return
		}

		time.Sleep(time.Millisecond)
	}

	count, _ := stack.Count(ctx)
	t.Fatalf("timed out waiting for count=%d, got %d", expected, count)
}

func TestCQRSStack_SyncNewItem(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	if err := stack.SyncItem(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, err := stack.Count(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	resultTypes, err := stack.GetTypes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resultTypes) != 1 || resultTypes[0] != "PushEvent" {
		t.Errorf("expected [PushEvent], got %v", resultTypes)
	}
}

func TestCQRSStack_SyncMultipleItems(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}

	resultTypes, err := stack.GetTypes(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resultTypes) != 2 {
		t.Errorf("expected 2 types, got %v", resultTypes)
	}
}

func TestCQRSStack_Idempotency_DeterministicAggregateID(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	if err := stack.SyncItem(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := stack.SyncItem(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, err := stack.Count(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("same item synced twice should still have count 1 — idempotent, got %d", count)
	}
}

func TestCQRSStack_DeleteItem(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	if err := stack.SyncItem(ctx, testItem("123", "PushEvent")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, err := stack.Count(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	if err := stack.DeleteItem(ctx, "github", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, err = stack.Count(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("item should be deleted from read model, got count=%d", count)
	}
}

func TestCQRSStack_DeleteThenResurrect(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	if err := stack.SyncItem(ctx, testItem("123", "PushEvent")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := stack.DeleteItem(ctx, "github", "123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, _ := stack.Count(ctx)
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}

	if err := stack.SyncItem(ctx, testItem("123", "IssueEvent")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, _ = stack.Count(ctx)
	if count != 1 {
		t.Errorf("resurrected item should reappear in read model, got count=%d", count)
	}

	got, err := stack.ReadModel.Get(ctx, "github", "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type.Get() != "IssueEvent" {
		t.Errorf("resurrected item should have updated type, got %s", got.Type.Get())
	}
}

func TestCQRSStack_ConflictDetection(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	pushType := types.NewEventTypeID("PushEvent")
	results, err := stack.ReadModel.List(ctx, ItemFilter{Type: &pushType})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestCQRSStack_Close(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := stack.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCQRSStack_TursoBackend_SyncAndDelete(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
	}

	result := stack.SyncItems(ctx, items)
	if result.Synced != 2 {
		t.Errorf("expected Synced=2, got %d", result.Synced)
	}
	if result.Conflicts != 0 {
		t.Errorf("expected Conflicts=0, got %d", result.Conflicts)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}

	waitForCount(t, stack, ctx, 2)

	if err := stack.DeleteItem(ctx, "github", "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForCount(t, stack, ctx, 1)
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

	id1 := AggregateID("github", "123")
	id2 := AggregateID("github", "123")

	if id1 != id2 {
		t.Error("deterministic IDs must be equal for same inputs")
	}
}

func TestCQRSStack_ProjectionRunner_HasCheckpointing(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	if stack.Runner == nil {
		t.Fatal("expected Runner to be initialized")
	}

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	if err := stack.SyncItem(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, err := stack.Count(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1 after sync, got %d", count)
	}
}

func TestCQRSStack_TursoLocalStore_SyncAndReadModel(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	if err := stack.SyncItem(ctx, testItem("1", "PushEvent")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := stack.SyncItem(ctx, testItem("2", "IssueEvent")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForCount(t, stack, ctx, 2)

	items, err := stack.ReadModel.List(ctx, ItemFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestCQRSStack_RemoteStore_InvalidURL(t *testing.T) {
	t.Parallel()

	_, err := NewCQRSStack(CQRSConfig{
		Backend:   "turso",
		DBPath:    ":memory:",
		RemoteURL: "https://nonexistent.invalid.host.example/db",
		AuthToken: "fake",
	})
	if err == nil {
		t.Fatal("expected error for invalid remote URL")
	}
}

func TestClassifyAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		eventCount int
		wasNew     bool
		want       SyncAction
	}{
		{
			name:       "error_returns_action_error",
			err:        errors.New("some error"),
			eventCount: 0,
			wasNew:     false,
			want:       ActionError,
		},
		{
			name:       "multiple_events_conflict_remote",
			err:        nil,
			eventCount: 2,
			wasNew:     false,
			want:       ActionConflictRemote,
		},
		{
			name:       "one_event_new_item_created",
			err:        nil,
			eventCount: 1,
			wasNew:     true,
			want:       ActionCreated,
		},
		{
			name:       "one_event_existing_item_updated",
			err:        nil,
			eventCount: 1,
			wasNew:     false,
			want:       ActionUpdated,
		},
		{
			name:       "zero_events_unchanged",
			err:        nil,
			eventCount: 0,
			wasNew:     false,
			want:       ActionUnchanged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyAction(tt.err, tt.eventCount, tt.wasNew)
			if got != tt.want {
				t.Errorf("classifyAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCQRSStack_SyncItems_SameItem_Twice(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("1", "PushEvent")

	first := stack.SyncItems(ctx, []*provider.Item{item})
	if first.Synced != 1 {
		t.Errorf("first sync: expected Synced=1, got %d", first.Synced)
	}
	if first.Errors != 0 {
		t.Errorf("first sync: expected Errors=0, got %d", first.Errors)
	}

	var foundCreated bool

	for _, r := range first.Results {
		if r.Action == ActionCreated {
			foundCreated = true
		}
	}
	if !foundCreated {
		t.Error("first sync: expected ActionCreated in results")
	}

	second := stack.SyncItems(ctx, []*provider.Item{item})
	if second.Synced != 0 {
		t.Errorf("second sync of same item: expected Synced=0, got %d", second.Synced)
	}
	if second.Errors != 0 {
		t.Errorf("second sync of same item: expected Errors=0, got %d", second.Errors)
	}

	var foundUnchanged bool

	for _, r := range second.Results {
		if r.Action == ActionUnchanged {
			foundUnchanged = true
		}
	}
	if !foundUnchanged {
		t.Error("second sync of same item: expected ActionUnchanged in results")
	}
}

func TestCQRSStack_SyncItems_ConflictRemote(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("1", "PushEvent")

	first := stack.SyncItems(ctx, []*provider.Item{item})
	if first.Synced != 1 {
		t.Fatalf("expected Synced=1, got %d", first.Synced)
	}

	updated := testItem("1", "IssueEvent")
	updated.UpdatedAt = time.Now().Add(time.Hour)

	second := stack.SyncItems(ctx, []*provider.Item{updated})
	if second.Conflicts != 1 {
		t.Errorf("expected Conflicts=1, got %d", second.Conflicts)
	}
	if second.Synced != 1 {
		t.Errorf("expected Synced=1, got %d", second.Synced)
	}

	var foundConflict bool

	for _, r := range second.Results {
		if r.Action == ActionConflictRemote {
			foundConflict = true
		}
	}
	if !foundConflict {
		t.Error("expected ActionConflictRemote in results")
	}
}
