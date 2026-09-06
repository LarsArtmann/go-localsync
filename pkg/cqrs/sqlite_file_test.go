package cqrs

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// newFileDBStack returns a stack backed by a real SQLite file in a t.TempDir,
// exercising WAL, the connection pool, and restart behavior that :memory:
// hides. Always close the returned stack.
func newFileDBStack(t *testing.T) (*CQRSStack, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "localsync.db")

	stack, err := NewCQRSStack(CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
	testutil.MustNoError(t, err)

	return stack, dbPath
}

// TestSQLiteFileDB_Roundtrip is the basic file-backed sanity: sync, project,
// read back through the read model.
func TestSQLiteFileDB_Roundtrip(t *testing.T) {
	t.Parallel()

	stack, _ := newFileDBStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	testutil.MustNoError(t, stack.SyncItem(ctx, testItem("file-1", "PushEvent")))
	waitForCount(t, stack, ctx, 1)

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

// TestSQLiteFileDB_ParallelSourceSyncs is the WAL concurrency smoke: several
// sources sync concurrently against one file-backed database. The pool pins
// MaxOpenConns(1), so writes serialize; the assertion is that every sync
// lands (no SQLITE_BUSY deaths, no lost items) and the projection converges.
func TestSQLiteFileDB_ParallelSourceSyncs(t *testing.T) {
	t.Parallel()

	stack, _ := newFileDBStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	const sources = 4
	const itemsPerSource = 10

	var wg sync.WaitGroup

	errs := make(chan error, sources*itemsPerSource)

	for s := range sources {
		wg.Add(1)

		go func(source int) {
			defer wg.Done()

			for i := range itemsPerSource {
				src := "ps-" + string(rune('a'+source))
				sourceID := string(rune('0'+i%10)) + "-" + string(rune('a'+i/10))

				if err := stack.SyncItem(ctx, testItemWithSource(src, sourceID)); err != nil {
					errs <- err
				}
			}
		}(s)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("parallel sync failed: %v", err)
	}

	waitForCount(t, stack, ctx, sources*itemsPerSource)

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != sources*itemsPerSource {
		t.Errorf("expected %d items after parallel syncs, got %d", sources*itemsPerSource, count)
	}
}

// testItemWithSource builds a valid provider item under the given source.
// Parallel sources never collide on the (source, source_id) primary key.
func testItemWithSource(source, sourceID string) *provider.Item {
	item := testItem(sourceID, "PushEvent")
	item.Source = id.NewProviderID(source)

	return item
}
