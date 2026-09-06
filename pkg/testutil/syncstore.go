package testutil

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// SyncStoreListBehavior provides a default List implementation for mock
// implementations of sync.SyncStore. Embed it in a mock type to share the
// behavior:
//
//	type mockSyncStore struct {
//	    testutil.SyncStoreListBehavior
//	    // additional fields; implement SyncItems to produce a *sync.BatchOutcome
//	}
//
// Since the v0.6 BatchOutcome consolidation, the store seam's write side is
// SyncItems(ctx, items) *sync.BatchOutcome — a value, not (result, error):
// per-item failures surface in the outcome's Results, and the aggregate
// counts live directly on it. Mocks keep their own SyncItems; this embed
// covers only the read side.
type SyncStoreListBehavior struct {
	Items   []*model.Item
	ListErr error
}

// List returns the configured Items or ListErr.
func (b *SyncStoreListBehavior) List(
	_ context.Context,
	_ model.ItemFilter,
) ([]*model.Item, error) {
	if b.ListErr != nil {
		return nil, b.ListErr
	}

	return b.Items, nil
}
