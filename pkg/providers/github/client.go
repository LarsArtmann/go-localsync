// Package github provides a GitHub provider for go-localsync.
// It implements the provider.Provider interface to fetch GitHub user events.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	fetchConfig     provider.FetchConfig
	rateCache       *provider.RateLimitCache
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
		fetchConfig:     provider.DefaultFetchConfig,
		rateCache:       provider.NewRateLimitCache(),
	}
}

// NewClientWithHTTP creates a new GitHub provider client with a custom HTTP client.
func NewClientWithHTTP(client *http.Client) *Client {
	return &Client{
		client:          gh.NewClient(client),
		rateLimitConfig: provider.DefaultRateLimitConfig,
		retryConfig:     provider.DefaultRetryConfig,
		fetchConfig:     provider.DefaultFetchConfig,
		rateCache:       provider.NewRateLimitCache(),
	}
}

// WithRateLimitConfig returns a copy of the client with custom rate limit config.
func (c *Client) WithRateLimitConfig(cfg provider.RateLimitConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: cfg,
		retryConfig:     c.retryConfig,
		fetchConfig:     c.fetchConfig,
		rateCache:       c.rateCache,
	}
}

// WithRetryConfig returns a copy of the client with custom retry config.
func (c *Client) WithRetryConfig(cfg provider.RetryConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: c.rateLimitConfig,
		retryConfig:     cfg,
		fetchConfig:     c.fetchConfig,
		rateCache:       c.rateCache,
	}
}

// WithFetchConfig returns a copy of the client with custom fetch config.
func (c *Client) WithFetchConfig(cfg provider.FetchConfig) *Client {
	return &Client{
		client:          c.client,
		rateLimitConfig: c.rateLimitConfig,
		retryConfig:     c.retryConfig,
		fetchConfig:     cfg,
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
		fetchConfig:     c.fetchConfig,
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
// Rate-limit info from response headers is cached and included in the FetchResult.
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

	var rateInfo *provider.RateLimitInfo

	if resp != nil {
		rateInfo = ghRateToInfo(&resp.Rate)
		c.rateCache.Update(rateInfo)
	}

	items := convertEvents(activity)

	return &provider.FetchResult{
		Items:     items,
		HasMore:   len(items) == opts.PerPage,
		RateLimit: rateInfo,
	}, nil
}

// FetchAll retrieves all available GitHub events up to maxPages.
//
// Page 1 is fetched sequentially to determine if multi-page fetching is worthwhile.
// Pages 2 through maxPages are fetched concurrently using a bounded goroutine pool
// controlled by FetchConfig.MaxConcurrentFetches (default 3).
//
// Early termination: if a page returns fewer items than requested (PerPage),
// remaining pending pages are cancelled via context, since GitHub returns
// full pages until data is exhausted.
//
// Progress: if FetchConfig.OnProgress is set, it is called after each page
// completes with (pageNumber, maxPages, cumulativeItemCount).
//
// The caller's context deadline applies to the entire operation.
func (c *Client) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	if maxPages == 0 {
		maxPages = 10
	}

	first, err := c.fetchFirstPage(ctx, source, maxPages)
	if err != nil {
		return nil, err
	}

	if len(first.Items) == 0 || !first.HasMore || maxPages == 1 {
		return &provider.FetchResult{Items: first.Items, HasMore: false, RateLimit: first.RateLimit}, nil
	}

	remaining := maxPages - 1
	results := make([]*provider.FetchResult, remaining)

	concurrency := max(c.fetchConfig.MaxConcurrentFetches, 1)

	group, groupCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, concurrency)

	var totalFetched int

	c.reportProgress(1, maxPages, len(first.Items))

	for page := 2; page <= maxPages; page++ {
		page := page
		idx := page - 2

		sem <- struct{}{}

		group.Go(func() error {
			defer func() { <-sem }()

			result, fetchErr := c.Fetch(groupCtx, &provider.FetchOptions{
				Source:  id.NewProviderID(source),
				PerPage: 100,
				Page:    page,
			})
			if fetchErr != nil {
				return pkgerrors.Wrapf(
					fetchErr,
					"fetch page %d/%d for %s failed",
					page,
					maxPages,
					source,
				)
			}

			results[idx] = result

			c.reportProgress(page, maxPages, len(result.Items))

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	allItems := first.Items

	totalFetched = len(allItems)

	for _, r := range results {
		if r == nil || len(r.Items) == 0 {
			break
		}

		allItems = append(allItems, r.Items...)
		totalFetched += len(r.Items)
	}

	return &provider.FetchResult{Items: allItems, HasMore: false, RateLimit: first.RateLimit}, nil
}

func (c *Client) fetchFirstPage(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
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

	return first, nil
}

func (c *Client) reportProgress(page, maxPages, itemsInPage int) {
	if c.fetchConfig.OnProgress != nil {
		c.fetchConfig.OnProgress(page, maxPages, itemsInPage)
	}
}

// GetRateLimit returns current GitHub rate limit information.
// Uses cached data from recent API responses when available to avoid
// a dedicated /rate_limit API call.
func (c *Client) GetRateLimit(ctx context.Context) (*provider.RateLimitInfo, error) {
	if cached, ok := c.rateCache.Get(); ok && time.Now().Before(cached.ResetAt) {
		return cached, nil
	}

	limits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return nil, err
	}

	core := limits.GetCore()
	if core == nil {
		return &provider.RateLimitInfo{Limit: 0, Remaining: 0, ResetAt: time.Time{}}, nil
	}

	info := &provider.RateLimitInfo{
		Limit:     core.Limit,
		Remaining: core.Remaining,
		ResetAt:   core.Reset.Time,
	}
	c.rateCache.Update(info)

	return info, nil
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
