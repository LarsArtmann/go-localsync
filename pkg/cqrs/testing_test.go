package cqrs

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func assertEventType(t *testing.T, evt event.Event, want event.Type) {
	t.Helper()

	testutil.AssertEqual(t, evt.Type(), want, "type")
}

func unmarshalConflictPayload(t *testing.T, evt event.Event) ItemConflictFoundPayload {
	t.Helper()

	return unmarshalTestPayload[ItemConflictFoundPayload](t, evt)
}

func unmarshalSyncedPayload(t *testing.T, evt event.Event) ItemSyncedPayload {
	t.Helper()

	return unmarshalTestPayload[ItemSyncedPayload](t, evt)
}

func unmarshalTestPayload[T any](t *testing.T, evt event.Event) T {
	t.Helper()

	payload, err := event.DecodePayloadAuto[T](evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return payload
}

func newMemoryStack(t *testing.T) *CQRSStack {
	t.Helper()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	testutil.MustNoError(t, err)

	return stack
}

// setupMemoryStack is the standard parallel test fixture: it marks the test
// as parallel, creates an in-memory CQRS stack, registers cleanup, and returns
// the stack together with a background context. Extracts the t.Parallel() +
// newMemoryStack + defer Close + context.Background() boilerplate.
func setupMemoryStack(t *testing.T) (*CQRSStack, context.Context) {
	t.Helper()
	t.Parallel()

	stack := newMemoryStack(t)
	t.Cleanup(func() { _ = stack.Close() })

	return stack, context.Background()
}

func newSQLiteMemoryStack(t *testing.T) *CQRSStack {
	t.Helper()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "sqlite", DBPath: ":memory:"})
	testutil.MustNoError(t, err)

	return stack
}

func testSyncedPayload(sourceID, eventType string) ItemSyncedPayload {
	return ItemSyncedPayload{
		Source:    "github",
		SourceID:  sourceID,
		Type:      eventType,
		CreatedAt: time.Now().UnixNano(),
		UpdatedAt: time.Now().UnixNano(),
	}
}

func testActiveState(sourceID, eventType string) SyncItemState {
	return SyncItemState{
		Item: &model.Item{
			ExternalID: id.NewExternalID(sourceID),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID(eventType),
		},
	}
}

func testStateWithTimestamp(sourceID, eventType string, updatedAt time.Time) SyncItemState {
	return SyncItemState{
		Item: &model.Item{
			ExternalID: id.NewExternalID(sourceID),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID(eventType),
			UpdatedAt:  updatedAt,
		},
	}
}

func testTombstonedState(sourceID string) SyncItemState {
	return SyncItemState{
		Item: &model.Item{
			ExternalID: id.NewExternalID(sourceID),
			Tombstone:  model.NewTombstone(model.ReasonUpstreamGone),
		},
	}
}

func mustNewTestEvent(eventType event.Type, payload any) *event.ImmutableEvent {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	aggID := cqrsid.NewAggregateID()

	evt, err := event.NewEvent(eventType, aggID, aggregateType, 1, data)
	if err != nil {
		panic(err)
	}

	return evt
}

func testItem(sourceID, itemType string) *provider.Item {
	return &provider.Item{
		ExternalID: id.NewExternalID(sourceID),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID(itemType),
		Attributes: map[string]string{
			"actor_login": "testuser",
			"repo_name":   "owner/repo",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		RawJSON:   json.RawMessage(`{"test":true}`),
	}
}

// syncTestItem is a shorthand for "sync this sourceID/type as a test item".
// Reduces the `testutil.MustNoError(t, stack.SyncItem(ctx, testItem("X", "Y")))`
// pattern that appears in many tests to a single call.
func syncTestItem(t *testing.T, stack *CQRSStack, ctx context.Context, sourceID, itemType string) {
	t.Helper()

	testutil.MustNoError(t, stack.SyncItem(ctx, testItem(sourceID, itemType)))
}

// syncTestItems is a shorthand for syncing multiple items at once.
// Returns the SyncSummary so callers can assert on Synced/Conflicts/Errors.
// Reduces the `items := []*provider.Item{...}; _ = stack.SyncItems(ctx, items)` pattern
// at sites that ignore the return value.
func syncTestItems(t *testing.T, stack *CQRSStack, ctx context.Context, pairs ...string) {
	t.Helper()

	_ = stack.SyncItems(ctx, testItems(pairs...))
}

// syncTestItemsResult returns the SyncSummary for a multi-item sync.
// Used when the test needs to assert on Synced/Conflicts/Errors.
func syncTestItemsResult(t *testing.T, stack *CQRSStack, ctx context.Context, pairs ...string) *synclib.SyncSummary {
	t.Helper()

	return stack.SyncItems(ctx, testItems(pairs...))
}

// testItems constructs a slice of items from (id, type) pairs.
//
//	testItems("1", "PushEvent", "2", "IssueEvent")
func testItems(pairs ...string) []*provider.Item {
	return testutil.BuildPairs(testItem, pairs...)
}

func testDataItem(sourceID, itemType string) *model.Item {
	return toDataItem(testItem(sourceID, itemType))
}

// testFutureNow returns time.Now() truncated to milliseconds and offset by delta.
// Used to construct test timestamps that are clearly in the future or past.
func testFutureNow(delta time.Duration) time.Time {
	return time.Now().Truncate(time.Millisecond).Add(delta)
}

func waitForCount(t *testing.T, stack *CQRSStack, ctx context.Context, expected int64) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		count, err := stack.Count(ctx, model.ItemFilter{})
		testutil.MustNoError(t, err)

		if count == expected {
			return
		}

		time.Sleep(time.Millisecond)
	}

	count, _ := stack.Count(ctx, model.ItemFilter{})
	t.Fatalf("timed out waiting for count=%d, got %d", expected, count)
}

func newUpdatedAtLWWResolver(t *testing.T) *crdt.LWWResolver[*model.Item] {
	t.Helper()

	return NewUpdatedAtLWWResolver()
}

func subscribeAll(t *testing.T, stack *CQRSStack) func(minCount int) []event.Event {
	t.Helper()

	var captured []event.Event
	var mu sync.Mutex

	handler := func(_ context.Context, evt event.Event) error {
		mu.Lock()
		captured = append(captured, evt)
		mu.Unlock()

		return nil
	}

	testutil.MustNoError(t, stack.SubscribeAll(handler))

	return func(minCount int) []event.Event {
		t.Helper()

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			if len(captured) >= minCount {
				mu.Unlock()
				break
			}
			mu.Unlock()
			time.Sleep(time.Millisecond)
		}

		mu.Lock()
		evts := captured
		mu.Unlock()

		return evts
	}
}
