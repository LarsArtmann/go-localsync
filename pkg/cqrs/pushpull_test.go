package cqrs

import (
	"context"
	"testing"
)

func TestCQRSStack_Push_NoSyncDB(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	if err := stack.Push(context.Background()); err != nil {
		t.Errorf("expected nil error when syncDB is nil, got %v", err)
	}
}

func TestCQRSStack_Pull_NoSyncDB(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	changed, err := stack.Pull(context.Background())
	if err != nil {
		t.Errorf("expected nil error when syncDB is nil, got %v", err)
	}
	if changed {
		t.Error("expected false when syncDB is nil")
	}
}

func TestCQRSStack_Push_TursoLocalDB(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	if err := stack.Push(context.Background()); err != nil {
		t.Errorf("expected nil error for local turso (no syncDB), got %v", err)
	}
}

func TestCQRSStack_Pull_TursoLocalDB(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	changed, err := stack.Pull(context.Background())
	if err != nil {
		t.Errorf("expected nil error for local turso (no syncDB), got %v", err)
	}
	if changed {
		t.Error("expected false for local turso (no syncDB)")
	}
}

func TestCQRSStack_SyncAfterPushPull(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "turso", DBPath: ":memory:"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("push-pull-1", "PushEvent")

	if err := stack.SyncItem(ctx, item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = stack.Pull(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = stack.Push(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForCount(t, stack, ctx, 1)
}
