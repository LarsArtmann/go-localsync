package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/providers/github"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
)

type githubTestWorld struct {
	ctx       context.Context
	server    *httptest.Server
	client    *github.Client
	result    *provider.FetchResult
	rateLimit *provider.RateLimitInfo
	err       error
	callCount int
}

func (w *githubTestWorld) fetchFor(source string) {
	w.result, w.err = w.client.Fetch(w.ctx, &provider.FetchOptions{Source: source})
}

func (w *githubTestWorld) withRetryConfig() {
	w.client = w.client.WithRetryConfig(testhelpers.TestRetryConfig())
}

func newGitHubTestClient(server *httptest.Server) *github.Client {
	httpClient := &http.Client{}
	client := github.NewClientWithHTTP(httpClient)
	c, err := client.WithBaseURL(server.URL + "/")
	if err != nil {
		panic(fmt.Sprintf("WithBaseURL failed: %v", err))
	}

	return c
}

func newGitHubTestClientWithoutRateLimit(server *httptest.Server) *github.Client {
	return newGitHubTestClient(server).WithRateLimitConfig(provider.RateLimitConfig{Enabled: false})
}

func TestBDD_FetchValidUser(t *testing.T) {
	world := githubTestWorld{ctx: context.Background()}

	world.server = httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			world.callCount++
			if !strings.Contains(r.URL.Path, "/users/octocat/events") {
				t.Errorf("expected path to contain /users/octocat/events, got %s", r.URL.Path)
			}

			events := []*gh.Event{
				testhelpers.NewTestEvent(
					"event-123",
					"PushEvent",
					time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				),
				testhelpers.NewTestEvent(
					"event-456",
					"IssuesEvent",
					time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
				),
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(events)
		}),
	)
	defer world.server.Close()

	world.client = newGitHubTestClientWithoutRateLimit(world.server)
	world.fetchFor("octocat")

	if world.err != nil {
		t.Fatalf("expected no error, got %v", world.err)
	}
	if len(world.result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(world.result.Items))
	}
	if world.result.Items[0].ExternalID.Get() != "event-123" {
		t.Errorf("expected ExternalID=event-123, got %s", world.result.Items[0].ExternalID.Get())
	}
	if world.result.Items[1].ExternalID.Get() != "event-456" {
		t.Errorf("expected ExternalID=event-456, got %s", world.result.Items[1].ExternalID.Get())
	}
	if world.result.Items[0].Type.Get() != "PushEvent" {
		t.Errorf("expected Type=PushEvent, got %s", world.result.Items[0].Type.Get())
	}
	if world.result.Items[1].Type.Get() != "IssuesEvent" {
		t.Errorf("expected Type=IssuesEvent, got %s", world.result.Items[1].Type.Get())
	}
	for _, item := range world.result.Items {
		if item.ActorLogin.Get() != "octocat" {
			t.Errorf("expected ActorLogin=octocat, got %s", item.ActorLogin.Get())
		}
		if !strings.Contains(item.ActorAvatarURL, "avatars.githubusercontent.com") {
			t.Errorf(
				"expected avatar URL to contain avatars.githubusercontent.com, got %s",
				item.ActorAvatarURL,
			)
		}
		if item.RepoName.Get() != "octocat/Hello-World" {
			t.Errorf("expected RepoName=octocat/Hello-World, got %s", item.RepoName.Get())
		}
		if !strings.Contains(item.RepoURL, "api.github.com/repos") {
			t.Errorf("expected RepoURL to contain api.github.com/repos, got %s", item.RepoURL)
		}
		if len(item.RawJSON) == 0 {
			t.Error("expected non-empty RawJSON")
		}
		if !strings.Contains(string(item.RawJSON), `"id":"`) {
			t.Errorf("expected RawJSON to contain id field, got %s", string(item.RawJSON))
		}
	}
	if !world.result.Items[0].CreatedAt.Equal(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)) {
		t.Errorf("expected CreatedAt=2024-01-15 10:30:00, got %v", world.result.Items[0].CreatedAt)
	}
}

