package cqrs

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

type ReadModel interface {
	Get(ctx context.Context, source string, sourceID types.ExternalID) (*provider.Item, error)
	List(ctx context.Context, filter provider.ItemFilter) ([]*provider.Item, error)
	Count(ctx context.Context, filter provider.ItemFilter) (int64, error)
	GetTypes(ctx context.Context) ([]string, error)
	Upsert(ctx context.Context, item *provider.Item) error
	Delete(ctx context.Context, source string, sourceID types.ExternalID) error
	Close() error
}
