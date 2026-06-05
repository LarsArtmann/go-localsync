package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/query"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// mockReadWriter is a concrete implementation for interface verification.
type mockReadWriter[T any] struct {
	data map[model.Key]T
}

func newMockReadWriter[T any]() *mockReadWriter[T] {
	return &mockReadWriter[T]{data: make(map[model.Key]T)}
}

func (m *mockReadWriter[T]) Get(_ context.Context, key model.Key) (T, error) {
	var zero T

	item, ok := m.data[key]
	if !ok {
		return zero, errors.New("not found")
	}

	return item, nil
}

func (m *mockReadWriter[T]) List(_ context.Context, _ query.Query[T]) (query.Page[T], error) {
	return query.Page[T]{}, nil
}

func (m *mockReadWriter[T]) Count(_ context.Context, _ query.Query[T]) (int64, error) {
	return 0, nil
}

func (m *mockReadWriter[T]) Upsert(_ context.Context, _ T) error {
	return nil
}

func (m *mockReadWriter[T]) Delete(_ context.Context, _ model.Key) error {
	return nil
}

func (m *mockReadWriter[T]) Close() error {
	return nil
}

func TestRepositoryInterfaceSatisfaction(t *testing.T) {
	t.Parallel()

	// Verify *mockReadWriter[*model.ItemView] satisfies Repository[*model.ItemView]
	var _ Repository[*model.ItemView] = (*mockReadWriter[*model.ItemView])(nil)
}

func TestObservableDelegates(t *testing.T) {
	t.Parallel()

	base := newMockReadWriter[string]()
	obs := NewObservable(base)

	ctx := context.Background()
	key := model.ItemKey(id.NewProviderID("github"), id.NewExternalID("123"))

	// Get should delegate to base
	_, err := obs.Get(ctx, key)
	if err == nil {
		t.Error("expected error from empty mock")
	}

	// List should delegate to base
	page, err := obs.List(ctx, query.Query[string]{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if page.Items != nil {
		t.Error("expected nil items from empty mock")
	}

	// Count should delegate to base
	count, err := obs.Count(ctx, query.Query[string]{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}
