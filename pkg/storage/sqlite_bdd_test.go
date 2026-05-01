package storage_test

import (
	"context"
	"errors"
	"time"

	"github.com/larsartmann/go-localsync/internal/database"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// storageTestWorld holds shared test state for BDD scenarios.
type storageTestWorld struct {
	ctx   context.Context
	store storage.Storage
	db    interface{ Close() error }
	err   error
	items []*provider.Item
	item  *provider.Item
	count int64
	types []string
}

// getCountsAndTypes retrieves count and types from the store for test assertions.
func (w *storageTestWorld) getCountsAndTypes() {
	w.count, w.err = w.store.Count(w.ctx)
	Expect(w.err).ToNot(HaveOccurred())

	w.types, w.err = w.store.GetTypes(w.ctx)
}

// assertCountEquals asserts the store count matches the expected value.
func (w *storageTestWorld) assertCountEquals(expected int64) {
	count, err := w.store.Count(w.ctx)
	Expect(err).ToNot(HaveOccurred())
	Expect(count).To(Equal(expected))
}

// upsert is a helper to insert a test item into storage.
func (w *storageTestWorld) upsert(id, eventType, actor, repo string, createdAt time.Time) {
	_ = w.store.Upsert(w.ctx, testhelpers.NewStorageItem(id, eventType, actor, repo, createdAt))
}

// testItem holds parameters for creating a test storage item.
type testItem struct {
	id        string
	eventType string
	actor     string
	repo      string
	createdAt time.Time
}

// upsertItems inserts multiple test items into storage.
func (w *storageTestWorld) upsertItems(items ...testItem) {
	for _, item := range items {
		w.upsert(item.id, item.eventType, item.actor, item.repo, item.createdAt)
	}
}

// testDataMultiActor returns test items from multiple actors.
func testDataMultiActor(now time.Time) []testItem {
	a1 := testItem{id: "1", eventType: "PushEvent", actor: "alice", repo: "repo1", createdAt: now}
	a2 := testItem{id: "2", eventType: "IssuesEvent", actor: "bob", repo: "repo2", createdAt: now}
	a3 := testItem{id: "3", eventType: "PushEvent", actor: "alice", repo: "repo3", createdAt: now}
	return []testItem{a1, a2, a3}
}

// testDataMultiRepo returns test items from multiple repos.
func testDataMultiRepo(now time.Time) []testItem {
	r1 := testItem{
		id:        "1",
		eventType: "PushEvent",
		actor:     "alice",
		repo:      "owner/repo-a",
		createdAt: now,
	}
	r2 := testItem{
		id:        "2",
		eventType: "IssuesEvent",
		actor:     "bob",
		repo:      "owner/repo-b",
		createdAt: now,
	}
	r3 := testItem{
		id:        "3",
		eventType: "PushEvent",
		actor:     "charlie",
		repo:      "owner/repo-a",
		createdAt: now,
	}
	return []testItem{r1, r2, r3}
}

// pushEventsForFiltering returns test items with push events for filter tests.
func pushEventsForFiltering(now time.Time) []testItem {
	i1 := testItem{id: "1", eventType: "PushEvent", actor: "alice", repo: "repo1", createdAt: now}
	i2 := testItem{id: "2", eventType: "IssuesEvent", actor: "bob", repo: "repo2", createdAt: now}
	i3 := testItem{id: "3", eventType: "PushEvent", actor: "charlie", repo: "repo3", createdAt: now}
	i4 := testItem{
		id:        "4",
		eventType: "PullRequestEvent",
		actor:     "alice",
		repo:      "repo1",
		createdAt: now,
	}
	return []testItem{i1, i2, i3, i4}
}

// statisticsTestData returns test items for statistics tests.
func statisticsTestData(now time.Time) []testItem {
	return []testItem{
		{id: "1", eventType: "PushEvent", actor: "alice", repo: "repo", createdAt: now},
		{id: "2", eventType: "PushEvent", actor: "bob", repo: "repo", createdAt: now},
		{id: "3", eventType: "IssuesEvent", actor: "alice", repo: "repo", createdAt: now},
		{id: "4", eventType: "PullRequestEvent", actor: "bob", repo: "repo", createdAt: now},
	}
}

var _ = Describe("SQLite Storage", func() {
	var world storageTestWorld

	BeforeEach(func() {
		world = storageTestWorld{
			ctx: context.Background(),
		}

		db, err := database.Open(":memory:")
		Expect(err).ToNot(HaveOccurred())
		world.db = db
		world.store = storage.NewSQLiteStorage(db)
	})

	AfterEach(func() {
		if world.db != nil {
			_ = world.db.Close()
		}
	})

	Describe("as a developer building an offline-first dashboard", func() {
		Context("when I store GitHub events", func() {
			BeforeEach(func() {
				// Given: I have several GitHub events to store
				now := time.Now()
				world.items = []*provider.Item{
					testhelpers.NewStorageItem(
						"1",
						"PushEvent",
						"alice",
						"alice/repo1",
						now.Add(-3*time.Hour),
					),
					testhelpers.NewStorageItem(
						"2",
						"IssuesEvent",
						"bob",
						"bob/repo2",
						now.Add(-2*time.Hour),
					),
					testhelpers.NewStorageItem(
						"3",
						"PullRequestEvent",
						"alice",
						"alice/repo1",
						now.Add(-1*time.Hour),
					),
				}
			})

			JustBeforeEach(func() {
				// When: I store each event
				for _, item := range world.items {
					world.err = world.store.Upsert(world.ctx, item)
					if world.err != nil {
						break
					}
				}
			})

			It("should succeed without errors", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should persist all events", func() {
				world.assertCountEquals(3)
			})

			It("should preserve the complete JSON payload", func() {
				items, err := world.store.GetItems(world.ctx, 10, 0)
				Expect(err).ToNot(HaveOccurred())
				Expect(items).To(HaveLen(3))

				// Verify raw JSON is preserved
				for _, item := range items {
					Expect(item.RawJSON).ToNot(BeEmpty())
					Expect(item.RawJSON).To(ContainSubstring(`"id":"`))
				}
			})
		})

		Context("when I store the same event twice (idempotency)", func() {
			BeforeEach(func() {
				// Given: I have one event
				world.item = testhelpers.NewStorageItem(
					"duplicate-id",
					"PushEvent",
					"user",
					"repo",
					time.Now(),
				)
			})

			JustBeforeEach(func() {
				// When: I store it twice
				world.err = world.store.Upsert(world.ctx, world.item)
				Expect(world.err).ToNot(HaveOccurred())

				world.err = world.store.Upsert(world.ctx, world.item)
			})

			It("should not fail", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should only store one copy", func() {
				world.assertCountEquals(1)
			})
		})

		Context("when I query for the latest event", func() {
			BeforeEach(func() {
				// Given: I have events in chronological order
				now := time.Now()
				world.upsert("old", "PushEvent", "user", "repo", now.Add(-2*time.Hour))
				world.upsert("middle", "IssuesEvent", "user", "repo", now.Add(-1*time.Hour))
				world.upsert("newest", "PullRequestEvent", "user", "repo", now)
			})

			JustBeforeEach(func() {
				// When: I get the latest
				world.item, world.err = world.store.GetLatest(world.ctx)
			})

			It("should return the most recent event", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.item.ExternalID.Get()).To(Equal("newest"))
			})
		})

		Context("when I query for the latest event in an empty database", func() {
			JustBeforeEach(func() {
				// When: I query for latest in empty storage
				world.item, world.err = world.store.GetLatest(world.ctx)
			})

			It("should return ErrNotFound", func() {
				Expect(world.err).To(HaveOccurred())
				Expect(errors.Is(world.err, pkgerrors.ErrNotFound)).To(BeTrue())
			})
		})

		Context("when I filter events by type", func() {
			BeforeEach(func() {
				world.upsertItems(pushEventsForFiltering(time.Now())...)
			})

			JustBeforeEach(func() {
				// When: I filter by PushEvent
				world.items, world.err = world.store.GetItemsByType(world.ctx, "PushEvent", 100, 0)
			})

			It("should only return PushEvents", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.items).To(HaveLen(2))
				for _, item := range world.items {
					Expect(item.Type.Get()).To(Equal("PushEvent"))
				}
			})
		})

		Context("when I filter events by actor", func() {
			BeforeEach(func() {
				world.upsertItems(testDataMultiActor(time.Now())...)
			})

			JustBeforeEach(func() {
				// When: I filter by alice
				world.items, world.err = world.store.GetItemsByActor(world.ctx, "alice", 100, 0)
			})

			It("should only return alice's events", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.items).To(HaveLen(2))
				for _, item := range world.items {
					Expect(item.ActorLogin.Get()).To(Equal("alice"))
				}
			})
		})

		Context("when I filter events by repository", func() {
			BeforeEach(func() {
				world.upsertItems(testDataMultiRepo(time.Now())...)
			})

			JustBeforeEach(func() {
				// When: I filter by owner/repo-a
				world.items, world.err = world.store.GetItemsByRepo(
					world.ctx,
					"owner/repo-a",
					100,
					0,
				)
			})

			It("should only return events from that repo", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.items).To(HaveLen(2))
				for _, item := range world.items {
					Expect(item.RepoName.Get()).To(Equal("owner/repo-a"))
				}
			})
		})

		Context("when I request statistics", func() {
			BeforeEach(func() {
				world.upsertItems(statisticsTestData(time.Now())...)
			})

			JustBeforeEach(func() {
				world.getCountsAndTypes()
			})

			It("should return the total count", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.count).To(Equal(int64(4)))
			})

			It("should return all distinct types", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.types).To(ConsistOf("PushEvent", "IssuesEvent", "PullRequestEvent"))
			})

			It("should count by type correctly", func() {
				pushCount, err := world.store.CountByType(world.ctx, "PushEvent")
				Expect(err).ToNot(HaveOccurred())
				Expect(pushCount).To(Equal(int64(2)))

				issuesCount, err := world.store.CountByType(world.ctx, "IssuesEvent")
				Expect(err).ToNot(HaveOccurred())
				Expect(issuesCount).To(Equal(int64(1)))
			})
		})

		Context("when I paginate through events", func() {
			BeforeEach(func() {
				// Given: I have many events
				now := time.Now()
				for i := range 25 {
					item := testhelpers.NewStorageItem(
						string(rune('A'+i)),
						"PushEvent",
						"user",
						"repo",
						now.Add(time.Duration(i)*time.Minute),
					)
					_ = world.store.Upsert(world.ctx, item)
				}
			})

			It("should support offset-based pagination", func() {
				// When: I request page 1
				page1, err := world.store.GetItems(world.ctx, 10, 0)
				Expect(err).ToNot(HaveOccurred())
				Expect(page1).To(HaveLen(10))

				// When: I request page 2
				page2, err := world.store.GetItems(world.ctx, 10, 10)
				Expect(err).ToNot(HaveOccurred())
				Expect(page2).To(HaveLen(10))

				// When: I request page 3 (partial)
				page3, err := world.store.GetItems(world.ctx, 10, 20)
				Expect(err).ToNot(HaveOccurred())
				Expect(page3).To(HaveLen(5))
			})
		})
	})
})
