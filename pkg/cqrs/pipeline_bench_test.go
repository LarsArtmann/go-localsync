package cqrs

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// Full-pipeline benchmarks: batch → dispatch → decide → persist → project.
// These complement the micro-benchmarks (adapter, readmodel, stack) by
// measuring the end-to-end path a consumer's sync run actually takes.

// BenchmarkPipeline_Sync10kItems drives a 10k-item batch through the memory
// backend: dispatch, decider, store, bus, projection. The memory backend
// isolates the engine cost from SQLite I/O.
func BenchmarkPipeline_Sync10kItems(b *testing.B) {
	stack, err := NewCQRSStack(CQRSConfig{Backend: backendMemory})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	items := benchProviderItemsRange(0, 10_000)

	b.ResetTimer()

	for b.Loop() {
		summary := stack.SyncItems(ctx, items)
		if summary.Errors > 0 {
			b.Fatalf("sync errors: %d", summary.Errors)
		}
	}
}

// BenchmarkPipeline_Replay10kEvents measures catch-up cost after a restart:
// 10k events are persisted first, then each iteration reopens a fresh stack
// and replays the journal through the projection (the ADR-0006 bounded-replay
// path; the checkpoint table is empty in a fresh temp DB copy is NOT used —
// the same file is reused, so only the FIRST iteration replays from zero;
// subsequent iterations measure checkpoint-bounded no-op catch-up).
func BenchmarkPipeline_Replay10kEvents(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "replay-bench.db")

	seed, err := NewCQRSStack(CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	const total = 10_000
	const batchSize = 500

	for offset := 0; offset < total; offset += batchSize {
		end := offset + batchSize
		if end > total {
			end = total
		}

		if summary := seed.SyncItems(ctx, benchProviderItemsRange(offset, end)); summary.Errors > 0 {
			b.Fatalf("seed errors: %d", summary.Errors)
		}
	}

	waitForCountTB(b, seed, ctx, total)

	if err := seed.Close(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		replay, rerr := NewCQRSStack(CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
		if rerr != nil {
			b.Fatal(rerr)
		}

		waitForCountTB(b, replay, ctx, total)

		if cerr := replay.Close(); cerr != nil {
			b.Fatal(cerr)
		}
	}
}

// BenchmarkPipeline_SQLiteGrowth runs successive batches against ONE SQLite
// file so the database grows across iterations — exposing write-amplification
// or index-degradation curves that fresh-database benchmarks hide.
func BenchmarkPipeline_SQLiteGrowth(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "growth-bench.db")

	stack, err := NewCQRSStack(CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	batch := 0

	b.ResetTimer()

	for b.Loop() {
		base := batch * 100
		batch++

		if summary := stack.SyncItems(ctx, benchProviderItemsRange(base, base+100)); summary.Errors > 0 {
			b.Fatalf("growth batch errors: %d", summary.Errors)
		}
	}
}

func benchProviderItemsRange(from, to int) []*provider.Item {
	items := make([]*provider.Item, 0, to-from)

	for i := from; i < to; i++ {
		now := time.Now()

		items = append(items, &provider.Item{
			ID:         id.NewItemID(),
			ExternalID: id.NewExternalID(fmt.Sprintf("pipe-%d", i)),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			Attributes: map[string]string{
				"actor_login": "benchuser",
				"repo_name":   fmt.Sprintf("bench/repo-%d", i%64),
			},
			CreatedAt: now,
			UpdatedAt: now,
			RawJSON:   []byte(fmt.Sprintf(`{"bench":%d,"payload":"%s"}`, i, fillerPayload)),
		})
	}

	return items
}

const fillerPayload = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// waitForCountTB is the testing.TB generalization of the test helper.
func waitForCountTB(tb testing.TB, stack *CQRSStack, ctx context.Context, want int) {
	tb.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		count, err := stack.Count(ctx, model.ItemFilter{})
		if err != nil {
			tb.Fatal(err)
		}

		if int(count) >= want {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	tb.Fatalf("timed out waiting for count=%d", want)
}
