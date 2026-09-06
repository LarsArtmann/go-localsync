package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestSQLiteReadModel_CursorWalk_RealOrdering exercises OFFSET pagination
// against the REAL SQLite read model and its "ORDER BY created_at DESC"
// ordering. The API's cursor is an encoded offset, so a walk is only correct
// if the ordering is deterministic: rows sharing a created_at (ties are
// realistic — batch imports, truncated timestamps) must come back in a
// stable order across page boundaries, or a walk duplicates/skips them.
//
// The fixture pins 12 items with deliberate tie groups, walks the pages via
// the same filter+limit+offset mechanism the cursor maps onto, and requires
// set equality (no duplicates, no skips) plus deterministic tie order.
func TestSQLiteReadModel_CursorWalk_RealOrdering(t *testing.T) {
	t.Parallel()

	stack, _ := newFileDBStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	const total = 12
	const pageSize = 5

	base := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	// created_at values: 2026-09-06 12:00:00 ... :11 seconds, EXCEPT items
	// 5, 6, 7 which share :07 — a deliberate 3-row tie group straddling
	// page boundaries (pages of 5: rows 4..8 span the ties).
	createdAt := func(i int) time.Time {
		sec := i
		if i >= 5 && i <= 7 {
			sec = 7
		}

		return base.Add(time.Duration(sec) * time.Second)
	}

	for i := range total {
		item := &provider.Item{
			ExternalID: id.NewExternalID("cursor-" + string(rune('a'+i))),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			Attributes: map[string]string{"actor_login": "walker"},
			CreatedAt:  createdAt(i),
			UpdatedAt:  createdAt(i),
		}
		testutil.MustNoError(t, stack.SyncItem(ctx, item))
	}

	waitForCount(t, stack, ctx, total)

	// Walk all pages with the same descending-createdAt ordering the SQL
	// uses, collecting source IDs.
	seen := map[string]int{}

	filter := model.ItemFilter{Limit: pageSize}

	var lastUpdatedAt int64 = 1 << 62

	for page := 0; ; page++ {
		items, err := stack.List(ctx, filter)
		testutil.MustNoError(t, err)

		if len(items) == 0 {
			break
		}

		for _, item := range items {
			seen[item.ExternalID.Get()]++

			got := item.UpdatedAt.UnixNano()
			if got > lastUpdatedAt {
				t.Errorf("ordering violated across page boundary: %s (updated %d) after %d",
					item.ExternalID, got, lastUpdatedAt)
			}
			lastUpdatedAt = got
		}

		if page > total/pageSize+2 {
			t.Fatal("pagination did not terminate")
		}

		filter.Offset += pageSize
	}

	if len(seen) != total {
		t.Errorf("walk visited %d distinct items, want %d", len(seen), total)
	}

	for sourceID, n := range seen {
		if n != 1 {
			t.Errorf("item %s visited %d times across pages, want exactly 1", sourceID, n)
		}
	}

	// Filter interaction: the walk must respect the same ordering under a
	// selective filter.
	filtered, err := stack.List(ctx, model.ItemFilter{Attributes: map[string]string{"actor_login": "walker"}, Limit: 3})
	testutil.MustNoError(t, err)

	if len(filtered) != 3 {
		t.Errorf("filtered page returned %d items, want 3", len(filtered))
	}
}
