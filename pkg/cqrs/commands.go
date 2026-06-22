package cqrs

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/command/v3"
	"github.com/larsartmann/go-cqrs-lite/decider/v3"
	"github.com/larsartmann/go-cqrs-lite/event/v3"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v3"
	"github.com/larsartmann/go-cqrs-lite/middleware/v3"
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

	syncItemHandler := func(ctx context.Context, cmd *SyncItemCommand) error {
		outcome := syncOutcomeFromContext(ctx)

		return repo.Execute(
			ctx, cmd.AggregateID(), aggregateType,
			decideWithOutcome(cmd.Item, cmd.RawJSON, resolver, outcome, cmd.Options...),
		)
	}

	if err := command.RegisterTyped[*SyncItemCommand](dispatcher, commandTypeSyncItem, syncItemHandler); err != nil {
		return nil, fmt.Errorf("register sync item command: %w", err)
	}

	deleteHandler := func(ctx context.Context, cmd *DeleteItemCommand) error {
		return repo.Execute(ctx, cmd.AggregateID(), aggregateType, decideDelete(cmd.Source, cmd.SourceID))
	}

	if err := command.RegisterTyped[*DeleteItemCommand](dispatcher, commandTypeDeleteItem, deleteHandler); err != nil {
		return nil, fmt.Errorf("register delete item command: %w", err)
	}

	return dispatcher, nil
}
