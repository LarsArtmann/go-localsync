package github

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/event"
	"golang.org/x/oauth2"
)

// Fetcher defines the interface for fetching GitHub events.
// This interface enables testing with mocks and decouples the sync logic from the concrete client.
type Fetcher interface {
	FetchEvents(ctx context.Context, username string, opts *FetchOptions) ([]*event.Event, error)
	FetchAllEvents(ctx context.Context, username string, maxPages int) ([]*event.Event, error)
	GetRateLimit(ctx context.Context) (*gh.RateLimits, *gh.Response, error)
}

type Client struct {
	client *gh.Client
}

func NewClient(token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		client: gh.NewClient(tc),
	}
}

func NewClientWithHTTP(client *http.Client) *Client {
	return &Client{
		client: gh.NewClient(client),
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

	activity, _, err := c.client.Activity.ListEventsPerformedByUser(ctx, username, false, &gh.ListOptions{
		Page:    opts.Page,
		PerPage: opts.PerPage,
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
