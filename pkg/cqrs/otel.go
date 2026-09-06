package cqrs

import (
	"context"
	"time"

	"github.com/larsartmann/go-cqrs-lite/middleware/v4"
	cqrsotel "github.com/larsartmann/go-cqrs-lite/otel/v4"
	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// withBatchSpan opens the localsync.sync_items span around a batch run when
// telemetry is enabled and injects the span context into run.
func withBatchSpan(
	ctx context.Context,
	bundle *middleware.OTelBundle,
	run func(context.Context) *synclib.BatchOutcome,
) *synclib.BatchOutcome {
	ctx, span := cqrsotel.StartSpan(
		ctx, bundle.Tracer(), "localsync.sync_items", cqrsotel.SpanKindInternal,
	)
	defer span.End()

	return run(ctx)
}

// projectionMetrics adapts a CQRSConfig.OTel bundle's metric instruments to
// the projectionhost.MetricsRecorder interface, so projection catch-up health
// (event throughput, errors, dead-letters, worker restarts, checkpoint lag)
// flows into the same cqrs.operation.* instruments as command and event
// middleware — one metrics pipeline, operation="projection".
type projectionMetrics struct {
	histogram cqrsotel.Float64Histogram
	counter   cqrsotel.Int64Counter
}

var _ projectionhost.MetricsRecorder = (*projectionMetrics)(nil)

func newProjectionMetrics(bundle *middleware.OTelBundle) *projectionMetrics {
	recorder := bundle.Recorder()

	return &projectionMetrics{
		histogram: recorder.Histogram(),
		counter:   recorder.Counter(),
	}
}

func projectionAttrs(projectionName, eventType, status string) []cqrsotel.KeyValue {
	attrs := []cqrsotel.KeyValue{
		cqrsotel.AttrString("operation", "projection"),
		cqrsotel.AttrString("projection", projectionName),
		cqrsotel.AttrString(cqrsotel.AttrStatus, status),
	}
	if eventType != "" {
		attrs = append(attrs, cqrsotel.AttrString("event_type", eventType))
	}

	return attrs
}

func (m *projectionMetrics) EventProcessed(projectionName, eventType string, duration time.Duration) {
	attrs := projectionAttrs(projectionName, eventType, cqrsotel.StatusSuccess)
	ctx := context.Background()

	m.histogram.Record(ctx, float64(duration.Milliseconds()), cqrsotel.MetricWithAttributes(attrs...))
	m.counter.Add(ctx, 1, cqrsotel.CounterAddWithAttributes(attrs...))
}

func (m *projectionMetrics) EventErrored(projectionName, eventType string) {
	m.counter.Add(
		context.Background(), 1,
		cqrsotel.CounterAddWithAttributes(projectionAttrs(projectionName, eventType, cqrsotel.StatusError)...),
	)
}

func (m *projectionMetrics) EventDeadLettered(projectionName, eventType string) {
	m.counter.Add(
		context.Background(), 1,
		cqrsotel.CounterAddWithAttributes(projectionAttrs(projectionName, eventType, "dead_lettered")...),
	)
}

func (m *projectionMetrics) WorkerRestarted(projectionName string) {
	m.counter.Add(
		context.Background(), 1,
		cqrsotel.CounterAddWithAttributes(projectionAttrs(projectionName, "", "worker_restarted")...),
	)
}

func (m *projectionMetrics) WorkerFailed(projectionName string) {
	m.counter.Add(
		context.Background(), 1,
		cqrsotel.CounterAddWithAttributes(projectionAttrs(projectionName, "", "worker_failed")...),
	)
}

func (m *projectionMetrics) CheckpointAdvanced(projectionName string, lag time.Duration) {
	attrs := append(
		projectionAttrs(projectionName, "", cqrsotel.StatusSuccess),
		cqrsotel.AttrInt64("checkpoint_lag_ms", lag.Milliseconds()),
	)
	m.counter.Add(
		context.Background(), 1,
		cqrsotel.CounterAddWithAttributes(attrs...),
	)
}
