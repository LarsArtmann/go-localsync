package cqrs

import (
	"context"
	"testing"
)

func TestCQRSStack_Push_NoSyncDB(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	if err := stack.Push(context.Background()); err != nil {
		t.Errorf("expected nil error for %s backend (no syncDB), got %v", "memory", err)
	}
}

func TestCQRSStack_Pull_NoSyncDB(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	changed, err := stack.Pull(context.Background())
	if err != nil {
		t.Errorf("expected nil error for %s backend (no syncDB), got %v", "memory", err)
	}
	if changed {
		t.Error("expected false when syncDB is nil")
	}
}

func TestCQRSStack_Push_TursoLocalDB(t *testing.T) {
	t.Parallel()

	stack := newTursoMemoryStack(t)
	defer func() { _ = stack.Close() }()

	if err := stack.Push(context.Background()); err != nil {
		t.Errorf("expected nil error for %s backend (no syncDB), got %v", "turso", err)
	}
}

func TestCQRSStack_Pull_TursoLocalDB(t *testing.T) {
	t.Parallel()

	stack := newTursoMemoryStack(t)
	defer func() { _ = stack.Close() }()

	changed, err := stack.Pull(context.Background())
	if err != nil {
		t.Errorf("expected nil error for %s backend (no syncDB), got %v", "turso", err)
	}
	if changed {
		t.Error("expected false for local turso (no syncDB)")
	}
}

func TestCQRSStack_SyncAfterPushPull(t *testing.T) {
	t.Parallel()

	stack := newTursoMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("push-pull-1", "PushEvent")

	mustNoError(t, stack.SyncItem(ctx, item))

	_, err := stack.Pull(ctx)
	mustNoError(t, err)

	mustNoError(t, stack.Push(ctx))

	waitForCount(t, stack, ctx, 1)
}
