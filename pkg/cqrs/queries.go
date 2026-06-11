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

func handleListItems(rm ReadModel) query.Handler {
	return func(ctx context.Context, q query.Query) (any, error) {
		lq, ok := q.(*ListItemsQuery)
		if !ok {
			return nil, fmt.Errorf("expected *ListItemsQuery, got %T: %w", q, errQueryTypeMismatch)
		}

		return rm.List(ctx, lq.Filter)
	}
}

func handleGetItem(rm ReadModel) query.Handler {
	return func(ctx context.Context, q query.Query) (any, error) {
		gq, ok := q.(*GetItemQuery)
		if !ok {
			return nil, fmt.Errorf("expected *GetItemQuery, got %T: %w", q, errQueryTypeMismatch)
		}

		return rm.Get(ctx, gq.Source, gq.SourceID)
	}
}

func handleCountItems(rm ReadModel) query.Handler {
	return func(ctx context.Context, q query.Query) (any, error) {
		cq, ok := q.(*CountItemsQuery)
		if !ok {
			return nil, fmt.Errorf("expected *CountItemsQuery, got %T: %w", q, errQueryTypeMismatch)
		}

		return rm.Count(ctx, cq.Filter)
	}
}

func handleGetTypes(rm ReadModel) query.Handler {
	return func(ctx context.Context, q query.Query) (any, error) {
		_, ok := q.(*GetTypesQuery)
		if !ok {
			return nil, fmt.Errorf("expected *GetTypesQuery, got %T: %w", q, errQueryTypeMismatch)
		}

		return rm.GetTypes(ctx)
	}
}
