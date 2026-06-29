package cqrs

import (
	"encoding/json"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/event/v3"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// SyncOutcome captures what the decider decided for a single item sync, so the
// caller (SyncItems) can classify the result without re-inspecting the emitted
// events. It is carried on SyncItemCommand.outcome (set by SyncItems, read by
// the command handler) rather than smuggled through context.Value.
type SyncOutcome struct {
	WasNew           bool
	ConflictDetected bool
	ConflictWinner   ConflictWinner
	EventCount       int
}

// decideWithOutcome wraps decideSync, recording what happened into outcome.
// Conflict is detected by the presence of an EventItemConflictFound event — the
// decider's explicit signal — not by counting events, so a future non-conflict
// event added to the sync path can never be misread as a conflict.
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
			outcome.ConflictDetected = false
		}

		events, err := decideSync(item, rawJSON, resolver, opts...)(state, ver)
		if err != nil {
			return nil, err
		}

		if outcome != nil {
			outcome.EventCount = len(events)

			if hasConflictEvent(events) {
				outcome.ConflictDetected = true

				var cp ItemConflictFoundPayload
				if err := json.Unmarshal(events[0].Payload(), &cp); err != nil {
					return nil, fmt.Errorf("decode conflict payload: %w", err)
				}

				outcome.ConflictWinner = ParseConflictWinner(cp.Winner)
			}
		}

		return events, nil
	}
}

// hasConflictEvent reports whether the decider emitted an EventItemConflictFound.
// syncEvents always orders it before the ItemSynced event, so checking the first
// event's type is sufficient and unambiguous.
func hasConflictEvent(events []event.Event) bool {
	return len(events) > 0 && events[0].Type() == EventItemConflictFound
}
