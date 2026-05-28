package api

import (
	"context"
	"net/http"
	"time"

	"charm.land/log/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
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

	s := &Server{
		api:    api,
		mux:    mux,
		syncer: syncer,
		store:  store,
		logger: logger,
	}

	s.registerRoutes()

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	huma.Register(s.api, huma.Operation{
		OperationID: "list-items",
		Method:      http.MethodGet,
		Path:        "/items",
		Summary:     "List synced items",
		Tags:        []string{"Items"},
	}, s.listItems)

	huma.Register(s.api, huma.Operation{
		OperationID: "get-stats",
		Method:      http.MethodGet,
		Path:        "/stats",
		Summary:     "Get sync statistics",
		Tags:        []string{"Stats"},
	}, s.getStats)

	huma.Register(s.api, huma.Operation{
		OperationID: "trigger-sync",
		Method:      http.MethodPost,
		Path:        "/sync",
		Summary:     "Trigger a sync operation",
		Tags:        []string{"Sync"},
	}, s.triggerSync)

	huma.Register(s.api, huma.Operation{
		OperationID: "health-check",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
		Tags:        []string{"Health"},
	}, s.healthCheck)
}

// ListItemsInput defines the query parameters for listing items.
type ListItemsInput struct {
	Type       string    `query:"type" doc:"Filter by event type" example:"PushEvent"`
	ActorLogin string    `query:"actor" doc:"Filter by actor login" example:"larsartmann"`
	RepoName   string    `query:"repo" doc:"Filter by repository name" example:"larsartmann/go-localsync"`
	Source     string    `query:"source" doc:"Filter by source provider" example:"github"`
	Since      time.Time `query:"since" doc:"Filter items updated since this time (RFC3339)"`
	Limit      int       `query:"limit" doc:"Maximum items to return" default:"100"`
	Offset     int       `query:"offset" doc:"Offset for pagination" default:"0"`
}

// ListItemsOutput defines the response for listing items.
type ListItemsOutput struct {
	Body struct {
		Items []*provider.Item `json:"items" doc:"List of sync items"`
		Total int64            `json:"total" doc:"Total number of items matching the filter"`
	}
}

func (s *Server) listItems(ctx context.Context, input *ListItemsInput) (*ListItemsOutput, error) {
	filter := provider.ItemFilter{
		Type:       nil,
		ActorLogin: nil,
		RepoName:   nil,
		Source:     nil,
		Since:      nil,
		Limit:      input.Limit,
		Offset:     input.Offset,
	}

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

	resp := &ListItemsOutput{}
	resp.Body.Items = items
	resp.Body.Total = total

	return resp, nil
}

// StatsOutput defines the response for statistics.
type StatsOutput struct {
	Body struct {
		TotalItems int64    `json:"totalItems" doc:"Total number of synced items"`
		ItemTypes  []string `json:"itemTypes" doc:"List of distinct item types"`
	}
}

func (s *Server) getStats(ctx context.Context, _ *struct{}) (*StatsOutput, error) {
	count, err := s.store.CountItems(ctx, provider.ItemFilter{})
	if err != nil {
		return nil, err
	}

	types, err := s.store.GetItemTypes(ctx)
	if err != nil {
		return nil, err
	}

	resp := &StatsOutput{}
	resp.Body.TotalItems = count
	resp.Body.ItemTypes = types

	return resp, nil
}

// SyncInput defines the request body for triggering a sync.
type SyncInput struct {
	Body struct {
		Source   string `json:"source" doc:"Source to sync" example:"larsartmann"`
		MaxPages int    `json:"maxPages" doc:"Maximum pages to fetch" default:"0"`
	}
}

// SyncOutput defines the response for a sync operation.
type SyncOutput struct {
	Body struct {
		Fetched int `json:"fetched" doc:"Number of items fetched"`
		Skipped int `json:"skipped" doc:"Number of items skipped"`
		Errors  int `json:"errors" doc:"Number of errors"`
	}
}

func (s *Server) triggerSync(ctx context.Context, input *SyncInput) (*SyncOutput, error) {
	opts := &synclib.SyncOptions{
		Source:   input.Body.Source,
		MaxPages: input.Body.MaxPages,
	}

	if err := opts.Validate(); err != nil {
		return nil, huma.Error400BadRequest("invalid sync options", err)
	}

	result, err := s.syncer.Sync(ctx, opts)
	if err != nil {
		return nil, err
	}

	resp := &SyncOutput{}
	resp.Body.Fetched = result.Fetched
	resp.Body.Skipped = result.Skipped
	resp.Body.Errors = result.Errors

	return resp, nil
}

// HealthOutput defines the response for the health check.
type HealthOutput struct {
	Body struct {
		Status string `json:"status" doc:"Health status"`
	}
}

func (s *Server) healthCheck(_ context.Context, _ *struct{}) (*HealthOutput, error) {
	resp := &HealthOutput{}
	resp.Body.Status = "healthy"

	return resp, nil
}
