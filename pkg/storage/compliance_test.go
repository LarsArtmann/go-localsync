package storage_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/internal/database"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
	"github.com/larsartmann/go-localsync/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StorageFactory creates a Storage and a cleanup function.
type StorageFactory func(t *testing.T) (storage.Storage, func())

func makeItem(id, source, itemType, actor, repo string, createdAt time.Time) *provider.Item {
	return &provider.Item{
		ID:         types.NewItemID(id),
		Source:     types.NewProviderID(source),
		Type:       types.NewEventTypeID(itemType),
		ActorLogin: types.NewActorID(actor),
		RepoName:   types.NewRepoID(repo),
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
		RawJSON:    []byte(`{}`),
	}
}

// TestStorageCompliance_SQLite runs the compliance suite against SQLiteStorage.
func TestStorageCompliance_SQLite(t *testing.T) {
	factory := func(t *testing.T) (storage.Storage, func()) {
		db, err := database.Open(":memory:")
		require.NoError(t, err)
		s := storage.NewSQLiteStorage(db)
		return s, func() { _ = s.Close() }
	}
	testStorageCompliance(t, factory)
}

// TestStorageCompliance_Memory runs the compliance suite against MemoryStorage.
func TestStorageCompliance_Memory(t *testing.T) {
	factory := func(t *testing.T) (storage.Storage, func()) {
		s := storage.NewMemoryStorage()
		return s, func() { _ = s.Close() }
	}
	testStorageCompliance(t, factory)
}

func testStorageCompliance(t *testing.T, factory StorageFactory) {
	ctx := context.Background()

	t.Run("Upsert_and_GetByID", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		item := makeItem("1", "github", "PushEvent", "alice", "org/repo", time.Now())
		require.NoError(t, s.Upsert(ctx, item))

		got, err := s.GetByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, item.ID.Get(), got.ID.Get())
		assert.Equal(t, item.Source.Get(), got.Source.Get())
		assert.Equal(t, item.Type.Get(), got.Type.Get())
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		_, err := s.GetByID(ctx, types.NewItemID("nonexistent"))
		assert.ErrorIs(t, err, pkgerrors.ErrNotFound)
	})

	t.Run("Upsert_Idempotent", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		item := makeItem("1", "github", "PushEvent", "alice", "org/repo", time.Now())
		require.NoError(t, s.Upsert(ctx, item))

		item.ActorLogin = types.NewActorID("bob")
		require.NoError(t, s.Upsert(ctx, item))

		got, err := s.GetByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, "bob", got.ActorLogin.Get())
	})

	t.Run("UpsertBatch", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		items := []*provider.Item{
			makeItem("1", "github", "PushEvent", "alice", "org/repo1", now),
			makeItem("2", "github", "IssueEvent", "bob", "org/repo2", now.Add(1*time.Second)),
		}
		require.NoError(t, s.UpsertBatch(ctx, items))

		count, err := s.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("UpsertBatch_Empty", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		err := s.UpsertBatch(ctx, nil)
		assert.NoError(t, err)
	})

	t.Run("GetLatest", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "IssueEvent", "bob", "org/repo", now.Add(1*time.Hour))))

		latest, err := s.GetLatest(ctx)
		require.NoError(t, err)
		assert.Equal(t, "2", latest.ID.Get())
	})

	t.Run("GetLatest_Empty", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		_, err := s.GetLatest(ctx)
		assert.ErrorIs(t, err, pkgerrors.ErrNotFound)
	})

	t.Run("GetItems_Pagination", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		for i := range 5 {
			require.NoError(t, s.Upsert(ctx, makeItem(
				string(rune('a'+i)), "github", "PushEvent", "alice", "org/repo",
				now.Add(time.Duration(i)*time.Minute),
			)))
		}

		items, err := s.GetItems(ctx, 3, 0)
		require.NoError(t, err)
		assert.Len(t, items, 3)

		items, err = s.GetItems(ctx, 3, 3)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("GetItemsByType", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "IssueEvent", "bob", "org/repo", now.Add(1*time.Second))))

		items, err := s.GetItemsByType(ctx, "PushEvent", 10, 0)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "PushEvent", items[0].Type.Get())
	})

	t.Run("GetItemsByActor", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "PushEvent", "bob", "org/repo", now.Add(1*time.Second))))

		items, err := s.GetItemsByActor(ctx, "alice", 10, 0)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "alice", items[0].ActorLogin.Get())
	})

	t.Run("GetItemsByRepo", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo-a", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "PushEvent", "bob", "org/repo-b", now.Add(1*time.Second))))

		items, err := s.GetItemsByRepo(ctx, "org/repo-a", 10, 0)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "org/repo-a", items[0].RepoName.Get())
	})

	t.Run("GetItemsBySource", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "gitlab", "PushEvent", "bob", "org/repo", now.Add(1*time.Second))))

		items, err := s.GetItemsBySource(ctx, "github", 10, 0)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "github", items[0].Source.Get())
	})

	t.Run("GetItemsSince", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		old := now.Add(-2 * time.Hour)
		recent := now.Add(-5 * time.Minute)
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", old)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "PushEvent", "bob", "org/repo", recent)))

		cutoff := now.Add(-1 * time.Hour)
		items, err := s.GetItemsSince(ctx, cutoff)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "2", items[0].ID.Get())
	})

	t.Run("BatchGetByIDs", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "IssueEvent", "bob", "org/repo", now.Add(1*time.Second))))

		items, err := s.BatchGetByIDs(ctx, []types.ItemID{
			types.NewItemID("1"),
			types.NewItemID("2"),
			types.NewItemID("missing"),
		})
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("BatchGetByIDs_Empty", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		items, err := s.BatchGetByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, items)
	})

	t.Run("Delete", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", time.Now())))
		require.NoError(t, s.Delete(ctx, types.NewItemID("1")))

		_, err := s.GetByID(ctx, types.NewItemID("1"))
		assert.ErrorIs(t, err, pkgerrors.ErrNotFound)
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		err := s.Delete(ctx, types.NewItemID("nonexistent"))
		if err != nil {
			assert.ErrorIs(t, err, pkgerrors.ErrNotFound)
		}
	})

	t.Run("DeleteAll", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "IssueEvent", "bob", "org/repo", now.Add(1*time.Second))))

		require.NoError(t, s.DeleteAll(ctx))

		count, err := s.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("Count", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "IssueEvent", "bob", "org/repo", now.Add(1*time.Second))))

		count, err := s.Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("CountByType", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "PushEvent", "bob", "org/repo", now.Add(1*time.Second))))
		require.NoError(t, s.Upsert(ctx, makeItem("3", "github", "IssueEvent", "carol", "org/repo", now.Add(2*time.Second))))

		count, err := s.CountByType(ctx, "PushEvent")
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("GetTypes", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		now := time.Now()
		require.NoError(t, s.Upsert(ctx, makeItem("1", "github", "PushEvent", "alice", "org/repo", now)))
		require.NoError(t, s.Upsert(ctx, makeItem("2", "github", "IssueEvent", "bob", "org/repo", now.Add(1*time.Second))))

		types, err := s.GetTypes(ctx)
		require.NoError(t, err)
		sort.Strings(types)
		assert.Equal(t, []string{"IssueEvent", "PushEvent"}, types)
	})

	t.Run("Close", func(t *testing.T) {
		s, cleanup := factory(t)
		defer cleanup()

		assert.NoError(t, s.Close())
	})
}
