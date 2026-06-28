package sync

import (
	"context"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestRegression_Sync_ErrorPropagation guards the P1.3 fix: Sync() must return a
// non-nil error and surface per-item failures when an item fails to sync,
// instead of silently swallowing them.
func TestRegression_Sync_ErrorPropagation(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")
	syncer, store := newTestSyncer(items)
	store.actions = []SyncAction{ActionCreated, ActionError}

	res, err := syncer.Sync(context.Background(), testSyncOpts())
	if err == nil {
		t.Fatal("expected non-nil error when an item fails to sync")
	}

	if res == nil || res.Errors != 1 {
		t.Fatalf("expected 1 error in result, got res=%+v", res)
	}

	if len(res.ItemErrors) != 1 {
		t.Errorf("expected 1 per-item error recorded, got %d", len(res.ItemErrors))
	}
}

// TestRegression_Sync_ReconcileOptIn guards the P2.4 design: reconciliation is
// opt-in via SyncOptions.Reconcile and off by default.
func TestRegression_Sync_ReconcileOptIn(t *testing.T) {
	t.Parallel()

	t.Run("off by default", func(t *testing.T) {
		t.Parallel()

		syncer, store := newTestSyncer(testSyncItems("1", "PushEvent"))
		store.reconcileResult = 5 // would tombstone 5 if called

		res, err := syncer.Sync(context.Background(), testSyncOpts())
		testutil.MustNoError(t, err)

		if store.reconcileCalled {
			t.Error("reconcile must NOT run by default")
		}

		if res.Tombstoned != 0 {
			t.Errorf("expected 0 tombstoned when reconcile is off, got %d", res.Tombstoned)
		}
	})

	t.Run("on when opted in", func(t *testing.T) {
		t.Parallel()

		items := testSyncItems("1", "PushEvent", "2", "IssueEvent")
		syncer, store := newTestSyncer(items)
		store.reconcileResult = 3

		opts := testSyncOpts()
		opts.Reconcile = true

		res, err := syncer.Sync(context.Background(), opts)
		testutil.MustNoError(t, err)

		if !store.reconcileCalled {
			t.Fatal("reconcile must run when opts.Reconcile is true")
		}

		if res.Tombstoned != 3 {
			t.Errorf("expected 3 tombstoned, got %d", res.Tombstoned)
		}

		if len(store.reconcileSeen) != 2 {
			t.Errorf("expected 2 seen keys forwarded to Reconcile, got %d", len(store.reconcileSeen))
		}
	})
}
