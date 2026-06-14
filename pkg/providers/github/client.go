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
)

const providerName = "github"

// Client implements provider.Provider for GitHub.
type Client struct {
	client          *gh.Client
	rateLimitConfig provider.RateLimitConfig
	retryConfig     provider.RetryConfig
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
		err      error
	)

	err = c.withRetry(ctx, func() error {
		activity, _, err = c.client.Activity.ListEventsPerformedByUser(
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

	items := convertEvents(activity)

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
			Source:  id.NewProviderID(source),
			PerPage: 100,
			Page:    page,
		})
		if err != nil {
			return nil, pkgerrors.Wrapf(
				err,
				"fetch page %d/%d for %s failed (fetched %d items)",
				page,
				maxPages,
				source,
				len(allItems),
			)
		}

		if len(result.Items) == 0 {
			break
		}

		allItems = append(allItems, result.Items...)
		if !result.HasMore {
			break
		}
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
