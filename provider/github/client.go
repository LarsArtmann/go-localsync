// Package github provides a GitHub events provider for go-localsync.
//
// It implements provider.Provider on top of the go-github-kit kernel:
// token authentication, rate-limit gating with wait-until-reset, budget
// tracking from response headers, and retry with backoff are applied by
// the kernel's transport stack on every call. The core go-localsync
// module stays free of GitHub dependencies; consumers opt in by
// requiring this module.
package github

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"charm.land/log/v2"
	githubkit "github.com/LarsArtmann/go-github-kit"
	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
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

// snapshotToInfo converts a kernel rate-limit snapshot to provider-agnostic RateLimitInfo.
func snapshotToInfo(snapshot githubkit.RateLimitSnapshot) *provider.RateLimitInfo {
	return &provider.RateLimitInfo{
		Limit:     snapshot.Limit,
		Remaining: snapshot.Remaining,
		ResetAt:   snapshot.ResetAt,
	}
}

// Client implements provider.Provider for GitHub on top of the go-github-kit
// kernel: token auth, rate-limit gating, budget tracking from response
// headers, and retry with backoff are applied by the kernel's transport
// stack on every call.
type Client struct {
	kernel          *githubkit.Kernel
	initErr         error
	token           string
	httpClient      *http.Client
	baseURL         string
	rateLimitConfig RateLimitConfig
	retryConfig     provider.RetryConfig
	fetchConfig     FetchConfig
	etagConfig      *githubkit.ETagOptions
}

var _ provider.Provider = (*Client)(nil)

// NewClient creates a new GitHub provider client with the given token.
func NewClient(token string) *Client {
	return newClient(token, nil)
}

// NewClientWithHTTP creates a new GitHub provider client with a custom HTTP client.
func NewClientWithHTTP(client *http.Client) *Client {
	return newClient("", client)
}

func newClient(token string, httpClient *http.Client) *Client {
	client := &Client{
		token:           token,
		httpClient:      httpClient,
		rateLimitConfig: DefaultRateLimitConfig,
		retryConfig:     provider.DefaultRetryConfig,
		fetchConfig:     DefaultFetchConfig,
	}
	client.rebuildKernel()

	return client
}

// derive returns a functional copy with one configuration field replaced,
// rebuilding the kernel so the change takes effect immediately. A kernel
// construction failure is captured in the copy and surfaces on its next call.
func (c *Client) derive(mutate func(next *Client)) *Client {
	next := &Client{
		token:           c.token,
		httpClient:      c.httpClient,
		baseURL:         c.baseURL,
		rateLimitConfig: c.rateLimitConfig,
		retryConfig:     c.retryConfig,
		fetchConfig:     c.fetchConfig,
		etagConfig:      c.etagConfig,
	}
	mutate(next)
	next.rebuildKernel()

	return next
}

// rebuildKernel constructs the go-github-kit kernel from the client's current
// configuration. Construction cannot fail for an already-validated base URL
// and an explicit token; any failure is stored in initErr and returned by
// every call until a corrected copy is derived.
func (c *Client) rebuildKernel() {
	opts := []githubkit.Option{
		githubkit.WithPAT(c.token),
		rateLimitOption(c.rateLimitConfig),
		retryOption(c.retryConfig),
	}

	if c.httpClient != nil {
		opts = append(opts, githubkit.WithHTTPClient(c.httpClient))
	}

	if c.baseURL != "" {
		opts = append(opts, githubkit.WithBaseURL(c.baseURL))
	}

	if c.etagConfig != nil {
		opts = append(opts, githubkit.WithETagCache(c.etagConfig))
	}

	c.kernel, c.initErr = githubkit.New(opts...)
}

// WithRateLimitConfig returns a copy of the client with custom rate limit config.
func (c *Client) WithRateLimitConfig(cfg RateLimitConfig) *Client {
	return c.derive(func(next *Client) { next.rateLimitConfig = cfg })
}

// WithRetryConfig returns a copy of the client with custom retry config.
func (c *Client) WithRetryConfig(cfg provider.RetryConfig) *Client {
	return c.derive(func(next *Client) { next.retryConfig = cfg })
}

// WithFetchConfig returns a copy of the client with custom fetch config.
func (c *Client) WithFetchConfig(cfg FetchConfig) *Client {
	return c.derive(func(next *Client) { next.fetchConfig = cfg })
}

// WithETagCache enables the kernel's conditional GET cache: unchanged
// re-fetches replay stored ETags as If-None-Match and GitHub answers 304
// (one request spent, zero rate-limit budget counted against the data
// endpoints). Off by default; opts zero fields get kit defaults
// (256 entries, 8 MiB bodies). Returns a derived copy.
func (c *Client) WithETagCache(opts githubkit.ETagOptions) *Client {
	return c.derive(func(next *Client) {
		copied := opts
		next.etagConfig = &copied
	})
}

