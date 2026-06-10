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
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func assertClientName(t *testing.T, c *Client) {
	t.Helper()

	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.Name() != "github" {
		t.Errorf("expected name=github, got %s", c.Name())
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

func newRateLimitCoreServer(t *testing.T, core *gh.Rate) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resources": gh.RateLimits{Core: core},
		})
	}))
}

// exhaustedRate returns a GitHub rate limit struct that is fully exhausted
// and resets at the given time — used by wait-for-rate-limit tests.
func exhaustedRate(resetAt time.Time) *gh.Rate {
	return &gh.Rate{
		Limit:     5000,
		Remaining: 0,
		Reset:     gh.Timestamp{Time: resetAt},
	}
}

func newTestClient(server *httptest.Server) *Client {
	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{Enabled: false})

	return client
}

func TestNewClient(t *testing.T) {
	assertClientName(t, NewClient("test-token"))
}

func TestNewClientWithHTTP(t *testing.T) {
	assertClientName(t, NewClientWithHTTP(&http.Client{}))
}

func TestFetch_DefaultOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.AssertEqual(t, r.URL.Query().Get("per_page"), "100", "per_page")
		testutil.AssertEqual(t, r.URL.Query().Get("page"), "1", "page")

		events := []*gh.Event{
			newTestEvent(
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
	result, err := fetchFromTestClient(client, "testuser")
	testutil.MustNoError(t, err)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ExternalID.Get() != "123" {
		testutil.AssertEqual(t, result.Items[0].ExternalID.Get(), "123", "ExternalID")
	}
	if result.Items[0].ID.String() == "" {
		t.Error("expected non-empty ID")
	}
	testutil.AssertEqual(t, result.Items[0].Type.Get(), "PushEvent", "Type")
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
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, result.Items, 0, "items")
}

func TestFetch_ZeroPerPage_DefaultsTo100(t *testing.T) {
	server, _, expectedPage := createTestServerForFetch(t, 100, 1)
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(
		context.Background(),
		&provider.FetchOptions{Source: "testuser", PerPage: 0, Page: expectedPage},
	)
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, result.Items, 0, "items")
}

func createTestServerForFetch(
	t *testing.T,
	expectedPerPage, expectedPage int,
) (*httptest.Server, int, int) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.AssertEqual(t, r.URL.Query().Get("per_page"), strconv.Itoa(expectedPerPage), "per_page")
		testutil.AssertEqual(t, r.URL.Query().Get("page"), strconv.Itoa(expectedPage), "page")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*gh.Event{})
	}))

	return server, expectedPerPage, expectedPage
}

func TestFetch_APIError(t *testing.T) {
	server := newErrorTestServer(http.StatusNotFound, "Not Found")
	defer server.Close()

	client := newTestClient(server)
	result, err := fetchFromTestClient(client, "nonexistent")
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
					&gh.Event{ID: new(page + "-" + string(rune('0'+i))), Type: new("PushEvent")},
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
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, result.Items, 200, "items")
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
	testutil.MustNoError(t, err)
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
	testutil.MustNoError(t, err)
	testutil.AssertLen(t, result.Items, 1, "items")
	if callCount > 2 {
		t.Errorf("expected at most 2 calls, got %d", callCount)
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
				t.Errorf(
					"%s: expected %v, got %v",
					tt.assertField,
					tt.config,
					tt.getConfig(newClient),
				)
			}
			if client.client != newClient.client {
				t.Errorf("%s: expected same underlying client", tt.assertField)
			}
		})
	}
}
