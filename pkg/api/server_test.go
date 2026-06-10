package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

type mockProvider struct{}

type syncInputBody struct {
	Source   string `json:"source"`
	MaxPages int    `json:"maxPages"`
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Fetch(_ context.Context, _ *provider.FetchOptions) (*provider.FetchResult, error) {
	return &provider.FetchResult{Items: nil, HasMore: false}, nil
}

func (m *mockProvider) FetchAll(_ context.Context, _ string, _ int) (*provider.FetchResult, error) {
	return &provider.FetchResult{Items: nil, HasMore: false}, nil
}

func (m *mockProvider) GetRateLimit(_ context.Context) (*provider.RateLimitInfo, error) {
	return nil, errors.New("not implemented")
}

type mockSyncStore struct {
	testutil.SyncStoreListBehavior

	countErr error
	typesErr error
	types    []string
}

func (m *mockSyncStore) SyncItems(_ context.Context, items []*provider.Item) *synclib.SyncSummary {
	summary := &synclib.SyncSummary{Results: make([]synclib.ItemSyncResult, 0, len(items))}

	for _, item := range items {
		summary.Synced++
		summary.Results = append(summary.Results, synclib.ItemSyncResult{
			SourceID: item.ExternalID.Get(),
			Action:   synclib.ActionCreated,
		})
	}

	return summary
}

func (m *mockSyncStore) CountItems(_ context.Context, _ provider.ItemFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}

	return int64(len(m.Items)), nil
}

func (m *mockSyncStore) GetItemTypes(_ context.Context) ([]string, error) {
	if m.typesErr != nil {
		return nil, m.typesErr
	}

	return m.types, nil
}

func (m *mockSyncStore) Close() error { return nil }

func testItem(itemID, eventType string) *model.Item {
	now := time.Now()

	return &model.Item{
		ID:            id.NewItemID(),
		ExternalID:    id.NewExternalID(itemID),
		Source:        id.NewProviderID("github"),
		Type:          id.NewEventTypeID(eventType),
		ActorLogin:    id.NewActorID("testuser"),
		RepoName:      id.NewRepoID("test/repo"),
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: schema.CurrentVersion(),
	}
}

func newTestServer(store synclib.SyncStore) *Server {
	provider := &mockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(provider, store, logger)

	return NewServer(syncer, logger)
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	server := newTestServer(&mockSyncStore{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Status string `json:"status"`
	}

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Status != "healthy" {
		t.Errorf("expected status healthy, got %s", body.Status)
	}
}

func TestGetStats(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{
		SyncStoreListBehavior: testutil.SyncStoreListBehavior{
			Items: []*model.Item{
				testItem("1", "PushEvent"),
				testItem("2", "IssueEvent"),
			},
		},
		types: []string{"PushEvent", "IssueEvent"},
	}

	server := newTestServer(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		TotalItems int64    `json:"totalItems"`
		ItemTypes  []string `json:"itemTypes"`
	}

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.TotalItems != 2 {
		t.Errorf("expected totalItems=2, got %d", body.TotalItems)
	}

	if len(body.ItemTypes) != 2 {
		t.Errorf("expected 2 item types, got %d", len(body.ItemTypes))
	}
}

func TestGetStats_StoreError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{countErr: errors.New("count failed")}
	server := newTestServer(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestListItems(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{
		SyncStoreListBehavior: testutil.SyncStoreListBehavior{
			Items: []*model.Item{
				testItem("1", "PushEvent"),
				testItem("2", "IssueEvent"),
			},
		},
	}

	server := newTestServer(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?limit=10", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Items []*ItemResponse `json:"items"`
		Total int64           `json:"total"`
	}

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Total != 2 {
		t.Errorf("expected total=2, got %d", body.Total)
	}

	if len(body.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(body.Items))
	}
}

func TestListItems_WithFilter(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{
		SyncStoreListBehavior: testutil.SyncStoreListBehavior{
			Items: []*model.Item{
				testItem("1", "PushEvent"),
				testItem("2", "IssueEvent"),
			},
		},
	}

	server := newTestServer(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?type=PushEvent", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestListItems_StoreError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{SyncStoreListBehavior: testutil.SyncStoreListBehavior{ListErr: errors.New("list failed")}}
	server := newTestServer(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestTriggerSync(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	server := newTestServer(store)

	payload := syncInputBody{
		Source:   "testuser",
		MaxPages: 1,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/sync", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Fetched int `json:"fetched"`
		Skipped int `json:"skipped"`
		Errors  int `json:"errors"`
	}

	err = json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Fetched != 0 {
		t.Errorf("expected fetched=0, got %d", body.Fetched)
	}
}

func TestTriggerSync_InvalidOptions(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	server := newTestServer(store)

	payload := syncInputBody{
		Source:   "",
		MaxPages: 1,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/sync", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
