package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/provider"
)

func TestCQRSStack_CorrelationID_Propagated(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	waitFor := subscribeAll(t, stack)

	ctx := context.Background()
	syncTestItems(t, stack, ctx, "corr-1", "PushEvent", "corr-2", "IssueEvent")

	evts := waitFor(2)

	if len(evts) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(evts))
	}

	firstCorrID := evts[0].Metadata().CorrelationID
	if firstCorrID.String() == "" {
		t.Error("expected correlation ID on first event, got empty")
	}

	for i, evt := range evts {
		corrID := evt.Metadata().CorrelationID
		if corrID != firstCorrID {
			t.Errorf("event %d: expected same correlation ID %s, got %s", i, firstCorrID, corrID)
		}
	}
}

func TestCQRSStack_CorrelationID_UniquePerSyncRun(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	waitFor := subscribeAll(t, stack)

	ctx := context.Background()

	_ = stack.SyncItems(ctx, []*provider.Item{testItem("run-1", "PushEvent")})
	_ = stack.SyncItems(ctx, []*provider.Item{testItem("run-2", "IssueEvent")})

	evts := waitFor(2)

	if len(evts) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(evts))
	}

	corrID1 := evts[0].Metadata().CorrelationID
	corrID2 := evts[1].Metadata().CorrelationID
	if corrID1.String() == "" || corrID2.String() == "" {
		t.Fatal("expected non-empty correlation IDs")
	}
	if corrID1 == corrID2 {
		t.Error("expected different correlation IDs for different sync runs")
	}
}
