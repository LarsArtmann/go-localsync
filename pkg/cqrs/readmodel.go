package cqrs

import (
	"context"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
)

type ReadModel interface {
	Get(ctx context.Context, source, sourceID string) (*provider.Item, error)
	List(ctx context.Context, filter ItemFilter) ([]*provider.Item, error)
	Count(ctx context.Context, filter ItemFilter) (int64, error)
	GetTypes(ctx context.Context) ([]string, error)
	Upsert(ctx context.Context, item *provider.Item) error
	Delete(ctx context.Context, source, sourceID string) error
	Close() error
}

type ItemFilter struct {
	Type       *string
	ActorLogin *string
	RepoName   *string
	Source     *string
	Since      *time.Time
	Limit      int
	Offset     int
}
