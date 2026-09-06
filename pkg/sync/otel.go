package sync

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// WithTracer enables an OpenTelemetry span around each sync run: full syncs
// open "localsync.sync", incremental runs open "localsync.sync_incremental".
// The CQRS batch path already spans "localsync.sync_items" — wiring the same
// tracer here puts the fetch/validate/reconcile phases of a run under the
// batch span in one trace. Nil (default) disables tracing with zero overhead;
// consumers with an OTel bundle pass the bundle's tracer (CQRSStack.OTel()
// exposes it) so both layers share one TracerProvider.
func WithTracer(tracer trace.Tracer) Option {
	return func(s *Syncer) { s.tracer = tracer }
}

// withSyncSpan opens name around run when a tracer is configured, recording
// the terminal error on the span. It is the single tracing wrapper for the
// sync entry points so both paths record identical status semantics.
func (s *Syncer) withSyncSpan(
	ctx context.Context,
	name string,
	run func(ctx context.Context) (*SyncResult, error),
) (*SyncResult, error) {
	if s.tracer == nil {
		return run(ctx)
	}

	spanCtx, span := s.tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	result, err := run(spanCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return result, err
}
