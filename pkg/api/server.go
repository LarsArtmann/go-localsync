package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"charm.land/log/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// Server exposes the sync system via HTTP using Huma + net/http.
type Server struct {
	api    huma.API
	mux    *http.ServeMux
	syncer *synclib.Syncer
	store  synclib.SyncStore
	logger *log.Logger
}

// NewServer creates an HTTP API server backed by the given syncer and store.
func NewServer(syncer *synclib.Syncer, store synclib.SyncStore, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}

	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("Go-LocalSync API", "1.0.0")
	api := humago.New(mux, cfg)

	srv := &Server{
		api:    api,
		mux:    mux,
		syncer: syncer,
		store:  store,
		logger: logger,
	}

	srv.registerRoutes()

	return srv
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	register(
		s.api,
		"list-items",
		http.MethodGet,
		"/items",
		"List synced items",
		[]string{"Items"},
		s.listItems,
	)
	register(
		s.api,
		"get-stats",
		http.MethodGet,
		"/stats",
		"Get sync statistics",
		[]string{"Stats"},
		s.getStats,
	)
	register(
		s.api,
		"trigger-sync",
		http.MethodPost,
		"/sync",
		"Trigger a sync operation",
		[]string{"Sync"},
		s.triggerSync,
	)
	register(
		s.api,
		"health-check",
		http.MethodGet,
		"/health",
		"Health check",
		[]string{"Health"},
		s.healthCheck,
	)
}

//nolint:exhaustruct
func register[I, O any](
	api huma.API,
	opID, method, path, summary string,
	tags []string,
	handler func(context.Context, *I) (*O, error),
) {
	huma.Register(api, huma.Operation{
		OperationID: opID,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Tags:        tags,
	}, handler)
}

// ListItemsInput defines the query parameters for listing items.
//
//nolint:tagalign
type ListItemsInput struct {
	Type       string    `doc:"Filter by event type"                           example:"PushEvent"                query:"type"`
	ActorLogin string    `doc:"Filter by actor login"                          example:"larsartmann"              query:"actor"`
	RepoName   string    `doc:"Filter by repository name"                      example:"larsartmann/go-localsync" query:"repo"`
	Source     string    `doc:"Filter by source provider"                      example:"github"                   query:"source"`
	Since      time.Time `doc:"Filter items updated since this time (RFC3339)"                                    query:"since"`
	Limit      int       `doc:"Maximum items to return"                                                           query:"limit"  default:"100"`
	Offset     int       `doc:"Offset for pagination"                                                             query:"offset" default:"0"`
}

