package sync

import (
	"context"
	"testing"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

type actionMockSyncStore struct {
	mockSyncStore

	actions []SyncAction
	errIdx  int
}

func (m *actionMockSyncStore) SyncItems(_ context.Context, items []*provider.Item) *SyncSummary {
	summary := &SyncSummary{Results: make([]ItemSyncResult, 0, len(items))}

	for range items {
		action := ActionCreated
		if m.errIdx < len(m.actions) {
			action = m.actions[m.errIdx]
			m.errIdx++
		}

		summary.Results = append(summary.Results, ItemSyncResult{Action: action})
		switch action {
		case ActionCreated, ActionUpdated, ActionConflictRemote, ActionConflictLocal:
			summary.Synced++
		case ActionError:
			summary.Errors++
		case ActionUnchanged:
		}
	}

	return summary
}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertInt(t, result.Fetched, 2, "Fetched")
	testutil.AssertInt(t, result.Errors, 1, "Errors")
	testutil.AssertInt(t, result.Upserted, 1, "Upserted")
}

func TestConflictAwareSyncer_Conflicts(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")

	store := &actionMockSyncStore{actions: []SyncAction{ActionConflictRemote, ActionUnchanged}}
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

func TestConflictAwareSyncer_StoreErrors(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")

	store := &actionMockSyncStore{actions: []SyncAction{ActionCreated, ActionError}}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertInt(t, result.Fetched, 2, "Fetched")
	testutil.AssertInt(t, result.Errors, 2, "Errors")
	testutil.AssertInt(t, result.Upserted, 0, "Upserted")
}
