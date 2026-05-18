package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-token")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Name() != "github" {
		t.Errorf("expected name=github, got %s", client.Name())
	}
}

func TestNewClientWithHTTP(t *testing.T) {
	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Name() != "github" {
		t.Errorf("expected name=github, got %s", client.Name())
	}
}

func newTestClient(server *httptest.Server) *Client {
	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = testhelpers.MustParseURL(server.URL)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{Enabled: false})

	return client
}

var newErrorTestServer = testhelpers.NewErrorTestServer
var testRetryConfig = testhelpers.TestRetryConfig
var newFailingThenSucceedingTestServer = testhelpers.NewFailingThenSucceedingTestServer

func TestFetch_DefaultOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !containsSubstring(r.URL.Path, "/users/testuser/events") {
			t.Errorf("expected path to contain /users/testuser/events, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("per_page") != "100" {
			t.Errorf("expected per_page=100, got %s", r.URL.Query().Get("per_page"))
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("expected page=1, got %s", r.URL.Query().Get("page"))
		}

		events := []*gh.Event{
			testhelpers.NewTestEvent(
				"123",
				"PushEvent",
				time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(events)
	}))
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ExternalID.Get() != "123" {
		t.Errorf("expected ExternalID=123, got %s", result.Items[0].ExternalID.Get())
	}
	if result.Items[0].ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	if result.Items[0].Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", result.Items[0].Type.Get())
	}
	if result.Items[0].ActorLogin.Get() != "octocat" {
		t.Errorf("expected ActorLogin=octocat, got %s", result.Items[0].ActorLogin.Get())
	}
	if result.Items[0].RepoName.Get() != "octocat/Hello-World" {
		t.Errorf("expected RepoName=octocat/Hello-World, got %s", result.Items[0].RepoName.Get())
	}
}

func TestFetch_CustomOptions(t *testing.T) {
	server, expectedPerPage, expectedPage := createTestServerForFetch(t, 50, 2)
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(
		context.Background(),
		&provider.FetchOptions{Source: "testuser", PerPage: expectedPerPage, Page: expectedPage},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func TestFetch_ZeroPerPage_DefaultsTo100(t *testing.T) {
	server, _, expectedPage := createTestServerForFetch(t, 100, 1)
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(
		context.Background(),
		&provider.FetchOptions{Source: "testuser", PerPage: 0, Page: expectedPage},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func createTestServerForFetch(
	t *testing.T,
	expectedPerPage, expectedPage int,
) (*httptest.Server, int, int) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") != strconv.Itoa(expectedPerPage) {
			t.Errorf("expected per_page=%d, got %s", expectedPerPage, r.URL.Query().Get("per_page"))
		}
		if r.URL.Query().Get("page") != strconv.Itoa(expectedPage) {
			t.Errorf("expected page=%d, got %s", expectedPage, r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*gh.Event{})
	}))

	return server, expectedPerPage, expectedPage
}

