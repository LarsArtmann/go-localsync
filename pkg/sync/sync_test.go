package sync

import (
	"context"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	items []*provider.Item
	err   error
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Fetch(_ context.Context, _ *provider.FetchOptions) (*provider.FetchResult, error) {
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
	require.NoError(t, err)
	assert.Equal(t, 2, result.Fetched)

	count, _ := stack.Count(ctx)
	assert.Equal(t, int64(2), count)
}

func TestSyncer_Sync_EmptyResult(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Fetched)
	assert.Equal(t, 0, result.Errors)
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
	require.NoError(t, err)
	assert.Equal(t, 1, result.Fetched)
	assert.Equal(t, 1, result.Errors)
}

func TestSyncer_Sync_NilOptions(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	defer func() { _ = syncer.Close() }()

	_, err := syncer.Sync(context.Background(), nil)
	assert.Error(t, err)
}

func TestSyncer_SyncIncremental_FallsBackToFull(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{testSyncItem("1", "PushEvent")}

	syncer, stack := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Fetched)

	count, _ := stack.Count(ctx)
	assert.Equal(t, int64(1), count)
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
	require.NoError(t, err)

	stats, err := syncer.GetStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalItems)
	assert.Contains(t, stats.ItemTypes, "PushEvent")
	assert.Contains(t, stats.ItemTypes, "IssueEvent")
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
	result, err := cas.SyncWithConflictDetection(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Fetched)
	assert.Equal(t, 2, result.Upserted)
	assert.Equal(t, 0, result.Conflicts)
	assert.Equal(t, 0, result.Skipped)

	count, _ := stack.Count(ctx)
	assert.Equal(t, int64(2), count)
}

func TestConflictAwareSyncer_NoChange_Skipped(t *testing.T) {
	t.Parallel()

	item := testSyncItem("1", "PushEvent")

	syncer, stack := newTestSyncer([]*provider.Item{item})
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()

	require.NoError(t, stack.SyncItem(ctx, item))

	syncer.provider = &mockProvider{items: []*provider.Item{item}}
	result, err := cas.SyncWithConflictDetection(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Fetched)
	assert.Equal(t, 0, result.Upserted)
	assert.Equal(t, 1, result.Skipped)
}

func TestConflictAwareSyncer_RemoteWins(t *testing.T) {
	t.Parallel()

	oldItem := testSyncItem("1", "PushEvent")
	oldItem.UpdatedAt = time.Now().Add(-2 * time.Hour)

	syncer, stack := newTestSyncer([]*provider.Item{oldItem})
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()

	require.NoError(t, stack.SyncItem(ctx, oldItem))

	newItem := testSyncItem("1", "IssueEvent")
	newItem.UpdatedAt = time.Now()

	syncer.provider = &mockProvider{items: []*provider.Item{newItem}}
	result, err := cas.SyncWithConflictDetection(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Fetched)
	assert.Equal(t, 1, result.Upserted)
	assert.Equal(t, 1, result.Conflicts)
}

func TestConflictAwareSyncer_LocalWins(t *testing.T) {
	t.Parallel()

	newItem := testSyncItem("1", "PushEvent")
	newItem.UpdatedAt = time.Now()

	syncer, stack := newTestSyncer([]*provider.Item{newItem})
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	ctx := context.Background()

	require.NoError(t, stack.SyncItem(ctx, newItem))

	oldRemote := testSyncItem("1", "IssueEvent")
	oldRemote.UpdatedAt = time.Now().Add(-2 * time.Hour)

	syncer.provider = &mockProvider{items: []*provider.Item{oldRemote}}
	result, err := cas.SyncWithConflictDetection(ctx, &SyncOptions{Source: "testuser", MaxPages: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Fetched)
	assert.Equal(t, 1, result.Conflicts)
	assert.Equal(t, 1, result.Skipped)
}

func TestConflictAwareSyncer_NilOptions(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	cas := NewConflictAwareSyncer(syncer)
	defer func() { _ = cas.Close() }()

	_, err := cas.SyncWithConflictDetection(context.Background(), nil)
	assert.Error(t, err)
}
