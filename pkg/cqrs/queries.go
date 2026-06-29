package cqrs

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/middleware/v3"
	"github.com/larsartmann/go-cqrs-lite/query/v3"
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

	dispatcher.Use(middleware.QueryLogging(newSlogLogger()))

	if err := query.RegisterTyped(
		dispatcher, queryTypeListItem,
		func(ctx context.Context, q *ListItemsQuery) ([]*model.Item, error) {
			return rm.List(ctx, q.Filter)
		},
	); err != nil {
		return nil, fmt.Errorf("register list items query: %w", err)
	}

	if err := query.RegisterTyped(
		dispatcher, queryTypeGetItem,
		func(ctx context.Context, q *GetItemQuery) (*model.Item, error) {
			return rm.Get(ctx, q.Source, q.SourceID)
		},
	); err != nil {
		return nil, fmt.Errorf("register get item query: %w", err)
	}

	if err := query.RegisterTyped(
		dispatcher, queryTypeCountItem,
		func(ctx context.Context, q *CountItemsQuery) (int64, error) {
			return rm.Count(ctx, q.Filter)
		},
	); err != nil {
		return nil, fmt.Errorf("register count items query: %w", err)
	}

	if err := query.RegisterTyped(
		dispatcher, queryTypeGetTypes,
		func(ctx context.Context, _ *GetTypesQuery) ([]string, error) {
			return rm.GetTypes(ctx)
		},
	); err != nil {
		return nil, fmt.Errorf("register get types query: %w", err)
	}

	return dispatcher, nil
}
