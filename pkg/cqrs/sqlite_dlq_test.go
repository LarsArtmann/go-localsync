package cqrs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestSQLiteStore_WiresPersistentDLQ verifies the C017 fix: the sqlite branch
// of the store factory must hand the projection runner a dead-letter store
// backed by the same SQLite file, so captured poison events survive restarts.
func TestSQLiteStore_WiresPersistentDLQ(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq-wiring.db")

	sr, err := createStoreAndBus(context.Background(), CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
	testutil.MustNoError(t, err)
	defer closeStoreResult(t, sr)

	if sr.dlq == nil {
		t.Fatal("sqlite backend must wire a dead-letter store")
	}
	if _, ok := sr.dlq.(*projectionhost.SQLiteDeadLetterStore); !ok {
		t.Fatalf("sqlite backend must use the persistent SQLite DLQ, got %T", sr.dlq)
	}
}

// TestSQLiteStore_MemoryBackendKeepsMemoryDLQ pins the other half of the
// contract: the memory backend pairs its ephemeral store with an in-memory
// dead-letter store — DLQ lifetime matches event-store lifetime.
func TestSQLiteStore_MemoryBackendKeepsMemoryDLQ(t *testing.T) {
	t.Parallel()

	sr, err := createStoreAndBus(context.Background(), CQRSConfig{Backend: backendMemory})
	testutil.MustNoError(t, err)

	if _, ok := sr.dlq.(*projectionhost.MemoryDeadLetterStore); !ok {
		t.Fatalf("memory backend must use the in-memory DLQ, got %T", sr.dlq)
	}
}

// TestSQLiteDeadLetterStore_PersistsAcrossReopen proves the durability the C017
// fix relies on: an entry captured against a file-backed database is still
// listable after the database is closed and reopened.
func TestSQLiteDeadLetterStore_PersistsAcrossReopen(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq-persist.db")
	ctx := context.Background()

	entry := sampleDeadLetterEntry(t, "dlq-persist-1")

	func() {
		sr, err := createStoreAndBus(ctx, CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
		testutil.MustNoError(t, err)
		defer closeStoreResult(t, sr)

		testutil.MustNoError(t, sr.dlq.Store(ctx, entry))
	}()

	// Reopen through the same factory path a restarted stack takes.
	sr2, err := createStoreAndBus(ctx, CQRSConfig{Backend: backendSQLite, DBPath: dbPath})
	testutil.MustNoError(t, err)
	defer closeStoreResult(t, sr2)

	entries, err := sr2.dlq.List(ctx, entry.ProjectionName)
	testutil.MustNoError(t, err)

	if len(entries) != 1 {
		t.Fatalf("expected 1 dead-letter entry after reopen, got %d", len(entries))
	}

	got := entries[0]
	if got.EventID != entry.EventID {
		t.Errorf("EventID mismatch: want %q, got %q", entry.EventID, got.EventID)
	}
	if got.Error != entry.Error {
		t.Errorf("Error mismatch: want %q, got %q", entry.Error, got.Error)
	}
	if got.Event.Type() != entry.Event.Type() {
		t.Errorf("event type mismatch: want %q, got %q", entry.Event.Type(), got.Event.Type())
	}
}

func sampleDeadLetterEntry(t *testing.T, sourceID string) projectionhost.DeadLetterEntry {
	t.Helper()

	evts, err := event.NewEvents(
		MustStreamID("github", id.NewSourceID(sourceID)),
		aggregateType,
		event.Version(1),
		[]event.Type{EventItemSynced},
		[]any{ItemSyncedPayload{
			ItemID:    "ulid-1",
			Source:    "github",
			SourceID:  sourceID,
			Type:      "PushEvent",
			CreatedAt: 1,
			UpdatedAt: 2,
		}},
	)
	testutil.MustNoError(t, err)

	return projectionhost.DeadLetterEntry{
		ProjectionName: projectionName,
		EventID:        evts[0].ID().String(),
		EventType:      EventItemSynced.String(),
		StreamID:       evts[0].StreamID().String(),
		Event:          evts[0],
		Error:          "poison payload",
		FailedAt:       time.Now().UTC(),
	}
}

func closeStoreResult(t *testing.T, sr storeResult) {
	t.Helper()

	if sr.db != nil {
		_ = sr.db.Close()
	}
}
