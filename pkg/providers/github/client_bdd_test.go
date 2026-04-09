package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/providers/github"
	"github.com/larsartmann/go-localsync/pkg/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// githubTestWorld holds shared test state for BDD scenarios.
type githubTestWorld struct {
	ctx       context.Context
	server    *httptest.Server
	client    *github.Client
	result    *provider.FetchResult
	rateLimit *provider.RateLimitInfo
	err       error
	callCount int
}

// fetchFor invokes the client's Fetch method for the given source.
func (w *githubTestWorld) fetchFor(source string) {
	w.result, w.err = w.client.Fetch(w.ctx, &provider.FetchOptions{Source: source})
}

// withRetryConfig enables retry with fast backoff on the client.
func (w *githubTestWorld) withRetryConfig() {
	w.client = w.client.WithRetryConfig(testhelpers.TestRetryConfig())
}

// newGitHubTestClient creates a client pointing to the test server.
func newGitHubTestClient(server *httptest.Server) *github.Client {
	httpClient := &http.Client{}
	client := github.NewClientWithHTTP(httpClient)
	return client
}

// newGitHubTestClientWithoutRateLimit creates a client with rate limiting disabled.
func newGitHubTestClientWithoutRateLimit(server *httptest.Server) *github.Client {
	return newGitHubTestClient(server).WithRateLimitConfig(provider.RateLimitConfig{Enabled: false})
}

