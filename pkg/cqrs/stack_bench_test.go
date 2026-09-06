package cqrs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func benchItems(n int) []*provider.Item {
	items := make([]*provider.Item, 0, n)
	for i := range n {
		items = append(items, &provider.Item{
			ID:       id.NewItemID(),
			SourceID: id.NewSourceID(fmt.Sprintf("bench-%d", i)),
			Source:   id.NewProviderID("github"),
			Type:     id.NewEventTypeID("PushEvent"),
			Attributes: map[string]string{
				"actor_login": "benchuser",
				"repo_name":   "bench/repo",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			RawJSON:   []byte(`{"bench":true}`),
		})
	}
	return items
}

func BenchmarkSyncItems(b *testing.B) {
	for _, size := range []int{1, 10, 100} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			ctx := context.Background()
			items := benchItems(size)
			b.ResetTimer()
			for range b.N {
				stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
				if err != nil {
					b.Fatal(err)
				}
				stack.SyncItems(ctx, items)
				_ = stack.Close()
			}
		})
	}
}

func BenchmarkSyncItems_ExistingItems(b *testing.B) {
	ctx := context.Background()
	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = stack.Close() }()
	items := benchItems(100)
	stack.SyncItems(ctx, items)
	for b.Loop() {
		stack.SyncItems(ctx, items)
	}
}
