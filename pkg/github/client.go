package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/storage"
	"golang.org/x/oauth2"
)

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

func (c *Client) FetchEvents(ctx context.Context, username string, opts *FetchOptions) ([]*storage.Event, error) {
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
		return nil, fmt.Errorf("fetching events for user %s: %w", username, err)
	}

	events := make([]*storage.Event, 0, len(activity))
	for _, e := range activity {
		event, err := convertEvent(e)
		if err != nil {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

func (c *Client) FetchAllEvents(ctx context.Context, username string, maxPages int) ([]*storage.Event, error) {
	if maxPages == 0 {
		maxPages = 10
	}

	var allEvents []*storage.Event
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

func convertEvent(e *gh.Event) (*storage.Event, error) {
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

	return &storage.Event{
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
	return c.client.RateLimits(ctx)
}
