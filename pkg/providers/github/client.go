// Package github provides a GitHub provider for go-localsync.
// It implements the provider.Provider interface to fetch GitHub user events.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"charm.land/log/v2"
	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

const providerName = "github"

// rateLimitCache stores rate-limit info extracted from API response headers.
// Uses the provider-agnostic provider.RateLimitInfo type so the cache concept
// could be reused for non-GitHub providers. Shared across With* copies so
// config changes don't lose the cache.
type rateLimitCache struct {
	mu   sync.Mutex
	info *provider.RateLimitInfo
}

// update stores the authoritative rate-limit info from an API response.
// Overwrites any local decrement-based estimate.
func (c *rateLimitCache) update(info *provider.RateLimitInfo) {
	if info == nil || info.Limit == 0 {
		return
	}

	c.mu.Lock()
	c.info = info
	c.mu.Unlock()
}

// decrement locally decrements the remaining count by n after dispatching
// API calls, giving a conservative estimate between API responses.
// The next API response will overwrite with the authoritative value.
func (c *rateLimitCache) decrement(n int) {
	if c == nil || n <= 0 {
		return
	}

	c.mu.Lock()
	if c.info != nil {
		c.info.Remaining -= n
		if c.info.Remaining < 0 {
			c.info.Remaining = 0
		}
	}
	c.mu.Unlock()
}

func (c *rateLimitCache) get() (*provider.RateLimitInfo, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.info, c.info != nil
}

// ghRateToInfo converts a GitHub SDK rate struct to provider-agnostic RateLimitInfo.
func ghRateToInfo(rate *gh.Rate) *provider.RateLimitInfo {
	if rate == nil {
		return nil
	}

	return &provider.RateLimitInfo{
		Limit:     rate.Limit,
		Remaining: rate.Remaining,
		ResetAt:   rate.Reset.Time,
	}
}

// Client implements provider.Provider for GitHub.
type Client struct {
	client          *gh.Client
	rateLimitConfig provider.RateLimitConfig
	retryConfig     provider.RetryConfig
	rateCache       *rateLimitCache
}

var _ provider.Provider = (*Client)(nil)

// newHTTPClient creates a configured HTTP client with timeout and tuned transport
// for GitHub API interaction.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// NewClient creates a new GitHub provider client with the given token.
func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	tc.Timeout = 30 * time.Second

	if tr, ok := tc.Transport.(*oauth2.Transport); ok {
		if base := tr.Base; base == nil {
			tr.Base = newHTTPClient().Transport
		}
	}

	return &Client{
		client:          gh.NewClient(tc),
		rateLimitConfig: provider.DefaultRateLimitConfig,
		retryConfig:     provider.DefaultRetryConfig,
		rateCache:       &rateLimitCache{},
	}
}

// NewClientWithHTTP creates a new GitHub provider client with a custom HTTP client.
func NewClientWithHTTP(client *http.Client) *Client {
	return &Client{
		client:          gh.NewClient(client),
		rateLimitConfig: provider.DefaultRateLimitConfig,
		retryConfig:     provider.DefaultRetryConfig,
		rateCache:       &rateLimitCache{},
	}
}

// WithRateLimitConfig returns a copy of the client with custom rate limit config.
func (c *Client) WithRateLimitConfig(cfg provider.RateLimitConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: cfg,
		retryConfig:     c.retryConfig,
		rateCache:       c.rateCache,
	}
}

// WithRetryConfig returns a copy of the client with custom retry config.
func (c *Client) WithRetryConfig(cfg provider.RetryConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: c.rateLimitConfig,
		retryConfig:     cfg,
		rateCache:       c.rateCache,
	}
}

// WithBaseURL returns a copy of the client with the given base URL.
func (c *Client) WithBaseURL(rawURL string) (*Client, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL %q: %w", rawURL, err)
	}

	next := &Client{
		client:          c.client,
		rateLimitConfig: c.rateLimitConfig,
		retryConfig:     c.retryConfig,
		rateCache:       c.rateCache,
	}
	next.client.BaseURL = parsed

	return next, nil
}

// Name returns the provider identifier.
func (c *Client) Name() string {
	return providerName
}

