package sync

import (
	"context"
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
	synced []*provider.Item
	items  []*provider.Item
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
	return m.items, nil
}

func (m *mockSyncStore) CountItems(_ context.Context, _ provider.ItemFilter) (int64, error) {
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

func (m *mockSyncStore) Close() error { return nil }

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
