package storage_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	"github.com/larsartmann/go-localsync/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Memory Storage Edge Cases", func() {
	var (
		ctx   context.Context
		store *storage.MemoryStorage
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = storage.NewMemoryStorage()
	})

	AfterEach(func() {
		_ = store.Close()
	})

	Describe("as a developer building a production sync pipeline", func() {
		Context("when 100 goroutines concurrently insert unique items", func() {
			It("should not lose any items despite concurrent writes", func() {
				const n = 100
				var wg sync.WaitGroup
				var errors atomic.Int32

				wg.Add(n)
				for i := range n {
					go func(seq int) {
						defer wg.Done()
						item := testhelpers.NewStorageItem(
							fmt.Sprintf("concurrent-%03d", seq),
							"PushEvent", "user", "repo", time.Now(),
						)
						err := store.Upsert(ctx, item)
						if err != nil {
							errors.Add(1)
						}
					}(i)
				}
				wg.Wait()

				Expect(int(errors.Load())).To(Equal(0))

				count, err := store.Count(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(n)))
			})
		})

		Context("when I query GetItemsSince with an exact timestamp boundary", func() {
			It("should exclude items created exactly at the boundary (strictly after)", func() {
				boundary := time.Now()

				beforeItem := testhelpers.NewStorageItem(
					"before",
					"PushEvent",
					"alice",
					"repo",
					boundary.Add(-1*time.Second),
				)
				exactItem := testhelpers.NewStorageItem(
					"exact",
					"PushEvent",
					"bob",
					"repo",
					boundary,
				)
				afterItem := testhelpers.NewStorageItem(
					"after",
					"PushEvent",
					"charlie",
					"repo",
					boundary.Add(1*time.Second),
				)

				Expect(store.Upsert(ctx, beforeItem)).To(Succeed())
				Expect(store.Upsert(ctx, exactItem)).To(Succeed())
				Expect(store.Upsert(ctx, afterItem)).To(Succeed())

				items, err := store.GetItemsSince(ctx, boundary)
				Expect(err).ToNot(HaveOccurred())

				ids := make(map[string]bool)
				for _, item := range items {
					ids[item.ExternalID.Get()] = true
				}
				Expect(ids).ToNot(HaveKey("before"))
				Expect(ids).ToNot(HaveKey("exact"))
				Expect(ids).To(HaveKey("after"))
			})
		})

		Context("when I call UpsertBatch with an empty slice", func() {
			It("should be a no-op without error", func() {
				Expect(store.UpsertBatch(ctx, []*provider.Item{})).To(Succeed())

				count, err := store.Count(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(0)))
			})
		})

		Context("when I call UpsertBatch with a nil slice", func() {
			It("should be a no-op without error", func() {
				Expect(store.UpsertBatch(ctx, nil)).To(Succeed())

				count, err := store.Count(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(0)))
			})
		})

		Context("when I BatchGetByExternalIDs with a mix of existing and missing IDs", func() {
			It("should return only the items that exist", func() {
				now := time.Now()
				Expect(
					store.Upsert(
						ctx,
						testhelpers.NewStorageItem("exists-1", "PushEvent", "alice", "repo", now),
					),
				).To(Succeed())
				Expect(
					store.Upsert(
						ctx,
						testhelpers.NewStorageItem("exists-2", "IssuesEvent", "bob", "repo", now),
					),
				).To(Succeed())

				items, err := store.BatchGetByExternalIDs(ctx, []types.ExternalID{
					types.NewExternalID("exists-1"),
					types.NewExternalID("missing-1"),
					types.NewExternalID("exists-2"),
					types.NewExternalID("missing-2"),
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(items).To(HaveLen(2))

				found := make(map[string]bool)
				for _, item := range items {
					found[item.ExternalID.Get()] = true
				}
				Expect(found).To(HaveKeyWithValue("exists-1", true))
				Expect(found).To(HaveKeyWithValue("exists-2", true))
			})
		})

		Context("when I BatchGetByExternalIDs with all missing IDs", func() {
			It("should return an empty slice without error", func() {
				items, err := store.BatchGetByExternalIDs(ctx, []types.ExternalID{
					types.NewExternalID("ghost-1"),
					types.NewExternalID("ghost-2"),
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(items).To(BeEmpty())
			})
		})

		Context("when I filter items by source provider", func() {
			It("should only return items from that source", func() {
				now := time.Now()
				ghItem := testhelpers.NewStorageItem("gh-1", "PushEvent", "alice", "repo", now)
				ghItem.Source = types.NewProviderID("github")

				glItem := testhelpers.NewStorageItem("gl-1", "PushEvent", "bob", "repo", now)
				glItem.Source = types.NewProviderID("gitlab")

				Expect(store.Upsert(ctx, ghItem)).To(Succeed())
				Expect(store.Upsert(ctx, glItem)).To(Succeed())

				items, err := store.GetItemsBySource(ctx, "github", 100, 0)
				Expect(err).ToNot(HaveOccurred())
				Expect(items).To(HaveLen(1))
				Expect(items[0].Source.Get()).To(Equal("github"))
			})
		})

		Context("when I paginate through a large dataset", func() {
			It("should allow complete traversal via offset and limit", func() {
				base := time.Now()
				for i := range 50 {
					Expect(store.Upsert(ctx, testhelpers.NewStorageItem(
						"item-"+string(rune('A'+i%26))+string(rune('0'+i/26)),
						"PushEvent", "user", "repo",
						base.Add(time.Duration(i)*time.Second),
					))).To(Succeed())
				}

				var allIDs []string
				pageSize := 10
				offset := 0

				for {
					page, err := store.GetItems(ctx, pageSize, offset)
					Expect(err).ToNot(HaveOccurred())
					if len(page) == 0 {
						break
					}
					for _, item := range page {
						allIDs = append(allIDs, item.ExternalID.Get())
					}
					offset += pageSize
				}

				Expect(allIDs).To(HaveLen(50))
			})
		})

		Context("when I upsert the same ID with different sources", func() {
			It("should keep only the latest version", func() {
				now := time.Now()
				v1 := testhelpers.NewStorageItem("same-id", "PushEvent", "alice", "repo1", now)
				v1.Source = types.NewProviderID("github")

				v2 := testhelpers.NewStorageItem("same-id", "IssuesEvent", "bob", "repo2", now)
				v2.Source = types.NewProviderID("gitlab")

				Expect(store.Upsert(ctx, v1)).To(Succeed())
				Expect(store.Upsert(ctx, v2)).To(Succeed())

				count, err := store.Count(ctx)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(int64(1)))

				item, err := store.GetByExternalID(ctx, types.NewExternalID("same-id"))
				Expect(err).ToNot(HaveOccurred())
				Expect(item.Type.Get()).To(Equal("IssuesEvent"))
				Expect(item.Source.Get()).To(Equal("gitlab"))
				Expect(item.ActorLogin.Get()).To(Equal("bob"))
			})
		})
	})
})
