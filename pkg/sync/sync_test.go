package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStorage implements storage.Storage for testing.
type mockStorage struct {
	items          []*provider.Item
	latestItem     *provider.Item
	upsertErr      error
	latestErr      error
	countResult    int64
	countErr       error
	typesResult    []string
	typesErr       error
	countByType    int64
	countByTypeErr error
	closeErr       error
}

func (m *mockStorage) Upsert(ctx context.Context, item *provider.Item) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}

	m.items = append(m.items, item)

	return nil
}

func (m *mockStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}

	return m.latestItem, nil
}

func (m *mockStorage) GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error) {
	return m.items, nil
}

func (m *mockStorage) GetItemsByType(
	ctx context.Context,
	itemType string,
	limit, offset int,
) ([]*provider.Item, error) {
	return m.items, nil
}

func (m *mockStorage) GetItemsByActor(
	ctx context.Context,
	actorLogin string,
	limit, offset int,
) ([]*provider.Item, error) {
	return m.items, nil
}

func (m *mockStorage) GetItemsByRepo(
	ctx context.Context,
	repoName string,
	limit, offset int,
) ([]*provider.Item, error) {
	return m.items, nil
}

func (m *mockStorage) Count(ctx context.Context) (int64, error) {
	return m.countResult, m.countErr
}

func (m *mockStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
	return m.countByType, m.countByTypeErr
}

func (m *mockStorage) GetTypes(ctx context.Context) ([]string, error) {
	return m.typesResult, m.typesErr
}

func (m *mockStorage) Close() error {
	return m.closeErr
}

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	name      string
	result    *provider.FetchResult
	rateLimit *provider.RateLimitInfo
	err       error
}

func (m *mockProvider) Name() string {
	if m.name == "" {
		return "mock"
	}

	return m.name
}

func (m *mockProvider) Fetch(
	ctx context.Context,
	opts *provider.FetchOptions,
) (*provider.FetchResult, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.result, nil
}

func (m *mockProvider) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.result, nil
}

func (m *mockProvider) GetRateLimit(ctx context.Context) (*provider.RateLimitInfo, error) {
	return m.rateLimit, m.err
}

func TestNewSyncer(t *testing.T) {
	t.Run("creates syncer with provided logger", func(t *testing.T) {
		mockStore := &mockStorage{}
		logger := log.New(nil)

		syncer := NewSyncer(nil, mockStore, logger)

		require.NotNil(t, syncer)
	})

	t.Run("uses default logger when nil", func(t *testing.T) {
		mockStore := &mockStorage{}

		syncer := NewSyncer(nil, mockStore, nil)

		require.NotNil(t, syncer)
		require.NotNil(t, syncer.logger)
	})
}

// newMockProviderWithItems creates a mock provider with standard test items.
func newMockProviderWithItems() *mockProvider {
	return &mockProvider{
		result: &provider.FetchResult{
			Items: []*provider.Item{
				{ID: types.NewItemID("1"), Type: types.NewEventTypeID("PushEvent")},
				{ID: types.NewItemID("2"), Type: types.NewEventTypeID("IssuesEvent")},
			},
		},
	}
}

