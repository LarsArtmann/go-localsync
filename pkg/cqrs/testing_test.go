package cqrs

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v2"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v2"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func mustNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertEqual[T comparable](t *testing.T, got, want T, label string) {
	t.Helper()

	if got != want {
		t.Errorf("expected %s=%v, got %v", label, want, got)
	}
}

func assertEventType(t *testing.T, evt event.Event, want event.Type) {
	t.Helper()

	assertEqual(t, evt.Type(), want, "type")
}

func assertItemType(t *testing.T, item *provider.Item, want string) {
	t.Helper()

	assertEqual(t, item.Type.Get(), want, "Type")
}

func assertExternalID(t *testing.T, item *provider.Item, want string) {
	t.Helper()

	assertEqual(t, item.ExternalID.Get(), want, "ExternalID")
}

func newMemoryStack(t *testing.T) *CQRSStack {
	t.Helper()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	mustNoError(t, err)

	return stack
}

func newTursoMemoryStack(t *testing.T) *CQRSStack {
	t.Helper()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: ":memory:"})
	mustNoError(t, err)

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
		Item: &provider.Item{
			ExternalID: id.NewExternalID(sourceID),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID(eventType),
		},
	}
}

func testDeletedState(sourceID string) SyncItemState {
	return SyncItemState{
		Item:    &provider.Item{ExternalID: id.NewExternalID(sourceID)},
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

func waitForCount(t *testing.T, stack *CQRSStack, ctx context.Context, expected int64) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		count, err := stack.Count(ctx)
		mustNoError(t, err)

		if count == expected {
			return
		}

		time.Sleep(time.Millisecond)
	}

	count, _ := stack.Count(ctx)
	t.Fatalf("timed out waiting for count=%d, got %d", expected, count)
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

	mustNoError(t, stack.Bus.SubscribeAll(handler))

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
