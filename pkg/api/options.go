package api

import "net/http"

// ServerOption customizes the HTTP API server. Options are additive; the
// zero-value server (no options) behaves exactly as before.
type ServerOption func(*serverOptions)

type serverOptions struct {
	metricsHandler http.Handler
	apiKey         string
	ratePerMinute  int
	bucket         *tokenBucket
}

// WithMetricsHandler serves the given handler under GET /metrics. Consumers
// with an OTel bundle typically pass a Prometheus reader handler
// (promhttp.Handler() or the otel exporters/prometheus bridge) so the
// cqrs.operation.* instruments — command/event throughput plus projection
// catch-up health — become scrapeable. The SDK stays exporter-agnostic: it
// never imports a metrics backend, it just mounts yours.
func WithMetricsHandler(handler http.Handler) ServerOption {
	return func(o *serverOptions) {
		o.metricsHandler = handler
	}
}
