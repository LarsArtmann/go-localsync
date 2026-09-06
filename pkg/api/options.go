package api

import (
	"net/http"

	"charm.land/log/v2"
)

// ServerOption customizes the HTTP API server. Options are additive; the
// zero-value server (no options) behaves exactly as before.
type ServerOption func(*serverOptions)

type serverOptions struct {
	metricsHandler http.Handler
	apiKey         string
	ratePerMinute  int
	bucket         *tokenBucket
	// perClient switches the /sync guard from the single global bucket to one
	// bucket per keyExtractor key (see WithRateLimiter). Unset when the
	// global WithRateLimit scope is active.
	perClient    bool
	keyExtractor func(*http.Request) string
	// logLevel applies to the server's logger (the one passed to NewServer,
	// including the log.Default() fallback). Nil (default) leaves the
	// logger's level untouched.
	logLevel *log.Level
}

// WithMetricsHandler serves the given handler under GET /metrics. Consumers
// with an OTel bundle typically pass a Prometheus reader handler
// (promhttp.Handler() or the otel exporters/prometheus bridge) so the
// cqrs.operation.* instruments — command/event throughput plus projection
// catch-up health — become scrapeable. The SDK stays exporter-agnostic: it
// never imports a metrics backend, it just mounts yours.
//
// Auth posture (decided 2026-09-06): /metrics is deliberately NOT a public
// path — when WithAPIKey is set, scraping requires the key, because metrics
// leak operational detail (source names, item volumes, error rates).
// Consumers scraping through an unauthenticated local sidecar can run a
// second, key-less server on loopback or front /metrics with their own proxy
// that injects the key.
func WithMetricsHandler(handler http.Handler) ServerOption {
	return func(o *serverOptions) {
		o.metricsHandler = handler
	}
}
