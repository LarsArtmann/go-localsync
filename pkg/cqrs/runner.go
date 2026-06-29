package cqrs

import (
	"context"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/projection/v3"
	"github.com/larsartmann/go-cqrs-lite/projectionhost/v3"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// startProjectionRunner wires the read-model projector to the event bus for
// live delivery and to a projectionhost.Host for resilient catch-up.
//
// Delivery model (see ADR-0006):
//   - Live events are delivered synchronously by the watermill EventBus
//     (BlockPublishUntilSubscriberAck), preserving read-your-writes after each
//     repo.Execute.
//   - Catch-up runs in a projectionhost.Host worker: it drains the
//     SeekableJournal from the last checkpoint (bounded replay), auto-restarts
//     on crash with backoff, and captures poison messages to a dead-letter
//     queue so a single bad event can never block catch-up.
//
// The projection is idempotent (Upsert/Delete keyed by source+source_id), so
// overlap between the catch-up tail and live delivery is harmless. The
// checkpoint is an optimization + resilience boundary, not a correctness
// requirement — it bounds replay work and survives failure.
func startProjectionRunner(
	sr storeResult,
	proj projection.Projection,
) (context.CancelFunc, <-chan struct{}, error) {
	if subErr := sr.bus.SubscribeAll(proj.Handle); subErr != nil {
		return nil, nil, pkgerrors.Wrap(subErr, "subscribe projection")
	}

	host, err := projectionhost.New(
		sr.journal, sr.cpStore,
		projectionhost.WithLogger(newSlogLogger()),
		// Capture events that fail to project more than 3 times so a single
		// poison message can never permanently block catch-up. The capture is
		// logged via the host logger; the checkpoint then advances past it.
		projectionhost.WithDeadLetterStore(projectionhost.NewMemoryDeadLetterStore(), 3),
	)
	if err != nil {
		return nil, nil, pkgerrors.Wrap(err, "create projection host")
	}

	if regErr := host.Register(proj); regErr != nil {
		return nil, nil, pkgerrors.Wrap(regErr, "register projection with host")
	}

	ctx, cancel := context.WithCancel(context.Background())

	if startErr := host.Start(ctx); startErr != nil {
		cancel()

		return nil, nil, pkgerrors.Wrap(startErr, "start projection host")
	}

	drainDone := make(chan struct{})
	go drainHostOnCancel(ctx, host, drainDone)

	return cancel, drainDone, nil
}

// drainHostOnCancel waits for the runner context to be cancelled (Close or
// shutdown) and then gracefully drains the projection host, waiting up to 30s
// for in-flight events to finish. Errors are logged, not fatal — a slow drain
// must not block stack Close beyond the host's own timeout. drainDone is closed
// on exit so Close can wait for the drain to finish before closing the db.
func drainHostOnCancel(ctx context.Context, host *projectionhost.Host, drainDone chan<- struct{}) {
	<-ctx.Done()

	if err := host.Stop(); err != nil {
		log.Error("projection host graceful drain failed", "error", err)
	}

	close(drainDone)
}
