package testutil

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/provider"
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
	Items   []*provider.Item
	ListErr error
}

// ListItems returns the configured Items or ListErr.
func (b *SyncStoreListBehavior) ListItems(
	_ context.Context,
	_ provider.ItemFilter,
) ([]*provider.Item, error) {
	if b.ListErr != nil {
		return nil, b.ListErr
	}

	return b.Items, nil
}
