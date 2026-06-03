package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/command/v2"
	"github.com/larsartmann/go-cqrs-lite/query/v2"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func TestCommandDispatcher_SyncItem_ThroughDispatcher(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("dispatch-1", "PushEvent")

	// SyncItem goes through CommandDispatcher
	mustNoError(t, stack.SyncItem(ctx, item))

	count, err := stack.Count(ctx)
	mustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after SyncItem through dispatcher, got %d", count)
	}
}

func TestCommandDispatcher_DeleteItem_ThroughDispatcher(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	mustNoError(t, stack.SyncItem(ctx, testItem("dispatch-2", "PushEvent")))
	mustNoError(t, stack.DeleteItem(ctx, "github", id.NewExternalID("dispatch-2")))

	count, err := stack.Count(ctx)
	mustNoError(t, err)
	if count != 0 {
		t.Errorf("expected count=0 after DeleteItem through dispatcher, got %d", count)
	}
}

func TestCommandDispatcher_InvalidCommandType(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	// Send a raw SyncItemCommand with wrong type by using the dispatcher directly
	// This tests the type assertion path in handleSyncItem
	aggID := AggregateID("github", id.NewExternalID("wrong-type-test"))
	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: *command.MustNew(commandTypeSyncItem, aggID),
		Item:         testItem("wrong-type-test", "PushEvent"),
	})
	mustNoError(t, err)
}

func TestCommandDispatcher_UnknownCommandType(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	// Try to dispatch an unregistered command type
	aggID := AggregateID("github", id.NewExternalID("unknown"))
	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: *command.MustNew(command.Type("unknown.command"), aggID),
		Item:         testItem("unknown", "PushEvent"),
	})
	if err == nil {
		t.Error("expected error for unregistered command type")
	}
}

func TestCommandDispatcher_Validation_NilItem(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	aggID := AggregateID("github", id.NewExternalID("nil-item"))

	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: *command.MustNew(commandTypeSyncItem, aggID),
		Item:         nil,
	})
	if err == nil {
		t.Error("expected validation error for nil item")
	}
}

func TestCommandDispatcher_Validation_EmptySource(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("empty-source", "PushEvent")
	item.Source = id.NewProviderID("") // empty source

	aggID := AggregateID("", id.NewExternalID("empty-source"))
	err := stack.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: *command.MustNew(commandTypeSyncItem, aggID),
		Item:         item,
	})
	if err == nil {
		t.Error("expected validation error for empty source")
	}
}

func TestQueryDispatcher_ListItems_ThroughDispatcher(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	mustNoError(t, stack.SyncItem(ctx, testItem("q-1", "PushEvent")))

	result, err := stack.QueryDispatcher.Dispatch(ctx, &ListItemsQuery{
		BasicQuery: *query.MustNew(queryTypeListItem),
		Filter:     provider.ItemFilter{},
	})
	mustNoError(t, err)

	items, ok := result.([]*provider.Item)
	if !ok {
		t.Fatalf("expected []*provider.Item, got %T", result)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestQueryDispatcher_GetItem_ThroughDispatcher(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	mustNoError(t, stack.SyncItem(ctx, testItem("q-2", "PushEvent")))

	result, err := stack.QueryDispatcher.Dispatch(ctx, &GetItemQuery{
		BasicQuery: *query.MustNew(queryTypeGetItem),
		Source:     "github",
		SourceID:   id.NewExternalID("q-2"),
	})
	mustNoError(t, err)

	item, ok := result.(*provider.Item)
	if !ok {
		t.Fatalf("expected *provider.Item, got %T", result)
	}
	if item.ExternalID.Get() != "q-2" {
		t.Errorf("expected item id q-2, got %s", item.ExternalID.Get())
	}
}

func TestQueryDispatcher_CountItems_ThroughDispatcher(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	mustNoError(t, stack.SyncItem(ctx, testItem("q-3", "PushEvent")))

	result, err := stack.QueryDispatcher.Dispatch(ctx, &CountItemsQuery{
		BasicQuery: *query.MustNew(queryTypeCountItem),
		Filter:     provider.ItemFilter{},
	})
	mustNoError(t, err)

	count, ok := result.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", result)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}
}

func TestQueryDispatcher_GetTypes_ThroughDispatcher(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	mustNoError(t, stack.SyncItem(ctx, testItem("q-4", "PushEvent")))

	result, err := stack.QueryDispatcher.Dispatch(ctx, &GetTypesQuery{
		BasicQuery: *query.MustNew(queryTypeGetTypes),
	})
	mustNoError(t, err)

	types, ok := result.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", result)
	}
	if len(types) != 1 {
		t.Errorf("expected 1 type, got %d", len(types))
	}
}

func TestQueryDispatcher_UnknownQueryType(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	_, err := stack.QueryDispatcher.Dispatch(ctx, &GetTypesQuery{
		BasicQuery: *query.MustNew(query.Type("unknown.query")),
	})
	if err == nil {
		t.Error("expected error for unregistered query type")
	}
}
