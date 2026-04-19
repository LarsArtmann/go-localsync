package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConflictAwareSyncer(t *testing.T) {
	t.Run("creates syncer wrapping base", func(t *testing.T) {
		mockProv := &testhelpers.MockProvider{NameVal: "test-provider"}
		mockStore := &testhelpers.MockStorage{}
		base := NewSyncer(mockProv, mockStore, nil)

		syncer := NewConflictAwareSyncer(base)

		require.NotNil(t, syncer)
		assert.Equal(t, base, syncer.Syncer)
	})
}

func TestConflictAwareSyncer_SyncWithConflictDetection(t *testing.T) {
	t.Run("returns error for nil options", func(t *testing.T) {
		base := NewSyncer(&testhelpers.MockProvider{}, &testhelpers.MockStorage{}, nil)
		syncer := NewConflictAwareSyncer(base)

		result, err := syncer.SyncWithConflictDetection(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for empty source", func(t *testing.T) {
		base := NewSyncer(&testhelpers.MockProvider{}, &testhelpers.MockStorage{}, nil)
		syncer := NewConflictAwareSyncer(base)

		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{Source: ""})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, pkgerrors.ErrInvalidInput))
	})

	t.Run("upserts new items when no conflicts", func(t *testing.T) {
		now := time.Now()
		mockProv := &testhelpers.MockProvider{
			ItemsVal: []*provider.Item{
				testhelpers.NewMinimalTestItem("1", "PushEvent", now),
				testhelpers.NewMinimalTestItem("2", "IssuesEvent", now),
			},
		}
		mockStore := &testhelpers.MockStorage{}

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

		mockProv := &testhelpers.MockProvider{
			ItemsVal: []*provider.Item{updatedItem},
		}
		mockStore := &testhelpers.MockStorage{
			ItemsVal: []*provider.Item{existingItem},
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
		mockProv := &testhelpers.MockProvider{
			FetchErr: errors.New("fetch error"),
		}
		mockStore := &testhelpers.MockStorage{}

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
		mockProv := &testhelpers.MockProvider{
			ItemsVal: []*provider.Item{
				testhelpers.NewMinimalTestItem("1", "PushEvent", time.Now()),
			},
		}
		mockStore := &testhelpers.MockStorage{
			UpsertErrVal: errors.New("upsert error"),
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

	t.Run("skips unchanged items", func(t *testing.T) {
		now := time.Now()
		existingItem := testhelpers.NewMinimalTestItem("1", "PushEvent", now)

		mockProv := &testhelpers.MockProvider{
			ItemsVal: []*provider.Item{existingItem},
		}
		mockStore := &testhelpers.MockStorage{
			ItemsVal: []*provider.Item{existingItem},
		}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Fetched)
		assert.Equal(t, 1, result.Skipped)
		assert.Equal(t, 0, result.Conflicts)
		assert.Equal(t, 0, result.Upserted)
	})
}

func TestConflictAwareSyncer_LWWResolution(t *testing.T) {
	t.Run("remote wins with later timestamp", func(t *testing.T) {
		now := time.Now()
		existingItem := testhelpers.NewMinimalTestItem("1", "PushEvent", now)
		newerItem := testhelpers.NewMinimalTestItem("1", "IssuesEvent", now.Add(2*time.Hour))

		mockProv := &testhelpers.MockProvider{
			ItemsVal: []*provider.Item{newerItem},
		}
		mockStore := &testhelpers.MockStorage{
			ItemsVal: []*provider.Item{existingItem},
		}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, result.Conflicts)
		assert.Equal(t, 1, result.Upserted)
	})

	t.Run("local wins with later timestamp", func(t *testing.T) {
		now := time.Now()
		localItem := testhelpers.NewMinimalTestItem("1", "PushEvent", now.Add(2*time.Hour))
		olderRemote := testhelpers.NewMinimalTestItem("1", "IssuesEvent", now)

		mockProv := &testhelpers.MockProvider{
			ItemsVal: []*provider.Item{olderRemote},
		}
		mockStore := &testhelpers.MockStorage{
			ItemsVal: []*provider.Item{localItem},
		}

		syncer := NewConflictAwareSyncer(NewSyncer(mockProv, mockStore, nil))
		result, err := syncer.SyncWithConflictDetection(context.Background(), &SyncOptions{
			Source:   "testuser",
			MaxPages: 1,
		})

		require.NoError(t, err)
		assert.Equal(t, 1, result.Conflicts)
		assert.Equal(t, 1, result.Upserted)
	})
}
