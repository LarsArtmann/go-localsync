package cqrs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/command/v2"
	"github.com/larsartmann/go-cqrs-lite/decider/v2"
	"github.com/larsartmann/go-cqrs-lite/query/v2"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// commandLoggingMiddleware logs command dispatches.
func commandLoggingMiddleware(logger *log.Logger) command.Middleware {
	return func(next command.Handler) command.Handler {
		return func(ctx context.Context, cmd command.Command) error {
			start := time.Now()
			err := next(ctx, cmd)
			duration := time.Since(start)

			if err != nil {
				logger.Error(
					"command dispatch failed",
					"type", cmd.Type(),
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info(
					"command dispatch succeeded",
					"type", cmd.Type(),
					"duration", duration,
				)
			}

			return err
		}
	}
}

// commandValidationMiddleware validates commands before dispatch.
func commandValidationMiddleware() command.Middleware {
	return func(next command.Handler) command.Handler {
		return func(ctx context.Context, cmd command.Command) error {
			switch cmdTyped := cmd.(type) {
			case *SyncItemCommand:
				if cmdTyped.Item == nil {
					return fmt.Errorf("sync item command: item is nil: %w", errCommandTypeMismatch)
				}

				if cmdTyped.Item.Source.Get() == "" {
					return fmt.Errorf(
						"sync item command: source is empty: %w",
						errCommandTypeMismatch,
					)
				}
			case *DeleteItemCommand:
				if cmdTyped.Source == "" {
					return fmt.Errorf(
						"delete item command: source is empty: %w",
						errCommandTypeMismatch,
					)
				}
			}

			return next(ctx, cmd)
		}
	}
}

// queryLoggingMiddleware logs query dispatches.
func queryLoggingMiddleware(logger *log.Logger) query.Middleware {
	return func(next query.Handler) query.Handler {
		return func(ctx context.Context, queryArg query.Query) (any, error) {
			start := time.Now()
			result, err := next(ctx, queryArg)
			duration := time.Since(start)

			if err != nil {
				logger.Error(
					"query dispatch failed",
					"type", queryArg.Type(),
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info(
					"query dispatch succeeded",
					"type", queryArg.Type(),
					"duration", duration,
				)
			}

			return result, err
		}
	}
}

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

// SyncItemCommand dispatches a sync operation for a single item.
type SyncItemCommand struct {
	command.BasicCommand

	Item *provider.Item
}

// DeleteItemCommand dispatches a delete operation for a single item.
type DeleteItemCommand struct {
	command.BasicCommand

	Source   string
	SourceID id.ExternalID
}

// ListItemsQuery queries items matching a filter.
type ListItemsQuery struct {
	query.BasicQuery

	Filter provider.ItemFilter
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

	Filter provider.ItemFilter
}

// GetTypesQuery returns all distinct item types.
type GetTypesQuery struct {
	query.BasicQuery
}

func wireCommandDispatcher(
	repo *decider.Repository[SyncItemState],
	resolver crdt.ConflictResolver[*provider.Item],
) (*command.Dispatcher, error) {
	dispatcher := command.NewDispatcher()

	dispatcher.Use(commandLoggingMiddleware(log.Default()))
	dispatcher.Use(commandValidationMiddleware())

	if err := dispatcher.Register(commandTypeSyncItem, handleSyncItem(repo, resolver)); err != nil {
		return nil, fmt.Errorf("register sync item command: %w", err)
	}

	if err := dispatcher.Register(commandTypeDeleteItem, handleDeleteItem(repo)); err != nil {
		return nil, fmt.Errorf("register delete item command: %w", err)
	}

	return dispatcher, nil
}

func handleSyncItem(
	repo *decider.Repository[SyncItemState],
	resolver crdt.ConflictResolver[*provider.Item],
) command.Handler {
	return func(ctx context.Context, cmd command.Command) error {
		syncCmd, ok := cmd.(*SyncItemCommand)
		if !ok {
			return fmt.Errorf("expected *SyncItemCommand, got %T: %w", cmd, errCommandTypeMismatch)
		}

		return repo.Execute(ctx, syncCmd.AggregateID(), aggregateType, DecideSync(syncCmd.Item, resolver))
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

		return repo.Execute(ctx, delCmd.AggregateID(), aggregateType, DecideDelete(delCmd.Source, delCmd.SourceID))
	}
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
