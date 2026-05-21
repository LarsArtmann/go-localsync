package cqrs

import (
	"context"
	"errors"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/command"
	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/query"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

var (
	errCommandTypeMismatch = errors.New("command type mismatch")
	errQueryTypeMismatch   = errors.New("query type mismatch")
)

const (
	commandTypeSyncItem   command.Type = "sync_item.sync"
	commandTypeDeleteItem command.Type = "sync_item.delete"

	queryTypeListItem  query.Type = "sync_item.list"
	queryTypeGetItem   query.Type = "sync_item.get"
	queryTypeCountItem query.Type = "sync_item.count"
	queryTypeGetTypes  query.Type = "sync_item.get_types"
)

type SyncItemCommand struct {
	command.Core

	Item *provider.Item
}

type DeleteItemCommand struct {
	command.Core

	Source   string
	SourceID string
}

type ListItemsQuery struct {
	query.Core

	Filter ItemFilter
}

type GetItemQuery struct {
	query.Core

	Source   string
	SourceID string
}

type CountItemsQuery struct {
	query.Core

	Filter ItemFilter
}

type GetTypesQuery struct {
	query.Core
}

func wireCommandDispatcher(
	repo *decider.Repository[SyncItemState],
) (*command.Dispatcher, error) {
	dispatcher := command.NewDispatcher()

	if err := dispatcher.Register(commandTypeSyncItem, handleSyncItem(repo)); err != nil {
		return nil, fmt.Errorf("register sync item command: %w", err)
	}

	if err := dispatcher.Register(commandTypeDeleteItem, handleDeleteItem(repo)); err != nil {
		return nil, fmt.Errorf("register delete item command: %w", err)
	}

	return dispatcher, nil
}

func handleSyncItem(repo *decider.Repository[SyncItemState]) command.Handler {
	return func(ctx context.Context, cmd command.Command) error {
		syncCmd, ok := cmd.(*SyncItemCommand)
		if !ok {
			return fmt.Errorf("expected *SyncItemCommand, got %T: %w", cmd, errCommandTypeMismatch)
		}

		aggID := AggregateID(syncCmd.Item.Source.Get(), syncCmd.Item.ExternalID.Get())

		return repo.Execute(ctx, aggID, aggregateType, DecideSync(syncCmd.Item))
	}
}

func handleDeleteItem(repo *decider.Repository[SyncItemState]) command.Handler {
	return func(ctx context.Context, cmd command.Command) error {
		delCmd, ok := cmd.(*DeleteItemCommand)
		if !ok {
			return fmt.Errorf(
				"expected *DeleteItemCommand, got %T: %w", cmd, errCommandTypeMismatch,
			)
		}

		aggID := AggregateID(delCmd.Source, delCmd.SourceID)

		return repo.Execute(ctx, aggID, aggregateType, DecideDelete(delCmd.Source, delCmd.SourceID))
	}
}

func wireQueryDispatcher(rm ReadModel) (*query.Dispatcher, error) {
	dispatcher := query.NewDispatcher()

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
