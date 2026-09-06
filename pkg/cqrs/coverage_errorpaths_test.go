package cqrs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// The tests in this file target error paths and adapter branches that the
// happy-path suites leave uncovered (store-factory failure branches,
// CountByType across all three surfaces, legacy payload upcasting).

func TestMemoryReadModel_CountByType(t *testing.T) {
	t.Parallel()

	rm := NewMemoryReadModel()
	ctx := context.Background()

	upsertMemoryItem(t, rm, "cbt-1", "PushEvent")
	upsertMemoryItem(t, rm, "cbt-2", "PushEvent")
	upsertMemoryItem(t, rm, "cbt-3", "IssueEvent")

	counts, err := rm.CountByType(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)

	if counts["PushEvent"] != 2 || counts["IssueEvent"] != 1 {
		t.Errorf("CountByType = %v, want PushEvent=2 IssueEvent=1", counts)
	}

	pushType := id.NewEventTypeID("PushEvent")

	filtered, err := rm.CountByType(ctx, model.ItemFilter{Type: &pushType})
	testutil.MustNoError(t, err)

	if filtered["PushEvent"] != 2 || filtered["IssueEvent"] != 0 {
		t.Errorf("filtered CountByType = %v, want only PushEvent", filtered)
	}
}

func TestCQRSStack_CountByType_Promoted(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "scbt-1", "PushEvent")
	syncTestItem(t, stack, ctx, "scbt-2", "WatchEvent")

	waitForCount(t, stack, ctx, 2)

	counts, err := stack.CountByType(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)

	if counts["PushEvent"] != 1 || counts["WatchEvent"] != 1 {
		t.Errorf("stack.CountByType = %v, want one of each", counts)
	}
}

func TestSQLiteReadModel_CountByType(t *testing.T) {
	t.Parallel()

	stack := newSQLiteMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()

	syncTestItem(t, stack, ctx, "sqcbt-1", "PushEvent")
	syncTestItem(t, stack, ctx, "sqcbt-2", "PushEvent")
	syncTestItem(t, stack, ctx, "sqcbt-3", "ReleaseEvent")

	waitForCount(t, stack, ctx, 3)

	counts, err := stack.CountByType(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)

	if counts["PushEvent"] != 2 || counts["ReleaseEvent"] != 1 {
		t.Errorf("sqlite CountByType = %v, want PushEvent=2 ReleaseEvent=1", counts)
	}
}

// TestCreateStore_BadSQLitePath covers the store-factory failure branch: an
// unopenable DB path must surface a wrapped error (not panic, not nil).
func TestCreateStore_BadSQLitePath(t *testing.T) {
	t.Parallel()

	badPath := filepath.Join(t.TempDir(), "subdir-that-does-not", "exist", "db.sqlite")

	_, err := createStoreAndBus(context.Background(), CQRSConfig{Backend: backendSQLite, DBPath: badPath})
	if err == nil {
		t.Fatal("expected an error for an unopenable sqlite path")
	}
}

func TestCreateStore_UnknownBackend(t *testing.T) {
	t.Parallel()

	_, err := createStoreAndBus(context.Background(), CQRSConfig{Backend: "postgres"})
	if err == nil {
		t.Fatal("expected an error for an unknown backend")
	}
}

// TestUpcastLegacyAttributes_FullMatrix walks every legacy V1/V2 field into
// the Attributes map and checks empty payloads stay empty.
func TestUpcastLegacyAttributes_FullMatrix(t *testing.T) {
	t.Parallel()

	full := upcastLegacyAttributes(ItemSyncedPayload{
		ActorLogin:     "octocat",
		ActorAvatarURL: "https://avatars.example/u/1",
		RepoName:       "octo/hello",
		RepoURL:        "https://github.com/octo/hello",
	})

	for _, key := range []string{"actor_login", "actor_avatar_url", "repo_name", "repo_url"} {
		if full[key] == "" {
			t.Errorf("legacy field %q missing from upcast attributes: %v", key, full)
		}
	}

	empty := upcastLegacyAttributes(ItemSyncedPayload{})
	if len(empty) != 0 {
		t.Errorf("empty legacy payload must upcast to empty attributes, got %v", empty)
	}
}

func upsertMemoryItem(t *testing.T, rm ReadModel, externalID, itemType string) {
	t.Helper()

	item := testItem(externalID, itemType)

	testutil.MustNoError(t, rm.Upsert(context.Background(), toDataItem(item)))
}

