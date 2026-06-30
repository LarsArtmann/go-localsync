package sync

import (
	"context"
	"errors"
	"testing"

	"charm.land/log/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestConflictAwareSyncer_NewItems(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")

	syncer, _ := newTestSyncer(items)
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()
	result, err := cas.SyncWithConflictDetection(
		ctx,
		testSyncOpts(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertInt(t, result.Fetched, 2, "Fetched")
	testutil.AssertInt(t, result.Upserted, 2, "Upserted")
	testutil.AssertInt(t, result.Conflicts, 0, "Conflicts")
	testutil.AssertInt(t, result.Skipped, 0, "Skipped")
}

func TestConflictAwareSyncer_NilOptions(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	_, err := cas.SyncWithConflictDetection(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConflictAwareSyncer_InvalidItems_CountedInErrors(t *testing.T) {
	t.Parallel()

	invalidItem := &provider.Item{ID: id.NewItemID()}
	validItem := testSyncItem("1", "PushEvent")

	mockProv := &testutil.MockProvider{Items: []*provider.Item{invalidItem, validItem}}
	store := &mockSyncStore{}
	syncer := NewSyncer(mockProv, store, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	result, err := cas.SyncWithConflictDetection(
		context.Background(),
		testSyncOpts(),
	)
	if !errors.Is(err, pkgerrors.ErrPartialSync) {
		t.Fatalf("expected ErrPartialSync for partial failure, got: %v", err)
	}
	testutil.AssertInt(t, result.Fetched, 2, "Fetched")
	testutil.AssertInt(t, result.Errors, 1, "Errors")
	testutil.AssertInt(t, result.Upserted, 1, "Upserted")
}

func TestConflictAwareSyncer_Conflicts(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")

	store := &mockSyncStore{actions: []SyncAction{ActionConflictRemote, ActionUnchanged}}
	mockProv := &testutil.MockProvider{Items: items}
	syncer := NewSyncer(mockProv, store, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()
	result, err := cas.SyncWithConflictDetection(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertInt(t, result.Fetched, 2, "Fetched")
	testutil.AssertInt(t, result.Conflicts, 1, "Conflicts")
	testutil.AssertInt(t, result.Upserted, 1, "Upserted")
	testutil.AssertInt(t, result.Skipped, 1, "Skipped")
}

// TestConflictAwareSyncer_LocalWinsConflictIsNotUpserted pins the contract
// that a LOCAL-wins conflict is counted as a conflict but NOT an upsert: when
// the resolver keeps the existing item, no new remote data is written, so
// Upserted must stay 0. This guards the intentional difference between
// SyncSummary.Synced (an event is emitted to re-confirm local) and
// ConflictResult.Upserted (remote data persisted) — the two metrics legitimately
// diverge for ActionConflictLocal, and this test prevents either side drifting.
func TestConflictAwareSyncer_LocalWinsConflictIsNotUpserted(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent")

	store := &mockSyncStore{actions: []SyncAction{ActionConflictLocal}}
	mockProv := &testutil.MockProvider{Items: items}
	syncer := NewSyncer(mockProv, store, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	result, err := cas.SyncWithConflictDetection(context.Background(), testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertInt(t, result.Conflicts, 1, "Conflicts (local-wins is a conflict)")
	testutil.AssertInt(t, result.Upserted, 0, "Upserted (local-wins writes no remote data)")
	testutil.AssertInt(t, result.Errors, 0, "Errors")
}

func TestConflictAwareSyncer_StoreErrors(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")

	store := &mockSyncStore{actions: []SyncAction{ActionCreated, ActionError}}
	mockProv := &testutil.MockProvider{Items: items}
	syncer := NewSyncer(mockProv, store, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()
	result, err := cas.SyncWithConflictDetection(ctx, testSyncOpts())
	if !errors.Is(err, pkgerrors.ErrPartialSync) {
		t.Fatalf("expected ErrPartialSync for partial failure, got: %v", err)
	}
	testutil.AssertInt(t, result.Fetched, 2, "Fetched")
	testutil.AssertInt(t, result.Upserted, 1, "Upserted")
	testutil.AssertInt(t, result.Errors, 1, "Errors")
}

func TestConflictAwareSyncer_AllInvalidItems(t *testing.T) {
	t.Parallel()

	invalidItem1 := &provider.Item{ID: id.NewItemID()}
	invalidItem2 := &provider.Item{ID: id.NewItemID()}

	mockProv := &testutil.MockProvider{Items: []*provider.Item{invalidItem1, invalidItem2}}
	store := &mockSyncStore{}
	syncer := NewSyncer(mockProv, store, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	result, err := cas.SyncWithConflictDetection(
		context.Background(),
		testSyncOpts(),
	)
	if !errors.Is(err, pkgerrors.ErrPartialSync) {
		t.Fatalf("expected ErrPartialSync for partial failure, got: %v", err)
	}
	testutil.AssertInt(t, result.Fetched, 2, "Fetched")
	testutil.AssertInt(t, result.Errors, 2, "Errors")
	testutil.AssertInt(t, result.Upserted, 0, "Upserted")
}

// TestConflictAwareSyncer_RetainsItemErrors guards the split-brain fix: the
// conflict-aware path must retain per-item error detail in ItemErrors (it
// previously only counted them), matching what SyncResult already does.
func TestConflictAwareSyncer_RetainsItemErrors(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")
	syncer, store := newTestSyncer(items)
	store.actions = []SyncAction{ActionCreated, ActionError}
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	result, err := cas.SyncWithConflictDetection(context.Background(), testSyncOpts())
	if !errors.Is(err, pkgerrors.ErrPartialSync) {
		t.Fatalf("expected ErrPartialSync for partial failure, got: %v", err)
	}

	testutil.AssertInt(t, result.Errors, 1, "Errors")

	if len(result.ItemErrors) != 1 {
		t.Fatalf("expected 1 per-item error retained, got %d", len(result.ItemErrors))
	}
}
