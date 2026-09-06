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
		ID:       id.NewItemID(),
		SourceID: id.NewSourceID(extID),
		Source:   id.NewProviderID(source),
		Type:     id.NewEventTypeID(eventType),
		Attributes: map[string]string{
			"actor_login": "user",
			"repo_name":   "repo",
		},
		CreatedAt: ts,
		UpdatedAt: ts,
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
				extID := id.NewSourceID(fmt.Sprintf("w%d-i%d", writerID, i))
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
			err := rm.Upsert(ctx, newItem)
			if err != nil {
				t.Errorf("background upsert: %v", err)
			}
		}
	}()

	for range readers {
		go func() {
			defer wg.Done()

			for range readsPerReader {
				_, err := rm.List(ctx, model.ItemFilter{})
				if err != nil {
					t.Errorf("concurrent list: %v", err)
				}
				_, err = rm.Count(ctx, model.ItemFilter{})
				if err != nil {
					t.Errorf("concurrent count: %v", err)
				}
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
	extID := id.NewSourceID("contested")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		for range 100 {
			item := concurrentTestItem(source, extID.Get(), "PushEvent")
			err := rm.Upsert(ctx, item)
			if err != nil {
				t.Errorf("concurrent upsert: %v", err)
			}
		}
	}()

	go func() {
		defer wg.Done()

		for range 100 {
			err := rm.Tombstone(ctx, source, extID, model.NewTombstone(model.ReasonUserHidden))
			if err != nil {
				t.Errorf("concurrent tombstone: %v", err)
			}
		}
	}()

	wg.Wait()
}
