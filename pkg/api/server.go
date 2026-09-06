package api

import (
	"context"
	"net/http"
	"sync"

	"charm.land/log/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// Server exposes the sync system via HTTP using Huma + net/http.
type Server struct {
	api    huma.API
	mux    *http.ServeMux
	syncer *synclib.Syncer
	store  synclib.SyncStore
	logger *log.Logger
	opts   *serverOptions

	clientMu      sync.Mutex
	clientBuckets map[string]*tokenBucket
}

// NewServer creates an HTTP API server backed by the given syncer.
func NewServer(syncer *synclib.Syncer, logger *log.Logger, opts ...ServerOption) *Server {
	// Populate the user-facing error templates so HandleErrorDetailed can render
	// What/Why/Fix/WayOut for any error surfaced at this boundary. Idempotent.
	pkgerrors.RegisterErrorTemplates()

	if logger == nil {
		logger = log.Default()
	}

	options := &serverOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if options.ratePerMinute > 0 {
		options.bucket = newTokenBucket(options.ratePerMinute)
	}

	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("Go-LocalSync API", "1.0.0")
	api := humago.New(mux, cfg)

	srv := &Server{
		api:           api,
		mux:           mux,
		syncer:        syncer,
		store:         syncer.Store(),
		logger:        logger,
		opts:          options,
		clientBuckets: map[string]*tokenBucket{},
	}

	srv.registerRoutes()

	if options.apiKey != "" {
		registerSecurityScheme(api)
	}

	return srv
}

// ServeHTTP implements http.Handler. The mux is wrapped by the auth
// middleware when WithAPIKey is set; /health and the OpenAPI docs stay public.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.rateLimited(s.authenticated(s.mux)).ServeHTTP(w, r)
}

// errorStatuses returns the non-2xx statuses an operation can produce given
// the enabled options, so the OpenAPI document carries a truthful error
// schema per endpoint (consumers can generate matching error handling).
func (s *Server) errorStatuses(withAuth, withRateLimit bool) []int {
	statuses := []int{http.StatusInternalServerError}
	if withAuth {
		statuses = append(statuses, http.StatusUnauthorized)
	}

	if withRateLimit {
		statuses = append(statuses, http.StatusTooManyRequests)
	}

	return statuses
}

func (s *Server) registerRoutes() {
	authEnabled := s.opts != nil && s.opts.apiKey != ""
	rateLimited := s.opts != nil && s.opts.bucket != nil

	registerWithErrors(
		s.api,
		"list-items",
		http.MethodGet,
		"/items",
		"List synced items",
		[]string{"Items"},
		append([]int{http.StatusBadRequest}, s.errorStatuses(authEnabled, false)...),
		s.listItems,
	)
	registerWithErrors(
		s.api,
		"get-stats",
		http.MethodGet,
		"/stats",
		"Get sync statistics",
		[]string{"Stats"},
		s.errorStatuses(authEnabled, false),
		s.getStats,
	)
	registerWithErrors(
		s.api,
		"trigger-sync",
		http.MethodPost,
		"/sync",
		"Trigger a sync operation",
		[]string{"Sync"},
		// The declared timeout statuses match pkgerrors.HTTPStatus, the single
		// error→status translator: a canceled request surfaces 499 (client
		// closed), a deadline 504 (gateway timeout). 408 is never produced by
		// that mapping (verified 2026-09-06; previously mis-declared).
		append(
			[]int{
				http.StatusBadRequest,
				pkgerrors.StatusClientClosedRequest,
				http.StatusGatewayTimeout,
			},
			s.errorStatuses(authEnabled, rateLimited)...,
		),
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

	if s.opts != nil && s.opts.metricsHandler != nil {
		s.mux.Handle("/metrics", s.opts.metricsHandler)
	}
}

func register[I, O any](
	api huma.API,
	opID, method, path, summary string,
	tags []string,
	handler func(context.Context, *I) (*O, error),
) {
	registerWithErrors(api, opID, method, path, summary, tags, nil, handler)
}

func registerWithErrors[I, O any](
	api huma.API,
	opID, method, path, summary string,
	tags []string,
	errorStatuses []int,
	handler func(context.Context, *I) (*O, error),
) {
	huma.Register(api, huma.Operation{
		OperationID: opID,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Tags:        tags,
		Errors:      errorStatuses,
	}, handler)
}
