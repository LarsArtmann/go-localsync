package cqrs

import (
	"encoding/json"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/event"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// SyncItemState is the aggregate state for a single sync item, reconstructed from events.
// It wraps a *provider.Item (nil when no events applied) plus a Deleted flag.
type SyncItemState struct {
	Item    *provider.Item
	Deleted bool
}

// InitialState is the zero state for a new SyncItem aggregate.
//
//nolint:gochecknoglobals
var InitialState = SyncItemState{
	Item:    nil,
	Deleted: false,
}

// IsNew returns true if no events have been applied (Item is nil).
func (s SyncItemState) IsNew() bool {
	return s.Item == nil
}

// Fold applies a single event to the SyncItemState, returning the new state.
func Fold(state SyncItemState, evt event.Event) (SyncItemState, error) {
	switch evt.Type() {
	case EventItemSynced:
		return foldItemSynced(evt)
	case EventItemConflictFound:
		return state, nil
	case EventItemDeleted:
		state.Deleted = true

		return state, nil
	default:
		//nolint:err113 // dynamic error for unknown event type
		return state, fmt.Errorf(
			"unknown event type: %s",
			evt.Type(),
		)
	}
}

func foldItemSynced(evt event.Event) (SyncItemState, error) {
	var payload ItemSyncedPayload

	err := json.Unmarshal(evt.Payload(), &payload)
	if err != nil {
		return SyncItemState{}, fmt.Errorf("unmarshal ItemSyncedPayload: %w", err)
	}

	return SyncItemState{
		Item: &provider.Item{
			ID:             parseItemID(payload.ItemID),
			ExternalID:     types.NewExternalID(payload.SourceID),
			Source:         types.NewProviderID(payload.Source),
			Type:           types.NewEventTypeID(payload.Type),
			ActorLogin:     types.NewActorID(payload.ActorLogin),
			ActorAvatarURL: payload.ActorAvatarURL,
			RepoName:       types.NewRepoID(payload.RepoName),
			RepoURL:        payload.RepoURL,
			CreatedAt:      fromUnixNano(payload.CreatedAt),
			UpdatedAt:      fromUnixNano(payload.UpdatedAt),
			RawJSON:        payload.RawJSON,
		},
		Deleted: false,
	}, nil
}

// DecideSync returns a DecideFunc that syncs an incoming provider.Item.
func DecideSync(
	item *provider.Item,
) func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

		if state.Deleted || state.IsNew() {
			return syncEvents(item, aggID, currentVersion, false), nil
		}

		if !HasChanged(state.Item, item) {
			return nil, nil
		}

		return syncEvents(item, aggID, currentVersion, true), nil
	}
}

// DecideDelete returns a DecideFunc that marks an item as deleted.
func DecideDelete(
	source, sourceID string,
) func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		if state.Deleted || state.IsNew() {
			return nil, nil
		}

		aggID := AggregateID(source, sourceID)

		evt, err := newEvent(EventItemDeleted, aggID, int(currentVersion)+1, ItemDeletedPayload{
			Source:   source,
			SourceID: sourceID,
		})
		if err != nil {
			return nil, err
		}

		return []event.Event{evt}, nil
	}
}

const syncEventsInitialCap = 2

func syncEvents(
	item *provider.Item,
	aggID id.AggregateID,
	version event.Version,
	isConflict bool,
) []event.Event {
	events := make([]event.Event, 0, syncEventsInitialCap)
	ver := int(version)

	if isConflict {
		evt, err := newEvent(EventItemConflictFound, aggID, ver+1, ItemConflictFoundPayload{
			Source:          item.Source.Get(),
			SourceID:        item.ExternalID.Get(),
			LocalUpdatedAt:  unixNano(item.UpdatedAt),
			RemoteUpdatedAt: unixNano(item.UpdatedAt),
			Winner:          "remote",
		})
		if err == nil {
			events = append(events, evt)
		}
	}

	evt, err := newEvent(
		EventItemSynced,
		aggID,
		ver+len(events)+1,
		itemToPayload(item),
	)
	if err == nil {
		events = append(events, evt)
	}

	return events
}

func newEvent(
	eventType event.Type,
	aggID id.AggregateID,
	version int,
	payload any,
) (*event.Core, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload for %s: %w", eventType, err)
	}

	core, err := event.NewEvent(eventType, aggID, aggregateType, version, data)
	if err != nil {
		return nil, fmt.Errorf("create event %s: %w", eventType, err)
	}

	return core, nil
}

func itemToPayload(item *provider.Item) ItemSyncedPayload {
	return ItemSyncedPayload{
		ItemID:         item.ID.String(),
		Source:         item.Source.Get(),
		SourceID:       item.ExternalID.Get(),
		Type:           item.Type.Get(),
		ActorLogin:     item.ActorLogin.Get(),
		ActorAvatarURL: item.ActorAvatarURL,
		RepoName:       item.RepoName.Get(),
		RepoURL:        item.RepoURL,
		CreatedAt:      unixNano(item.CreatedAt),
		UpdatedAt:      unixNano(item.UpdatedAt),
		RawJSON:        item.RawJSON,
	}
}

func HasChanged(local, remote *provider.Item) bool {
	return local.UpdatedAt != remote.UpdatedAt ||
		local.Type.Get() != remote.Type.Get() ||
		local.ActorLogin.Get() != remote.ActorLogin.Get() ||
		local.RepoName.Get() != remote.RepoName.Get()
}

func parseItemID(s string) types.ItemID {
	if s == "" {
		return types.NewItemID()
	}

	return types.MustParseItemID(s)
}
