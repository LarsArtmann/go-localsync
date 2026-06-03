package cqrs

import (
	"context"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func TestCQRSStack_TursoBackend_SyncAndDelete(t *testing.T) {
	t.Parallel()

	stack := newTursoMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := []*provider.Item{
		testItem("1", "PushEvent"),
		testItem("2", "IssueEvent"),
	}

	result := stack.SyncItems(ctx, items)
	if result.Synced != 2 {
		t.Errorf("expected Synced=2, got %d", result.Synced)
	}
	if result.Conflicts != 0 {
		t.Errorf("expected Conflicts=0, got %d", result.Conflicts)
	}
	if result.Errors != 0 {
		t.Errorf("expected Errors=0, got %d", result.Errors)
	}

	waitForCount(t, stack, ctx, 2)

	mustNoError(t, stack.DeleteItem(ctx, "github", id.NewExternalID("1")))

	waitForCount(t, stack, ctx, 1)
}

func TestCQRSStack_TursoLocalStore_SyncAndReadModel(t *testing.T) {
	t.Parallel()

	stack := newTursoMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	mustNoError(t, stack.SyncItem(ctx, testItem("1", "PushEvent")))
	mustNoError(t, stack.SyncItem(ctx, testItem("2", "IssueEvent")))

	waitForCount(t, stack, ctx, 2)

	items, err := stack.ReadModel.List(ctx, provider.ItemFilter{})
	mustNoError(t, err)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestCQRSStack_OutboxPoller_PublishesEvents(t *testing.T) {
	t.Parallel()

	stack := newTursoMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	mustNoError(t, stack.SyncItem(ctx, testItem("outbox-1", "PushEvent")))

	waitForCount(t, stack, ctx, 1)

	count, err := stack.Count(ctx)
	mustNoError(t, err)
	if count != 1 {
		t.Errorf("expected count=1 after outbox poller, got %d", count)
	}
}

func TestCQRSStack_ProjectionRunner_ReplaysOnRestart(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "replay-test.db")

	ctx := context.Background()

	stack1, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: dbPath})
	mustNoError(t, err)

	mustNoError(t, stack1.SyncItem(ctx, testItem("replay-1", "PushEvent")))
	mustNoError(t, stack1.SyncItem(ctx, testItem("replay-2", "IssueEvent")))

	waitForCount(t, stack1, ctx, 2)

	mustNoError(t, stack1.Close())

	stack2, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: dbPath})
	mustNoError(t, err)
	defer func() { _ = stack2.Close() }()

	waitForCount(t, stack2, ctx, 2)

	count, err := stack2.Count(ctx)
	mustNoError(t, err)
	if count != 2 {
		t.Errorf("expected count=2 after replay, got %d", count)
	}
}
