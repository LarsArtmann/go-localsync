package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
	_ "modernc.org/sqlite" // register the pure-Go sqlite driver for the sqlite backend
)

func waitForProjection(t *testing.T, ctx context.Context, stack *cqrs.CQRSStack, want int64) {
	t.Helper()

	testutil.WaitForCount(t, ctx, func(ctx context.Context) (int64, error) {
		return stack.Count(ctx, model.ItemFilter{})
	}, want)
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()

	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", s, err)
	}

	return v
}

func makeTestItem(t *testing.T, sourceID, eventType, date string) *provider.Item {
	t.Helper()

	ts := mustParseTime(t, date)

	return &provider.Item{
		SourceID: id.NewSourceID(sourceID),
		Source:   id.NewProviderID("github"),
		Type:     id.NewEventTypeID(eventType),
		Attributes: map[string]string{
			"actor_login": "testuser",
			"repo_name":   "test/repo",
		},
		CreatedAt: ts,
		UpdatedAt: ts,
		RawJSON:   []byte(`{}`),
	}
}

// TestIntegration_APIListItemsRoundtrip verifies that items synced through
// the CQRS stack are correctly exposed via the HTTP API.
func TestIntegration_APIListItemsRoundtrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{
		makeTestItem(t, "1", "PushEvent", "2024-01-01T00:00:00Z"),
		makeTestItem(t, "2", "IssueEvent", "2024-01-02T00:00:00Z"),
	}

	stack.SyncItems(ctx, items)

	// Wait for projection to catch up.
	waitForProjection(t, ctx, stack, 2)

	provider := &testutil.MockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(provider, stack, logger)
	server := NewServer(syncer, logger)

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/items", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)

	var body struct {
		Items []*ItemResponse `json:"items"`
		Total int64           `json:"total"`
	}

	err = json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Total != 2 {
		t.Errorf("expected total=2, got %d", body.Total)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(body.Items))
	}

	for _, item := range body.Items {
		if item.Source != "github" {
			t.Errorf("expected Source=github, got %s", item.Source)
		}
		if item.ID == "" {
			t.Error("expected non-empty ID")
		}
	}
}

// TestIntegration_APIStatsRoundtrip verifies that aggregate statistics
// exposed via the API reflect the synced items.
func TestIntegration_APIStatsRoundtrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{
		makeTestItem(t, "1", "PushEvent", "2024-01-01T00:00:00Z"),
	}

	stack.SyncItems(ctx, items)

	waitForProjection(t, ctx, stack, 1)

	provider := &testutil.MockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(provider, stack, logger)
	server := NewServer(syncer, logger)

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)

	var body struct {
		TotalItems int64    `json:"totalItems"`
		ItemTypes  []string `json:"itemTypes"`
	}

	err = json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.TotalItems != 1 {
		t.Errorf("expected totalItems=1, got %d", body.TotalItems)
	}
	testutil.AssertLen(t, body.ItemTypes, 1, "item types")
}

// TestIntegration_APIFilterAndPagination verifies that query parameters
// for type, source, and pagination work end-to-end.
func TestIntegration_APIFilterAndPagination(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	testutil.MustNoError(t, err)
	t.Cleanup(func() { _ = stack.Close() })

	items := []*provider.Item{
		makeTestItem(t, "1", "PushEvent", "2024-01-01T00:00:00Z"),
		makeTestItem(t, "2", "PushEvent", "2024-01-02T00:00:00Z"),
		makeTestItem(t, "3", "IssueEvent", "2024-01-03T00:00:00Z"),
	}

	stack.SyncItems(ctx, items)
	waitForProjection(t, ctx, stack, 3)

	mockProvider := &testutil.MockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(mockProvider, stack, logger)
	server := NewServer(syncer, logger)

	t.Run("filter_by_type", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/items?type=PushEvent", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		testutil.AssertStatusOK(t, rec)

		var body struct {
			Items []*ItemResponse `json:"items"`
			Total int64           `json:"total"`
		}

		err := json.Unmarshal(rec.Body.Bytes(), &body)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if body.Total != 2 {
			t.Errorf("expected total=2 for PushEvent filter, got %d", body.Total)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/items?limit=1&offset=1", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		testutil.AssertStatusOK(t, rec)

		var body struct {
			Items []*ItemResponse `json:"items"`
			Total int64           `json:"total"`
		}

		err := json.Unmarshal(rec.Body.Bytes(), &body)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if len(body.Items) != 1 {
			t.Errorf("expected 1 item with limit=1, got %d", len(body.Items))
		}

		if body.Total != 3 {
			t.Errorf("expected total=3 (all items), got %d", body.Total)
		}
	})
}

