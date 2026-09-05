package cqrs

import (
	"context"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/middleware/v4"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

func newNoopOTelBundle(t *testing.T) *middleware.OTelBundle {
	t.Helper()

	bundle, err := middleware.NewOTelBundle(
		trace.NewNoopTracerProvider().Tracer("localsync-test"),
		noopmetric.NewMeterProvider().Meter("localsync-test"),
	)
	testutil.MustNoError(t, err)

	return bundle
}

// TestCQRSStack_OTel_NoopBundle_Works verifies the opt-in telemetry surface:
// a stack constructed with a bundle wires its command/event middleware and
// the projection metrics adapter, while sync behavior is unchanged.
func TestCQRSStack_OTel_NoopBundle_Works(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory", OTel: newNoopOTelBundle(t)})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	if stack.OTel() == nil {
		t.Fatal("OTel() must return the configured bundle")
	}

	ctx := context.Background()
	result := stack.SyncItems(ctx, testItems("otel-1", "PushEvent", "otel-2", "IssueEvent"))
	if result.Synced != 2 {
		t.Errorf("expected Synced=2 with otel enabled, got %d", result.Synced)
	}

	waitForCount(t, stack, ctx, 2)
}

// TestCQRSStack_OTel_Nil_LeavesTelemetryOff pins the default: no bundle, no
// telemetry, identical sync results.
func TestCQRSStack_OTel_Nil_LeavesTelemetryOff(t *testing.T) {
	t.Parallel()

	stack, err := NewCQRSStack(CQRSConfig{Backend: "memory"})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	if stack.OTel() != nil {
		t.Fatal("OTel() must be nil when no bundle was configured")
	}

	ctx := context.Background()
	result := stack.SyncItems(ctx, []*provider.Item{testItem("otel-off", "PushEvent")})
	if result.Synced != 1 {
		t.Errorf("expected Synced=1 with otel off, got %d", result.Synced)
	}

	waitForCount(t, stack, ctx, 1)
}

// TestProjectionMetrics_Interface pins the adapter contract: projectionMetrics
// satisfies projectionhost.MetricsRecorder and every callback is safe to call
// (noop instruments swallow observations).
func TestProjectionMetrics_Interface(t *testing.T) {
	t.Parallel()

	metrics := newProjectionMetrics(newNoopOTelBundle(t))

	metrics.EventProcessed("sync-items", "sync_item.synced", 1000)
	metrics.EventErrored("sync-items", "sync_item.synced")
	metrics.EventDeadLettered("sync-items", "sync_item.synced")
	metrics.WorkerRestarted("sync-items")
	metrics.WorkerFailed("sync-items")
	metrics.CheckpointAdvanced("sync-items", 2500)
}
