package cqrs

import (
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/codec/v2"
	"github.com/larsartmann/go-cqrs-lite/decider/v2"
	"github.com/larsartmann/go-cqrs-lite/event/v2"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v2"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// SyncItemState is the aggregate state for a single sync item, reconstructed from events.
// It wraps a *model.Item (nil when no events applied) plus a Deleted flag.
type SyncItemState struct {
	Item    *model.Item
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
		state.Item = nil

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
	payload, err := event.DecodePayload[ItemSyncedPayload](evt, codec.JSONCodec{})
	if err != nil {
		return SyncItemState{}, fmt.Errorf(
			"decode ItemSyncedPayload for event %s: %w",
			evt.ID(),
			err,
		)
	}

	item, err := DataItemFromPayload(payload)
	if err != nil {
		return SyncItemState{}, fmt.Errorf("reconstruct item from payload: %w", err)
	}

	return SyncItemState{Item: item, Deleted: false}, nil
}

// DecideSync returns a decider.DecideFunc that syncs an incoming model.Item.
// If resolver is nil, remote-wins is used as the default strategy.
// rawJSON is the original provider payload, stored in the event for full-fidelity replay.
func DecideSync(
	item *model.Item,
	rawJSON []byte,
	resolver crdt.ConflictResolver[*model.Item],
	opts ...event.Option,
) decider.DecideFunc[SyncItemState] {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		aggID := AggregateID(item.Source.Get(), item.ExternalID)

		if state.Deleted || state.IsNew() {
			return syncEvents(item, rawJSON, aggID, currentVersion, nil, opts...)
		}

		if !HasChanged(state.Item, item) {
			return nil, nil
		}

		winner, winnerLabel := resolveConflict(resolver, state.Item, item)

		return syncEvents(winner, rawJSON, aggID, currentVersion, &conflictMeta{
			localUpdatedAt:  state.Item.UpdatedAt,
			remoteUpdatedAt: item.UpdatedAt,
			winner:          winnerLabel,
		}, opts...)
	}
}

// DecideDelete returns a decider.DecideFunc that marks an item as deleted.
func DecideDelete(
	source string, sourceID id.ExternalID,
	opts ...event.Option,
) decider.DecideFunc[SyncItemState] {
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
				SourceID: sourceID.Get(),
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

const (
	conflictWinnerRemote = "remote"
	conflictWinnerLocal  = "local"
)

// conflictMeta carries conflict-specific metadata for event construction.
type conflictMeta struct {
	localUpdatedAt  time.Time
	remoteUpdatedAt time.Time
	winner          string
}

// resolveConflict delegates conflict resolution to the resolver, or defaults to remote-wins.
func resolveConflict(
	resolver crdt.ConflictResolver[*model.Item],
	local, remote *model.Item,
) (*model.Item, string) {
	if resolver == nil {
		return remote, conflictWinnerRemote
	}

	conflict := &crdt.Conflict[*model.Item]{
		Local:     local,
		Remote:    remote,
		LocalVC:   crdt.NewVectorClock(),
		RemoteVC:  crdt.NewVectorClock(),
		Timestamp: time.Now(),
	}

	winner, err := resolver.Resolve(conflict)
	if err != nil {
		return remote, conflictWinnerRemote
	}

	if winner == local {
		return local, conflictWinnerLocal
	}

	return remote, conflictWinnerRemote
}

func syncEvents(
	item *model.Item,
	rawJSON []byte,
	aggID cqrsid.AggregateID,
	version event.Version,
	conflict *conflictMeta,
	opts ...event.Option,
) ([]event.Event, error) {
	eventTypes := []event.Type{EventItemSynced}
	payloads := []any{DataItemToPayload(item, rawJSON)}

	if conflict != nil {
		eventTypes = []event.Type{EventItemConflictFound, EventItemSynced}
		payloads = []any{
			ItemConflictFoundPayload{
				Source:          item.Source.Get(),
				SourceID:        item.ExternalID.Get(),
				LocalUpdatedAt:  unixNano(conflict.localUpdatedAt),
				RemoteUpdatedAt: unixNano(conflict.remoteUpdatedAt),
				Winner:          conflict.winner,
			},
			DataItemToPayload(item, rawJSON),
		}
	}

	evts, err := event.NewEvents(aggID, aggregateType, version, eventTypes, payloads, opts...)
	if err != nil {
		return nil, fmt.Errorf(
			"create events for %s/%s (version=%d, conflict=%v): %w",
			aggID,
			item.ExternalID.Get(),
			version,
			conflict,
			err,
		)
	}

	return evts, nil
}

// HasChanged returns true if the remote item differs from the local item
// in any tracked field (UpdatedAt, Type, ActorLogin, RepoName, RepoURL).
func HasChanged(local, remote *model.Item) bool {
	return !local.UpdatedAt.Equal(remote.UpdatedAt) ||
		local.Type.Get() != remote.Type.Get() ||
		local.ActorLogin.Get() != remote.ActorLogin.Get() ||
		local.RepoName.Get() != remote.RepoName.Get() ||
		local.RepoURL != remote.RepoURL
}

func parseItemID(s string) (id.ItemID, error) {
	if s == "" {
		return id.NewItemID(), nil
	}

	return id.ParseItemID(s)
}
