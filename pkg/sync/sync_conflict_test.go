package sync

import (
	"context"
	"testing"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
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

	items := []*provider.Item{
		testSyncItem("1", "PushEvent"),
		testSyncItem("2", "IssueEvent"),
	}

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
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Upserted != 2 {
		t.Errorf("expected Upserted=2, got %d", result.Upserted)
	}
	if result.Conflicts != 0 {
		t.Errorf("expected Conflicts=0, got %d", result.Conflicts)
	}
	if result.Skipped != 0 {
		t.Errorf("expected Skipped=0, got %d", result.Skipped)
	}
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

	mockProv := &mockProvider{items: []*provider.Item{invalidItem, validItem}}
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
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Errors != 1 {
		t.Errorf("expected Errors=1 (invalid item), got %d", result.Errors)
	}
	if result.Upserted != 1 {
		t.Errorf("expected Upserted=1 (valid item), got %d", result.Upserted)
	}
}

func TestConflictAwareSyncer_Conflicts(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{
		testSyncItem("1", "PushEvent"),
		testSyncItem("2", "IssueEvent"),
	}

	store := &actionMockSyncStore{actions: []SyncAction{ActionConflictRemote, ActionUnchanged}}
	mockProv := &mockProvider{items: items}
	syncer := NewSyncer(mockProv, store, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()
	result, err := cas.SyncWithConflictDetection(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Conflicts != 1 {
		t.Errorf("expected Conflicts=1, got %d", result.Conflicts)
	}
	if result.Upserted != 1 {
		t.Errorf("expected Upserted=1, got %d", result.Upserted)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", result.Skipped)
	}
}

func TestConflictAwareSyncer_StoreErrors(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{
		testSyncItem("1", "PushEvent"),
		testSyncItem("2", "IssueEvent"),
	}

	store := &actionMockSyncStore{actions: []SyncAction{ActionCreated, ActionError}}
	mockProv := &mockProvider{items: items}
	syncer := NewSyncer(mockProv, store, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()
	result, err := cas.SyncWithConflictDetection(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Upserted != 1 {
		t.Errorf("expected Upserted=1, got %d", result.Upserted)
	}
	if result.Errors != 1 {
		t.Errorf("expected Errors=1, got %d", result.Errors)
	}
}

func TestConflictAwareSyncer_AllInvalidItems(t *testing.T) {
	t.Parallel()

	invalidItem1 := &provider.Item{ID: id.NewItemID()}
	invalidItem2 := &provider.Item{ID: id.NewItemID()}

	mockProv := &mockProvider{items: []*provider.Item{invalidItem1, invalidItem2}}
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
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Errors != 2 {
		t.Errorf("expected Errors=2 (all invalid), got %d", result.Errors)
	}
	if result.Upserted != 0 {
		t.Errorf("expected Upserted=0, got %d", result.Upserted)
	}
}