func TestFetch_APIError(t *testing.T) {
	server := newErrorTestServer(http.StatusNotFound, "Not Found")
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "nonexistent"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if !errors.Is(err, pkgerrors.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestFetchAll_MultiplePages(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page := r.URL.Query().Get("page")
		perPage := r.URL.Query().Get("per_page")

		var events []*gh.Event

		switch page {
		case "1", "2":
			for i := range 100 {
				events = append(
					events,
					&gh.Event{ID: new(page+"-"+string(rune('0'+i))), Type: new("PushEvent")},
				)
			}
		default:
			events = []*gh.Event{}
		}

		if perPage != "100" {
			t.Errorf("expected per_page=100, got %s", perPage)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(events)
	}))
	defer server.Close()

	client := newTestClient(server)
	result, err := client.FetchAll(context.Background(), "testuser", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 200 {
		t.Errorf("expected 200 items, got %d", len(result.Items))
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestFetchAll_DefaultMaxPages(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.FetchAll(context.Background(), "testuser", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestFetchAll_StopsOnEmptyPage(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page := r.URL.Query().Get("page")

		if page == "1" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]*gh.Event{{ID: new("1"), Type: new("PushEvent")}})

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	client := newTestClient(server)
	result, err := client.FetchAll(context.Background(), "testuser", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
	if callCount > 2 {
		t.Errorf("expected at most 2 calls, got %d", callCount)
	}
}

func TestConvertEvent_FullEvent(t *testing.T) {
	ghEvent := &gh.Event{
		ID:   new("12345"),
		Type: new("PushEvent"),
		Actor: &gh.User{
			Login:     new("actor"),
			AvatarURL: new("https://avatar.url"),
		},
		Repo: &gh.Repository{
			Name: new("owner/repo"),
			URL:  new("https://api.github.com/repos/owner/repo"),
		},
		CreatedAt: &gh.Timestamp{Time: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)},
	}

	item, err := convertEvent(ghEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ExternalID.Get() != "12345" {
		t.Errorf("expected ExternalID=12345, got %s", item.ExternalID.Get())
	}
	if item.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	if item.Source.Get() != "github" {
		t.Errorf("expected Source=github, got %s", item.Source.Get())
	}
	if item.Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", item.Type.Get())
	}
	if item.ActorLogin.Get() != "actor" {
		t.Errorf("expected ActorLogin=actor, got %s", item.ActorLogin.Get())
	}
	if item.ActorAvatarURL != "https://avatar.url" {
		t.Errorf("expected ActorAvatarURL=https://avatar.url, got %s", item.ActorAvatarURL)
	}
	if item.RepoName.Get() != "owner/repo" {
		t.Errorf("expected RepoName=owner/repo, got %s", item.RepoName.Get())
	}
	if item.RepoURL != "https://api.github.com/repos/owner/repo" {
		t.Errorf("expected RepoURL=https://api.github.com/repos/owner/repo, got %s", item.RepoURL)
	}
	if !item.CreatedAt.Equal(time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)) {
		t.Errorf("expected CreatedAt=2024-06-15 10:30:00, got %v", item.CreatedAt)
	}
	if len(item.RawJSON) == 0 {
		t.Error("expected non-empty RawJSON")
	}
}

func TestConvertEvent_MinimalEvent(t *testing.T) {
	ghEvent := &gh.Event{
		ID:        new("999"),
		Type:      new("WatchEvent"),
		CreatedAt: nil,
	}

	item, err := convertEvent(ghEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ExternalID.Get() != "999" {
		t.Errorf("expected ExternalID=999, got %s", item.ExternalID.Get())
	}
	if item.ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	if item.Type.Get() != "WatchEvent" {
		t.Errorf("expected Type=WatchEvent, got %s", item.Type.Get())
	}
	if item.ActorLogin.Get() != "" {
		t.Errorf("expected empty ActorLogin, got %s", item.ActorLogin.Get())
	}
	if item.ActorAvatarURL != "" {
		t.Errorf("expected empty ActorAvatarURL, got %s", item.ActorAvatarURL)
	}
	if item.RepoName.String() != "" {
		t.Errorf("expected empty RepoName, got %s", item.RepoName)
	}
	if item.RepoURL != "" {
		t.Errorf("expected empty RepoURL, got %s", item.RepoURL)
	}
	if item.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestConvertEvent_NilActorAndRepo(t *testing.T) {
	ghEvent := &gh.Event{
		ID:        new("1"),
		Type:      new("CreateEvent"),
		Actor:     nil,
		Repo:      nil,
		CreatedAt: &gh.Timestamp{Time: time.Now()},
	}

	item, err := convertEvent(ghEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ActorLogin.String() != "" {
		t.Errorf("expected empty ActorLogin, got %s", item.ActorLogin)
	}
	if item.RepoName.String() != "" {
		t.Errorf("expected empty RepoName, got %s", item.RepoName)
	}
}

func TestGetRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !containsSubstring(r.URL.Path, "/rate_limit") {
			t.Errorf("expected path to contain /rate_limit, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testhelpers.RateLimitResponse(4999))
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = testhelpers.MustParseURL(server.URL)

	limits, err := client.GetRateLimit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limits == nil {
		t.Fatal("expected non-nil limits")
	}
	if limits.Limit != 5000 {
		t.Errorf("expected Limit=5000, got %d", limits.Limit)
	}
	if limits.Remaining != 4999 {
		t.Errorf("expected Remaining=4999, got %d", limits.Remaining)
	}
}

func TestFetch_RetryOnServerError(t *testing.T) {
	server, callCount := newFailingThenSucceedingTestServer(3)
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRetryConfig(testRetryConfig())

	_, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *callCount != 3 {
		t.Errorf("expected 3 calls, got %d", *callCount)
	}
}

func TestFetch_NoRetryOnClientError(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRetryConfig(testRetryConfig())

	_, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      any
		getConfig   func(*Client) any
		withConfig  func(*Client, any) *Client
		assertField string
	}{
		{
			name:        "RateLimitConfig",
			config:      provider.RateLimitConfig{Enabled: true, MinRemaining: 100},
			getConfig:   func(c *Client) any { return c.rateLimitConfig },
			withConfig:  func(c *Client, cfg any) *Client { return c.WithRateLimitConfig(cfg.(provider.RateLimitConfig)) },
			assertField: "rateLimitConfig",
		},
		{
			name:        "RetryConfig",
			config:      provider.RetryConfig{Enabled: true, MaxRetries: 5},
			getConfig:   func(c *Client) any { return c.retryConfig },
			withConfig:  func(c *Client, cfg any) *Client { return c.WithRetryConfig(cfg.(provider.RetryConfig)) },
			assertField: "retryConfig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient("test-token")
			newClient := tt.withConfig(client, tt.config)
			if tt.getConfig(newClient) != tt.config {
				t.Errorf("%s: expected %v, got %v", tt.assertField, tt.config, tt.getConfig(newClient))
			}
			if client.client != newClient.client {
				t.Errorf("%s: expected same underlying client", tt.assertField)
			}
		})
	}
}

func TestGetRateLimit_NilCore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resources": gh.RateLimits{
				Core: nil,
			},
		})
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = testhelpers.MustParseURL(server.URL)

	limits, err := client.GetRateLimit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limits == nil {
		t.Fatal("expected non-nil limits")
	}
	if limits.Limit != 0 {
		t.Errorf("expected Limit=0, got %d", limits.Limit)
	}
	if limits.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %d", limits.Remaining)
	}
	if !limits.ResetAt.IsZero() {
		t.Errorf("expected zero ResetAt, got %v", limits.ResetAt)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"5xx server error", http.StatusInternalServerError, true},
		{"429 rate limited", http.StatusTooManyRequests, true},
		{"400 bad request", http.StatusBadRequest, false},
		{"401 unauthorized", http.StatusUnauthorized, false},
		{"403 forbidden", http.StatusForbidden, false},
		{"404 not found", http.StatusNotFound, false},
		{"200 OK", http.StatusOK, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &gh.ErrorResponse{Response: &http.Response{StatusCode: tt.statusCode}}
			got := isRetryableError(err)
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestWrapGitHubError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{"unauthorized", http.StatusUnauthorized, pkgerrors.ErrInvalidToken},
		{"forbidden", http.StatusForbidden, pkgerrors.ErrRateLimited},
		{"not found", http.StatusNotFound, pkgerrors.ErrUserNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghErr := &gh.ErrorResponse{
				Response: &http.Response{StatusCode: tt.statusCode},
			}
			wrapped := wrapGitHubError(ghErr, "testuser")
			if !errors.Is(wrapped, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, wrapped)
			}
		})
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstringHelper(s, sub))
}

func containsSubstringHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}