// TestNewSQLiteReadModel_NilDB covers the db-nil guard branch.
func TestNewSQLiteReadModel_NilDB(t *testing.T) {
	t.Parallel()

	_, err := newSQLiteReadModel(context.Background(), nil)
	if err == nil {
		t.Fatal("expected an error for a nil db handle")
	}
}

// TestDataItemToPayload_NilItem covers the nil guard; a nil item must yield
// an empty payload rather than panic.
func TestDataItemToPayload_NilItem(t *testing.T) {
	t.Parallel()

	if got := dataItemToPayload(nil, nil); got.ItemID != "" || got.Source != "" {
		t.Errorf("nil item must map to an empty payload, got %+v", got)
	}
}

// TestToDataItem_CarriesRawJSON walks the provider->data item adapter,
// including the RawJSON passthrough branch.
func TestToDataItem_CarriesRawJSON(t *testing.T) {
	t.Parallel()

	src := testItem("adapter-1", "PushEvent")
	src.RawJSON = []byte(`{"hello":true}`)

	got := toDataItem(src)

	if got.ContentHash.String() != hashRawJSON(src.RawJSON) {
		t.Errorf("ContentHash = %q, want sha256 of RawJSON", got.ContentHash)
	}

	if got.SchemaVersion != schemaCurrentVersionForTest() {
		t.Errorf("SchemaVersion = %d, want current", got.SchemaVersion)
	}

	if toDataItem(nil) != nil {
		t.Error("nil provider item must map to nil data item")
	}
}

func schemaCurrentVersionForTest() schema.Version { return schema.CurrentVersion() }

func TestCQRSConfig_Validate(t *testing.T) {
	t.Parallel()

	if err := (CQRSConfig{}).Validate(); err != nil {
		t.Errorf("empty backend defaults to memory, must validate: %v", err)
	}

	if err := (CQRSConfig{Backend: backendMemory}).Validate(); err != nil {
		t.Errorf("memory backend must validate: %v", err)
	}

	if err := (CQRSConfig{Backend: backendSQLite, DBPath: "x.db"}).Validate(); err != nil {
		t.Errorf("sqlite backend must validate: %v", err)
	}

	if err := (CQRSConfig{Backend: "postgres"}).Validate(); err == nil {
		t.Error("unknown backend must be rejected")
	}
}

// TestSyncItemsWithResolver_NilFallsBack verifies the nil fast path.
func TestSyncItemsWithResolver_NilFallsBack(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	summary := stack.SyncItemsWithResolver(ctx, testItems("rnil-1", "PushEvent", "rnil-2", "IssueEvent"), nil)
	if summary.Synced != 2 {
		t.Errorf("nil resolver must behave like SyncItems, got %d synced", summary.Synced)
	}

	waitForCount(t, stack, ctx, 2)
}

// TestCleanupFailedConstruction_ReleasesAllResources drives the error-path
// cleanup directly with every resource populated: cancel runs, the drain
// channel is awaited, and each closer is invoked (memory store/bus are not
// io.Closers; the read model is).
func TestCleanupFailedConstruction_ReleasesAllResources(t *testing.T) {
	t.Parallel()

	sr, err := createStoreAndBus(context.Background(), CQRSConfig{Backend: backendMemory})
	testutil.MustNoError(t, err)

	cancelled := false
	drainDone := make(chan struct{})
	close(drainDone)

	cleanupFailedConstruction(sr, NewMemoryReadModel(), func() { cancelled = true }, drainDone)

	if !cancelled {
		t.Error("cancelRunner must be invoked")
	}

	cleanupFailedConstruction(sr, nil, nil, nil)
}

// TestUpcastLegacyAttributes_RoundTripThroughAccessors pins the single
// source of truth for attribute keys: the adapter folds legacy fields into
// Attributes under the model.Attr* constants, and the model's typed
// accessors read those exact entries back. A key drift in either direction
// fails here.
func TestUpcastLegacyAttributes_RoundTripThroughAccessors(t *testing.T) {
	t.Parallel()

	attrs := upcastLegacyAttributes(ItemSyncedPayload{
		ActorLogin:     "octocat",
		ActorAvatarURL: "https://avatars.example/u/1",
		RepoName:       "octo/hello",
		RepoURL:        "https://github.com/octo/hello",
	})

	item := &model.Item{Attributes: attrs}

	if item.ActorLogin() != "octocat" {
		t.Errorf("ActorLogin() = %q, want octocat", item.ActorLogin())
	}

	if item.RepoName() != "octo/hello" {
		t.Errorf("RepoName() = %q, want octo/hello", item.RepoName())
	}

	if item.RepoURL() != "https://github.com/octo/hello" {
		t.Errorf("RepoURL() = %q", item.RepoURL())
	}
}
