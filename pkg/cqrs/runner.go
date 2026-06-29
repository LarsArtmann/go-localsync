package cqrs

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/event/v3"
	"github.com/larsartmann/go-cqrs-lite/projection/v3"
)

// startProjectionRunner wires the read-model projector to the event bus and,
// for persistent backends, replays historical events from the journal.
//
// Delivery model:
//   - Live events are delivered synchronously by the watermill EventBus
//     (BlockPublishUntilSubscriberAck), preserving read-your-writes after each
//     repo.Execute.
//   - Historical replay runs in a background goroutine. The projection is
//     idempotent (Upsert/Delete keyed by source+source_id), so overlap between
//     the replay tail and live delivery is harmless and needs no checkpoint or
//     dedup set.
//
// go-cqrs-lite v3.2 re-introduced the projection/ module (ADR-0037), moving
// the Projection interface from event/ to projection/. This in-process replay
// still avoids the full stack.Materialize + watermill.CatchUpSubscriber
// machinery, which would require re-encoding go-localsync's custom ReadModel
// into a kv.TypedStore.
func startProjectionRunner(
	sr storeResult,
	proj projection.Projection,
) (context.CancelFunc, error) {
	if subErr := sr.bus.SubscribeAll(proj.Handle); subErr != nil {
		return nil, fmt.Errorf("subscribe projection: %w", subErr)
	}

	if sr.loader == nil {
		return func() {}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	go replayJournal(ctx, sr.loader, proj)

	return cancel, nil
}

// replayJournal loads every persisted event and forwards the ones the
// projection cares about to proj.Handle. Errors are logged, not fatal: a single
// bad event must not block catch-up of the rest.
func replayJournal(ctx context.Context, loader event.Journal, proj projection.Projection) {
	events, err := loader.ReadAll(ctx)
	if err != nil {
		log.Error("projection journal replay failed", "error", err)

		return
	}

	wanted := make(map[event.Type]struct{}, len(proj.EventTypes()))
	for _, t := range proj.EventTypes() {
		wanted[t] = struct{}{}
	}

	for _, evt := range events {
		if ctx.Err() != nil {
			return
		}

		if _, ok := wanted[evt.Type()]; !ok {
			continue
		}

		if err := proj.Handle(ctx, evt); err != nil {
			log.Error("projection replay handler failed", "eventID", evt.ID(), "error", err)
		}
	}
}
