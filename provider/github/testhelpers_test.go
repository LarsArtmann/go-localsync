package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

func newTestEvent(id, eventType string, createdAt time.Time) *gh.Event {
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

func newErrorTestServer(statusCode int, message string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(gh.ErrorResponse{Message: message})
	}))
}

// writeEmptyEventsResponse writes an empty GitHub events list with the JSON
// content type. Shared by every test server that needs to mock a "no events"
// response from the GitHub API.
func writeEmptyEventsResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode([]*gh.Event{})
}

func newFailingThenSucceedingTestServer(attempts int) (*httptest.Server, *int) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < attempts {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		writeEmptyEventsResponse(w)
	}))

	return server, &callCount
}

func testRetryConfig() provider.RetryConfig {
	return provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
	}
}

func newRetryTestClient(server *httptest.Server) *Client {
	return newTestClient(server).WithRetryConfig(testRetryConfig())
}

func fetchFromTestClient(
	client *Client,
	source string,
) (*provider.FetchResult, error) {
	return client.Fetch(context.Background(), &provider.FetchOptions{Source: id.NewProviderID(source)})
}

func newGitHubErrorResponse(statusCode int) *gh.ErrorResponse {
	return &gh.ErrorResponse{Response: &http.Response{StatusCode: statusCode}}
}

const defaultGitHubRateLimit = 5000

func rateLimitResponse(remaining int) map[string]any {
	return map[string]any{
		"resources": gh.RateLimits{
			Core: &gh.Rate{
				Limit:     defaultGitHubRateLimit,
				Remaining: remaining,
				Reset:     gh.Timestamp{Time: time.Now().Add(1 * time.Hour)},
			},
		},
	}
}
