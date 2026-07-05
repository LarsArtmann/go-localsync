package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
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
		ExternalID: id.NewExternalID(sourceID),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID(eventType),
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