// TestIntegration_APICursorPagination_SQLite drives cursor pagination against
// the REAL SQLite read model (not the fake store the unit pagination tests
// use): three items, two-per-page walk via X-Next-Cursor, then equality with
// a direct store List — proving cursor pages partition the store's own
// ordering with no overlap and no gaps.
func TestIntegration_APICursorPagination_SQLite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "sqlite"})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{
		makeTestItem(t, "cursor-1", "PushEvent", "2024-01-01T00:00:00Z"),
		makeTestItem(t, "cursor-2", "PushEvent", "2024-01-02T00:00:00Z"),
		makeTestItem(t, "cursor-3", "IssueEvent", "2024-01-03T00:00:00Z"),
	}

	testutil.MustNoError(t, stack.SyncItem(ctx, items[0]))
	testutil.MustNoError(t, stack.SyncItem(ctx, items[1]))
	testutil.MustNoError(t, stack.SyncItem(ctx, items[2]))

	waitForProjection(t, ctx, stack, 3)

	mockProvider := &testutil.MockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(mockProvider, stack, logger)
	server := NewServer(syncer, logger)

	var got []string

	cursor := ""
	pages := 0

	for {
		path := "/items?limit=2"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		testutil.AssertStatusOK(t, rec)

		var body struct {
			Items []struct {
				SourceID string `json:"sourceId"`
			} `json:"items"`
			Total int64 `json:"total"`
		}

		err = json.Unmarshal(rec.Body.Bytes(), &body)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if body.Total != 3 {
			t.Errorf("page %d: total must always be 3, got %d", pages+1, body.Total)
		}

		for _, item := range body.Items {
			got = append(got, item.SourceID)
		}

		pages++

		cursor = rec.Header().Get("X-Next-Cursor")
		if cursor == "" {
			break
		}

		if pages > 10 {
			t.Fatal("cursor pagination did not terminate within 10 pages")
		}
	}

	if pages != 2 {
		t.Errorf("expected exactly 2 pages of 3 items at limit=2, got %d pages", pages)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 items across pages, got %d: %v", len(got), got)
	}

	seen := map[string]bool{}
	for _, sid := range got {
		if seen[sid] {
			t.Errorf("sourceId %s returned twice across cursor pages", sid)
		}

		seen[sid] = true
	}

	// The concatenated pages must reproduce the store's own List ordering.
	direct, listErr := stack.List(ctx, model.ItemFilter{})
	testutil.MustNoError(t, listErr)

	if len(direct) != len(got) {
		t.Fatalf("direct List returned %d items, API pages carried %d", len(direct), len(got))
	}

	for i, item := range direct {
		if item.SourceID.Get() != got[i] {
			t.Fatalf("page order diverges from store order at %d: store=%s api=%s", i, item.SourceID.Get(), got[i])
		}
	}
}

// waitForLiveCount polls the store's DEFAULT view (tombstoned excluded) until
// it holds exactly want items — the tombstone projection signal: the count
// drops when the tombstoned row flips.
func waitForLiveCount(t *testing.T, ctx context.Context, stack *cqrs.CQRSStack, want int64) {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		count, err := stack.Count(ctx, model.ItemFilter{})
		testutil.MustNoError(t, err)

		if count == want {
			return
		}

		time.Sleep(time.Millisecond)
	}

	count, _ := stack.Count(ctx, model.ItemFilter{})
	t.Fatalf("timed out waiting for live count=%d, got %d", want, count)
}

