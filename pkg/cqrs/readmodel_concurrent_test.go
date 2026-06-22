package cqrs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

func concurrentTestItem(source, extID, eventType string) *model.Item {
	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	return &model.Item{
		ID:         id.NewItemID(),
		ExternalID: id.NewExternalID(extID),
		Source:     id.NewProviderID(source),
		Type:       id.NewEventTypeID(eventType),
		ActorLogin: id.NewActorLogin("user"),
		RepoName:   id.NewRepoID("repo"),
		CreatedAt:  ts,
		UpdatedAt:  ts,
	}
}

func TestMemoryReadModel_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	const writers = 10
	const itemsPerWriter = 50

	var wg sync.WaitGroup
	wg.Add(writers)

	for w := range writers {
		go func(writerID int) {
			defer wg.Done()

			for i := range itemsPerWriter {
				extID := id.NewExternalID(fmt.Sprintf("w%d-i%d", writerID, i))
				item := concurrentTestItem("github", extID.Get(), "PushEvent")

				if err := rm.Upsert(ctx, item); err != nil {
					t.Errorf("writer %d upsert %d: %v", writerID, i, err)
				}
			}
		}(w)
	}

	wg.Wait()

	total := rm.Len()
	if total != writers*itemsPerWriter {
		t.Errorf("expected %d items, got %d", writers*itemsPerWriter, total)
	}
}

func TestMemoryReadModel_ConcurrentReadDuringWrites(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	item := concurrentTestItem("github", "ext1", "PushEvent")

	if err := rm.Upsert(ctx, item); err != nil {
		t.Fatal(err)
	}

	const readers = 20
	const readsPerReader = 100

	var wg sync.WaitGroup
	wg.Add(readers + 1)

	go func() {
		defer wg.Done()

		for range readers * readsPerReader {
			newItem := concurrentTestItem("github", "new", "PushEvent")
			_ = rm.Upsert(ctx, newItem)
		}
	}()

	for range readers {
		go func() {
			defer wg.Done()

			for range readsPerReader {
				_, _ = rm.List(ctx, model.ItemFilter{})
				_, _ = rm.Count(ctx, model.ItemFilter{})
				_, _ = rm.GetTypes(ctx)
			}
		}()
	}

	wg.Wait()
}

func TestMemoryReadModel_ConcurrentUpsertDelete(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	source := "github"
	extID := id.NewExternalID("contested")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		for range 100 {
			item := concurrentTestItem(source, extID.Get(), "PushEvent")
			_ = rm.Upsert(ctx, item)
		}
	}()

	go func() {
		defer wg.Done()

		for range 100 {
			_ = rm.Delete(ctx, source, extID)
		}
	}()

	wg.Wait()
}
