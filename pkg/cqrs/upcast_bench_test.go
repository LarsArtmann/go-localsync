package cqrs

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/schema/v4"
	cqrsstorage "github.com/larsartmann/go-cqrs-lite/storage/v4"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// BenchmarkConflict_SyncExisting runs the full sync pipeline against items
// that ALREADY exist with divergent content, so the conflict resolver runs
// per item — the most expensive per-item path (conflict detection +
// resolution + ItemConflictFound + ItemSynced, no insert fast path).
func BenchmarkConflict_SyncExisting(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "conflict-bench.db")

	resolver, rerr := crdt.NewLWWResolver[*model.Item](func(i *model.Item) time.Time { return i.UpdatedAt })
	if rerr != nil {
		b.Fatal(rerr)
	}

	stack, err := NewCQRSStack(CQRSConfig{
		Backend:          backendSQLite,
		DBPath:           dbPath,
		ConflictResolver: resolver,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	const perIteration = 200

	existing := make([]*provider.Item, 0, perIteration)
	for i := range perIteration {
		now := time.Now().Add(-time.Hour)

		existing = append(existing, &provider.Item{
			ExternalID: id.NewExternalID("conflict-" + strconv.Itoa(i)),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			Attributes: map[string]string{"actor_login": "bencher"},
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	if summary := stack.SyncItems(ctx, existing); summary.Errors > 0 {
		b.Fatalf("seed errors: %d", summary.Errors)
	}

	// Diverge every item so each re-sync is a guaranteed conflict with a
	// remote-newer LWW outcome.
	b.ResetTimer()

	for b.Loop() {
		divergent := make([]*provider.Item, 0, perIteration)

		for i, item := range existing {
			next := *item
			next.UpdatedAt = time.Now().Add(time.Duration(i) * time.Millisecond)
			next.Attributes = map[string]string{"actor_login": "bencher", "rev": strconv.Itoa(i)}
			divergent = append(divergent, &next)
		}

		if summary := stack.SyncItems(ctx, divergent); summary.Conflicts != perIteration {
			b.Fatalf("expected %d conflicts, got %d", perIteration, summary.Conflicts)
		}
	}
}

// seedUpcastStream persists count ItemSynced events into a fresh SQLite file
// as either raw V1 payloads (upcast pipeline runs on every read) or native
// V3 payloads (pass-through fast path).
func seedUpcastStream(b *testing.B, dbPath string, legacy bool) {
	b.Helper()

	ctx := context.Background()

	sr, err := createStoreAndBus(ctx, CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
	if err != nil {
		b.Fatal(err)
	}

	aggID := AggregateID("github", id.NewExternalID("upcast-bench"))
	ref := cqrsid.NewStreamRef(aggregateType, aggID)

	const count = 1_000

	for i := range count {
		var payload ItemSyncedPayload
		var version event.SchemaVersion

		if legacy {
			payload = legacyV1Payload()
			payload.SourceID = "upcast-" + strconv.Itoa(i)
			version = legacySchemaV1
		} else {
			payload = ItemSyncedPayload{
				Source: "github", SourceID: "upcast-" + strconv.Itoa(i), Type: "PushEvent",
				Attributes: map[string]string{"actor_login": "native"},
			}
			version = currentSchemaV
		}

		events, evErr := event.NewEvents(aggID, aggregateType, event.Version(i),
			[]event.Type{EventItemSynced}, []any{payload}, event.WithSchemaVersion(version))
		if evErr != nil {
			b.Fatal(evErr)
		}

		if sErr := sr.store.Save(ctx, ref, events, event.Version(i)); sErr != nil {
			b.Fatal(sErr)
		}
	}

	if closer, ok := sr.store.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			b.Fatal(err)
		}
	}

	if sr.db != nil {
		if err := sr.db.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUpcastedLegacyRead isolates the schema-evolution cost: loading a
// stream of 1k raw V1 events (upcast pipeline on every read) versus a native
// V3 stream of the same size (pass-through). The delta is the honest price
// of legacy compatibility.
func BenchmarkUpcastedLegacyRead(b *testing.B) {
	ctx := context.Background()

	for _, tc := range []struct {
		name   string
		legacy bool
	}{
		{"legacy-v1-upcast-on-read", true},
		{"native-v3-passthrough", false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			dbPath := filepath.Join(b.TempDir(), "upcast-bench.db")
			seedUpcastStream(b, dbPath, tc.legacy)

			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				b.Fatal(err)
			}
			defer func() { _ = db.Close() }()

			raw, err := cqrsstorage.NewSQLiteEventStore(db)
			if err != nil {
				b.Fatal(err)
			}

			store := event.DecorateStore(raw, nil, schema.UpcastSourceTransform(newLegacyUpcasters()...))

			aggID := AggregateID("github", id.NewExternalID("upcast-bench"))
			ref := cqrsid.NewStreamRef(aggregateType, aggID)

			b.ResetTimer()

			for b.Loop() {
				loaded, loadErr := store.Load(ctx, ref)
				if loadErr != nil {
					b.Fatal(loadErr)
				}

				if len(loaded) != 1_000 {
					b.Fatalf("expected 1000 events, got %d", len(loaded))
				}
			}
		})
	}
}