// ItemResponse is the API DTO for a synced item.
type ItemResponse struct {
	ID             string    `json:"id"`
	ExternalID     string    `json:"externalId"`
	Source         string    `json:"source"`
	Type           string    `json:"type"`
	ActorLogin     string    `json:"actorLogin"`
	ActorAvatarURL string    `json:"actorAvatarUrl,omitempty"`
	RepoName       string    `json:"repoName"`
	RepoURL        string    `json:"repoUrl,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func toItemResponse(item *model.Item) *ItemResponse {
	if item == nil {
		return nil
	}

	return &ItemResponse{
		ID:             item.ID.String(),
		ExternalID:     item.ExternalID.Get(),
		Source:         item.Source.Get(),
		Type:           item.Type.Get(),
		ActorLogin:     item.ActorLogin.Get(),
		ActorAvatarURL: item.ActorAvatarURL,
		RepoName:       item.RepoName.Get(),
		RepoURL:        item.RepoURL,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

// ListItemsOutput defines the response for listing items.
type ListItemsOutput struct {
	Body struct {
		Items []*ItemResponse `doc:"List of sync items"                        json:"items"`
		Total int64           `doc:"Total number of items matching the filter" json:"total"`
	}
}

func (s *Server) listItems(ctx context.Context, input *ListItemsInput) (*ListItemsOutput, error) {
	var filter provider.ItemFilter

	filter.Limit = input.Limit
	filter.Offset = input.Offset

	if input.Type != "" {
		t := id.NewEventTypeID(input.Type)
		filter.Type = &t
	}

	if input.ActorLogin != "" {
		a := id.NewActorID(input.ActorLogin)
		filter.ActorLogin = &a
	}

	if input.RepoName != "" {
		r := id.NewRepoID(input.RepoName)
		filter.RepoName = &r
	}

	if input.Source != "" {
		src := id.NewProviderID(input.Source)
		filter.Source = &src
	}

	if !input.Since.IsZero() {
		filter.Since = &input.Since
	}

	items, err := s.store.ListItems(ctx, filter)
	if err != nil {
		return nil, err
	}

	total, err := s.store.CountItems(ctx, filter)
	if err != nil {
		return nil, err
	}

	var resp ListItemsOutput

	resp.Body.Items = make([]*ItemResponse, len(items))
	for i, item := range items {
		resp.Body.Items[i] = toItemResponse(item)
	}
	resp.Body.Total = total

	return &resp, nil
}

// StatsOutput defines the response for statistics.
type StatsOutput struct {
	Body struct {
		TotalItems int64    `doc:"Total number of synced items" json:"totalItems"`
		ItemTypes  []string `doc:"List of distinct item types"  json:"itemTypes"`
	}
}

func (s *Server) getStats(ctx context.Context, _ *struct{}) (*StatsOutput, error) {
	var filter provider.ItemFilter

	count, err := s.store.CountItems(ctx, filter)
	if err != nil {
		return nil, err
	}

	types, err := s.store.GetItemTypes(ctx)
	if err != nil {
		return nil, err
	}

	var resp StatsOutput

	resp.Body.TotalItems = count
	resp.Body.ItemTypes = types

	return &resp, nil
}

// SyncInput defines the request body for triggering a sync.
type SyncInput struct {
	Body struct {
		Source   string `doc:"Source to sync" example:"larsartmann"        json:"source"`
		MaxPages int    `default:"0"          doc:"Maximum pages to fetch" json:"maxPages"`
	}
}

// SyncOutput defines the response for a sync operation.
type SyncOutput struct {
	Body struct {
		Fetched int `doc:"Number of items fetched" json:"fetched"`
		Skipped int `doc:"Number of items skipped" json:"skipped"`
		Errors  int `doc:"Number of errors"        json:"errors"`
	}
}

func (s *Server) triggerSync(ctx context.Context, input *SyncInput) (*SyncOutput, error) {
	var opts synclib.SyncOptions

	opts.Source = input.Body.Source
	opts.MaxPages = input.Body.MaxPages

	err := opts.Validate()
	if err != nil {
		return nil, huma.Error400BadRequest("invalid sync options", err)
	}

	result, err := s.syncer.Sync(ctx, &opts)
	if err != nil {
		return nil, mapSyncError(err)
	}

	var resp SyncOutput

	resp.Body.Fetched = result.Fetched
	resp.Body.Skipped = result.Skipped
	resp.Body.Errors = result.Errors

	return &resp, nil
}

// HealthOutput defines the response for the health check.
type HealthOutput struct {
	Body struct {
		Status string `doc:"Health status" json:"status"`
	}
}

func (s *Server) healthCheck(_ context.Context, _ *struct{}) (*HealthOutput, error) {
	var resp HealthOutput

	resp.Body.Status = "healthy"

	return &resp, nil
}

func mapSyncError(err error) error {
	switch {
	case errors.Is(err, pkgerrors.ErrRateLimited):
		return huma.Error429TooManyRequests("rate limited", err)
	case errors.Is(err, pkgerrors.ErrInvalidToken):
		return huma.Error401Unauthorized("invalid token", err)
	case errors.Is(err, pkgerrors.ErrUserNotFound):
		return huma.Error404NotFound("user not found", err)
	case errors.Is(err, pkgerrors.ErrDatabase):
		return huma.Error500InternalServerError("database error", err)
	case errors.Is(err, pkgerrors.ErrInvalidInput):
		return huma.Error400BadRequest("invalid input", err)
	default:
		return huma.Error503ServiceUnavailable("sync failed", err)
	}
}