func TestSyncer_Sync(t *testing.T) {
	t.Run("returns error for nil options", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		result, err := syncer.Sync(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("syncs items successfully", func(t *testing.T) {
		mockProv := newMockProviderWithItems()
		mockStore := &mockStorage{}
		syncer := NewSyncer(mockProv, mockStore, nil)

		result, err := syncer.Sync(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Len(t, mockStore.items, 2)
	})

	t.Run("returns error when fetch fails", func(t *testing.T) {
		mockProv := &mockProvider{
			err: errors.New("fetch error"),
		}
		mockStore := &mockStorage{}
		syncer := NewSyncer(mockProv, mockStore, nil)

		result, err := syncer.Sync(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("counts errors when upsert fails", func(t *testing.T) {
		mockProv := newMockProviderWithItems()
		mockStore := &mockStorage{
			upsertErr: errors.New("upsert error"),
		}
		syncer := NewSyncer(mockProv, mockStore, nil)

		result, err := syncer.Sync(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 2, result.Errors)
	})
}

func TestSyncer_SyncIncremental(t *testing.T) {
	t.Run("returns error for nil options", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		result, err := syncer.SyncIncremental(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestSyncer_GetStats(t *testing.T) {
	t.Run("returns stats successfully", func(t *testing.T) {
		mockStore := &mockStorage{
			countResult: 100,
			typesResult: []string{"PushEvent", "IssuesEvent"},
			countByType: 50,
		}
		syncer := NewSyncer(nil, mockStore, nil)

		stats, err := syncer.GetStats(context.Background())

		require.NoError(t, err)
		require.NotNil(t, stats)
		assert.Equal(t, int64(100), stats.TotalItems)
		assert.Equal(t, []string{"PushEvent", "IssuesEvent"}, stats.ItemTypes)
	})

	t.Run("returns error when count fails", func(t *testing.T) {
		mockStore := &mockStorage{
			countErr: errors.New("count error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)

		stats, err := syncer.GetStats(context.Background())

		require.Error(t, err)
		assert.Nil(t, stats)
		assert.Contains(t, err.Error(), "count error")
	})

	t.Run("returns error when get types fails", func(t *testing.T) {
		mockStore := &mockStorage{
			countResult: 100,
			typesErr:    errors.New("types error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)

		stats, err := syncer.GetStats(context.Background())

		require.Error(t, err)
		assert.Nil(t, stats)
		assert.Contains(t, err.Error(), "types error")
	})
}

func TestSyncer_Close(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		err := syncer.Close()

		require.NoError(t, err)
	})

	t.Run("returns error on close failure", func(t *testing.T) {
		mockStore := &mockStorage{
			closeErr: errors.New("close error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)

		err := syncer.Close()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "close error")
	})
}

func TestSyncResult(t *testing.T) {
	result := &SyncResult{
		Fetched: 100,
		Skipped: 20,
		Errors:  2,
	}

	assert.Equal(t, 100, result.Fetched)
	assert.Equal(t, 20, result.Skipped)
	assert.Equal(t, 2, result.Errors)
}

func TestProcessIncrementalItems(t *testing.T) {
	t.Run("skips items older than cutoff", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		cutoff := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		latestItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: cutoff,
		}

		items := []*provider.Item{
			{ID: types.NewItemID("2"), Type: types.NewEventTypeID("PushEvent"), CreatedAt: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)},
			{ID: types.NewItemID("3"), Type: types.NewEventTypeID("IssuesEvent"), CreatedAt: time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC)},
		}

		result := syncer.processIncrementalItems(context.Background(), latestItem, items)

		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 1, result.Skipped)
		assert.Equal(t, 1, len(mockStore.items))
		assert.Equal(t, "3", mockStore.items[0].ID.Get())
	})

	t.Run("handles nil latest item", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		items := []*provider.Item{
			{ID: types.NewItemID("1"), Type: types.NewEventTypeID("PushEvent"), CreatedAt: time.Now()},
		}

		result := syncer.processIncrementalItems(context.Background(), nil, items)

		require.NotNil(t, result)
		assert.Equal(t, 1, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Equal(t, 1, len(mockStore.items))
	})

	t.Run("handles identical timestamps at cutoff boundary", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		sameTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		cutoffItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: sameTime,
		}

		items := []*provider.Item{
			{ID: types.NewItemID("2"), Type: types.NewEventTypeID("PushEvent"), CreatedAt: sameTime},
			{ID: types.NewItemID("3"), Type: types.NewEventTypeID("IssuesEvent"), CreatedAt: sameTime.Add(1)},
		}

		result := syncer.processIncrementalItems(context.Background(), cutoffItem, items)

		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Equal(t, 2, len(mockStore.items))
	})

	t.Run("handles empty items slice", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		cutoffItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: time.Now(),
		}

		result := syncer.processIncrementalItems(context.Background(), cutoffItem, []*provider.Item{})

		require.NotNil(t, result)
		assert.Equal(t, 0, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Equal(t, 0, result.Errors)
	})

	t.Run("handles clock skew with future items", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		now := time.Now()
		futureTime := now.Add(24 * time.Hour)
		pastTime := now.Add(-24 * time.Hour)

		cutoffItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: now,
		}

		items := []*provider.Item{
			{ID: types.NewItemID("2"), Type: types.NewEventTypeID("PushEvent"), CreatedAt: pastTime},
			{ID: types.NewItemID("3"), Type: types.NewEventTypeID("IssuesEvent"), CreatedAt: futureTime},
		}

		result := syncer.processIncrementalItems(context.Background(), cutoffItem, items)

		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 1, result.Skipped)
		assert.Equal(t, 1, len(mockStore.items))
		assert.Equal(t, "3", mockStore.items[0].ID.Get())
	})
}
