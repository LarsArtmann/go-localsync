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
	"github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-token")
	require.NotNil(t, client)
}

func TestNewClientWithHTTP(t *testing.T) {
	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	require.NotNil(t, client)
}

func TestFetchEvents_DefaultOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/users/testuser/events")
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		assert.Equal(t, "1", r.URL.Query().Get("page"))

		events := []*gh.Event{
			{
				ID:   gh.Ptr("123"),
				Type: gh.Ptr("PushEvent"),
				Actor: &gh.User{
					Login:     gh.Ptr("testuser"),
					AvatarURL: gh.Ptr("https://avatar.url"),
				},
				Repo: &gh.Repository{
					Name: gh.Ptr("test/repo"),
					URL:  gh.Ptr("https://api.github.com/repos/test/repo"),
				},
				CreatedAt: &gh.Timestamp{Time: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

	events, err := client.FetchEvents(context.Background(), "testuser", nil)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "123", events[0].GithubID)
	assert.Equal(t, "PushEvent", events[0].Type)
	assert.Equal(t, "testuser", events[0].ActorLogin)
	assert.Equal(t, "test/repo", events[0].RepoName)
}

func TestFetchEvents_CustomOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "50", r.URL.Query().Get("per_page"))
		assert.Equal(t, "2", r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

	events, err := client.FetchEvents(context.Background(), "testuser", &FetchOptions{PerPage: 50, Page: 2})
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestFetchEvents_ZeroPerPage_DefaultsTo100(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

	events, err := client.FetchEvents(context.Background(), "testuser", &FetchOptions{PerPage: 0, Page: 1})
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestFetchEvents_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gh.ErrorResponse{Message: "Not Found"})
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

	events, err := client.FetchEvents(context.Background(), "nonexistent", nil)
	require.Error(t, err)
	assert.Nil(t, events)
	assert.ErrorIs(t, err, errors.ErrUserNotFound)
}

func TestFetchAllEvents_MultiplePages(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page := r.URL.Query().Get("page")

		var events []*gh.Event
		if page == "1" {
			events = []*gh.Event{{ID: gh.Ptr("1"), Type: gh.Ptr("PushEvent")}}
		} else if page == "2" {
			events = []*gh.Event{{ID: gh.Ptr("2"), Type: gh.Ptr("PullRequestEvent")}}
		} else {
			events = []*gh.Event{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

	events, err := client.FetchAllEvents(context.Background(), "testuser", 3)
	require.NoError(t, err)
	assert.Len(t, events, 2)
	assert.Equal(t, 3, callCount) // Should call until empty page
}

func TestFetchAllEvents_DefaultMaxPages(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

	_, err := client.FetchAllEvents(context.Background(), "testuser", 0)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount) // Stops at first empty page
}

func TestFetchAllEvents_StopsOnEmptyPage(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		page := r.URL.Query().Get("page")

		if page == "1" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]*gh.Event{{ID: gh.Ptr("1"), Type: gh.Ptr("PushEvent")}})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*gh.Event{})
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

	events, err := client.FetchAllEvents(context.Background(), "testuser", 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.LessOrEqual(t, callCount, 2) // Should stop at empty page, not iterate all 10
}

func TestConvertEvent_FullEvent(t *testing.T) {
	ghEvent := &gh.Event{
		ID:   gh.Ptr("12345"),
		Type: gh.Ptr("PushEvent"),
		Actor: &gh.User{
			Login:     gh.Ptr("actor"),
			AvatarURL: gh.Ptr("https://avatar.url"),
		},
		Repo: &gh.Repository{
			Name: gh.Ptr("owner/repo"),
			URL:  gh.Ptr("https://api.github.com/repos/owner/repo"),
		},
		CreatedAt: &gh.Timestamp{Time: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)},
	}

	evt, err := convertEvent(ghEvent)
	require.NoError(t, err)
	assert.Equal(t, "12345", evt.GithubID)
	assert.Equal(t, "PushEvent", evt.Type)
	assert.Equal(t, "actor", evt.ActorLogin)
	assert.Equal(t, "https://avatar.url", evt.ActorAvatarURL)
	assert.Equal(t, "owner/repo", evt.RepoName)
	assert.Equal(t, "https://api.github.com/repos/owner/repo", evt.RepoURL)
	assert.Equal(t, time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC), evt.CreatedAt)
	assert.NotEmpty(t, evt.RawJSON)
}

func TestConvertEvent_MinimalEvent(t *testing.T) {
	ghEvent := &gh.Event{
		ID:        gh.Ptr("999"),
		Type:      gh.Ptr("WatchEvent"),
		CreatedAt: nil, // Will use time.Now()
	}

	evt, err := convertEvent(ghEvent)
	require.NoError(t, err)
	assert.Equal(t, "999", evt.GithubID)
	assert.Equal(t, "WatchEvent", evt.Type)
	assert.Empty(t, evt.ActorLogin)
	assert.Empty(t, evt.ActorAvatarURL)
	assert.Empty(t, evt.RepoName)
	assert.Empty(t, evt.RepoURL)
	assert.False(t, evt.CreatedAt.IsZero())
}

func TestConvertEvent_NilActorAndRepo(t *testing.T) {
	ghEvent := &gh.Event{
		ID:        gh.Ptr("1"),
		Type:      gh.Ptr("CreateEvent"),
		Actor:     nil,
		Repo:      nil,
		CreatedAt: &gh.Timestamp{Time: time.Now()},
	}

	evt, err := convertEvent(ghEvent)
	require.NoError(t, err)
	assert.Empty(t, evt.ActorLogin)
	assert.Empty(t, evt.RepoName)
}

func TestGetRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/rate_limit")
		w.Header().Set("Content-Type", "application/json")
		// GitHub API returns rate limits wrapped in "resources"
		json.NewEncoder(w).Encode(map[string]interface{}{
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

	limits, resp, err := client.GetRateLimit(context.Background())
	require.NoError(t, err)
	require.NotNil(t, limits)
	require.NotNil(t, resp)
	assert.Equal(t, 5000, limits.Core.Limit)
	assert.Equal(t, 4999, limits.Core.Remaining)
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	u.Path = u.Path + "/"
	return u
}
