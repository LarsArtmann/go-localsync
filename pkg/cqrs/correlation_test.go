package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
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
//
// Assertions run against events loaded from the STORE, not the bus: the
// watermill protocol maps the scalar CausationID and Custom metadata but not
// the typed ADR-0031 Causation pointer onto delivered messages, so the
// durable stream is the authoritative place to assert enrichment (delivered
// events keep the command.type/command.id Custom fallbacks, asserted too).
func TestCQRSStack_Causation_PropagatedToEvents(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	waitFor := subscribeAll(t, stack)

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "caus-1", "PushEvent")
	waitForCount(t, stack, ctx, 1)

	testutil.MustNoError(t, stack.TombstoneItem(ctx, "github", id.NewExternalID("caus-1"), model.ReasonUserHidden))

	delivered := waitFor(2)
	if len(delivered) < 2 {
		t.Fatalf("expected sync + tombstone events, got %d", len(delivered))
	}

	for _, evt := range delivered {
		if evt.Metadata().CorrelationID.String() == "" {
			t.Errorf("%s event must carry a correlation ID, got empty", evt.Type())
		}
	}

	ref := cqrsid.NewStreamRef(aggregateType, AggregateID("github", id.NewExternalID("caus-1")))

	stored, loadErr := stack.Load(ctx, ref)
	testutil.MustNoError(t, loadErr)

	if len(stored) < 2 {
		t.Fatalf("expected 2 stored events, got %d", len(stored))
	}

	for _, evt := range stored {
		caus := evt.Metadata().Causation
		if caus == nil {
			t.Errorf("%s: expected causation metadata on stored event, got nil", evt.Type())

			continue
		}

		wantType := commandTypeSyncItem.String()
		if evt.Type() == EventItemTombstoned {
			wantType = commandTypeTombstone.String()
		}

		if caus.CommandType != wantType {
			t.Errorf("%s: causation command type: want %q, got %q", evt.Type(), wantType, caus.CommandType)
		}

		if caus.CommandID.String() == "" {
			t.Errorf("%s: causation command ID must be non-empty", evt.Type())
		}
	}

	var tombstoneCausationOnBus bool

	for _, evt := range delivered {
		if evt.Type() != EventItemTombstoned {
			continue
		}

		if cmdType, ok := evt.Metadata().Custom[event.MetadataKeyCommandType]; ok && cmdType != "" {
			tombstoneCausationOnBus = true
		}
	}

	if !tombstoneCausationOnBus {
		t.Error("expected delivered tombstone event to keep the command.type custom causation fallback")
	}
}