// ETagStats reports conditional-cache counters (hits = 304 revalidations
// served from cache, stored = 200s cached, entries = cache size). The
// second return is false when the ETag cache is disabled or the kernel
// failed to initialize.
func (c *Client) ETagStats() (githubkit.ETagStats, bool) {
	if c.kernel == nil || c.initErr != nil {
		return githubkit.ETagStats{}, false
	}

	return c.kernel.ETagStats()
}

// WithBaseURL returns a copy of the client with the given base URL.
func (c *Client) WithBaseURL(rawURL string) (*Client, error) {
	if _, err := url.Parse(rawURL); err != nil {
		return nil, fmt.Errorf("parse base URL %q: %w", rawURL, err)
	}

	return c.derive(func(next *Client) { next.baseURL = rawURL }), nil
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

	if c.initErr != nil {
		return nil, pkgerrors.Wrap(c.initErr, "github client initialization failed")
	}

	activity, resp, err := c.kernel.Activity.ListEventsPerformedByUser(
		ctx,
		opts.Source.Get(),
		false,
		&gh.ListOptions{
			Page:    opts.Page,
			PerPage: opts.PerPage,
		},
	)
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
// The walk is delegated to githubkit.FetchPages: page 1 is fetched
// sequentially to detect an exhausted endpoint, pages 2..maxPages run on a
// bounded pool (FetchConfig.MaxConcurrentFetches, default 3), and a short
// page stops the walk because GitHub returns full pages until data runs
// out. Progress callbacks (if configured) receive the page number, the
// page cap, and the cumulative item count, matching the kernel contract.
func (c *Client) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	if maxPages == 0 {
		maxPages = 10
	}

	var (
		rateInfo *provider.RateLimitInfo
		rateOnce sync.Once
	)

	items, err := githubkit.FetchPages(ctx, githubkit.PaginationOptions{
		MaxPages:    maxPages,
		PerPage:     100,
		Concurrency: c.fetchConfig.MaxConcurrentFetches,
		OnProgress:  c.reportProgress,
	}, func(ctx context.Context, page int) ([]*provider.Item, error) {
		result, fetchErr := c.Fetch(ctx, newFetchOptions(source, page))
		if fetchErr != nil {
			return nil, fetchErr
		}

		if page == 1 {
			rateOnce.Do(func() { rateInfo = result.RateLimit })
		}

		return result.Items, nil
	})
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "fetching all events for %s failed", source)
	}

	return &provider.FetchResult{Items: items, HasMore: false, RateLimit: rateInfo}, nil
}

// newFetchOptions builds FetchOptions for a single GitHub events page fetch.
// PerPage is fixed at the GitHub-recommended 100; callers vary Source and Page.
func newFetchOptions(source string, page int) *provider.FetchOptions {
	return &provider.FetchOptions{
		Source:  id.NewProviderID(source),
		PerPage: 100,
		Page:    page,
	}
}

func (c *Client) reportProgress(page, maxPages, itemsInPage int) {
	if c.fetchConfig.OnProgress != nil {
		c.fetchConfig.OnProgress(page, maxPages, itemsInPage)
	}
}

// zeroRateLimitInfo returns the sentinel RateLimitInfo used when GitHub's
// /rate_limit response carries no Core data.
func zeroRateLimitInfo() *provider.RateLimitInfo {
	return &provider.RateLimitInfo{Limit: 0, Remaining: 0, ResetAt: time.Time{}}
}

// GetRateLimit returns current GitHub rate limit information.
// Uses the kernel's header-fed budget snapshot when fresh to avoid
// a dedicated /rate_limit API call.
func (c *Client) GetRateLimit(ctx context.Context) (*provider.RateLimitInfo, error) {
	if c.initErr != nil {
		return nil, pkgerrors.Wrap(c.initErr, "github client initialization failed")
	}

	if snapshot, ok := c.kernel.RateLimitSnapshot(); ok && time.Now().Before(snapshot.ResetAt) {
		return snapshotToInfo(snapshot), nil
	}

	limits, _, err := c.kernel.RateLimit.Get(ctx)
	if err != nil {
		return nil, err
	}

	core := limits.GetCore()
	if core == nil {
		return zeroRateLimitInfo(), nil
	}

	info := &provider.RateLimitInfo{
		Limit:     core.Limit,
		Remaining: core.Remaining,
		ResetAt:   core.Reset.Time,
	}

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

	createdAt := time.Now().UTC()
	if e.CreatedAt != nil {
		createdAt = e.CreatedAt.Time
	}

	return &provider.Item{
		ID:         id.NewItemID(),
		ExternalID: id.NewExternalID(e.GetID()),
		Source:     id.NewProviderID(providerName),
		Type:       id.NewEventTypeID(e.GetType()),
		Attributes: map[string]string{
			"actor_login":      actorLogin,
			"actor_avatar_url": actorAvatarURL,
			"repo_name":        repoName,
			"repo_url":         repoURL,
		},
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		RawJSON:   rawJSON,
	}, nil
}
