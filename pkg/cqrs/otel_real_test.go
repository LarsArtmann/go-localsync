package cqrs

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/middleware/v4"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkmetricdata "go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// newRealTelemetry builds an OTelBundle backed by the real SDK: a
// ManualReader meter (metrics collectable on demand) and a tracer provider
// with a SpanRecorder (spans inspectable after End). Unlike the noop bundle
// tests, these assertions prove actual instrument names, attribute values,
// and span records.
func newRealTelemetry(t *testing.T) (*middleware.OTelBundle, *sdkmetric.ManualReader, *tracetest.SpanRecorder) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)) //nolint:exhaustruct // reader-only provider
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	recorder := tracetest.NewSpanRecorder()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)) //nolint:exhaustruct // recorder-only provider
	t.Cleanup(func() { _ = tracerProvider.Shutdown(context.Background()) })

	bundle, err := middleware.NewOTelBundle(
		tracerProvider.Tracer("localsync-real-test"),
		provider.Meter("localsync-real-test"),
	)
	testutil.MustNoError(t, err)

	return bundle, reader, recorder
}

// findProjectionDatapoint scans collected "cqrs.operation.count" datapoints
// for one matching the given cqrs.status under operation=projection.
func findProjectionDatapoint(
	t *testing.T,
	reader *sdkmetric.ManualReader,
	status string,
) (int64, bool) {
	t.Helper()

	var data sdkmetricdata.ResourceMetrics
	testutil.MustNoError(t, reader.Collect(context.Background(), &data))

	for _, scope := range data.ScopeMetrics {
		for _, metric := range scope.Metrics {
			if metric.Name != "cqrs.operation.count" {
				continue
			}

			sum, ok := metric.Data.(sdkmetricdata.Sum[int64])
			if !ok {
				continue
			}

			for _, dp := range sum.DataPoints {
				attrs := map[string]string{}
				for _, kv := range dp.Attributes.ToSlice() {
					attrs[string(kv.Key)] = kv.Value.Emit()
				}

				if attrs["operation"] == "projection" && attrs["cqrs.status"] == status {
					return dp.Value, true
				}
			}
		}
	}

	return 0, false
}

// TestOTel_RealMeter_ProjectionInstruments replaces noop-swallowed
// observations with real SDK assertions: the projection recorder increments
// cqrs.operation.count under operation=projection with the expected status
// attribute per callback — including the dead-letter path.
func TestOTel_RealMeter_ProjectionInstruments(t *testing.T) {
	t.Parallel()

	bundle, reader, _ := newRealTelemetry(t)
	recorder := newProjectionMetrics(bundle)

	const projection = "sync-items"
	const eventType = "sync_item.synced"

	recorder.EventProcessed(projection, eventType, 5*time.Millisecond)
	recorder.EventDeadLettered(projection, eventType)

	if v, ok := findProjectionDatapoint(t, reader, "success"); !ok || v < 1 {
		t.Errorf("cqrs.operation.count{operation=projection,status=success} must be >= 1, got %d (found=%v)", v, ok)
	}

	if v, ok := findProjectionDatapoint(t, reader, "dead_lettered"); !ok || v < 1 {
		t.Errorf("cqrs.operation.count{operation=projection,status=dead_lettered} must be >= 1, got %d (found=%v)", v, ok)
	}

	// A second processed event accumulates on the same datapoint (real
	// aggregation, not one-shot instruments).
	recorder.EventProcessed(projection, eventType, 5*time.Millisecond)

	if v, ok := findProjectionDatapoint(t, reader, "success"); !ok || v != 2 {
		t.Errorf("success counter must accumulate to exactly 2, got %d (found=%v)", v, ok)
	}
}

// TestOTel_RealSpan_SyncItems proves the batch span is a real recorded span:
// name, internal kind, and ended status after a sync run through a stack
// wired with the real bundle.
func TestOTel_RealSpan_SyncItems(t *testing.T) {
	t.Parallel()

	bundle, _, spanRecorder := newRealTelemetry(t)

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory", OTel: bundle})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	result := stack.SyncItems(ctx, []*provider.Item{testItem("otel-real-span", "PushEvent")})
	if result.Synced != 1 {
		t.Fatalf("expected Synced=1, got %d", result.Synced)
	}

	waitForCount(t, stack, ctx, 1)

	var syncItemsSpan sdktrace.ReadOnlySpan

	for _, span := range spanRecorder.Ended() {
		if span.Name() == "localsync.sync_items" {
			syncItemsSpan = span

			break
		}
	}

	if syncItemsSpan == nil {
		t.Fatal("localsync.sync_items span was not recorded")

		return
	}

	if syncItemsSpan.SpanKind() != trace.SpanKindInternal {
		t.Errorf("sync_items span kind = %v, want internal", syncItemsSpan.SpanKind())
	}

}
