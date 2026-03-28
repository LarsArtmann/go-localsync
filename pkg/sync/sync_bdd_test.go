package sync_test

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/internal/database"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
	"github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// syncTestWorld holds shared test state for BDD scenarios.
type syncTestWorld struct {
	ctx      context.Context
	provider *fakeProvider
	storage  storage.Storage
	syncer   *sync.Syncer
	result   *sync.SyncResult
	err      error
	db       interface{ Close() error }
}

// fakeProvider simulates a provider for BDD testing.
type fakeProvider struct {
	name       string
	items      []*provider.Item
	fetchErr   error
	fetchCalls int
}

func (f *fakeProvider) Name() string {
	if f.name == "" {
		return "fake"
	}
	return f.name
}

func (f *fakeProvider) Fetch(ctx context.Context, opts *provider.FetchOptions) (*provider.FetchResult, error) {
	f.fetchCalls++
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return &provider.FetchResult{Items: f.items, HasMore: false}, nil
}

func (f *fakeProvider) FetchAll(ctx context.Context, source string, maxPages int) (*provider.FetchResult, error) {
	f.fetchCalls++
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return &provider.FetchResult{Items: f.items, HasMore: false}, nil
}

func (f *fakeProvider) GetRateLimit(ctx context.Context) (*provider.RateLimitInfo, error) {
	return &provider.RateLimitInfo{Limit: 5000, Remaining: 4999}, nil
}

// newTestItem creates a test item with sensible defaults.
func newTestItem(id, eventType string, createdAt time.Time) *provider.Item {
	return &provider.Item{
		ID:         types.NewItemID(id),
		Source:     types.NewProviderID("fake"),
		Type:       types.NewEventTypeID(eventType),
		ActorLogin: types.NewActorID("testuser"),
		RepoName:   types.NewRepoID("test/repo"),
		CreatedAt:  createdAt,
		RawJSON:    json.RawMessage(`{"id":"` + id + `"}`),
	}
}

var _ = Describe("Sync Engine", func() {
	var world syncTestWorld

	BeforeEach(func() {
		world = syncTestWorld{
			ctx:      context.Background(),
			provider: &fakeProvider{},
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
				world.provider.items = []*provider.Item{
					newTestItem("1", "PushEvent", now.Add(-2*time.Hour)),
					newTestItem("2", "IssuesEvent", now.Add(-1*time.Hour)),
					newTestItem("3", "PullRequestEvent", now),
				}
			})

			JustBeforeEach(func() {
				// When: I perform a full sync
				world.result, world.err = world.syncer.Sync(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
			})

			It("should succeed without errors", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should report all items as fetched", func() {
				Expect(world.result.Fetched).To(Equal(3))
			})

			It("should store all items in the database", func() {
				count, err := world.storage.Count(world.ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(3)))
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
				world.provider.items = []*provider.Item{
					newTestItem("1", "PushEvent", now),
				}
			})

			JustBeforeEach(func() {
				// When: I sync twice
				_, world.err = world.syncer.Sync(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
				Expect(world.err).ToNot(HaveOccurred())

				world.result, world.err = world.syncer.Sync(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
			})

			It("should not duplicate items", func() {
				Expect(world.err).ToNot(HaveOccurred())
				count, err := world.storage.Count(world.ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(1)))
			})
		})

		Context("when I perform incremental sync with new items", func() {
			var firstSyncTime time.Time

			BeforeEach(func() {
				// Given: Storage already has one old item
				firstSyncTime = time.Now().Add(-2 * time.Hour)
				oldItem := newTestItem("old", "PushEvent", firstSyncTime)
				err := world.storage.Upsert(world.ctx, oldItem)
				Expect(err).ToNot(HaveOccurred())

				// And: Provider returns both old and new items
				world.provider.items = []*provider.Item{
					newTestItem("old", "PushEvent", firstSyncTime),
					newTestItem("new1", "IssuesEvent", time.Now().Add(-1*time.Hour)),
					newTestItem("new2", "PullRequestEvent", time.Now()),
				}
			})

			JustBeforeEach(func() {
				// When: I perform incremental sync
				world.result, world.err = world.syncer.SyncIncremental(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
			})

			It("should succeed", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should skip items older than the latest stored", func() {
				Expect(world.result.Skipped).To(BeNumerically(">=", 1))
			})

			It("should store new items", func() {
				count, err := world.storage.Count(world.ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(3)))
			})
		})

		Context("when the provider fails during sync", func() {
			BeforeEach(func() {
				// Given: A provider that fails
				world.provider.fetchErr = errors.New("network timeout")
			})

			JustBeforeEach(func() {
				// When: I try to sync
				world.result, world.err = world.syncer.Sync(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
			})

			It("should return an error", func() {
				Expect(world.err).To(HaveOccurred())
				Expect(world.err.Error()).To(ContainSubstring("network timeout"))
			})

			It("should not store any items", func() {
				count, err := world.storage.Count(world.ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(0)))
			})
		})

		Context("when I request statistics", func() {
			BeforeEach(func() {
				// Given: Storage has mixed event types
				now := time.Now()
				items := []*provider.Item{
					newTestItem("1", "PushEvent", now.Add(-3*time.Hour)),
					newTestItem("2", "PushEvent", now.Add(-2*time.Hour)),
					newTestItem("3", "IssuesEvent", now.Add(-1*time.Hour)),
				}
				world.provider.items = items

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
				world.provider.items = []*provider.Item{
					newTestItem("1", "PushEvent", time.Now()),
					newTestItem("2", "IssuesEvent", time.Now()),
				}

				// Use a storage that fails on upsert
				world.storage = &failingStorage{}
				world.syncer = sync.NewSyncer(world.provider, world.storage, log.New(nil))
			})

			JustBeforeEach(func() {
				// When: I sync
				world.result, world.err = world.syncer.Sync(world.ctx, &sync.SyncOptions{
					Source:   "testuser",
					MaxPages: 10,
				})
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

// failingStorage simulates a storage that always fails.
type failingStorage struct{}

func (f *failingStorage) Upsert(ctx context.Context, item *provider.Item) error {
	return errors.New("disk full")
}

func (f *failingStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	return nil, errors.New("not found")
}

func (f *failingStorage) GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error) {
	return nil, nil
}

func (f *failingStorage) GetItemsByType(ctx context.Context, itemType string, limit, offset int) ([]*provider.Item, error) {
	return nil, nil
}

func (f *failingStorage) GetItemsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*provider.Item, error) {
	return nil, nil
}

func (f *failingStorage) GetItemsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*provider.Item, error) {
	return nil, nil
}

func (f *failingStorage) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

func (f *failingStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
	return 0, nil
}

func (f *failingStorage) GetTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (f *failingStorage) Close() error {
	return nil
}
