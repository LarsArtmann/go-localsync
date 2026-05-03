package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCQRSStack_SyncNewItem(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	err = stack.SyncItem(ctx, item)
	require.NoError(t, err)

	count, err := stack.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	resultTypes, err := stack.GetTypes(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"PushEvent"}, resultTypes)
}

func TestCQRSStack_SyncMultipleItems(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
		testItem("3", "PushEvent"),
	}

	synced, conflicts, errors := stack.SyncItems(ctx, items)
	assert.Equal(t, 3, synced)
	assert.Equal(t, 0, conflicts)
	assert.Equal(t, 0, errors)

	count, err := stack.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	resultTypes, err := stack.GetTypes(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"IssueEvent", "PushEvent"}, resultTypes)
}

func TestCQRSStack_Idempotency_DeterministicAggregateID(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("123", "PushEvent")

	err = stack.SyncItem(ctx, item)
	require.NoError(t, err)

	err = stack.SyncItem(ctx, item)
	require.NoError(t, err)

	count, err := stack.Count(ctx)
	require.NoError(t, err)
	assert.Equal(
		t,
		int64(1),
		count,
		"same item synced twice should still have count 1 — idempotent",
	)
}

func TestCQRSStack_DeleteItem(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	require.NoError(t, stack.SyncItem(ctx, testItem("123", "PushEvent")))

	count, err := stack.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	require.NoError(t, stack.DeleteItem(ctx, "github", "123"))

	count, err = stack.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "item should be deleted from read model")
}

func TestCQRSStack_DeleteThenResurrect(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	require.NoError(t, stack.SyncItem(ctx, testItem("123", "PushEvent")))
	require.NoError(t, stack.DeleteItem(ctx, "github", "123"))

	count, _ := stack.Count(ctx)
	assert.Equal(t, int64(0), count)

	require.NoError(t, stack.SyncItem(ctx, testItem("123", "IssueEvent")))

	count, _ = stack.Count(ctx)
	assert.Equal(t, int64(1), count, "resurrected item should reappear in read model")

	got, err := stack.ReadModel.Get(ctx, "github", "123")
	require.NoError(t, err)
	assert.Equal(t, "IssueEvent", got.Type.Get(), "resurrected item should have updated type")
}

func TestCQRSStack_ConflictDetection(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	items := []*provider.Item{testItem("1", "PushEvent")}

	synced, conflicts, errors := stack.SyncItems(ctx, items)
	assert.Equal(t, 1, synced)
	assert.Equal(t, 0, conflicts)
	assert.Equal(t, 0, errors)

	updatedItem := testItem("1", "PushEvent")
	updatedItem.UpdatedAt = time.Now().Add(time.Hour)

	synced, conflicts, errors = stack.SyncItems(ctx, []*provider.Item{updatedItem})
	assert.Equal(t, 1, synced)
	assert.Equal(t, 1, conflicts, "updated item with newer timestamp should trigger conflict")
	assert.Equal(t, 0, errors)
}

func TestCQRSStack_FilterByType(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
		testItem("3", "PushEvent"),
	}

	synced, _, _ := stack.SyncItems(ctx, items)
	assert.Equal(t, 3, synced)

	pushType := "PushEvent"
	results, err := stack.ReadModel.List(ctx, ItemFilter{Type: &pushType})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestCQRSStack_Close(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	require.NoError(t, err)

	require.NoError(t, stack.Close())
}

func TestCQRSStack_InvalidBackend(t *testing.T) {
	t.Parallel()

	_, err := NewCQRSStack(CQRSConfig{Backend: "postgres"})
	assert.Error(t, err)
}

func TestCQRSStack_DeterministicAggregateID_Matches(t *testing.T) {
	t.Parallel()

	id1 := AggregateID("github", "123")
	id2 := AggregateID("github", "123")

	assert.Equal(t, id1, id2, "deterministic IDs must be equal for same inputs")
}
