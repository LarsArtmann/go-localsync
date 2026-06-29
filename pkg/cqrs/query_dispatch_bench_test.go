package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// These benchmarks compare the two read paths available on CQRSStack:
//   - direct ReadModel access (the current production path in stack_adapters.go)
//   - the QueryDispatcher (wired + tested, but bypassed at runtime)
//
// The data decides whether productionizing the QueryDispatcher (for its
// QueryLogging observability) has acceptable overhead on the hot read path.

func seedStack(b *testing.B) (*CQRSStack, context.Context) {
	b.Helper()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	stack.SyncItems(ctx, benchItems(1000))

	return stack, ctx
}

func BenchmarkReadDirect_List(b *testing.B) {
	stack, ctx := seedStack(b)
	defer func() { _ = stack.Close() }()

	filter := model.ItemFilter{Limit: 100, Offset: 0}
	b.ResetTimer()

	for range b.N {
		if _, err := stack.ReadModel.List(ctx, filter); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadDispatcher_List(b *testing.B) {
	stack, ctx := seedStack(b)
	defer func() { _ = stack.Close() }()

	b.ResetTimer()

	for range b.N {
		if _, err := stack.QueryDispatcher.Dispatch(ctx, &ListItemsQuery{
			BasicQuery: mustNewQuery(queryTypeListItem),
			Filter:     model.ItemFilter{Limit: 100, Offset: 0},
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadDirect_Count(b *testing.B) {
	stack, ctx := seedStack(b)
	defer func() { _ = stack.Close() }()

	filter := model.ItemFilter{}
	b.ResetTimer()

	for range b.N {
		if _, err := stack.ReadModel.Count(ctx, filter); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadDispatcher_Count(b *testing.B) {
	stack, ctx := seedStack(b)
	defer func() { _ = stack.Close() }()

	b.ResetTimer()

	for range b.N {
		if _, err := stack.QueryDispatcher.Dispatch(ctx, &CountItemsQuery{
			BasicQuery: mustNewQuery(queryTypeCountItem),
			Filter:     model.ItemFilter{},
		}); err != nil {
			b.Fatal(err)
		}
	}
}
