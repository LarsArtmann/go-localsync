package cqrs

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/query/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

const (
	queryTypeListItem  query.Type = "sync_item.list"
	queryTypeGetItem   query.Type = "sync_item.get"
	queryTypeCountItem query.Type = "sync_item.count"
	queryTypeGetTypes  query.Type = "sync_item.get_types"
)

func mustNewQuery(queryType query.Type) query.BasicQuery {
	q, err := query.New(queryType)
	if err != nil {
		panic(fmt.Sprintf("query.New(%s): %v", queryType, err))
	}

	return *q
}

// ListItemsQuery queries items matching a filter.
type ListItemsQuery struct {
	query.BasicQuery

	Filter model.ItemFilter
}

// GetItemQuery queries a single item by source and external ID.
type GetItemQuery struct {
	query.BasicQuery

	Source   string
	SourceID id.ExternalID
}

// CountItemsQuery counts items matching a filter.
type CountItemsQuery struct {
	query.BasicQuery

	Filter model.ItemFilter
}

// GetTypesQuery returns all distinct item types.
type GetTypesQuery struct {
	query.BasicQuery
}

func wireQueryDispatcher(rm ReadModel) (*query.Dispatcher, error) {
	dispatcher := query.NewDispatcher()

	dispatcher.Use(queryLoggingMiddleware(log.Default()))

	if err := dispatcher.Register(queryTypeListItem, handleListItems(rm)); err != nil {
		return nil, fmt.Errorf("register list items query: %w", err)
	}

	if err := dispatcher.Register(queryTypeGetItem, handleGetItem(rm)); err != nil {
		return nil, fmt.Errorf("register get item query: %w", err)
	}

	if err := dispatcher.Register(queryTypeCountItem, handleCountItems(rm)); err != nil {
		return nil, fmt.Errorf("register count items query: %w", err)
	}

	if err := dispatcher.Register(queryTypeGetTypes, handleGetTypes(rm)); err != nil {
		return nil, fmt.Errorf("register get types query: %w", err)
	}

	return dispatcher, nil
}

func typedQueryHandler[T query.Query](name string, fn func(ctx context.Context, q T) (any, error)) query.Handler {
	return func(ctx context.Context, q query.Query) (any, error) {
		typed, ok := q.(T)
		if !ok {
			return nil, fmt.Errorf("expected %s, got %T: %w", name, q, errQueryTypeMismatch)
		}

		return fn(ctx, typed)
	}
}

func handleListItems(rm ReadModel) query.Handler {
	return typedQueryHandler[*ListItemsQuery](
		"*ListItemsQuery",
		func(ctx context.Context, q *ListItemsQuery) (any, error) {
			return rm.List(ctx, q.Filter)
		},
	)
}

func handleGetItem(rm ReadModel) query.Handler {
	return typedQueryHandler[*GetItemQuery]("*GetItemQuery", func(ctx context.Context, q *GetItemQuery) (any, error) {
		return rm.Get(ctx, q.Source, q.SourceID)
	})
}

func handleCountItems(rm ReadModel) query.Handler {
	return typedQueryHandler[*CountItemsQuery](
		"*CountItemsQuery",
		func(ctx context.Context, q *CountItemsQuery) (any, error) {
			return rm.Count(ctx, q.Filter)
		},
	)
}

func handleGetTypes(rm ReadModel) query.Handler {
	return typedQueryHandler[*GetTypesQuery](
		"*GetTypesQuery",
		func(ctx context.Context, _ *GetTypesQuery) (any, error) {
			return rm.GetTypes(ctx)
		},
	)
}
