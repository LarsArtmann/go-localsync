// Package github provides a GitHub provider for go-localsync.
// It implements the provider.Provider interface to fetch GitHub user events.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"golang.org/x/oauth2"
)

// Client implements provider.Provider for GitHub.
type Client struct {
	client          *gh.Client
	rateLimitConfig provider.RateLimitConfig
	retryConfig     provider.RetryConfig
}

// NewClient creates a new GitHub provider client with the given token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return &Client{
		client:          gh.NewClient(tc),
		rateLimitConfig: provider.DefaultRateLimitConfig,
		retryConfig:     provider.DefaultRetryConfig,
	}
}

// NewClientWithHTTP creates a new GitHub provider client with a custom HTTP client.
func NewClientWithHTTP(client *http.Client) *Client {
	return &Client{
		client:          gh.NewClient(client),
		rateLimitConfig: provider.DefaultRateLimitConfig,
		retryConfig:     provider.DefaultRetryConfig,
	}
}

// WithRateLimitConfig returns a copy of the client with custom rate limit config.
func (c *Client) WithRateLimitConfig(cfg provider.RateLimitConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: cfg,
		retryConfig:     c.retryConfig,
	}
}

// WithRetryConfig returns a copy of the client with custom retry config.
func (c *Client) WithRetryConfig(cfg provider.RetryConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: c.rateLimitConfig,
		retryConfig:     cfg,
	}
}

// Name returns the provider identifier.
func (c *Client) Name() string {
	return "github"
}

// Fetch retrieves a single page of GitHub events.
func (c *Client) Fetch(
	ctx context.Context,
	opts *provider.FetchOptions,
) (*provider.FetchResult, error) {
	if opts == nil {
		opts = &provider.FetchOptions{PerPage: 100, Page: 1}
	}

	if opts.PerPage == 0 {
		opts.PerPage = 100
	}

	if opts.Page == 0 {
		opts.Page = 1
	}

	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	var (
		activity []*gh.Event
		err      error
	)

	err = c.withRetry(ctx, func() error {
		activity, _, err = c.client.Activity.ListEventsPerformedByUser(
			ctx,
			opts.Source,
			false,
			&gh.ListOptions{
				Page:    opts.Page,
				PerPage: opts.PerPage,
			},
		)

		return err
	})
	if err != nil {
		return nil, wrapGitHubError(err, opts.Source)
	}

	items := make([]*provider.Item, 0, len(activity))
	for _, e := range activity {
		item, err := convertEvent(e)
		if err != nil {
			continue
		}

		items = append(items, item)
	}

	return &provider.FetchResult{
		Items:   items,
		HasMore: len(items) == opts.PerPage,
	}, nil
}

// FetchAll retrieves all available GitHub events up to maxPages.
func (c *Client) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	if maxPages == 0 {
		maxPages = 10
	}

	var allItems []*provider.Item

	for page := 1; page <= maxPages; page++ {
		result, err := c.Fetch(ctx, &provider.FetchOptions{
			Source:  source,
			PerPage: 100,
			Page:    page,
		})
		if err != nil {
			return nil, err
		}

		if len(result.Items) == 0 {
			break
		}

		allItems = append(allItems, result.Items...)
		if !result.HasMore {
			break
		}
	}

	return &provider.FetchResult{Items: allItems}, nil
}

// GetRateLimit returns current GitHub rate limit information.
func (c *Client) GetRateLimit(ctx context.Context) (*provider.RateLimitInfo, error) {
	limits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return nil, err
	}

	core := limits.GetCore()
	if core == nil {
		return nil, nil
	}

	return &provider.RateLimitInfo{
		Limit:     core.Limit,
		Remaining: core.Remaining,
		ResetAt:   core.Reset.Time,
	}, nil
}

func convertEvent(e *gh.Event) (*provider.Item, error) {
	rawJSON, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	var (
		actorLogin, actorAvatarURL string
		repoName, repoURL          string
	)

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

	return &provider.Item{
		ID:             e.GetID(),
		Source:         "github",
		Type:           e.GetType(),
		ActorLogin:     actorLogin,
		ActorAvatarURL: actorAvatarURL,
		RepoName:       repoName,
		RepoURL:        repoURL,
		CreatedAt:      createdAt,
		RawJSON:        rawJSON,
	}, nil
}

// waitForRateLimit checks the rate limit and waits if necessary.
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
			pkgerrors.ErrRateLimited, waitDuration, c.rateLimitConfig.MaxWait)
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
	ghErr := &gh.ErrorResponse{}
	if errors.As(err, &ghErr) {
		statusCode := ghErr.Response.StatusCode

		return statusCode >= 500 || statusCode == 429
	}

	return false
}

// wrapGitHubError converts GitHub API errors into typed errors.
func wrapGitHubError(err error, username string) error {
	ghErr := &gh.ErrorResponse{}
	if errors.As(err, &ghErr) {
		switch ghErr.Response.StatusCode {
		case http.StatusUnauthorized:
			return pkgerrors.WithUserDetail(pkgerrors.ErrInvalidToken, username)
		case http.StatusForbidden:
			return pkgerrors.WithUserDetail(pkgerrors.ErrRateLimited, username)
		case http.StatusNotFound:
			return pkgerrors.WithUserDetail(pkgerrors.ErrUserNotFound, username)
		}
	}

	return pkgerrors.WithUserDetail(pkgerrors.ErrSyncFailed, username)
}
