package cqrs

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/command/v4"
	"github.com/larsartmann/go-cqrs-lite/decider/v4"
	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/middleware/v4"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

const commandTypeSyncItem command.Type = "sync_item.sync"

const commandTypeTombstone command.Type = "sync_item.tombstone"

func mustNewCommand(cmdType command.Type, aggID cqrsid.StreamID) command.BasicCommand {
	cmd, err := command.New(cmdType, aggID)
	if err != nil {
		panic(fmt.Sprintf("command.New(%s): %v", cmdType, err))
	}

	return *cmd
}

// SyncItemCommand dispatches a sync operation for a single item.
//
// outcome is an optional out-parameter: SyncItems sets it so the command
// handler can record what the decider decided (new/conflict/unchanged), which
// SyncItems then reads to classify the result. It is nil for the single-item
// SyncItem path, which does not need classification.
type SyncItemCommand struct {
	command.BasicCommand

	Item    *model.Item
	RawJSON []byte
	Options []event.Option
	outcome *SyncOutcome
}

// TombstoneItemCommand dispatches a tombstone operation for a single item.
type TombstoneItemCommand struct {
	command.BasicCommand

	Source   string
	SourceID id.ExternalID
	Reason   model.TombstoneReason
}

func wireCommandDispatcher(
	repo *decider.Repository[SyncItemState],
	resolver crdt.ConflictResolver[*model.Item],
) (*command.Dispatcher, error) {
	dispatcher := command.NewDispatcher()

	dispatcher.Use(middleware.CommandLogging(newSlogLogger()))
	dispatcher.Use(commandValidationMiddleware())
	dispatcher.Use(middleware.CommandRetry(middleware.DefaultRetryConfig(), middleware.WithLogger(newSlogLogger())))

	syncItemHandler := func(ctx context.Context, cmd *SyncItemCommand) error {
		return repo.ExecuteRef(
			ctx,
			cqrsid.NewStreamRef(aggregateType, cmd.StreamID()),
			decideWithOutcome(cmd.Item, cmd.RawJSON, resolver, cmd.outcome, cmd.Options...),
		)
	}

	if err := command.RegisterTyped(dispatcher, commandTypeSyncItem, syncItemHandler); err != nil {
		return nil, pkgerrors.Wrap(err, "register sync item command")
	}

	tombstoneHandler := func(ctx context.Context, cmd *TombstoneItemCommand) error {
		return repo.ExecuteRef(
			ctx,
			cqrsid.NewStreamRef(aggregateType, cmd.StreamID()),
			decideTombstone(cmd.Source, cmd.SourceID, cmd.Reason),
		)
	}

	if err := command.RegisterTyped(
		dispatcher,
		commandTypeTombstone,
		tombstoneHandler,
	); err != nil {
		return nil, pkgerrors.Wrap(err, "register tombstone item command")
	}

	return dispatcher, nil
}
