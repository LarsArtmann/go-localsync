package sync

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
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

func newTestSyncer(items []*provider.Item) (*Syncer, *cqrs.CQRSStack) {
	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	p := &mockProvider{items: items}
	logger := log.Default()

	return NewSyncer(p, stack, logger), stack
}

func testSyncItem(externalID, eventType string) *provider.Item {
	now := time.Now()

	return &provider.Item{
		ID:         types.NewItemID(),
		ExternalID: types.NewExternalID(externalID),
		Source:     types.NewProviderID("github"),
		Type:       types.NewEventTypeID(eventType),
		ActorLogin: types.NewActorID("testuser"),
		RepoName:   types.NewRepoID("test/repo"),
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

	syncer, stack := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 2 {
		t.Errorf("expected Fetched=2, got %d", result.Fetched)
	}

	count, _ := stack.Count(ctx)
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
}

func TestSyncer_Sync_EmptyResult(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
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
		ID: types.NewItemID(),
	}

	syncer, _ := newTestSyncer([]*provider.Item{invalidItem})
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
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

	syncer, stack := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}

	count, _ := stack.Count(ctx)
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
	_, err := syncer.Sync(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
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

func TestConflictAwareSyncer_NewItems(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{
		testSyncItem("1", "PushEvent"),
		testSyncItem("2", "IssueEvent"),
	}

	syncer, stack := newTestSyncer(items)
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()
	result, err := cas.SyncWithConflictDetection(
		ctx,
		&SyncOptions{Source: "testuser", MaxPages: 10},
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

	count, _ := stack.Count(ctx)
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
}

func TestConflictAwareSyncer_NoChange_Skipped(t *testing.T) {
	t.Parallel()

	item := testSyncItem("1", "PushEvent")

	syncer, stack := newTestSyncer([]*provider.Item{item})
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()

	err := stack.SyncItem(ctx, item)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	syncer.provider = &mockProvider{items: []*provider.Item{item}}
	result, err := cas.SyncWithConflictDetection(
		ctx,
		&SyncOptions{Source: "testuser", MaxPages: 10},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}
	if result.Upserted != 0 {
		t.Errorf("expected Upserted=0, got %d", result.Upserted)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", result.Skipped)
	}
}

func TestConflictAwareSyncer_RemoteWins(t *testing.T) {
	t.Parallel()

	oldItem := testSyncItem("1", "PushEvent")
	oldItem.UpdatedAt = time.Now().Add(-2 * time.Hour)

	syncer, stack := newTestSyncer([]*provider.Item{oldItem})
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()

	err := stack.SyncItem(ctx, oldItem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newItem := testSyncItem("1", "IssueEvent")
	newItem.UpdatedAt = time.Now()

	syncer.provider = &mockProvider{items: []*provider.Item{newItem}}
	result, err := cas.SyncWithConflictDetection(
		ctx,
		&SyncOptions{Source: "testuser", MaxPages: 10},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}
	if result.Upserted != 1 {
		t.Errorf("expected Upserted=1, got %d", result.Upserted)
	}
	if result.Conflicts != 1 {
		t.Errorf("expected Conflicts=1, got %d", result.Conflicts)
	}
}

func TestConflictAwareSyncer_RemoteWinsAlways(t *testing.T) {
	t.Parallel()

	newItem := testSyncItem("1", "PushEvent")
	newItem.UpdatedAt = time.Now()

	syncer, stack := newTestSyncer([]*provider.Item{newItem})
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()

	err := stack.SyncItem(ctx, newItem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oldRemote := testSyncItem("1", "IssueEvent")
	oldRemote.UpdatedAt = time.Now().Add(-2 * time.Hour)

	syncer.provider = &mockProvider{items: []*provider.Item{oldRemote}}
	result, err := cas.SyncWithConflictDetection(
		ctx,
		&SyncOptions{Source: "testuser", MaxPages: 10},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}
	if result.Conflicts != 1 {
		t.Errorf("expected Conflicts=1, got %d", result.Conflicts)
	}
	if result.Upserted != 1 {
		t.Errorf("expected Upserted=1, got %d", result.Upserted)
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

func TestSyncer_SyncIncremental_WithExistingItems(t *testing.T) {
	t.Parallel()

	oldItem := testSyncItem("1", "PushEvent")
	oldItem.CreatedAt = time.Now().Add(-2 * time.Hour)
	oldItem.UpdatedAt = time.Now().Add(-2 * time.Hour)

	newItem := testSyncItem("2", "IssueEvent")
	newItem.CreatedAt = time.Now()
	newItem.UpdatedAt = time.Now()

	mockProv := &mockProvider{items: []*provider.Item{newItem}}
	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	syncer := NewSyncer(mockProv, stack, log.Default())

	err := stack.SyncItem(ctx, oldItem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := syncer.SyncIncremental(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fetched != 1 {
		t.Errorf("expected Fetched=1, got %d", result.Fetched)
	}
}

func TestSyncer_processIncrementalItems_SkipsOldItems(t *testing.T) {
	t.Parallel()

	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	defer func() { _ = stack.Close() }()

	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, stack, log.Default())

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

	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	defer func() { _ = stack.Close() }()

	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, stack, log.Default())

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

	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	defer func() { _ = stack.Close() }()

	mockProv := &mockProvider{items: nil}
	syncer := NewSyncer(mockProv, stack, log.Default())

	invalidItem := &provider.Item{ID: types.NewItemID()}
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

func TestConflictAwareSyncer_InvalidItems_CountedInErrors(t *testing.T) {
	t.Parallel()

	invalidItem := &provider.Item{ID: types.NewItemID()}
	validItem := testSyncItem("1", "PushEvent")

	mockProv := &mockProvider{items: []*provider.Item{invalidItem, validItem}}
	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	defer func() { _ = stack.Close() }()

	syncer := NewSyncer(mockProv, stack, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	result, err := cas.SyncWithConflictDetection(
		context.Background(),
		&SyncOptions{Source: "testuser", MaxPages: 10},
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

	invalidItem1 := &provider.Item{ID: types.NewItemID()}
	invalidItem2 := &provider.Item{ID: types.NewItemID()}

	mockProv := &mockProvider{items: []*provider.Item{invalidItem1, invalidItem2}}
	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	defer func() { _ = stack.Close() }()

	syncer := NewSyncer(mockProv, stack, log.Default())
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	result, err := cas.SyncWithConflictDetection(
		context.Background(),
		&SyncOptions{Source: "testuser", MaxPages: 10},
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
