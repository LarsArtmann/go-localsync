package cqrs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/event/v2"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
)

type syncOutcomeKey struct{}

type SyncOutcome struct {
	WasNew         bool
	EventCount     int
	ConflictWinner ConflictWinner
}

func contextWithSyncOutcome(ctx context.Context, outcome *SyncOutcome) context.Context {
	return context.WithValue(ctx, syncOutcomeKey{}, outcome)
}

func syncOutcomeFromContext(ctx context.Context) *SyncOutcome {
	o, _ := ctx.Value(syncOutcomeKey{}).(*SyncOutcome)

	return o
}

func decideWithOutcome(
	item *model.Item,
	rawJSON []byte,
	resolver crdt.ConflictResolver[*model.Item],
	outcome *SyncOutcome,
	opts ...event.Option,
) func(SyncItemState, event.Version) ([]event.Event, error) {
	return func(state SyncItemState, ver event.Version) ([]event.Event, error) {
		if outcome != nil {
			outcome.WasNew = state.IsNew()
		}

		events, err := decideSync(item, rawJSON, resolver, opts...)(state, ver)
		if err != nil {
			return nil, err
		}

		if outcome != nil {
			outcome.EventCount = len(events)

			if len(events) > 1 {
				var cp ItemConflictFoundPayload
				if err := json.Unmarshal(events[0].Payload(), &cp); err != nil {
					return nil, fmt.Errorf("decode conflict payload: %w", err)
				}

				outcome.ConflictWinner = ConflictWinner(cp.Winner)
			}
		}

		return events, nil
	}
}
