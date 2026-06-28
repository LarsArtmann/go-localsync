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

type syncInputBody struct {
	Source   string `json:"source"`
	MaxPages int    `json:"maxPages"`
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
			SourceID: item.ExternalID,
			Action:   synclib.ActionCreated,
		})
	}

	return summary
}

func (m *mockSyncStore) Count(_ context.Context, _ model.ItemFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}

	return int64(len(m.Items)), nil
}

func (m *mockSyncStore) CountByType(_ context.Context, _ model.ItemFilter) (map[string]int64, error) {
	if m.countErr != nil {
		return nil, m.countErr
	}

	counts := make(map[string]int64)

	for _, item := range m.Items {
		counts[item.Type.Get()]++
	}

	return counts, nil
}

func (m *mockSyncStore) GetTypes(_ context.Context) ([]string, error) {
	if m.typesErr != nil {
		return nil, m.typesErr
	}

	return m.types, nil
}

func (m *mockSyncStore) Close() error { return nil }

func (m *mockSyncStore) Reconcile(_ context.Context, _ string, _ []model.Key) (int, error) {
	return 0, nil
}

func testItem(itemID, eventType string) *model.Item {
	now := time.Now()

	return &model.Item{
		ID:            id.NewItemID(),
		ExternalID:    id.NewExternalID(itemID),
		Source:        id.NewProviderID("github"),
		Type:          id.NewEventTypeID(eventType),
		ActorLogin:    id.NewActorLogin("testuser"),
		RepoName:      id.NewRepoID("test/repo"),
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: schema.CurrentVersion(),
	}
}

func newTestServer(store synclib.SyncStore) *Server {
	provider := &testutil.MockProvider{}
	logger := log.Default()
	syncer := synclib.NewSyncer(provider, store, logger)

	return NewServer(syncer, logger)
}

func newJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	return req
}

// newGETRequest creates a GET request with the standard test context.
// Reduces the repeated "httptest.NewRequestWithContext(context.Background(),
// http.MethodGet, path, nil)" pattern to a single call.
func newGETRequest(_ *testing.T, path string) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
}

func newMockStoreWithItems() *mockSyncStore {
	return &mockSyncStore{
		SyncStoreListBehavior: testutil.SyncStoreListBehavior{
			Items: []*model.Item{
				testItem("1", "PushEvent"),
				testItem("2", "IssueEvent"),
			},
		},
	}
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	server := newTestServer(&mockSyncStore{})

	req := newGETRequest(t, "/health")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)

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

	req := newGETRequest(t, "/stats")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)

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

	testutil.AssertLen(t, body.ItemTypes, 2, "item types")
}

func TestGetStats_CountError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		errMsg string
	}{
		{name: "store error", errMsg: "count failed"},
		{name: "count query error", errMsg: "count query failed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &mockSyncStore{countErr: errors.New(tc.errMsg)}
			server := newTestServer(store)

			req := newGETRequest(t, "/stats")
			rec := httptest.NewRecorder()

			server.ServeHTTP(rec, req)

			testutil.AssertStatus(t, rec, http.StatusInternalServerError)
		})
	}
}

func TestListItems(t *testing.T) {
	t.Parallel()

	store := newMockStoreWithItems()
	server := newTestServer(store)

	req := newGETRequest(t, "/items?limit=10")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)

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

	store := newMockStoreWithItems()
	server := newTestServer(store)

	req := newGETRequest(t, "/items?type=PushEvent")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)
}

func TestListItems_StoreError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{SyncStoreListBehavior: testutil.SyncStoreListBehavior{ListErr: errors.New("list failed")}}
	server := newTestServer(store)

	req := newGETRequest(t, "/items")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusInternalServerError)
}

func TestTriggerSync(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	server := newTestServer(store)

	payload := syncInputBody{
		Source:   "testuser",
		MaxPages: 1,
	}

	req := newJSONRequest(t, http.MethodPost, "/sync", payload)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)

	var body struct {
		Fetched int `json:"fetched"`
		Skipped int `json:"skipped"`
		Errors  int `json:"errors"`
	}

	err := json.Unmarshal(rec.Body.Bytes(), &body)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Fetched != 0 {
		t.Errorf("expected fetched=0, got %d", body.Fetched)
	}
}

func TestListItems_CountError(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{
		SyncStoreListBehavior: testutil.SyncStoreListBehavior{
			Items: []*model.Item{testItem("1", "PushEvent")},
		},
		countErr: errors.New("count failed"),
	}
	server := newTestServer(store)

	req := newGETRequest(t, "/items")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusInternalServerError)
}

func TestListItems_AllFilterParams(t *testing.T) {
	t.Parallel()

	store := newMockStoreWithItems()
	server := newTestServer(store)

	req := newGETRequest(t, "/items?type=PushEvent&actor=testuser&repo=test/repo&source=github&limit=1&offset=0")
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatusOK(t, rec)
}

func TestTriggerSync_InvalidOptions(t *testing.T) {
	t.Parallel()

	store := &mockSyncStore{}
	server := newTestServer(store)

	payload := syncInputBody{
		Source:   "",
		MaxPages: 1,
	}

	req := newJSONRequest(t, http.MethodPost, "/sync", payload)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	testutil.AssertStatus(t, rec, http.StatusBadRequest)
}
