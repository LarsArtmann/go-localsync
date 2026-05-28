package sync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

type mockProvider struct {
	items []*provider.Item
	err   error
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Fetch(
	_ context.Context,
	_ *provider.FetchOptions,
) (*provider.FetchResult, error) {
	return &provider.FetchResult{Items: m.items, HasMore: false}, m.err
}

func (m *mockProvider) FetchAll(_ context.Context, _ string, _ int) (*provider.FetchResult, error) {
	return &provider.FetchResult{Items: m.items, HasMore: false}, m.err
}

func (m *mockProvider) GetRateLimit(_ context.Context) (*provider.RateLimitInfo, error) {
	return &provider.RateLimitInfo{Limit: 5000, Remaining: 4999}, nil
}

type mockSyncStore struct {
	synced       []*provider.Item
	items        []*provider.Item
	listErr      error
	countErr     error
	typeCountErr error
	closeErr     error
}

func (m *mockSyncStore) SyncItems(_ context.Context, items []*provider.Item) *SyncSummary {
	summary := &SyncSummary{Results: make([]ItemSyncResult, 0, len(items))}

	for _, item := range items {
		m.synced = append(m.synced, item)
		summary.Synced++
		summary.Results = append(summary.Results, ItemSyncResult{
			SourceID: item.ExternalID.Get(),
			Action:   ActionCreated,
		})
	}

	return summary
}

func (m *mockSyncStore) ListItems(_ context.Context, _ provider.ItemFilter) ([]*provider.Item, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	return m.items, nil
}

func (m *mockSyncStore) CountItems(_ context.Context, filter provider.ItemFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}

	if filter.Type != nil && m.typeCountErr != nil {
		return 0, m.typeCountErr
	}

	return int64(len(m.synced)), nil
}

func (m *mockSyncStore) GetItemTypes(_ context.Context) ([]string, error) {
	seen := make(map[string]bool)

	for _, item := range m.synced {
		seen[item.Type.Get()] = true
	}

	types := make([]string, 0, len(seen))

	for t := range seen {
		types = append(types, t)
	}

	return types, nil
}

func (m *mockSyncStore) Close() error { return m.closeErr }

func testSyncOpts() *SyncOptions {
	return &SyncOptions{Source: "testuser", MaxPages: 10}
}

func newTestSyncer(items []*provider.Item) (*Syncer, *mockSyncStore) {
	store := &mockSyncStore{}
	p := &mockProvider{items: items}
	logger := log.Default()

	return NewSyncer(p, store, logger), store
}

func testSyncItem(externalID, eventType string) *provider.Item {
	now := time.Now()

	return &provider.Item{
		ID:         id.NewItemID(),
		ExternalID: id.NewExternalID(externalID),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID(eventType),
		ActorLogin: id.NewActorID("testuser"),
		RepoName:   id.NewRepoID("test/repo"),
		CreatedAt:  now,
		UpdatedAt:  now,
		RawJSON:    []byte(`{}`),
	}
}

func TestSyncer_Sync(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{
		testSyncItem("1", "PushEvent"),
		testSyncItem("2", "IssueEvent"),
	}

	syncer, store := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}

	count, _ := store.CountItems(ctx, provider.ItemFilter{})
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
}

func TestSyncer_Sync_EmptyResult(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 0 {
		t.Errorf("expected Fetched=0, got %d", result.Fetched)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}
}

func TestSyncer_Sync_InvalidItem(t *testing.T) {
	t.Parallel()

	invalidItem := &provider.Item{
		ID: id.NewItemID(),
	}

	syncer, _ := newTestSyncer([]*provider.Item{invalidItem})
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}
	if result.Errors != 1 {
		t.Errorf("expected Errors=1, got %d", result.Errors)
	}
}

