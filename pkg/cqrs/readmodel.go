package cqrs

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// ReadModel is the CQRS read model interface for querying projected items.
// The read-side list/count/get-types methods come from model.ItemReader so
// both ReadModel and sync.SyncStore share the same declarations.
type ReadModel interface {
	model.ItemReader
	Get(ctx context.Context, source string, sourceID id.ExternalID) (*model.Item, error)
	Upsert(ctx context.Context, item *model.Item) error
	Tombstone(ctx context.Context, source string, sourceID id.ExternalID, tombstone model.Tombstone) error
	Close() error
}
