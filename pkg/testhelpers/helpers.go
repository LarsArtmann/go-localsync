package testhelpers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// Ptr returns a pointer to the given string.
//
//go:fix inline
func Ptr(s string) *string {
	return new(s)
}

// mustParseURL parses a URL and adds trailing slash (required by go-github).
func MustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}

	u.Path = u.Path + "/"

	return u
}

// NewTestEvent creates a test GitHub event with the specified parameters.
func NewTestEvent(id, eventType string, createdAt time.Time) *gh.Event {
	return &gh.Event{
		ID:   new(id),
		Type: new(eventType),
		Actor: &gh.User{
			Login:     new("octocat"),
			AvatarURL: new("https://avatars.githubusercontent.com/u/583231"),
		},
		Repo: &gh.Repository{
			Name: new("octocat/Hello-World"),
			URL:  new("https://api.github.com/repos/octocat/Hello-World"),
		},
		CreatedAt: &gh.Timestamp{Time: createdAt},
	}
}

// NewErrorTestServer creates an httptest.Server that returns a JSON error response.
func NewErrorTestServer(statusCode int, message string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gh.ErrorResponse{Message: message})
	}))
}

// NewFailingThenSucceedingTestServer creates a test server that fails with
// http.StatusInternalServerError for the first (attempts-1) requests and succeeds
// on the final attempt by returning an empty event list.
func NewFailingThenSucceedingTestServer(attempts int) (*httptest.Server, *int) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < attempts {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]*gh.Event{})
	}))

	return server, &callCount
}

// TestRetryConfig returns a retry config suitable for unit tests with fast backoff.
func TestRetryConfig() provider.RetryConfig {
	return provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}
}

// RateLimitResponse returns a JSON payload for GitHub rate limit responses.
func RateLimitResponse(remaining int) map[string]any {
	return map[string]any{
		"resources": gh.RateLimits{
			Core: &gh.Rate{
				Limit:     5000,
				Remaining: remaining,
				Reset:     gh.Timestamp{Time: time.Now().Add(1 * time.Hour)},
			},
		},
	}
}