func TestSyncer_Sync_NilOptions(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	defer func() { _ = syncer.Close() }()

	_, err := syncer.Sync(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSyncer_SyncIncremental_FallsBackToFull(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{testSyncItem("1", "PushEvent")}

	syncer, store := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}

	count, _ := store.CountItems(ctx, provider.ItemFilter{})
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

func TestSyncer_GetStats(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{
		testSyncItem("1", "PushEvent"),
		testSyncItem("2", "IssueEvent"),
		testSyncItem("3", "PushEvent"),
	}

	syncer, _ := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	_, err := syncer.Sync(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats, err := syncer.GetStats(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalItems != 3 {
		t.Errorf("expected TotalItems=3, got %d", stats.TotalItems)
	}
	found := false
	for _, t := range stats.ItemTypes {
		if t == "PushEvent" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ItemTypes to contain PushEvent, got %v", stats.ItemTypes)
	}
	found = false
	for _, t := range stats.ItemTypes {
		if t == "IssueEvent" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ItemTypes to contain IssueEvent, got %v", stats.ItemTypes)
	}
}

func TestSyncer_processIncrementalItems_SkipsOldItems(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())

	latestItem := testSyncItem("latest", "PushEvent")
	latestItem.CreatedAt = time.Now()

	oldItem := testSyncItem("old", "PushEvent")
	oldItem.CreatedAt = time.Now().Add(-5 * time.Hour)

	newItem := testSyncItem("new", "IssueEvent")
	newItem.CreatedAt = time.Now().Add(1 * time.Hour)

	result := syncer.processIncrementalItems(
		context.Background(),
		latestItem,
		[]*provider.Item{oldItem, newItem},
	)
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped=1 (old item), got %d", result.Skipped)
	}
}

func TestSyncer_processIncrementalItems_AllNewItems(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())

	items := []*provider.Item{
		testSyncItem("1", "PushEvent"),
		testSyncItem("2", "IssueEvent"),
	}

	result := syncer.processIncrementalItems(
		context.Background(),
		nil,
		items,
	)
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Skipped != 0 {
		t.Errorf("expected Skipped=0, got %d", result.Skipped)
	}
}

func TestSyncer_processIncrementalItems_InvalidItemSkipped(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())

	invalidItem := &provider.Item{ID: id.NewItemID()}
	validItem := testSyncItem("1", "PushEvent")

	result := syncer.processIncrementalItems(
		context.Background(),
		nil,
		[]*provider.Item{invalidItem, validItem},
	)
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Errors != 1 {
		t.Errorf("expected Errors=1 (invalid item), got %d", result.Errors)
	}
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

func TestSyncOptions_Validate(t *testing.T) {
	err := (&SyncOptions{}).Validate()
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error to contain 'required', got %v", err)
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

func TestSyncer_SyncIncremental_WithExistingItems(t *testing.T) {
	t.Parallel()

	now := time.Now()
	existingItem := testSyncItem("existing", "PushEvent")
	existingItem.CreatedAt = now.Add(-1 * time.Hour)

	newItem := testSyncItem("new", "PushEvent")
	newItem.CreatedAt = now.Add(1 * time.Hour)

	oldItem := testSyncItem("old", "PushEvent")
	oldItem.CreatedAt = now.Add(-2 * time.Hour)

	store := &mockSyncStore{items: []*provider.Item{existingItem}}
	mockProv := &mockProvider{items: []*provider.Item{newItem, oldItem}}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped=1 (old item before cutoff), got %d", result.Skipped)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}
}

func TestSyncer_SyncIncremental_AllItemsFiltered(t *testing.T) {
	t.Parallel()

	now := time.Now()
	existingItem := testSyncItem("existing", "PushEvent")
	existingItem.CreatedAt = now

	oldItem := testSyncItem("old", "PushEvent")
	oldItem.CreatedAt = now.Add(-1 * time.Hour)

	store := &mockSyncStore{items: []*provider.Item{existingItem}}
	mockProv := &mockProvider{items: []*provider.Item{oldItem}}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", result.Skipped)
	}
}

func TestSyncer_SyncIncremental_ListItemsError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{listErr: errors.New("list failed")}
	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	_, err := syncer.SyncIncremental(context.Background(), testSyncOpts())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSyncer_GetStats_CountError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{countErr: errors.New("count failed")}
	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	_, err := syncer.GetStats(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSyncer_GetStats_TypeCountError(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{testSyncItem("1", "PushEvent")}
	store := &mockSyncStore{}
	mockProv := &mockProvider{items: items}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	_, _ = syncer.Sync(ctx, testSyncOpts())

	store.typeCountErr = errors.New("type count failed")

	stats, err := syncer.GetStats(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalItems != 1 {
		t.Errorf("expected TotalItems=1, got %d", stats.TotalItems)
	}
	if len(stats.TypeCounts) != 0 {
		t.Errorf("expected empty TypeCounts on error, got %v", stats.TypeCounts)
	}
}

func TestSyncer_Close(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{closeErr: errors.New("close failed")}
	syncer := NewSyncer(&mockProvider{}, store, log.Default())

	err := syncer.Close()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

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
		case ActionCreated, ActionUpdated, ActionConflictRemote:
			summary.Synced++
		case ActionError:
			summary.Errors++
		case ActionUnchanged:
			// intentionally no-op
		}
	}

	return summary
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

func TestSyncer_reportProgress(t *testing.T) {
	t.Parallel()

	var called bool

	opts := &SyncOptions{
		Source:   "test",
		MaxPages: 1,
		OnProgress: func(fetched, skipped, errors int) {
			called = true

			if fetched != 1 {
				t.Errorf("expected fetched=1, got %d", fetched)
			}

			if skipped != 0 {
				t.Errorf("expected skipped=0, got %d", skipped)
			}

			if errors != 0 {
				t.Errorf("expected errors=0, got %d", errors)
			}
		},
	}

	syncer, _ := newTestSyncer([]*provider.Item{testSyncItem("1", "PushEvent")})
	defer func() { _ = syncer.Close() }()

	_, err := syncer.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("expected OnProgress to be called")
	}
}

func TestSyncer_reportProgress_NilCallback(t *testing.T) {
	t.Parallel()

	opts := &SyncOptions{
		Source:   "test",
		MaxPages: 1,
	}

	syncer, _ := newTestSyncer([]*provider.Item{testSyncItem("1", "PushEvent")})
	defer func() { _ = syncer.Close() }()

	_, err := syncer.Sync(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
