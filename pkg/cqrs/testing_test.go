package cqrs

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v2"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v2"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
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

	var payload T
	if err := json.Unmarshal(evt.Payload(), &payload); err != nil {
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

func testDeletedState(sourceID string) SyncItemState {
	return SyncItemState{
		Item:    &model.Item{ExternalID: id.NewExternalID(sourceID)},
		Deleted: true,
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
		ActorLogin: id.NewActorID("testuser"),
		RepoName:   id.NewRepoID("owner/repo"),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RawJSON:    json.RawMessage(`{"test":true}`),
	}
}

func testDataItem(sourceID, itemType string) *model.Item {
	return ToDataItem(testItem(sourceID, itemType))
}

func waitForCount(t *testing.T, stack *CQRSStack, ctx context.Context, expected int64) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		count, err := stack.Count(ctx)
		testutil.MustNoError(t, err)

		if count == expected {
			return
		}

		time.Sleep(time.Millisecond)
	}

	count, _ := stack.Count(ctx)
	t.Fatalf("timed out waiting for count=%d, got %d", expected, count)
}

func newUpdatedAtLWWResolver(t *testing.T) *crdt.LWWResolver[*model.Item] {
	t.Helper()

	resolver, err := crdt.NewLWWResolver[*model.Item](func(item *model.Item) time.Time {
		return item.UpdatedAt
	})
	if err != nil {
		t.Fatalf("unexpected LWW resolver error: %v", err)
	}

	return resolver
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

	testutil.MustNoError(t, stack.Bus.SubscribeAll(handler))

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
