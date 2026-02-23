package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/event"
	"golang.org/x/oauth2"
)

// RateLimitConfig configures rate limit handling behavior.
type RateLimitConfig struct {
	// Enabled controls whether rate limit checking is performed.
	Enabled bool
	// MinRemaining is the minimum remaining calls before waiting.
	MinRemaining int
	// MaxWait is the maximum time to wait for rate limit reset.
	MaxWait time.Duration
}

// DefaultRateLimitConfig provides sensible defaults for rate limit handling.
var DefaultRateLimitConfig = RateLimitConfig{
	Enabled:      true,
	MinRemaining: 10,
	MaxWait:      15 * time.Minute,
}

// RetryConfig configures retry behavior for transient errors.
type RetryConfig struct {
	// Enabled controls whether retry is performed.
	Enabled bool
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int
	// InitialBackoff is the initial backoff duration.
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration.
	MaxBackoff time.Duration
}

// DefaultRetryConfig provides sensible defaults for retry behavior.
var DefaultRetryConfig = RetryConfig{
	Enabled:        true,
	MaxRetries:     3,
	InitialBackoff: 1 * time.Second,
	MaxBackoff:     30 * time.Second,
}

// Fetcher defines the interface for fetching GitHub events.
// This interface enables testing with mocks and decouples the sync logic from the concrete client.
type Fetcher interface {
	FetchEvents(ctx context.Context, username string, opts *FetchOptions) ([]*event.Event, error)
	FetchAllEvents(ctx context.Context, username string, maxPages int) ([]*event.Event, error)
	GetRateLimit(ctx context.Context) (*gh.RateLimits, *gh.Response, error)
}

type Client struct {
	client          *gh.Client
	rateLimitConfig RateLimitConfig
	retryConfig     RetryConfig
}

func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		client:          gh.NewClient(tc),
		rateLimitConfig: DefaultRateLimitConfig,
		retryConfig:     DefaultRetryConfig,
	}
}

func NewClientWithHTTP(client *http.Client) *Client {
	return &Client{
		client:          gh.NewClient(client),
		rateLimitConfig: DefaultRateLimitConfig,
		retryConfig:     DefaultRetryConfig,
	}
}

// WithRateLimitConfig returns a copy of the client with custom rate limit config.
func (c *Client) WithRateLimitConfig(cfg RateLimitConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: cfg,
		retryConfig:     c.retryConfig,
	}
}

// WithRetryConfig returns a copy of the client with custom retry config.
func (c *Client) WithRetryConfig(cfg RetryConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: c.rateLimitConfig,
		retryConfig:     cfg,
	}
}

type FetchOptions struct {
	PerPage int
	Page    int
}

func (c *Client) FetchEvents(ctx context.Context, username string, opts *FetchOptions) ([]*event.Event, error) {
	if opts == nil {
		opts = &FetchOptions{PerPage: 100, Page: 1}
	}
	if opts.PerPage == 0 {
		opts.PerPage = 100
	}

	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	var activity []*gh.Event
	var err error
	err = c.withRetry(ctx, func() error {
		activity, _, err = c.client.Activity.ListEventsPerformedByUser(ctx, username, false, &gh.ListOptions{
			Page:    opts.Page,
			PerPage: opts.PerPage,
		})
		return err
	})
	if err != nil {
		return nil, wrapGitHubError(err, username)
	}

	events := make([]*event.Event, 0, len(activity))
	for _, e := range activity {
		event, err := convertEvent(e)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

func (c *Client) FetchAllEvents(ctx context.Context, username string, maxPages int) ([]*event.Event, error) {
	if maxPages == 0 {
		maxPages = 10
	}

	var allEvents []*event.Event
	for page := 1; page <= maxPages; page++ {
		events, err := c.FetchEvents(ctx, username, &FetchOptions{
			PerPage: 100,
			Page:    page,
		})
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			break
		}
		allEvents = append(allEvents, events...)
	}
	return allEvents, nil
}

func convertEvent(e *gh.Event) (*event.Event, error) {
	rawJSON, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	var actorLogin, actorAvatarURL string
	var repoName, repoURL string

	if e.Actor != nil {
		actorLogin = e.Actor.GetLogin()
		if e.Actor.GetAvatarURL() != "" {
			actorAvatarURL = e.Actor.GetAvatarURL()
		}
	}

	if e.Repo != nil {
		repoName = e.Repo.GetName()
		repoURL = e.Repo.GetURL()
	}

	createdAt := time.Now()
	if e.CreatedAt != nil {
		createdAt = e.CreatedAt.Time
	}

	return &event.Event{
		GithubID:       e.GetID(),
		Type:           e.GetType(),
		ActorLogin:     actorLogin,
		ActorAvatarURL: actorAvatarURL,
		RepoName:       repoName,
		RepoURL:        repoURL,
		CreatedAt:      createdAt,
		RawJSON:        rawJSON,
	}, nil
}

func (c *Client) GetRateLimit(ctx context.Context) (*gh.RateLimits, *gh.Response, error) {
	return c.client.RateLimit.Get(ctx)
}

// waitForRateLimit checks the rate limit and waits if necessary.
// Returns an error if the context is cancelled or max wait time is exceeded.
func (c *Client) waitForRateLimit(ctx context.Context) error {
	if !c.rateLimitConfig.Enabled {
		return nil
	}

	limits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to check rate limit: %w", err)
	}

	core := limits.GetCore()
	if core == nil {
		return nil
	}

	if core.Remaining > c.rateLimitConfig.MinRemaining {
		return nil
	}

	resetTime := core.Reset.Time
	waitDuration := time.Until(resetTime)

	if waitDuration <= 0 {
		return nil
	}

	if waitDuration > c.rateLimitConfig.MaxWait {
		return fmt.Errorf("%w: reset in %v (exceeds max wait %v)",
			errors.ErrRateLimited, waitDuration, c.rateLimitConfig.MaxWait)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitDuration):
		return nil
	}
}

// withRetry executes fn with exponential backoff retry for transient errors.
func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	if !c.retryConfig.Enabled {
		return fn()
	}

	var lastErr error
	backoff := c.retryConfig.InitialBackoff

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err
		if !isRetryableError(err) {
			return err
		}

		if attempt < c.retryConfig.MaxRetries {
			if backoff > c.retryConfig.MaxBackoff {
				backoff = c.retryConfig.MaxBackoff
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return lastErr
}

// isRetryableError determines if an error is transient and should be retried.
func isRetryableError(err error) bool {
	if ghErr, ok := err.(*gh.ErrorResponse); ok {
		statusCode := ghErr.Response.StatusCode
		return statusCode >= 500 || statusCode == 429
	}
	return false
}

// wrapGitHubError converts GitHub API errors into typed errors.
func wrapGitHubError(err error, username string) error {
	if ghErr, ok := err.(*gh.ErrorResponse); ok {
		switch ghErr.Response.StatusCode {
		case 401:
			return errors.WithUserDetail(errors.ErrInvalidToken, username)
		case 403:
			return errors.WithUserDetail(errors.ErrRateLimited, username)
		case 404:
			return errors.WithUserDetail(errors.ErrUserNotFound, username)
		}
	}
	return errors.WithUserDetail(errors.ErrSyncFailed, username)
}
