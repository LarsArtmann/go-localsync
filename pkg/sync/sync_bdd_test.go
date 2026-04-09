package sync_test

import (
	"context"
	"errors"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/internal/database"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
	"github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// syncTestWorld holds shared test state for BDD scenarios.
type syncTestWorld struct {
	ctx      context.Context
	provider *testhelpers.MockProvider
	storage  storage.Storage
	syncer   *sync.Syncer
	result   *sync.SyncResult
	err      error
	db       interface{ Close() error }
	count    int64
}

// sync invokes the syncer with default options.
func (w *syncTestWorld) sync() {
	w.result, w.err = w.syncer.Sync(w.ctx, &sync.SyncOptions{
		Source:   "testuser",
		MaxPages: 10,
	})
}

// syncIncremental invokes the syncer's incremental sync with default options.
func (w *syncTestWorld) syncIncremental() {
	w.result, w.err = w.syncer.SyncIncremental(w.ctx, &sync.SyncOptions{
		Source:   "testuser",
		MaxPages: 10,
	})
}

// countItems returns the count from storage.
func (w *syncTestWorld) countItems() int64 {
	w.count, w.err = w.storage.Count(w.ctx)
	Expect(w.err).ToNot(HaveOccurred())
	return w.count
}

var _ = Describe("Sync Engine", func() {
	var world syncTestWorld

	BeforeEach(func() {
		world = syncTestWorld{
			ctx:      context.Background(),
			provider: &testhelpers.MockProvider{},
		}

		// Create in-memory SQLite storage
		db, err := database.Open(":memory:")
		Expect(err).ToNot(HaveOccurred())
		world.db = db
		world.storage = storage.NewSQLiteStorage(db)
		world.syncer = sync.NewSyncer(world.provider, world.storage, log.New(nil))
	})

	AfterEach(func() {
		if world.db != nil {
			_ = world.db.Close()
		}
	})

	Describe("As a developer using go-localsync", func() {
		Context("when I sync data for the first time", func() {
			BeforeEach(func() {
				// Given: A provider with 3 events
				now := time.Now()
				world.provider.ItemsVal = []*provider.Item{
					testhelpers.NewTestItem("1", "PushEvent", now.Add(-2*time.Hour)),
					testhelpers.NewTestItem("2", "IssuesEvent", now.Add(-1*time.Hour)),
					testhelpers.NewTestItem("3", "PullRequestEvent", now),
				}
			})

			JustBeforeEach(func() {
				// When: I perform a full sync
				world.sync()
			})

			It("should succeed without errors", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should report all items as fetched", func() {
				Expect(world.result.Fetched).To(Equal(3))
			})

			It("should store all items in the database", func() {
				Expect(world.countItems()).To(Equal(int64(3)))
			})

			It("should preserve all event types", func() {
				types, err := world.storage.GetTypes(world.ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(types).To(ConsistOf("PushEvent", "IssuesEvent", "PullRequestEvent"))
			})
		})

		Context("when I sync the same data twice", func() {
			BeforeEach(func() {
				// Given: A provider with events
				now := time.Now()
				world.provider.ItemsVal = []*provider.Item{
					testhelpers.NewTestItem("1", "PushEvent", now),
				}
			})

			JustBeforeEach(func() {
				// When: I sync twice
				_, world.err = world.syncer.Sync(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
				Expect(world.err).ToNot(HaveOccurred())
				world.sync()
			})

			It("should not duplicate items", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.countItems()).To(Equal(int64(1)))
			})
		})

		Context("when I perform incremental sync with new items", func() {
			var firstSyncTime time.Time

			BeforeEach(func() {
				// Given: Storage already has one old item
				firstSyncTime = time.Now().Add(-2 * time.Hour)
				oldItem := testhelpers.NewTestItem("old", "PushEvent", firstSyncTime)
				err := world.storage.Upsert(world.ctx, oldItem)
				Expect(err).ToNot(HaveOccurred())

				// And: Provider returns both old and new items
				world.provider.ItemsVal = []*provider.Item{
					testhelpers.NewTestItem("old", "PushEvent", firstSyncTime),
					testhelpers.NewTestItem("new1", "IssuesEvent", time.Now().Add(-1*time.Hour)),
					testhelpers.NewTestItem("new2", "PullRequestEvent", time.Now()),
				}
			})

			JustBeforeEach(func() {
				// When: I perform incremental sync
				world.syncIncremental()
			})

			It("should succeed", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should skip items older than the latest stored", func() {
				Expect(world.result.Skipped).To(BeNumerically(">=", 1))
			})

			It("should store new items", func() {
				Expect(world.countItems()).To(Equal(int64(3)))
			})
		})

		Context("when the provider fails during sync", func() {
			BeforeEach(func() {
				// Given: A provider that fails
				world.provider.FetchErr = errors.New("network timeout")
			})

			JustBeforeEach(func() {
				// When: I try to sync
				world.sync()
			})

			It("should return an error", func() {
				Expect(world.err).To(HaveOccurred())
				Expect(world.err.Error()).To(ContainSubstring("network timeout"))
			})

			It("should not store any items", func() {
				Expect(world.countItems()).To(Equal(int64(0)))
			})
		})

		Context("when I request statistics", func() {
			BeforeEach(func() {
				// Given: Storage has mixed event types
				now := time.Now()
				items := []*provider.Item{
					testhelpers.NewTestItem("1", "PushEvent", now.Add(-3*time.Hour)),
					testhelpers.NewTestItem("2", "PushEvent", now.Add(-2*time.Hour)),
					testhelpers.NewTestItem("3", "IssuesEvent", now.Add(-1*time.Hour)),
				}
				world.provider.ItemsVal = items

				_, err := world.syncer.Sync(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
				Expect(err).ToNot(HaveOccurred())
			})

			JustBeforeEach(func() {
				// When: I get stats
				// Note: Stats are retrieved in the It blocks
			})

			It("should return total item count", func() {
				stats, err := world.syncer.GetStats(world.ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(stats.TotalItems).To(Equal(int64(3)))
			})

			It("should return counts per event type", func() {
				stats, err := world.syncer.GetStats(world.ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(stats.TypeCounts["PushEvent"]).To(Equal(int64(2)))
				Expect(stats.TypeCounts["IssuesEvent"]).To(Equal(int64(1)))
			})
		})

		Context("when storage fails during sync", func() {
			BeforeEach(func() {
				// Given: Provider returns items but we'll use a failing storage
				world.provider.ItemsVal = []*provider.Item{
					testhelpers.NewTestItem("1", "PushEvent", time.Now()),
					testhelpers.NewTestItem("2", "IssuesEvent", time.Now()),
				}

				// Use a storage that fails on upsert
				world.storage = &testhelpers.FailingStorage{}
				world.syncer = sync.NewSyncer(world.provider, world.storage, log.New(nil))
			})

			JustBeforeEach(func() {
				// When: I sync
				world.sync()
			})

			It("should complete sync but report errors", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.result.Errors).To(Equal(2))
			})
		})

		Context("when I pass nil options", func() {
			JustBeforeEach(func() {
				// When: I call sync with nil options
				world.result, world.err = world.syncer.Sync(world.ctx, nil)
			})

			It("should return an error", func() {
				Expect(world.err).To(HaveOccurred())
			})
		})
	})
})
