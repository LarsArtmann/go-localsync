package sync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	errorfamily "github.com/larsartmann/go-error-family"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type mockSyncStore struct {
	testutil.SyncStoreListBehavior

	synced          []*provider.Item
	actions         []SyncAction
	actionIdx       int
	countErr        error
	closeErr        error
	reconcileCalled bool
	reconcileSeen   []model.Key
	reconcileResult int
	reconcileErr    error
}

func (m *mockSyncStore) SyncItems(_ context.Context, items []*provider.Item) *BatchOutcome {
	summary := &BatchOutcome{Results: make([]ItemSyncResult, 0, len(items))}

	for _, item := range items {
		action := ActionCreated
		if len(m.actions) > 0 && m.actionIdx < len(m.actions) {
			action = m.actions[m.actionIdx]
			m.actionIdx++
		}

		if action != ActionError {
			m.synced = append(m.synced, item)
		}

		summary.Results = append(summary.Results, ItemSyncResult{
			SourceID: item.SourceID,
			Action:   action,
		})
		switch action {
		case ActionCreated, ActionUpdated, ActionConflictRemote, ActionConflictLocal:
			summary.Synced++
		case ActionError:
			summary.Errors++
		case ActionUnchanged, ActionTombstoned:
		}
	}

	return summary
}

func (m *mockSyncStore) Count(_ context.Context, _ model.ItemFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}

	return int64(len(m.synced)), nil
}

func (m *mockSyncStore) CountByType(_ context.Context, _ model.ItemFilter) (map[string]int64, error) {
	if m.countErr != nil {
		return nil, m.countErr
	}

	counts := make(map[string]int64)

	for _, item := range m.synced {
		counts[item.Type.Get()]++
	}

	return counts, nil
}

func (m *mockSyncStore) Close() error { return m.closeErr }

func (m *mockSyncStore) Reconcile(_ context.Context, _ string, seen []model.Key) (int, error) {
	m.reconcileCalled = true
	m.reconcileSeen = seen

	if m.reconcileErr != nil {
		return 0, m.reconcileErr
	}

	return m.reconcileResult, nil
}

func testSyncOpts() *SyncOptions {
	return &SyncOptions{Source: "testuser", MaxPages: 10}
}

func newTestSyncer(items []*provider.Item) (*Syncer, *mockSyncStore) {
	store := &mockSyncStore{}
	p := &testutil.MockProvider{Items: items}
	logger := log.Default()

	return NewSyncer(p, store, logger), store
}

func testSyncItem(sourceID, eventType string) *provider.Item {
	now := time.Now()

	return &provider.Item{
		ID:       id.NewItemID(),
		SourceID: id.NewSourceID(sourceID),
		Source:   id.NewProviderID("github"),
		Type:     id.NewEventTypeID(eventType),
		Attributes: map[string]string{
			"actor_login": "testuser",
			"repo_name":   "test/repo",
		},
		CreatedAt: now,
		UpdatedAt: now,
		RawJSON:   []byte(`{}`),
	}
}

// testSyncItems constructs a slice of test sync items from (sourceID, eventType) pairs.
//
//	testSyncItems("1", "PushEvent", "2", "IssueEvent")
func testSyncItems(pairs ...string) []*provider.Item {
	return testutil.BuildPairs(testSyncItem, pairs...)
}

func testDataItem(sourceID, eventType string) *model.Item {
	now := time.Now()

	return &model.Item{
		ID:       id.NewItemID(),
		SourceID: id.NewSourceID(sourceID),
		Source:   id.NewProviderID("github"),
		Type:     id.NewEventTypeID(eventType),
		Attributes: map[string]string{
			"actor_login": "testuser",
			"repo_name":   "test/repo",
		},
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: schema.CurrentVersion(),
	}
}

func TestSyncer_Sync(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent")

	syncer, store := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertEqual(t, result.Fetched, 2, "Fetched")

	count, _ := store.Count(ctx, model.ItemFilter{})
	testutil.AssertEqual(t, count, 2, "count")
}

func TestSyncer_Sync_EmptyResult(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertEqual(t, result.Fetched, 0, "Fetched")
	testutil.AssertEqual(t, result.Errors, 0, "Errors")
}

func TestSyncer_Sync_InvalidItem(t *testing.T) {
	t.Parallel()

	invalidItem := &provider.Item{
		ID: id.NewItemID(),
	}

	syncer, _ := newTestSyncer([]*provider.Item{invalidItem})
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.Sync(ctx, testSyncOpts())
	if !errors.Is(err, pkgerrors.ErrPartialSync) {
		t.Fatalf("expected ErrPartialSync when all items are invalid, got: %v", err)
	}
	testutil.AssertEqual(t, result.Fetched, 1, "Fetched")
	testutil.AssertEqual(t, result.Errors, 1, "Errors")
}

