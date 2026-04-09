package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-token")
	require.NotNil(t, client)
	assert.Equal(t, "github", client.Name())
}

func TestNewClientWithHTTP(t *testing.T) {
	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	require.NotNil(t, client)
	assert.Equal(t, "github", client.Name())
}

// newTestClient creates a client with rate limiting disabled for unit tests.
func newTestClient(server *httptest.Server) *Client {
	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = testhelpers.MustParseURL(server.URL)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{Enabled: false})

	return client
}

// newErrorTestServer creates an httptest.Server that returns a JSON error response.
var newErrorTestServer = testhelpers.NewErrorTestServer

// testRetryConfig returns a retry config suitable for unit tests with fast backoff.
var testRetryConfig = testhelpers.TestRetryConfig

// newFailingThenSucceedingTestServer creates a test server that fails with
// http.StatusInternalServerError for the first (attempts-1) requests and succeeds
// on the final attempt by returning an empty event list.
var newFailingThenSucceedingTestServer = testhelpers.NewFailingThenSucceedingTestServer

func TestFetch_DefaultOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/users/testuser/events")
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

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
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "123", result.Items[0].ID.Get())
	assert.Equal(t, "PushEvent", result.Items[0].Type.Get())
	assert.Equal(t, "octocat", result.Items[0].ActorLogin.Get())
	assert.Equal(t, "octocat/Hello-World", result.Items[0].RepoName.Get())
}

func TestFetch_CustomOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "50", r.URL.Query().Get("per_page"))
		assert.Equal(t, "2", r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(
		context.Background(),
		&provider.FetchOptions{Source: "testuser", PerPage: 50, Page: 2},
	)
	require.NoError(t, err)
	assert.Empty(t, result.Items)
}

func TestFetch_ZeroPerPage_DefaultsTo100(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(
		context.Background(),
		&provider.FetchOptions{Source: "testuser", PerPage: 0, Page: 1},
	)
	require.NoError(t, err)
	assert.Empty(t, result.Items)
}

func TestFetch_APIError(t *testing.T) {
	server := newErrorTestServer(http.StatusNotFound, "Not Found")
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "nonexistent"})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, pkgerrors.ErrUserNotFound)
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
			// Return perPage items so HasMore=true, simulating full pages
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
	require.NoError(t, err)
	assert.Len(t, result.Items, 200) // 100 from page 1 + 100 from page 2
	assert.Equal(t, 3, callCount)    // Page 1, 2 (full), 3 (empty, stops)
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
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
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
	require.NoError(t, err)
	assert.Len(t, result.Items, 1)
	assert.LessOrEqual(t, callCount, 2)
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
	require.NoError(t, err)
	assert.Equal(t, "12345", item.ID.Get())
	assert.Equal(t, "github", item.Source.Get())
	assert.Equal(t, "PushEvent", item.Type.Get())
	assert.Equal(t, "actor", item.ActorLogin.Get())
	assert.Equal(t, "https://avatar.url", item.ActorAvatarURL)
	assert.Equal(t, "owner/repo", item.RepoName.Get())
	assert.Equal(t, "https://api.github.com/repos/owner/repo", item.RepoURL)
	assert.Equal(t, time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC), item.CreatedAt)
	assert.NotEmpty(t, item.RawJSON)
}

func TestConvertEvent_MinimalEvent(t *testing.T) {
	ghEvent := &gh.Event{
		ID:        new("999"),
		Type:      new("WatchEvent"),
		CreatedAt: nil,
	}

	item, err := convertEvent(ghEvent)
	require.NoError(t, err)
	assert.Equal(t, "999", item.ID.Get())
	assert.Equal(t, "WatchEvent", item.Type.Get())
	assert.Empty(t, item.ActorLogin.Get())
	assert.Empty(t, item.ActorAvatarURL)
	assert.Empty(t, item.RepoName)
	assert.Empty(t, item.RepoURL)
	assert.False(t, item.CreatedAt.IsZero())
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
	require.NoError(t, err)
	assert.Empty(t, item.ActorLogin)
	assert.Empty(t, item.RepoName)
}

func TestGetRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rate_limit")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resources": gh.RateLimits{
				Core: &gh.Rate{
					Limit:     5000,
					Remaining: 4999,
					Reset:     gh.Timestamp{Time: time.Now().Add(1 * time.Hour)},
				},
			},
		})
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = testhelpers.MustParseURL(server.URL)

	limits, err := client.GetRateLimit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, limits)
	assert.Equal(t, 5000, limits.Limit)
	assert.Equal(t, 4999, limits.Remaining)
}

func TestFetch_RetryOnServerError(t *testing.T) {
	server, callCount := newFailingThenSucceedingTestServer(3)
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRetryConfig(testRetryConfig())

	_, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
	require.NoError(t, err)
	assert.Equal(t, 3, *callCount)
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
	require.Error(t, err)
	assert.Equal(t, 1, callCount)
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
			assert.Equal(t, tt.config, tt.getConfig(newClient))
			assert.Equal(t, client.client, newClient.client)
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
	require.NoError(t, err)
	require.NotNil(t, limits)
	assert.Equal(t, 0, limits.Limit)
	assert.Equal(t, 0, limits.Remaining)
	assert.True(t, limits.ResetAt.IsZero())
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
			assert.Equal(t, tt.want, got)
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
			assert.ErrorIs(t, wrapped, tt.wantErr)
		})
	}
}
