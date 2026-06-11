package cqrs

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/command/v2"
	"github.com/larsartmann/go-cqrs-lite/decider/v2"
	"github.com/larsartmann/go-cqrs-lite/event/v2"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v2"
	"github.com/larsartmann/go-cqrs-lite/middleware/v2"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

const commandTypeSyncItem command.Type = "sync_item.sync"

const commandTypeDeleteItem command.Type = "sync_item.delete"

func mustNewCommand(cmdType command.Type, aggID cqrsid.AggregateID) command.BasicCommand {
	cmd, err := command.New(cmdType, aggID)
	if err != nil {
		panic(fmt.Sprintf("command.New(%s): %v", cmdType, err))
	}

	return *cmd
}

// SyncItemCommand dispatches a sync operation for a single item.
type SyncItemCommand struct {
	command.BasicCommand

	Item    *model.Item
	RawJSON []byte
	Options []event.Option
}

// DeleteItemCommand dispatches a delete operation for a single item.
type DeleteItemCommand struct {
	command.BasicCommand

	Source   string
	SourceID id.ExternalID
}

func wireCommandDispatcher(
	repo *decider.Repository[SyncItemState],
	resolver crdt.ConflictResolver[*model.Item],
) (*command.Dispatcher, error) {
	dispatcher := command.NewDispatcher()

	dispatcher.Use(commandLoggingMiddleware(log.Default()))
	dispatcher.Use(commandValidationMiddleware())
	dispatcher.Use(middleware.CommandRetry(middleware.DefaultRetryConfig(), middleware.WithLogger(newSlogLogger())))

	syncItemHandler := func(ctx context.Context, cmd command.Command) error {
		syncCmd, ok := cmd.(*SyncItemCommand)
		if !ok {
			return fmt.Errorf("expected *SyncItemCommand, got %T: %w", cmd, errCommandTypeMismatch)
		}

		outcome := syncOutcomeFromContext(ctx)

		return repo.Execute(
			ctx, syncCmd.AggregateID(), aggregateType,
			decideWithOutcome(syncCmd.Item, syncCmd.RawJSON, resolver, outcome, syncCmd.Options...),
		)
	}

	if err := dispatcher.Register(commandTypeSyncItem, syncItemHandler); err != nil {
		return nil, fmt.Errorf("register sync item command: %w", err)
	}

	if err := dispatcher.Register(commandTypeDeleteItem, handleDeleteItem(repo)); err != nil {
		return nil, fmt.Errorf("register delete item command: %w", err)
	}

	return dispatcher, nil
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