// TestIntegration_APIItemsTombstoneVisibility_SQLite proves the tombstone
// read path against the REAL SQLite read model: tombstoned items vanish from
// the default /items view, reappear under ?includeTombstoned=true carrying a
// typed TombstoneInfo on the wire, and live items never carry the field.
func TestIntegration_APIItemsTombstoneVisibility_SQLite(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "sqlite"})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	testutil.MustNoError(t, stack.SyncItem(ctx, makeTestItem(t, "tomb-live", "PushEvent", "2024-01-01T00:00:00Z")))
	testutil.MustNoError(t, stack.SyncItem(ctx, makeTestItem(t, "tomb-gone", "PushEvent", "2024-01-02T00:00:00Z")))

	waitForProjection(t, ctx, stack, 2)

	testutil.MustNoError(t, stack.TombstoneItem(ctx, "github", id.NewSourceID("tomb-gone"), model.ReasonUserHidden))

	waitForLiveCount(t, ctx, stack, 1)

	mockProvider := &testutil.MockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(mockProvider, stack, logger)
	server := NewServer(syncer, logger)

	type wireItem struct {
		SourceID  string `json:"sourceId"`
		Tombstone *struct {
			Reason string    `json:"reason"`
			At     time.Time `json:"tombstonedAt"`
		} `json:"tombstone"`
	}

	getItems := func(query string) []wireItem {
		t.Helper()

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/items"+query, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		testutil.AssertStatusOK(t, rec)

		var body struct {
			Items []wireItem `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal %s response: %v\n%s", query, err, rec.Body.String())
		}

		return body.Items
	}

	defaultView := getItems("")
	if len(defaultView) != 1 || defaultView[0].SourceID != "tomb-live" {
		t.Fatalf("default view must show only the live item, got %+v", defaultView)
	}
	if defaultView[0].Tombstone != nil {
		t.Errorf("live item must not carry a tombstone object, got %+v", defaultView[0].Tombstone)
	}

	if raw := recBodyFor(t, server, ctx, "/items"); strings.Contains(raw, `"tombstone"`) {
		t.Errorf("default view JSON must not mention tombstone at all:\n%s", raw)
	}

	including := getItems("?includeTombstoned=true")
	if len(including) != 2 {
		t.Fatalf("includeTombstoned view must show both items, got %d", len(including))
	}

	for _, item := range including {
		switch item.SourceID {
		case "tomb-live":
			if item.Tombstone != nil {
				t.Errorf("live item must have no tombstone even in the including view: %+v", item.Tombstone)
			}
		case "tomb-gone":
			if item.Tombstone == nil {
				t.Fatal("tombstoned item must carry a tombstone object")
			}
			if item.Tombstone.Reason != string(model.ReasonUserHidden) {
				t.Errorf("tombstone reason = %q, want %q", item.Tombstone.Reason, model.ReasonUserHidden)
			}
			if item.Tombstone.At.IsZero() {
				t.Error("tombstone timestamp must be set on the wire")
			}
		default:
			t.Errorf("unexpected item %q in including view", item.SourceID)
		}
	}
}

func recBodyFor(t *testing.T, server *Server, ctx context.Context, path string) string {
	t.Helper()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	testutil.AssertStatusOK(t, rec)

	return rec.Body.String()
}

// TestOpenAPI_ItemResponseTombstoneSchema pins the generated OpenAPI schema:
// ItemResponse must declare the optional tombstone object (reason string +
// tombstonedAt date-time) and the /items path must expose the
// includeTombstoned boolean query parameter. The 499/504 declarations are
// already pinned elsewhere; this closes the Tombstone half of the schema
// contract. Assertions run on the rendered YAML — the exact wire document
// consumers download — so no huma-internal types can drift underneath.
func TestOpenAPI_ItemResponseTombstoneSchema(t *testing.T) {
	t.Parallel()

	mockProvider := &testutil.MockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(mockProvider, &mockSyncStore{}, logger)
	server := NewServer(syncer, logger)

	specBytes, specErr := server.api.OpenAPI().YAML()
	testutil.MustNoError(t, specErr)
	spec := string(specBytes)

	for _, required := range []string{
		"ItemResponse:",
		"tombstone:",
		"TombstoneInfo:", // huma registers the DTO by type name
		"reason:",
		"tombstonedAt:",
		"name: includeTombstoned",
	} {
		if !strings.Contains(spec, required) {
			t.Errorf("OpenAPI spec missing %q (Tombstone schema contract):\n%s", required, spec)
		}
	}

	if strings.Count(spec, "includeTombstoned") == 0 {
		t.Errorf("/items must declare the includeTombstoned query parameter:\n%s", spec)
	}
}