// Fetch retrieves a single page of GitHub events.
func (c *Client) Fetch(
	ctx context.Context,
	opts *provider.FetchOptions,
) (*provider.FetchResult, error) {
	if opts == nil {
		opts = &provider.FetchOptions{Source: id.NewProviderID(""), PerPage: 100, Page: 1}
	}

	if opts.PerPage == 0 {
		opts.PerPage = 100
	}

	if opts.Page == 0 {
		opts.Page = 1
	}

	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, pkgerrors.Wrapf(err, "rate limit check failed for %s", opts.Source)
	}

	var (
		activity []*gh.Event
		resp     *gh.Response
		err      error
	)

	err = c.withRetry(ctx, func() error {
		activity, resp, err = c.client.Activity.ListEventsPerformedByUser(
			ctx,
			opts.Source.Get(),
			false,
			&gh.ListOptions{
				Page:    opts.Page,
				PerPage: opts.PerPage,
			},
		)

		return err
	})
	if err != nil {
		return nil, pkgerrors.Wrapf(
			wrapGitHubError(err, opts.Source.Get()),
			"fetching events for %s failed (page %d)",
			opts.Source.Get(),
			opts.Page,
		)
	}

	if resp != nil {
		c.rateCache.update(ghRateToInfo(&resp.Rate))
	}

	items := convertEvents(activity)

	return &provider.FetchResult{
		Items:   items,
		HasMore: len(items) == opts.PerPage,
	}, nil
}

// FetchAll retrieves all available GitHub events up to maxPages.
// Pages after the first are fetched concurrently with a bounded goroutine
// pool (concurrency 3) to reduce wall-clock latency.
func (c *Client) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	if maxPages == 0 {
		maxPages = 10
	}

	first, err := c.Fetch(ctx, &provider.FetchOptions{
		Source:  id.NewProviderID(source),
		PerPage: 100,
		Page:    1,
	})
	if err != nil {
		return nil, pkgerrors.Wrapf(
			err,
			"fetch page 1/%d for %s failed",
			maxPages,
			source,
		)
	}

	if len(first.Items) == 0 || !first.HasMore || maxPages == 1 {
		return &provider.FetchResult{Items: first.Items, HasMore: false}, nil
	}

	remaining := maxPages - 1
	results := make([]*provider.FetchResult, remaining)

	g, ctx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, 3)

	for page := 2; page <= maxPages; page++ {
		page := page
		idx := page - 2
		sem <- struct{}{}

		g.Go(func() error {
			defer func() { <-sem }()

			result, err := c.Fetch(ctx, &provider.FetchOptions{
				Source:  id.NewProviderID(source),
				PerPage: 100,
				Page:    page,
			})
			if err != nil {
				return pkgerrors.Wrapf(
					err,
					"fetch page %d/%d for %s failed",
					page,
					maxPages,
					source,
				)
			}

			results[idx] = result

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	allItems := first.Items
	for _, r := range results {
		if r == nil || len(r.Items) == 0 {
			break
		}

		allItems = append(allItems, r.Items...)
	}

	return &provider.FetchResult{Items: allItems, HasMore: false}, nil
}

// GetRateLimit returns current GitHub rate limit information.
func (c *Client) GetRateLimit(ctx context.Context) (*provider.RateLimitInfo, error) {
	limits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return nil, err
	}

	core := limits.GetCore()
	if core == nil {
		return &provider.RateLimitInfo{Limit: 0, Remaining: 0, ResetAt: time.Time{}}, nil
	}

	return &provider.RateLimitInfo{
		Limit:     core.Limit,
		Remaining: core.Remaining,
		ResetAt:   core.Reset.Time,
	}, nil
}

func convertEvents(activity []*gh.Event) []*provider.Item {
	items := make([]*provider.Item, 0, len(activity))

	for _, e := range activity {
		item, err := convertEvent(e)
		if err != nil {
			log.Warn("failed to convert GitHub event", "eventID", e.GetID(), "error", err)

			continue
		}

		items = append(items, item)
	}

	return items
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
		ID:             id.NewItemID(),
		ExternalID:     id.NewExternalID(e.GetID()),
		Source:         id.NewProviderID(providerName),
		Type:           id.NewEventTypeID(e.GetType()),
		ActorLogin:     id.NewActorID(actorLogin),
		ActorAvatarURL: actorAvatarURL,
		RepoName:       id.NewRepoID(repoName),
		RepoURL:        repoURL,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
		RawJSON:        rawJSON,
	}, nil
}
