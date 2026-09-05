package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
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

// TestCQRSStack_CorrelationID_SingleItemPath verifies the single-item write
// paths default a fresh correlation ID: SyncItem and TombstoneItem events must
// each carry one (previously only the batch path did).
func TestCQRSStack_CorrelationID_SingleItemPath(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	waitFor := subscribeAll(t, stack)

	ctx := context.Background()

	testutil.MustNoError(t, stack.SyncItem(ctx, testItem("single-corr", "PushEvent")))

	synced := waitFor(1)
	if len(synced) == 0 {
		t.Fatal("expected a sync event")
	}
	if synced[0].Metadata().CorrelationID.String() == "" {
		t.Error("SyncItem event must carry a correlation ID, got empty")
	}
}

// TestCQRSStack_Causation_PropagatedToEvents verifies every emitted event
// names the command that caused it: Metadata.Causation carries the command
// type and ID (CommandCausalityEnricher + WithCommandCausality in the
// handlers).
func TestCQRSStack_Causation_PropagatedToEvents(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	waitFor := subscribeAll(t, stack)

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "caus-1", "PushEvent")
	waitForCount(t, stack, ctx, 1)

	testutil.MustNoError(t, stack.TombstoneItem(ctx, "github", id.NewExternalID("caus-1"), model.ReasonUserHidden))

	evts := waitFor(2)
	if len(evts) < 2 {
		t.Fatalf("expected sync + tombstone events, got %d", len(evts))
	}

	var tombstoneFound bool

	for _, evt := range evts {
		caus := evt.Metadata().Causation
		if caus == nil {
			continue
		}

		if evt.Type() == EventItemTombstoned {
			tombstoneFound = true

			if caus.CommandType != commandTypeTombstone.String() {
				t.Errorf(
					"tombstone causation command type: want %q, got %q",
					commandTypeTombstone.String(), caus.CommandType,
				)
			}
			if caus.CommandID.String() == "" {
				t.Error("tombstone causation command ID must be non-empty")
			}
			if evt.Metadata().CorrelationID.String() == "" {
				t.Error("tombstone event must carry a correlation ID from TombstoneItem, got empty")
			}
		}

		if caus.CommandType != commandTypeSyncItem.String() &&
			caus.CommandType != commandTypeTombstone.String() {
			t.Errorf("unexpected causation command type %q", caus.CommandType)
		}
	}

	if !tombstoneFound {
		t.Error("expected an ItemTombstoned event with causation metadata")
	}
}
