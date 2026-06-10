package sync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

type mockSyncStore struct {
	testutil.SyncStoreListBehavior

	synced       []*provider.Item
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

func (m *mockSyncStore) Count(_ context.Context, filter provider.ItemFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}

	if filter.Type != nil && m.typeCountErr != nil {
		return 0, m.typeCountErr
	}

	return int64(len(m.synced)), nil
}

func (m *mockSyncStore) GetTypes(_ context.Context) ([]string, error) {
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
	p := &testutil.MockProvider{Items: items}
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

func testDataItem(externalID, eventType string) *model.Item {
	now := time.Now()

	return &model.Item{
		ID:            id.NewItemID(),
		ExternalID:    id.NewExternalID(externalID),
		Source:        id.NewProviderID("github"),
		Type:          id.NewEventTypeID(eventType),
		ActorLogin:    id.NewActorID("testuser"),
		RepoName:      id.NewRepoID("test/repo"),
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: schema.CurrentVersion(),
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

	count, _ := store.Count(ctx, provider.ItemFilter{})
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

	count, _ := store.Count(ctx, provider.ItemFilter{})
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

func TestSyncer_Close(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{closeErr: errors.New("close failed")}
	syncer := NewSyncer(&testutil.MockProvider{}, store, log.Default())

	err := syncer.Close()
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
