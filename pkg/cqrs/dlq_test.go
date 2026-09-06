package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// mustValidItemSyncedEvent builds an ItemSynced event the projection can
// decode cleanly (empty ItemID means "generate one" per parseItemID).
func mustValidItemSyncedEvent(t *testing.T, sourceID string) event.Event {
	t.Helper()

	evts, err := event.NewEvents(
		MustStreamID("github", id.NewSourceID(sourceID)),
		aggregateType,
		event.Version(1),
		[]event.Type{EventItemSynced},
		[]any{ItemSyncedPayload{
			Source:     "github",
			SourceID:   sourceID,
			Type:       "PushEvent",
			Attributes: map[string]string{"actor_login": "replayer"},
			CreatedAt:  1,
			UpdatedAt:  2,
		}},
	)
	testutil.MustNoError(t, err)

	return evts[0]
}

// TestStack_DLQ_Surface exercises the SDK dead-letter surface end to end:
// list, count, surgical delete, purge, and replay of a still-poisonous entry.
func TestStack_DLQ_Surface(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	entry := sampleDeadLetterEntry(t, "dlq-surface")

	testutil.MustNoError(t, stack.dlq.Store(ctx, entry))

	entries, err := stack.DeadLetters(ctx, entry.ProjectionName)
	testutil.MustNoError(t, err)

	if len(entries) != 1 || entries[0].EventID != entry.EventID {
		t.Fatalf("expected the stored entry, got %d entries", len(entries))
	}

	count, err := stack.DeadLetterCount(ctx)
	testutil.MustNoError(t, err)

	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// The sample payload carries an unparseable ItemID, so replay must fail
	// again: the entry stays captured and is reported as StillFailing.
	result, err := stack.ReplayDeadLetters(ctx, entry.ProjectionName)
	testutil.MustNoError(t, err)

	if len(result.StillFailing) != 1 || len(result.Replayed) != 0 {
		t.Fatalf("expected 1 still-failing, 0 replayed; got %d/%d", len(result.StillFailing), len(result.Replayed))
	}

	testutil.MustNoError(t, stack.DeleteDeadLetter(ctx, entry.ProjectionName, entry.EventID))

	count, err = stack.DeadLetterCount(ctx)
	testutil.MustNoError(t, err)

	if count != 0 {
		t.Fatalf("expected count 0 after delete, got %d", count)
	}

	// A successful replay path: store a VALID payload entry; the projection
	// handles it, so replay must move it to Replayed and drop it from the DLQ.
	ok := sampleDeadLetterEntry(t, "dlq-surface-valid")
	ok.Event = mustValidItemSyncedEvent(t, "dlq-surface-valid")
	testutil.MustNoError(t, stack.dlq.Store(ctx, ok))

	result, err = stack.ReplayDeadLetters(ctx, "")
	testutil.MustNoError(t, err)

	if len(result.Replayed) != 1 || len(result.StillFailing) != 0 {
		t.Fatalf("expected 1 replayed, 0 still-failing; got %d/%d", len(result.Replayed), len(result.StillFailing))
	}

	count, err = stack.DeadLetterCount(ctx)
	testutil.MustNoError(t, err)

	if count != 0 {
		t.Fatalf("replayed entry must leave the DLQ, count %d", count)
	}

	testutil.MustNoError(t, stack.PurgeDeadLetters(ctx, ""))
}

// TestStack_DLQ_ReplayWithoutHost pins the guard on the (defensive) nil-host
// path: a stack whose projection host never started cannot replay.
func TestStack_DLQ_ReplayWithoutHost(t *testing.T) {
	t.Parallel()

	stack := &CQRSStack{} //nolint:exhaustruct // testing the zero-value guard

	_, err := stack.ReplayDeadLetters(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when no projection host is running")
	}
}
