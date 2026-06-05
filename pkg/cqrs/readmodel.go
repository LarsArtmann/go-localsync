package cqrs

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// ReadModel is the CQRS read model interface for querying projected items.
type ReadModel interface {
	Get(ctx context.Context, source string, sourceID id.ExternalID) (*model.Item, error)
	List(ctx context.Context, filter provider.ItemFilter) ([]*model.Item, error)
	Count(ctx context.Context, filter provider.ItemFilter) (int64, error)
	GetTypes(ctx context.Context) ([]string, error)
	Upsert(ctx context.Context, item *model.Item) error
	Delete(ctx context.Context, source string, sourceID id.ExternalID) error
	Close() error
}
