package cqrs

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
	_ "modernc.org/sqlite"
)

func TestCQRSStack_SQLiteBackend_SyncAndDelete(t *testing.T) {
	t.Parallel()

	stack := newSQLiteMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	result := stack.SyncItems(ctx, testItems("1", "PushEvent", "2", "IssueEvent"))
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

	testutil.MustNoError(t, stack.TombstoneItem(ctx, "github", id.NewSourceID("1"), model.ReasonUpstreamGone))

	waitForCount(t, stack, ctx, 1)
}

func TestCQRSStack_SQLiteLocalStore_SyncAndReadModel(t *testing.T) {
	t.Parallel()

	stack := newSQLiteMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "1", "PushEvent")
	syncTestItem(t, stack, ctx, "2", "IssueEvent")

	waitForCount(t, stack, ctx, 2)

	items, err := stack.List(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestCQRSStack_Projection_SubscribesEvents(t *testing.T) {
	t.Parallel()

	stack := newSQLiteMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "outbox-1", "PushEvent")

	waitForCount(t, stack, ctx, 1)

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after outbox poller, got %d", count)
	}
}

func TestCQRSStack_SQLiteRestart_PreservesData(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "replay-test.db")

	ctx := context.Background()

	stack1, err := NewCQRSStack(CQRSConfig{Backend: "sqlite", DBPath: dbPath})
	testutil.MustNoError(t, err)

	testutil.MustNoError(t, stack1.SyncItem(ctx, testItem("replay-1", "PushEvent")))
	testutil.MustNoError(t, stack1.SyncItem(ctx, testItem("replay-2", "IssueEvent")))

	waitForCount(t, stack1, ctx, 2)

	testutil.MustNoError(t, stack1.Close())

	stack2, err := NewCQRSStack(CQRSConfig{Backend: "sqlite", DBPath: dbPath})
	testutil.MustNoError(t, err)
	defer func() { _ = stack2.Close() }()

	waitForCount(t, stack2, ctx, 2)

	count, err := stack2.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 2 {
		t.Errorf("expected count=2 after restart, got %d", count)
	}

	got, err := stack2.Get(ctx, "github", id.NewSourceID("replay-1"))
	testutil.MustNoError(t, err)
	if got == nil {
		t.Fatal("item replay-1 should be readable after restart")
	}
	testutil.AssertEqual(t, got.Type.Get(), "PushEvent", "Type after restart")

	testutil.MustNoError(t, stack2.SyncItem(ctx, testItem("replay-3", "WatchEvent")))
	waitForCount(t, stack2, ctx, 3)
}

// TestCQRSStack_SQLiteCheckpoint_PersistsAcrossRestarts verifies that the
// projectionhost checkpoint survives a restart: after stack1 syncs events and
// closes, the checkpoint table must hold a non-zero entry. The second stack
// reopens and its projectionhost worker drains from the checkpoint, proving
// catch-up is bounded (ADR-0006).
//
// The host is a one-shot batch-drainer: it runs once at startup, catches up,
// and exits. New events after startup are delivered via the live bus. So the
// checkpoint is written by the SECOND stack's host (which finds pre-existing
// events in the journal from stack1).
func TestCQRSStack_SQLiteCheckpoint_PersistsAcrossRestarts(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "checkpoint-test.db")
	ctx := context.Background()

	stack1, err := NewCQRSStack(CQRSConfig{Backend: "sqlite", DBPath: dbPath})
	testutil.MustNoError(t, err)

	for i := range 5 {
		testutil.MustNoError(t, stack1.SyncItem(ctx, testItem("cp-"+string(rune('a'+i)), "PushEvent")))
	}

	waitForCount(t, stack1, ctx, 5)
	testutil.MustNoError(t, stack1.Close())

	// Second stack reopens; its projectionhost worker finds the 5 pre-existing
	// events in the journal, drains them from checkpoint zero, and saves a
	// checkpoint. The live bus also delivers, but the host independently
	// processes and persists its checkpoint.
	stack2, err := NewCQRSStack(CQRSConfig{Backend: "sqlite", DBPath: dbPath})
	testutil.MustNoError(t, err)

	// Wait for the host to drain the journal and persist its checkpoint.
	waitForCount(t, stack2, ctx, 5)
	time.Sleep(200 * time.Millisecond)

	testutil.MustNoError(t, stack2.Close())

	// Verify the checkpoint table has a persisted entry for our projection.
	// The table name is "checkpoints" (from go-cqrs-lite SQLiteInitSchema).
	db, err := sql.Open("sqlite", dbPath)
	testutil.MustNoError(t, err)
	defer func() { _ = db.Close() }()

	var cpCount int
	row := db.QueryRowContext(ctx, `SELECT count(*) FROM checkpoints`)
	testutil.MustNoError(t, row.Scan(&cpCount))

	if cpCount == 0 {
		t.Fatal("expected at least one checkpoint row after restart catch-up + close, got 0")
	}
}

// TestCQRSStack_Close_WaitsForProjectionDrain guards the lifecycle invariant
// that Close blocks until the projection-host drain goroutine has finished
// (regression for a Close/Close race where the db could be closed while the
// drain was still projecting in-flight events). After Close returns, the drain
// channel must be closed — proving the goroutine exited before the db did.
func TestCQRSStack_Close_WaitsForProjectionDrain(t *testing.T) {
	t.Parallel()

	stack := newSQLiteMemoryStack(t)
	ctx := context.Background()

	for i := range 10 {
		testutil.MustNoError(t, stack.SyncItem(ctx, testItem("drain-"+string(rune('a'+i)), "PushEvent")))
	}
	waitForCount(t, stack, ctx, 10)

	drainDone := stack.drainDone
	if drainDone == nil {
		t.Fatal("expected a non-nil drainDone channel on the stack")
	}

	testutil.MustNoError(t, stack.Close())

	select {
	case <-drainDone:
		// drain completed before Close returned — correct.
	default:
		t.Fatal("Close returned before the projection drain goroutine finished (drainDone still open)")
	}
}