func TestBDD_FetchAllPaginated(t *testing.T) {
	world := githubTestWorld{ctx: context.Background()}
	world.callCount = 0

	world.server = httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			world.callCount++
			page := r.URL.Query().Get("page")

			var events []*gh.Event

			switch page {
			case "1", "":
				for i := range 100 {
					events = append(events, &gh.Event{
						ID:   new("page1-" + string(rune('A'+i%26)) + string(rune('0'+i%10))),
						Type: new("PushEvent"),
					})
				}
			case "2":
				for i := range 50 {
					events = append(events, &gh.Event{
						ID:   new("page2-" + string(rune('A'+i%26))),
						Type: new("IssuesEvent"),
					})
				}
			default:
				events = []*gh.Event{}
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(events)
		}),
	)
	defer world.server.Close()

	world.client = newGitHubTestClientWithoutRateLimit(world.server)
	world.result, world.err = world.client.FetchAll(world.ctx, "testuser", 3)

	if world.err != nil {
		t.Fatalf("expected no error, got %v", world.err)
	}
	if len(world.result.Items) != 150 {
		t.Errorf("expected 150 items, got %d", len(world.result.Items))
	}
	if world.callCount > 3 {
		t.Errorf("expected at most 3 calls, got %d", world.callCount)
	}
}

func TestBDD_UserNotFound(t *testing.T) {
	world := githubTestWorld{ctx: context.Background()}
	world.server = testhelpers.NewErrorTestServer(http.StatusNotFound, "Not Found")
	defer world.server.Close()

	world.client = newGitHubTestClientWithoutRateLimit(world.server)
	world.fetchFor("nonexistent-user-xyz")

	if world.err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(world.err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got %v", world.err)
	}
}

func TestBDD_RateLimitInfo(t *testing.T) {
	world := githubTestWorld{ctx: context.Background()}

	world.server = httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rate_limit/" || r.URL.Path == "/rate_limit" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(testhelpers.RateLimitResponse(5))

				return
			}
		}),
	)
	defer world.server.Close()

	world.client = newGitHubTestClient(world.server)
	world.rateLimit, world.err = world.client.GetRateLimit(world.ctx)

	if world.err != nil {
		t.Fatalf("expected no error, got %v", world.err)
	}
	if world.rateLimit == nil {
		t.Fatal("expected non-nil rate limit")
	}
	if world.rateLimit.Remaining != 5 {
		t.Errorf("expected Remaining=5, got %d", world.rateLimit.Remaining)
	}
	if world.rateLimit.Limit != 5000 {
		t.Errorf("expected Limit=5000, got %d", world.rateLimit.Limit)
	}
	if !world.rateLimit.ResetAt.After(time.Now()) {
		t.Errorf("expected ResetAt to be in the future, got %v", world.rateLimit.ResetAt)
	}
}

func TestBDD_RetryOnServerError(t *testing.T) {
	world := githubTestWorld{ctx: context.Background()}
	var retryCountPtr *int
	world.server, retryCountPtr = testhelpers.NewFailingThenSucceedingTestServer(3)
	defer world.server.Close()

	world.client = newGitHubTestClientWithoutRateLimit(world.server)
	world.withRetryConfig()
	world.fetchFor("testuser")

	if world.err != nil {
		t.Fatalf("expected no error, got %v", world.err)
	}
	if *retryCountPtr < 2 {
		t.Errorf("expected at least 2 retries, got %d", *retryCountPtr)
	}
}

func TestBDD_NoRetryOnClientError(t *testing.T) {
	world := githubTestWorld{ctx: context.Background()}
	world.server = testhelpers.NewErrorTestServer(http.StatusBadRequest, "Bad Request")
	defer world.server.Close()

	world.client = newGitHubTestClientWithoutRateLimit(world.server)
	world.withRetryConfig()
	world.fetchFor("testuser")

	if world.err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBDD_ClientCreation(t *testing.T) {
	client := github.NewClient("ghp_test_token_12345")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Name() != "github" {
		t.Errorf("expected name=github, got %s", client.Name())
	}
}

func TestBDD_Configuration(t *testing.T) {
	client := github.NewClient("test-token")

	t.Run("custom rate limit config", func(t *testing.T) {
		cfg := provider.RateLimitConfig{
			Enabled:      true,
			MinRemaining: 100,
			MaxWait:      30 * time.Minute,
		}
		newClient := client.WithRateLimitConfig(cfg)
		if newClient == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("custom retry config", func(t *testing.T) {
		cfg := provider.RetryConfig{
			Enabled:        true,
			MaxRetries:     5,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     60 * time.Second,
		}
		newClient := client.WithRetryConfig(cfg)
		if newClient == nil {
			t.Fatal("expected non-nil client")
		}
	})
}
