package cqrs

import (
	"encoding/json"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

const aggregateType = "sync_item"

// DecideSync returns a DecideFunc that syncs an incoming provider.Item
// against the current aggregate state. It produces:
//   - 0 events if the item is unchanged
//   - 1 ItemSynced event if the item is new or updated
//   - 1 ItemConflictFound + 1 ItemSynced if a conflict was resolved
func DecideSync(item *provider.Item) func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		if state.Deleted {
			return newSyncEvents(item, currentVersion, false), nil
		}

		if state.IsNew() {
			return newSyncEvents(item, currentVersion, false), nil
		}

		if !state.hasChanged(item) {
			return nil, nil
		}

		return newSyncEvents(item, currentVersion, true), nil
	}
}

// DecideDelete returns a DecideFunc that marks an item as deleted.
func DecideDelete() func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		if state.Deleted || state.IsNew() {
			return nil, nil
		}

		aggID := aggregateID(state.Source, state.SourceID)

		evt, err := newEvent(EventItemDeleted, aggID, int(currentVersion)+1, ItemDeletedPayload{
			Source:   state.Source,
			SourceID: state.SourceID,
		})
		if err != nil {
			return nil, err
		}

		return []event.Event{evt}, nil
	}
}

func newSyncEvents(item *provider.Item, currentVersion event.Version, isConflict bool) []event.Event {
	aggID := aggregateID(item.Source.Get(), item.ExternalID.Get())
	events := make([]event.Event, 0, 2)
	ver := int(currentVersion)

	if isConflict {
		conflictEvt, err := newEvent(EventItemConflictFound, aggID, ver+1, ItemConflictFoundPayload{
			Source:          item.Source.Get(),
			SourceID:        item.ExternalID.Get(),
			LocalUpdatedAt:  unixNano(item.UpdatedAt),
			RemoteUpdatedAt: unixNano(item.UpdatedAt),
			Winner:          "remote",
		})
		if err == nil {
			events = append(events, conflictEvt)
		}
	}

	syncEvt, err := newEvent(EventItemSynced, aggID, ver+len(events)+1, itemToPayload(item))
	if err == nil {
		events = append(events, syncEvt)
	}

	return events
}

func newEvent(eventType event.Type, aggID id.AggregateID, version int, payload any) (*event.Core, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload for %s: %w", eventType, err)
	}

	return event.NewEvent(eventType, aggID, aggregateType, version, data)
}

// aggregateID creates an AggregateID for a sync item.
// Each call generates a new ULID — the caller should cache this per (source, sourceID) pair.
func aggregateID(source, sourceID string) id.AggregateID {
	return id.NewAggregateID()
}
