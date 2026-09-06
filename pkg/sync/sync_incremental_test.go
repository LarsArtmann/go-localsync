package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func newMockStoreWithItems(items ...*model.Item) *mockSyncStore {
	return &mockSyncStore{
		SyncStoreListBehavior: testutil.SyncStoreListBehavior{Items: items},
	}
}

func TestSyncer_processIncrementalItems_SkipsOldItems(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	mockProv := &testutil.MockProvider{Items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())

	latestItem := testDataItem("latest", "PushEvent")
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
	testutil.AssertEqual(t, result.Fetched, 2, "Fetched")
	testutil.AssertEqual(t, result.Skipped, 1, "Skipped")
}

func TestSyncer_processIncrementalItems_AllNewItems(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	mockProv := &testutil.MockProvider{Items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")

	result := syncer.processIncrementalItems(
		context.Background(),
		nil,
		items,
	)
	testutil.AssertEqual(t, result.Fetched, 2, "Fetched")
	testutil.AssertEqual(t, result.Skipped, 0, "Skipped")
}

func TestSyncer_processIncrementalItems_InvalidItemSkipped(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	mockProv := &testutil.MockProvider{Items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())

	invalidItem := &provider.Item{ID: id.NewItemID()}
	validItem := testSyncItem("1", "PushEvent")

	result := syncer.processIncrementalItems(
		context.Background(),
		nil,
		[]*provider.Item{invalidItem, validItem},
	)
	testutil.AssertEqual(t, result.Fetched, 2, "Fetched")
	testutil.AssertEqual(t, result.Errors, 1, "Errors")
}

func TestSyncer_reportProgress(t *testing.T) {
	t.Parallel()

	var called bool

	opts := &SyncOptions{
		Source:   "test",
		MaxPages: 1,
		OnProgress: func(fetched, skipped, errors int) {
			called = true

			testutil.AssertEqual(t, fetched, 1, "fetched")
			testutil.AssertEqual(t, skipped, 0, "skipped")
			testutil.AssertEqual(t, errors, 0, "errors")
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

func TestSyncer_SyncIncremental_WithExistingItems(t *testing.T) {
	t.Parallel()

	now := time.Now()
	existingItem := testSyncItem("existing", "PushEvent")
	existingItem.CreatedAt = now.Add(-1 * time.Hour)

	newItem := testSyncItem("new", "PushEvent")
	newItem.CreatedAt = now.Add(1 * time.Hour)

	oldItem := testSyncItem("old", "PushEvent")
	oldItem.CreatedAt = now.Add(-2 * time.Hour)

	store := newMockStoreWithItems(testDataItem("existing", "PushEvent"))
	mockProv := &testutil.MockProvider{Items: []*provider.Item{newItem, oldItem}}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertEqual(t, result.Fetched, 2, "Fetched")
	testutil.AssertEqual(t, result.Skipped, 1, "Skipped")
	testutil.AssertEqual(t, result.Errors, 0, "Errors")
}

func TestSyncer_SyncIncremental_AllItemsFiltered(t *testing.T) {
	t.Parallel()

	now := time.Now()
	existingItem := testSyncItem("existing", "PushEvent")
	existingItem.CreatedAt = now

	oldItem := testSyncItem("old", "PushEvent")
	oldItem.CreatedAt = now.Add(-1 * time.Hour)

	store := newMockStoreWithItems(testDataItem("existing", "PushEvent"))
	mockProv := &testutil.MockProvider{Items: []*provider.Item{oldItem}}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertEqual(t, result.Fetched, 1, "Fetched")
	testutil.AssertEqual(t, result.Skipped, 1, "Skipped")
}

func TestSyncer_SyncIncremental_ListItemsError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{SyncStoreListBehavior: testutil.SyncStoreListBehavior{ListErr: errors.New("list failed")}}
	mockProv := &testutil.MockProvider{Items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	_, err := syncer.SyncIncremental(context.Background(), testSyncOpts())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSyncer_Stats_CountError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{countErr: errors.New("count failed")}
	mockProv := &testutil.MockProvider{Items: nil}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	_, err := syncer.Stats(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSyncer_Stats_CountByTypeError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{countErr: errors.New("count by type failed")}
	mockProv := &testutil.MockProvider{}
	syncer := NewSyncer(mockProv, store, log.Default())
	defer func() { _ = syncer.Close() }()

	_, err := syncer.Stats(context.Background())
	if err == nil {
		t.Fatal("expected error when CountByType fails")
	}
}