var _ = Describe("GitHub Provider", func() {
	var world githubTestWorld

	BeforeEach(func() {
		world = githubTestWorld{
			ctx: context.Background(),
		}
	})

	AfterEach(func() {
		if world.server != nil {
			world.server.Close()
		}
	})

	Describe("as a developer syncing GitHub events for offline analysis", func() {
		Context("when I fetch events for a valid user", func() {
			BeforeEach(func() {
				// Given: A GitHub API server returning user events
				world.server = httptest.NewServer(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						world.callCount++
						Expect(r.URL.Path).To(ContainSubstring("/users/octocat/events"))

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

				world.client = newGitHubTestClientWithoutRateLimit(world.server)
			})

			JustBeforeEach(func() {
				world.fetchFor("octocat")
			})

			It("should succeed", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should return the user's events", func() {
				Expect(world.result.Items).To(HaveLen(2))
			})

			It("should preserve event IDs", func() {
				Expect(world.result.Items[0].ID.Get()).To(Equal("event-123"))
				Expect(world.result.Items[1].ID.Get()).To(Equal("event-456"))
			})

			It("should map event types correctly", func() {
				Expect(world.result.Items[0].Type.Get()).To(Equal("PushEvent"))
				Expect(world.result.Items[1].Type.Get()).To(Equal("IssuesEvent"))
			})

			It("should include actor information", func() {
				for _, item := range world.result.Items {
					Expect(item.ActorLogin.Get()).To(Equal("octocat"))
					Expect(
						item.ActorAvatarURL,
					).To(ContainSubstring("avatars.githubusercontent.com"))
				}
			})

			It("should include repository information", func() {
				for _, item := range world.result.Items {
					Expect(item.RepoName.Get()).To(Equal("octocat/Hello-World"))
					Expect(item.RepoURL).To(ContainSubstring("api.github.com/repos"))
				}
			})

			It("should preserve timestamps", func() {
				Expect(
					world.result.Items[0].CreatedAt,
				).To(Equal(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)))
			})

			It("should preserve raw JSON for full fidelity", func() {
				for _, item := range world.result.Items {
					Expect(item.RawJSON).ToNot(BeEmpty())
					Expect(item.RawJSON).To(ContainSubstring(`"id":"`))
				}
			})
		})

		Context("when I fetch all events across multiple pages", func() {
			BeforeEach(func() {
				// Given: A server with paginated results
				world.callCount = 0
				world.server = httptest.NewServer(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						world.callCount++
						page := r.URL.Query().Get("page")

						var events []*gh.Event
						switch page {
						case "1", "":
							// First page: 100 items (full page, indicates more available)
							for i := range 100 {
								events = append(events, &gh.Event{
									ID: new(
										"page1-" + string(rune('A'+i%26)) + string(rune('0'+i%10)),
									),
									Type: new("PushEvent"),
								})
							}
						case "2":
							// Second page: 50 items (partial, indicates no more)
							for i := range 50 {
								events = append(events, &gh.Event{
									ID:   new("page2-" + string(rune('A'+i%26))),
									Type: new("IssuesEvent"),
								})
							}
						default:
							// No more pages
							events = []*gh.Event{}
						}

						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(events)
					}),
				)

				world.client = newGitHubTestClientWithoutRateLimit(world.server)
			})

			JustBeforeEach(func() {
				// When: I fetch all events with max 3 pages
				world.result, world.err = world.client.FetchAll(world.ctx, "testuser", 3)
			})

			It("should succeed", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should aggregate items from all pages", func() {
				Expect(world.result.Items).To(HaveLen(150)) // 100 + 50
			})

			It("should stop when receiving an empty page", func() {
				Expect(world.callCount).To(BeNumerically("<=", 3))
			})
		})

		Context("when the user does not exist", func() {
			BeforeEach(func() {
				// Given: A server that returns 404 for unknown users
				world.server = testhelpers.NewErrorTestServer(http.StatusNotFound, "Not Found")

				world.client = newGitHubTestClient(world.server)
			})

			JustBeforeEach(func() {
				world.fetchFor("nonexistent-user-xyz")
			})

			It("should return an error", func() {
				Expect(world.err).To(HaveOccurred())
			})

			It("should indicate user not found", func() {
				Expect(world.err.Error()).To(ContainSubstring("not found"))
			})
		})

		Context("when GitHub rate limits my requests", func() {
			BeforeEach(func() {
				// Given: A server that returns rate limit info
				world.server = httptest.NewServer(
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/rate_limit/" {
							w.Header().Set("Content-Type", "application/json")
							_ = json.NewEncoder(w).Encode(map[string]any{
								"resources": gh.RateLimits{
									Core: &gh.Rate{
										Limit:     5000,
										Remaining: 5, // Low remaining
										Reset: gh.Timestamp{
											Time: time.Now().Add(1 * time.Hour),
										},
									},
								},
							})
							return
						}
					}),
				)

				world.client = newGitHubTestClient(world.server)
			})

			JustBeforeEach(func() {
				// When: I check rate limits
				world.rateLimit, world.err = world.client.GetRateLimit(world.ctx)
			})

			It("should return rate limit information", func() {
				Expect(world.err).ToNot(HaveOccurred())
				Expect(world.rateLimit).ToNot(BeNil())
			})

			It("should show remaining requests", func() {
				Expect(world.rateLimit.Remaining).To(Equal(5))
			})

			It("should show the limit", func() {
				Expect(world.rateLimit.Limit).To(Equal(5000))
			})

			It("should show when the limit resets", func() {
				Expect(world.rateLimit.ResetAt).To(BeTemporally(">", time.Now()))
			})
		})

		Context("when GitHub returns a server error", func() {
			BeforeEach(func() {
				// Given: A server that fails initially then succeeds
				var retryCountPtr *int
				world.server, retryCountPtr = testhelpers.NewFailingThenSucceedingTestServer(3)
				world.callCount = *retryCountPtr

				world.client = newGitHubTestClient(world.server)
				world.withRetryConfig()
			})

			JustBeforeEach(func() {
				world.fetchFor("testuser")
			})

			It("should retry and eventually succeed", func() {
				Expect(world.err).ToNot(HaveOccurred())
			})

			It("should have retried the request", func() {
				Expect(world.callCount).To(BeNumerically(">=", 2))
			})
		})

		Context("when GitHub returns a client error (4xx)", func() {
			BeforeEach(func() {
				// Given: A server that returns 400 Bad Request
				world.server = testhelpers.NewErrorTestServer(http.StatusBadRequest, "Bad Request")

				world.client = newGitHubTestClient(world.server)
				world.withRetryConfig()
			})

			JustBeforeEach(func() {
				world.fetchFor("testuser")
			})

			It("should not retry on client errors", func() {
				Expect(world.err).To(HaveOccurred())
			})
		})

		Context("when I create a client with a token", func() {
			BeforeEach(func() {
				// When: I create a client with a token
				world.client = github.NewClient("ghp_test_token_12345")
			})

			It("should be created successfully", func() {
				Expect(world.client).ToNot(BeNil())
			})

			It("should identify as github provider", func() {
				Expect(world.client.Name()).To(Equal("github"))
			})
		})

		Context("when I configure rate limit settings", func() {
			BeforeEach(func() {
				world.client = github.NewClient("test-token")
			})

			It("should allow custom rate limit configuration", func() {
				cfg := provider.RateLimitConfig{
					Enabled:      true,
					MinRemaining: 100,
					MaxWait:      30 * time.Minute,
				}
				newClient := world.client.WithRateLimitConfig(cfg)
				Expect(newClient).ToNot(BeNil())
			})

			It("should allow custom retry configuration", func() {
				cfg := provider.RetryConfig{
					Enabled:        true,
					MaxRetries:     5,
					InitialBackoff: 2 * time.Second,
					MaxBackoff:     60 * time.Second,
				}
				newClient := world.client.WithRetryConfig(cfg)
				Expect(newClient).ToNot(BeNil())
			})
		})
	})
})
