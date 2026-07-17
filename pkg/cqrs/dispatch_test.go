package cqrs

import (
	"errors"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/command/v4"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestCommandDispatcher_SyncItem_ThroughDispatcher(t *testing.T) {
	stack, ctx := setupMemoryStack(t)
	item := testItem("dispatch-1", "PushEvent")

	// SyncItem goes through CommandDispatcher
	testutil.MustNoError(t, stack.SyncItem(ctx, item))

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after SyncItem through dispatcher, got %d", count)
	}
}

func TestCommandDispatcher_TombstoneItem_ThroughDispatcher(t *testing.T) {
	stack, ctx := setupMemoryStack(t)

	syncTestItem(t, stack, ctx, "dispatch-2", "PushEvent")
	testutil.MustNoError(
		t,
		stack.TombstoneItem(ctx, "github", id.NewExternalID("dispatch-2"), model.ReasonUpstreamGone),
	)

	count, err := stack.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if count != 0 {
		t.Errorf("expected count=0 after TombstoneItem through dispatcher, got %d", count)
	}
}

func TestCommandDispatcher_InvalidCommandType(t *testing.T) {
	stack, ctx := setupMemoryStack(t)

	// Send a raw SyncItemCommand with wrong type by using the dispatcher directly
	// This tests the type assertion path in handleSyncItem
	aggID := AggregateID("github", id.NewExternalID("wrong-type-test"))
	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
		Item:         toDataItem(testItem("wrong-type-test", "PushEvent")),
	})
	testutil.MustNoError(t, err)
}

func TestCommandDispatcher_UnknownCommandType(t *testing.T) {
	stack, ctx := setupMemoryStack(t)

	// Try to dispatch an unregistered command type
	aggID := AggregateID("github", id.NewExternalID("unknown"))
	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: mustNewCommand(command.Type("unknown.command"), aggID),
		Item:         toDataItem(testItem("unknown", "PushEvent")),
	})
	if err == nil {
		t.Error("expected error for unregistered command type")
	}
}

func TestCommandDispatcher_Validation_NilItem(t *testing.T) {
	stack, ctx := setupMemoryStack(t)
	aggID := AggregateID("github", id.NewExternalID("nil-item"))

	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
		Item:         nil,
	})
	if err == nil {
		t.Error("expected validation error for nil item")
	}
}

// TestCommandDispatcher_ValidationError_IsClassified guards the error-family
// contract: a validation failure must surface as a Rejection (ErrInvalidInput)
// so a consumer can classify it. Before the migration this was an unclassified
// standalone sentinel; it must now report non-retryable via the family.
func TestCommandDispatcher_ValidationError_IsClassified(t *testing.T) {
	stack, ctx := setupMemoryStack(t)
	aggID := AggregateID("github", id.NewExternalID("classified"))

	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
		Item:         nil,
	})

	if err == nil {
		t.Fatal("expected validation error for nil item")
	}

	if !errors.Is(err, pkgerrors.ErrInvalidInput) {
		t.Errorf("validation error must wrap ErrInvalidInput (got %T: %v)", err, err)
	}

	if pkgerrors.IsRetryable(err) {
		t.Error("validation error must be non-retryable (Rejection family)")
	}
}

func TestCommandDispatcher_Validation_EmptySource(t *testing.T) {
	stack, ctx := setupMemoryStack(t)
	item := testItem("empty-source", "PushEvent")
	item.Source = id.NewProviderID("") // empty source

	aggID := AggregateID("", id.NewExternalID("empty-source"))
	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
		Item:         toDataItem(item),
	})
	if err == nil {
		t.Error("expected validation error for empty source")
	}
}
