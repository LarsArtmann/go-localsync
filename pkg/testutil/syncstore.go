package testutil

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// SyncStoreListBehavior provides a default ListItems implementation
// for mock implementations of provider.SyncStore. Embed it in a mock
// type to share the behavior:
//
//	type mockSyncStore struct {
//	    testutil.SyncStoreListBehavior
//	    // additional fields
//	}
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
