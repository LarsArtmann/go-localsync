package cqrs

import (
	"time"

	"github.com/larsartmann/go-cqrs-lite/decider/v4"
	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// SyncItemState is the aggregate state for a single sync item, reconstructed from events.
// Item is nil before the first sync. A tombstone is carried on Item.Tombstone rather
// than a separate flag, so the item's history is always preserved.
type SyncItemState struct {
	Item *model.Item
}

// InitialState is the zero state for a new SyncItem aggregate.
//
//nolint:gochecknoglobals // immutable zero-value sentinel, not mutable global state
var InitialState = SyncItemState{Item: nil}

// IsNew returns true if no events have been applied (Item is nil).
func (s SyncItemState) IsNew() bool {
	return s.Item == nil
}

// IsTombstoned reports whether the aggregate is currently hidden from the default view.
func (s SyncItemState) IsTombstoned() bool {
	return s.Item != nil && s.Item.IsTombstoned()
}

// ShouldResurrect reports whether an incoming sync should overwrite the current
// state without a change comparison: a brand-new aggregate, or a tombstoned one
// whose upstream version has reappeared (making it live again).
func (s SyncItemState) ShouldResurrect() bool {
	return s.IsNew() || s.IsTombstoned()
}

// fold applies a single event to the SyncItemState, returning the new state.
func fold(state SyncItemState, evt event.Event) (SyncItemState, error) {
	switch evt.Type() {
	case EventItemSynced:
		return foldItemSynced(evt)
	case EventItemConflictFound:
		return state, nil
	case EventItemTombstoned:
		return foldItemTombstoned(state, evt)
	default:
		// A corrupt or poison event stream: classify as Infrastructure so a
		// caller can distinguish a store/journal problem from a normal error.
		return state, pkgerrors.Wrapf(
			pkgerrors.ErrDatabase,
			"fold: unknown event type %q in state{tombstoned=%v}",
			evt.Type(),
			state.IsTombstoned(),
		)
	}
}

func foldItemSynced(evt event.Event) (SyncItemState, error) {
	item, err := decodeItemFromEvent(evt)
	if err != nil {
		return SyncItemState{}, err
	}

	return SyncItemState{Item: item}, nil
}

// foldItemTombstoned marks the aggregate's current item as tombstoned. The item
// is kept (not nilled) so its history survives and a later sync can resurrect it.
func foldItemTombstoned(state SyncItemState, evt event.Event) (SyncItemState, error) {
	payload, err := event.DecodePayloadAuto[ItemTombstonedPayload](evt)
	if err != nil {
		return SyncItemState{}, pkgerrors.Wrapf(err, "decode ItemTombstonedPayload for event %s", evt.ID())
	}

	if state.Item == nil {
		// Defensive: a tombstone with no prior live item is a no-op.
		return state, nil
	}

	tombstoned := *state.Item
	tombstoned.Tombstone = model.Tombstone{
		Reason: model.ParseTombstoneReason(payload.Reason),
		At:     fromUnixNano(payload.TombstonedAt),
	}
	state.Item = &tombstoned

	return state, nil
}

// decideSync returns a decider.DecideFunc that syncs an incoming model.Item.
// If resolver is nil, remote-wins is used as the default strategy.
// rawJSON is the original provider payload, stored in the event for full-fidelity replay.
func decideSync(
	item *model.Item,
	rawJSON []byte,
	resolver crdt.ConflictResolver[*model.Item],
	opts ...event.Option,
) decider.DecideFunc[SyncItemState] {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		aggID := AggregateID(item.Source.Get(), item.ExternalID)

		// By design (ADR-0005 addendum): resurrections bypass the resolver.
		// A tombstoned local is a deleted marker, not live content, and a sync
		// event is the only path back to "live" — a local-wins resolver vetoing
		// it would hide an upstream-restored item forever. Pinned by
		// TestDecideSync_ResurrectTombstonedItem_BypassesResolver.
		if state.ShouldResurrect() {
			return syncEvents(item, rawJSON, aggID, currentVersion, nil, opts...)
		}

		if !hasChanged(state.Item, item) {
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

// decideTombstone returns a decider.DecideFunc that hides an item by emitting an
// EventItemTombstoned with the given reason. It is a no-op for a brand-new
// aggregate (nothing to hide) and idempotent for an already-tombstoned item.
// A zero at means "now"; an explicit timestamp makes the decider deterministic
// (exact event assertions in tests, backdated tombstones in tooling).
func decideTombstone(
	source string,
	sourceID id.ExternalID,
	reason model.TombstoneReason,
	at time.Time,
	opts ...event.Option,
) decider.DecideFunc[SyncItemState] {
	return func(state SyncItemState, currentVersion event.Version) ([]event.Event, error) {
		if state.IsNew() || state.IsTombstoned() {
			return nil, nil
		}

		if at.IsZero() {
			at = time.Now().UTC()
		}

		aggID := AggregateID(source, sourceID)

		evts, err := event.NewEvents(
			aggID, aggregateType, currentVersion,
			[]event.Type{EventItemTombstoned},
			[]any{ItemTombstonedPayload{
				Source:       source,
				SourceID:     sourceID.Get(),
				Reason:       string(reason),
				TombstonedAt: unixNano(at),
			}},
			append([]event.Option{event.WithSchemaVersion(currentSchemaV)}, opts...)...,
		)
		if err != nil {
			return nil, pkgerrors.Wrapf(
				err,
				"create tombstone event for %s/%s (version=%d)",
				aggID,
				sourceID,
				currentVersion,
			)
		}

		return evts, nil
	}
}

// ConflictWinner indicates which side won a conflict.
type ConflictWinner string

const (
	ConflictWinnerRemote ConflictWinner = "remote"
	ConflictWinnerLocal  ConflictWinner = "local"
)

// ParseConflictWinner converts a winner string from an event payload into a
// ConflictWinner. Unknown values default to ConflictWinnerRemote (the safe
// fallback, matching the resolver-error behaviour in resolveConflict).
func ParseConflictWinner(s string) ConflictWinner {
	switch ConflictWinner(s) {
	case ConflictWinnerLocal:
		return ConflictWinnerLocal
	case ConflictWinnerRemote:
		return ConflictWinnerRemote
	default:
		return ConflictWinnerRemote
	}
}

// conflictMeta carries conflict-specific metadata for event construction.
type conflictMeta struct {
	localUpdatedAt  time.Time
	remoteUpdatedAt time.Time
	winner          ConflictWinner
}

// resolveConflict delegates conflict resolution to the resolver, or defaults to remote-wins.
func resolveConflict(
	resolver crdt.ConflictResolver[*model.Item],
	local, remote *model.Item,
) (*model.Item, ConflictWinner) {
	if resolver == nil {
		return remote, ConflictWinnerRemote
	}

	conflict := &crdt.Conflict[*model.Item]{
		Local:     local,
		Remote:    remote,
		Timestamp: time.Now().UTC(),
	}

	winner, err := resolver.Resolve(conflict)
	if err != nil {
		return remote, ConflictWinnerRemote
	}

	if winner == local {
		return local, ConflictWinnerLocal
	}

	return remote, ConflictWinnerRemote
}

func syncEvents(
	item *model.Item,
	rawJSON []byte,
	aggID cqrsid.StreamID,
	version event.Version,
	conflict *conflictMeta,
	opts ...event.Option,
) ([]event.Event, error) {
	eventTypes := []event.Type{EventItemSynced}
	payloads := []any{dataItemToPayload(item, rawJSON)}

	if conflict != nil {
		eventTypes = []event.Type{EventItemConflictFound, EventItemSynced}
		payloads = []any{
			ItemConflictFoundPayload{
				Source:          item.Source.Get(),
				SourceID:        item.ExternalID.Get(),
				LocalUpdatedAt:  unixNano(conflict.localUpdatedAt),
				RemoteUpdatedAt: unixNano(conflict.remoteUpdatedAt),
				Winner:          string(conflict.winner),
			},
			dataItemToPayload(item, rawJSON),
		}
	}

	// Stamp the CURRENT schema version: new events must never match the
	// legacy V1/V2 upcasters (a V1-stamped new event would pass through the
	// registry and be mutated in place, racing concurrent bus readers).
	opts = append(opts, event.WithSchemaVersion(currentSchemaV))

	evts, err := event.NewEvents(aggID, aggregateType, version, eventTypes, payloads, opts...)
	if err != nil {
		return nil, pkgerrors.Wrapf(
			err,
			"create events for %s/%s (version=%d, conflict=%v)",
			aggID,
			item.ExternalID.Get(),
			version,
			conflict,
		)
	}

	return evts, nil
}

// hasChanged returns true if the remote item differs from the local item.
// Change detection is fully provider-agnostic: ContentHash (a SHA-256 of the
// provider's raw JSON payload) is the primary signal, with UpdatedAt and Type
// as fallbacks for providers that don't set ContentHash. Provider-specific
// attribute changes are already captured by ContentHash.
func hasChanged(local, remote *model.Item) bool {
	contentChanged := local.ContentHash != "" &&
		remote.ContentHash != "" &&
		local.ContentHash != remote.ContentHash

	return contentChanged ||
		!local.UpdatedAt.Equal(remote.UpdatedAt) ||
		local.Type.Get() != remote.Type.Get()
}

func parseItemID(s string) (id.ItemID, error) {
	if s == "" {
		return id.NewItemID(), nil
	}

	return id.ParseItemID(s)
}
