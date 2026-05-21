package cqrs

import (
	"fmt"
	"time"

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
			"fold: unknown event type %q in state{deleted=%v}",
			evt.Type(),
			state.Deleted,
		)
	}
}

func foldItemSynced(evt event.Event) (SyncItemState, error) {
	payload, err := event.DecodePayload[ItemSyncedPayload](evt, event.JSONCodec{})
	if err != nil {
		return SyncItemState{}, fmt.Errorf(
			"decode ItemSyncedPayload for event %s: %w",
			evt.ID(),
			err,
		)
	}

	return SyncItemState{
		Item:    itemFromPayload(payload),
		Deleted: false,
	}, nil
}

func itemFromPayload(payload ItemSyncedPayload) *provider.Item {
	return &provider.Item{
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
	}
}

// DecideSync returns a DecideFunc that syncs an incoming provider.Item.
func DecideSync(
	item *provider.Item,
	opts ...event.Option,
) func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

		if state.Deleted || state.IsNew() {
			return syncEvents(item, aggID, currentVersion, false, time.Time{}, opts...)
		}

		if !HasChanged(state.Item, item) {
			return nil, nil
		}

		return syncEvents(item, aggID, currentVersion, true, state.Item.UpdatedAt, opts...)
	}
}

// DecideDelete returns a DecideFunc that marks an item as deleted.
func DecideDelete(
	source, sourceID string,
	opts ...event.Option,
) func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		if state.Deleted || state.IsNew() {
			return nil, nil
		}

		aggID := AggregateID(source, sourceID)

		evts, err := event.NewEvents(
			aggID, aggregateType, currentVersion,
			[]event.Type{EventItemDeleted},
			[]any{ItemDeletedPayload{
				Source:   source,
				SourceID: sourceID,
			}},
			opts...,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create delete event for %s/%s (version=%d): %w",
				aggID,
				sourceID,
				currentVersion,
				err,
			)
		}

		return evts, nil
	}
}

const conflictWinnerRemote = "remote"

func syncEvents(
	item *provider.Item,
	aggID id.AggregateID,
	version event.Version,
	isConflict bool,
	localUpdatedAt time.Time,
	opts ...event.Option,
) ([]event.Event, error) {
	eventTypes := []event.Type{EventItemSynced}
	payloads := []any{itemToPayload(item)}

	if isConflict {
		eventTypes = []event.Type{EventItemConflictFound, EventItemSynced}
		payloads = []any{
			ItemConflictFoundPayload{
				Source:          item.Source.Get(),
				SourceID:        item.ExternalID.Get(),
				LocalUpdatedAt:  unixNano(localUpdatedAt),
				RemoteUpdatedAt: unixNano(item.UpdatedAt),
				Winner:          conflictWinnerRemote,
			},
			itemToPayload(item),
		}
	}

	evts, err := event.NewEvents(aggID, aggregateType, version, eventTypes, payloads, opts...)
	if err != nil {
		return nil, fmt.Errorf(
			"create events for %s/%s (version=%d, isConflict=%v, localUpdatedAt=%v): %w",
			aggID,
			item.ExternalID.Get(),
			version,
			isConflict,
			localUpdatedAt,
			err,
		)
	}

	return evts, nil
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
	return !local.UpdatedAt.Equal(remote.UpdatedAt) ||
		local.Type.Get() != remote.Type.Get() ||
		local.ActorLogin.Get() != remote.ActorLogin.Get() ||
		local.RepoName.Get() != remote.RepoName.Get() ||
		local.RepoURL != remote.RepoURL
}

func parseItemID(s string) types.ItemID {
	if s == "" {
		return types.NewItemID()
	}

	return types.MustParseItemID(s)
}