func TestSyncer_Sync_NilOptions(t *testing.T) {
	t.Parallel()

	syncer, _ := newTestSyncer(nil)
	defer func() { _ = syncer.Close() }()

	_, err := syncer.Sync(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSyncer_SyncIncremental_FallsBackToFull(t *testing.T) {
	t.Parallel()

	items := []*provider.Item{testSyncItem("1", "PushEvent")}

	syncer, store := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	result, err := syncer.SyncIncremental(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertEqual(t, result.Fetched, 1, "Fetched")

	count, _ := store.Count(ctx, model.ItemFilter{})
	testutil.AssertEqual(t, count, 1, "count")
}

func TestSyncer_Stats(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent", "2", "IssueEvent", "3", "PushEvent")

	syncer, _ := newTestSyncer(items)
	defer func() { _ = syncer.Close() }()

	ctx := context.Background()
	_, err := syncer.Sync(ctx, testSyncOpts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats, err := syncer.Stats(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertEqual(t, stats.TotalItems, 3, "TotalItems")
	testutil.AssertContains(t, stats.ItemTypes, "PushEvent", "ItemTypes")
	testutil.AssertContains(t, stats.ItemTypes, "IssueEvent", "ItemTypes")
}

func TestSyncer_Close(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{closeErr: errors.New("close failed")}
	syncer := NewSyncer(&testutil.MockProvider{}, store, log.Default())

	err := syncer.Close()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSyncOptions_Validate(t *testing.T) {
	err := (&SyncOptions{}).Validate()
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error to contain 'required', got %v", err)
	}

	err = (&SyncOptions{Source: "github", MaxPages: -1}).Validate()
	if err == nil {
		t.Fatal("expected error for negative MaxPages")
	}

	fieldErr, ok := errors.AsType[*errorfamily.Error](err)
	if !ok || fieldErr.ErrorContext()["field"] != "maxPages" {
		t.Errorf("expected InvalidField context field=maxPages, got %v (ctx=%v)", err, fieldErr.ErrorContext())
	}

	if err := (&SyncOptions{Source: "github", MaxPages: 0}).Validate(); err != nil {
		t.Errorf("MaxPages 0 (unlimited) must be valid, got %v", err)
	}
}

// TestSyncOptions_Validate_RejectsNegativeMaxPages is covered in
// TestSyncOptions_Validate above; this file otherwise owns the tracer span
// tests introduced with WithTracer.

// TestSyncer_Tracer_RecordsSpansWithStatus proves WithTracer emits real,
// inspectable spans for both entry points, including error status when the
// run fails validation — not noop-swallowed wiring.
func TestSyncer_Tracer_RecordsSpansWithStatus(t *testing.T) {
	t.Parallel()

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	syncer, _ := newTestSyncer([]*provider.Item{testSyncItem("span-1", "PushEvent")})
	syncer.tracer = tp.Tracer("sync-tracer-test")

	ctx := context.Background()

	if _, err := syncer.Sync(ctx, testSyncOpts()); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	var syncSpan trace.Span
	_ = syncSpan

	var fullSpan sdktrace.ReadOnlySpan

	for _, span := range recorder.Ended() {
		if span.Name() == "localsync.sync" {
			fullSpan = span

			break
		}
	}

	if fullSpan == nil {
		t.Fatal("localsync.sync span was not recorded")
	}

	if fullSpan.SpanKind() != trace.SpanKindInternal {
		t.Errorf("span kind = %v, want internal", fullSpan.SpanKind())
	}

	if fullSpan.Status().Code != codes.Unset {
		t.Errorf("successful run must leave span status unset, got %v", fullSpan.Status().Code)
	}

	// Outcome attributes (parity with the CQRS batch span): the run's counts
	// must be readable from the span itself.
	attrs := map[string]int64{}
	for _, kv := range fullSpan.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsInt64()
	}

	for key, want := range map[string]int64{
		"localsync.fetched": 1,
		"localsync.skipped": 0,
		"localsync.errors":  0,
		"localsync.synced":  1,
	} {
		if got := attrs[key]; got != want {
			t.Errorf("span attribute %s = %d, want %d (attrs: %v)", key, got, want, attrs)
		}
	}

	// A validation failure happens inside the span and must be recorded.
	_, err := syncer.Sync(ctx, &SyncOptions{Source: "", MaxPages: 1})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var failSpan sdktrace.ReadOnlySpan
	for _, span := range recorder.Ended() {
		if span.Name() == "localsync.sync" && span.Status().Code == codes.Error {
			failSpan = span

			break
		}
	}

	if failSpan == nil {
		t.Fatal("failed run must record an error-status localsync.sync span")
	}

	if len(failSpan.Events()) == 0 {
		t.Error("error span must carry a recorded error event")
	}
}

// TestSyncer_Tracer_IncrementalSpan pins the incremental entry point's span
// name so dashboards can rely on it.
func TestSyncer_Tracer_IncrementalSpan(t *testing.T) {
	t.Parallel()

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	syncer, _ := newTestSyncer([]*provider.Item{testSyncItem("span-inc", "PushEvent")})
	syncer.tracer = tp.Tracer("sync-tracer-test")

	if _, err := syncer.SyncIncremental(context.Background(), testSyncOpts()); err != nil {
		t.Fatalf("incremental sync failed: %v", err)
	}

	for _, span := range recorder.Ended() {
		if span.Name() == "localsync.sync_incremental" {
			return
		}
	}

	t.Fatal("localsync.sync_incremental span was not recorded")
}
