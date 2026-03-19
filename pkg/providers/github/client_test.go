package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
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
	client.client.BaseURL = mustParseURL(server.URL)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{Enabled: false})

	return client
}

func TestFetch_DefaultOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/users/testuser/events")
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		events := []*gh.Event{
			{
				ID:   new("123"),
				Type: new("PushEvent"),
				Actor: &gh.User{
					Login:     new("testuser"),
					AvatarURL: new("https://avatar.url"),
				},
				Repo: &gh.Repository{
					Name: new("test/repo"),
					URL:  new("https://api.github.com/repos/test/repo"),
				},
				CreatedAt: &gh.Timestamp{Time: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	}))
	defer server.Close()

	client := newTestClient(server)
	result, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "123", result.Items[0].ID.Get())
	assert.Equal(t, "PushEvent", result.Items[0].Type.Get())
	assert.Equal(t, "testuser", result.Items[0].ActorLogin.Get())
	assert.Equal(t, "test/repo", result.Items[0].RepoName.Get())
}

func TestFetch_CustomOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "50", r.URL.Query().Get("per_page"))
		assert.Equal(t, "2", r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.Event{})
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
		json.NewEncoder(w).Encode([]*gh.Event{})
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gh.ErrorResponse{Message: "Not Found"})
	}))
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
		json.NewEncoder(w).Encode(events)
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
		json.NewEncoder(w).Encode([]*gh.Event{})
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
			json.NewEncoder(w).Encode([]*gh.Event{{ID: new("1"), Type: new("PushEvent")}})

			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.Event{})
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
		json.NewEncoder(w).Encode(map[string]any{
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
	client.client.BaseURL = mustParseURL(server.URL)

	limits, err := client.GetRateLimit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, limits)
	assert.Equal(t, 5000, limits.Limit)
	assert.Equal(t, 4999, limits.Remaining)
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	u.Path = u.Path + "/"

	return u
}

func TestFetch_RetryOnServerError(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRetryConfig(provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	})

	_, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
	require.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestFetch_NoRetryOnClientError(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRetryConfig(provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	})

	_, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
	require.Error(t, err)
	assert.Equal(t, 1, callCount)
}

func TestWithRateLimitConfig(t *testing.T) {
	client := NewClient("test-token")
	cfg := provider.RateLimitConfig{Enabled: true, MinRemaining: 100}
	newClient := client.WithRateLimitConfig(cfg)
	assert.Equal(t, cfg, newClient.rateLimitConfig)
	assert.Equal(t, client.client, newClient.client)
}

func TestWithRetryConfig(t *testing.T) {
	client := NewClient("test-token")
	cfg := provider.RetryConfig{Enabled: true, MaxRetries: 5}
	newClient := client.WithRetryConfig(cfg)
	assert.Equal(t, cfg, newClient.retryConfig)
	assert.Equal(t, client.client, newClient.client)
}
