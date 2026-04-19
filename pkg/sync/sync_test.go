package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/log/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testNilOptionsError(
	t *testing.T,
	syncFunc func(context.Context, *SyncOptions) (*SyncResult, error),
) {
	t.Helper()

	result, err := syncFunc(context.Background(), nil)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestNewSyncer(t *testing.T) {
	t.Run("creates syncer with provided logger", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{}
		logger := log.New(nil)

		syncer := NewSyncer(nil, mockStore, logger)

		require.NotNil(t, syncer)
	})

	t.Run("uses default logger when nil", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{}

		syncer := NewSyncer(nil, mockStore, nil)

		require.NotNil(t, syncer)
		require.NotNil(t, syncer.logger)
	})
}

func TestSyncer_Sync(t *testing.T) {
	t.Run("returns error for nil options", func(t *testing.T) {
		testNilOptionsError(t, func(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
			syncer := NewSyncer(nil, &testhelpers.MockStorage{}, nil)
			return syncer.Sync(ctx, opts)
		})
	})

	t.Run("returns error for empty source", func(t *testing.T) {
		syncer := NewSyncer(nil, &testhelpers.MockStorage{}, nil)
		result, err := syncer.Sync(context.Background(), &SyncOptions{Source: ""})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, pkgerrors.ErrInvalidInput))
	})

	t.Run("syncs items successfully", func(t *testing.T) {
		mockProv := testhelpers.NewMockProviderWithItems()
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(mockProv, mockStore, nil)

		result, err := syncer.Sync(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Len(t, mockStore.ItemsVal, 2)
	})

	t.Run("returns error when fetch fails", func(t *testing.T) {
		mockProv := &testhelpers.MockProvider{
			FetchErr: errors.New("fetch error"),
		}
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(mockProv, mockStore, nil)

		result, err := syncer.Sync(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("counts errors when upsert fails", func(t *testing.T) {
		mockProv := testhelpers.NewMockProviderWithItems()
		mockStore := &testhelpers.MockStorage{
			UpsertErrVal: errors.New("upsert error"),
		}
		syncer := NewSyncer(mockProv, mockStore, nil)

		result, err := syncer.Sync(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "batch upsert failed")
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 2, result.Errors)
	})

	t.Run("calls OnProgress callback", func(t *testing.T) {
		mockProv := testhelpers.NewMockProviderWithItems()
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(mockProv, mockStore, nil)

		var progressCalled bool
		var gotFetched, gotSkipped, gotErrors int

		result, err := syncer.Sync(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
			OnProgress: func(fetched, skipped, errors int) {
				progressCalled = true
				gotFetched = fetched
				gotSkipped = skipped
				gotErrors = errors
			},
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, progressCalled)
		assert.Equal(t, 2, gotFetched)
		assert.Equal(t, 0, gotSkipped)
		assert.Equal(t, 0, gotErrors)
	})
}

func TestSyncer_SyncIncremental(t *testing.T) {
	t.Run("returns error for nil options", func(t *testing.T) {
		testNilOptionsError(t, func(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
			syncer := NewSyncer(nil, &testhelpers.MockStorage{}, nil)
			return syncer.SyncIncremental(ctx, opts)
		})
	})
}

func TestSyncer_GetStats(t *testing.T) {
	t.Run("returns stats successfully", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{
			CountResultVal: 100,
			TypesResultVal: []string{"PushEvent", "IssuesEvent"},
			CountByTypeVal: 50,
		}
		syncer := NewSyncer(nil, mockStore, nil)

		stats, err := syncer.GetStats(context.Background())

		require.NoError(t, err)
		require.NotNil(t, stats)
		assert.Equal(t, int64(100), stats.TotalItems)
		assert.Equal(t, []string{"PushEvent", "IssuesEvent"}, stats.ItemTypes)
	})

	t.Run("returns error when count fails", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{
			CountErrVal: errors.New("count error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)

		stats, err := syncer.GetStats(context.Background())

		require.Error(t, err)
		assert.Nil(t, stats)
		assert.Contains(t, err.Error(), "count error")
	})

	t.Run("returns error when get types fails", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{
			CountResultVal: 100,
			TypesErrVal:    errors.New("types error"),
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
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		err := syncer.Close()

		require.NoError(t, err)
	})

	t.Run("returns error on close failure", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{
			CloseErrVal: errors.New("close error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)

		err := syncer.Close()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "close error")
	})
}

func TestProcessIncrementalItems(t *testing.T) {
	t.Run("skips items older than cutoff", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		cutoff := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		latestItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: cutoff,
		}

		items := []*provider.Item{
			testhelpers.NewMinimalTestItem(
				"2",
				"PushEvent",
				time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
			),
			testhelpers.NewMinimalTestItem(
				"3",
				"IssuesEvent",
				time.Date(2024, 1, 15, 13, 0, 0, 0, time.UTC),
			),
		}

		result, err := syncer.processIncrementalItems(context.Background(), latestItem, items)
		require.NoError(t, err)

		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 1, result.Skipped)
		assert.Equal(t, 1, len(mockStore.ItemsVal))
		assert.Equal(t, "3", mockStore.ItemsVal[0].ID.Get())
	})

	t.Run("handles nil latest item", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		items := []*provider.Item{
			testhelpers.NewMinimalTestItem("1", "PushEvent", time.Now()),
		}

		result, err := syncer.processIncrementalItems(context.Background(), nil, items)
		require.NoError(t, err)

		require.NotNil(t, result)
		assert.Equal(t, 1, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Equal(t, 1, len(mockStore.ItemsVal))
	})

	t.Run("handles identical timestamps at cutoff boundary", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		sameTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		cutoffItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: sameTime,
		}

		items := []*provider.Item{
			testhelpers.NewMinimalTestItem("2", "PushEvent", sameTime),
			testhelpers.NewMinimalTestItem("3", "IssuesEvent", sameTime.Add(1)),
		}

		result, err := syncer.processIncrementalItems(context.Background(), cutoffItem, items)
		require.NoError(t, err)

		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Equal(t, 2, len(mockStore.ItemsVal))
	})

	t.Run("handles empty items slice", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		cutoffItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: time.Now(),
		}

		result, err := syncer.processIncrementalItems(
			context.Background(),
			cutoffItem,
			[]*provider.Item{},
		)
		require.NoError(t, err)

		require.NotNil(t, result)
		assert.Equal(t, 0, result.Fetched)
		assert.Equal(t, 0, result.Skipped)
		assert.Equal(t, 0, result.Errors)
	})

	t.Run("handles clock skew with future items", func(t *testing.T) {
		mockStore := &testhelpers.MockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)

		now := time.Now()
		futureTime := now.Add(24 * time.Hour)
		pastTime := now.Add(-24 * time.Hour)

		cutoffItem := &provider.Item{
			ID:        types.NewItemID("1"),
			CreatedAt: now,
		}

		items := []*provider.Item{
			testhelpers.NewMinimalTestItem("2", "PushEvent", pastTime),
			testhelpers.NewMinimalTestItem("3", "IssuesEvent", futureTime),
		}

		result, err := syncer.processIncrementalItems(context.Background(), cutoffItem, items)
		require.NoError(t, err)

		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 1, result.Skipped)
		assert.Equal(t, 1, len(mockStore.ItemsVal))
		assert.Equal(t, "3", mockStore.ItemsVal[0].ID.Get())
	})
}
