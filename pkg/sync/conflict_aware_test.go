package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"charm.land/log/v2"
	localsync "github.com/larsartmann/go-localfirst/pkg/sync"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConflictAwareSyncer(t *testing.T) {
	t.Run("creates with defaults", func(t *testing.T) {
		mockProv := &mockProvider{name: "test-provider"}
		mockStore := &mockStorage{}
		base := NewSyncer(mockProv, mockStore, nil)

		syncer := NewConflictAwareSyncer(base)

		require.NotNil(t, syncer)
		assert.Equal(t, "test-provider", syncer.nodeID)
		assert.NotNil(t, syncer.resolver)
		assert.NotNil(t, syncer.clock)
	})

	t.Run("creates with custom options", func(t *testing.T) {
		mockProv := &mockProvider{name: "test-provider"}
		mockStore := &mockStorage{}
		customResolver := localsync.NewLWWResolver[*provider.Item](
			func(item *provider.Item) time.Time {
				return item.CreatedAt
			},
		)

		base := NewSyncer(mockProv, mockStore, log.New(nil))
		syncer := NewConflictAwareSyncer(base,
			WithConflictResolver(customResolver),
			WithNodeID("custom-node"),
		)

		require.NotNil(t, syncer)
		assert.Equal(t, "custom-node", syncer.nodeID)
		assert.Equal(t, customResolver, syncer.resolver)
	})
}

func TestConflictAwareSyncer_SyncWithConflictDetection(t *testing.T) {
	t.Run("returns error for nil options", func(t *testing.T) {
		base := NewSyncer(&mockProvider{}, &mockStorage{}, nil)
		syncer := NewConflictAwareSyncer(base)

		result, err := syncer.SyncWithConflictDetection(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for empty source", func(t *testing.T) {
		base := NewSyncer(&mockProvider{}, &mockStorage{}, nil)
		syncer := NewConflictAwareSyncer(base)

		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{Source: ""})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, pkgerrors.ErrInvalidInput))
	})

	t.Run("upserts new items when no conflicts", func(t *testing.T) {
		now := time.Now()
		mockProv := newMockProviderWithTestItems(
			testhelpers.NewMinimalTestItem("1", "PushEvent", now),
			testhelpers.NewMinimalTestItem("2", "IssuesEvent", now),
		)
		mockStore := &mockStorage{}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 2, result.Upserted)
		assert.Equal(t, 0, result.Conflicts)
		assert.Equal(t, 0, result.Errors)
	})

	t.Run("detects and resolves conflicts", func(t *testing.T) {
		now := time.Now()
		existingItem := testhelpers.NewMinimalTestItem("1", "PushEvent", now)
		updatedItem := testhelpers.NewMinimalTestItem("1", "IssuesEvent", now.Add(time.Hour))

		mockProv := &mockProvider{
			result: &provider.FetchResult{
				Items: []*provider.Item{updatedItem},
			},
		}
		mockStore := &mockStorage{
			items: []*provider.Item{existingItem},
		}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Fetched)
		assert.Equal(t, 1, result.Conflicts)
		assert.Equal(t, 1, result.Upserted)
	})

	t.Run("returns error when fetch fails", func(t *testing.T) {
		mockProv := &mockProvider{
			err: errors.New("fetch error"),
		}
		mockStore := &mockStorage{}

		base := NewSyncer(mockProv, mockStore, nil)
		syncer := NewConflictAwareSyncer(base)
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "fetch error")
	})

	t.Run("counts upsert errors", func(t *testing.T) {
		mockProv := &mockProvider{
			result: &provider.FetchResult{
				Items: []*provider.Item{
					testhelpers.NewMinimalTestItem("1", "PushEvent", time.Now()),
				},
			},
		}
		mockStore := &mockStorage{
			upsertErr: errors.New("upsert error"),
		}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Fetched)
		assert.Equal(t, 1, result.Errors)
	})
}

func TestConflictAwareSyncer_GetVectorClock(t *testing.T) {
	base := NewSyncer(&mockProvider{name: "test"}, &mockStorage{}, nil)
	syncer := NewConflictAwareSyncer(base)

	vc := syncer.GetVectorClock()
	require.NotNil(t, vc)
	assert.Equal(t, 0, len(vc))

	syncer.clock.Increment("test")
	vc = syncer.GetVectorClock()
	assert.Equal(t, int64(1), vc.Get("test"))

	vc.Increment("test")
	assert.Equal(t, int64(1), syncer.clock.Get("test"), "clone should be independent")
}

func TestConflictAwareSyncer_SyncOperations(t *testing.T) {
	t.Run("returns operations for fetched items", func(t *testing.T) {
		now := time.Now()
		mockProv := newMockProviderWithTestItems(
			testhelpers.NewMinimalTestItem("1", "PushEvent", now),
			testhelpers.NewMinimalTestItem("2", "IssuesEvent", now),
		)
		mockStore := &mockStorage{}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		ops, result, err := syncer.SyncOperations(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, ops, 2)
		assert.Equal(t, 2, result.Fetched)
		assert.Equal(t, 2, result.Upserted)

		for _, op := range ops {
			assert.Equal(t, localsync.OpCreate, op.Type)
			assert.Equal(t, "mock", op.NodeID)
			assert.NotNil(t, op.Payload)
		}
	})

	t.Run("returns error for nil options", func(t *testing.T) {
		base := NewSyncer(&mockProvider{}, &mockStorage{}, nil)
		syncer := NewConflictAwareSyncer(base)
		ops, result, err := syncer.SyncOperations(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, ops)
		assert.Nil(t, result)
	})

	t.Run("operations have incrementing vector clocks", func(t *testing.T) {
		now := time.Now()
		mockProv := &mockProvider{
			result: &provider.FetchResult{
				Items: []*provider.Item{
					testhelpers.NewMinimalTestItem("1", "PushEvent", now),
					testhelpers.NewMinimalTestItem("2", "IssuesEvent", now),
					testhelpers.NewMinimalTestItem("3", "PullRequestEvent", now),
				},
			},
		}
		mockStore := &mockStorage{}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		ops, _, err := syncer.SyncOperations(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.Len(t, ops, 3)

		assert.Equal(t, int64(1), ops[0].VectorClock.Get("mock"))
		assert.Equal(t, int64(2), ops[1].VectorClock.Get("mock"))
		assert.Equal(t, int64(3), ops[2].VectorClock.Get("mock"))
	})
}

func TestConflictAwareSyncer_Close(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewConflictAwareSyncer(NewSyncer(&mockProvider{}, mockStore, nil))

		err := syncer.Close()

		require.NoError(t, err)
	})
}

func TestConflictAwareSyncer_LWWResolution(t *testing.T) {
	t.Run("remote wins with later timestamp", func(t *testing.T) {
		now := time.Now()
		existingItem := testhelpers.NewMinimalTestItem("1", "PushEvent", now)
		newerItem := testhelpers.NewMinimalTestItem("1", "IssuesEvent", now.Add(2*time.Hour))

		mockProv := &mockProvider{
			result: &provider.FetchResult{
				Items: []*provider.Item{newerItem},
			},
		}
		mockStore := &mockStorage{
			items: []*provider.Item{existingItem},
		}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, result.Conflicts)
		assert.Equal(t, 1, result.Upserted)

		require.Len(t, mockStore.items, 2)
		lastItem := mockStore.items[1]
		assert.Equal(t, types.NewEventTypeID("IssuesEvent"), lastItem.Type,
			"newer item (IssuesEvent) should win via LWW")
	})
}
