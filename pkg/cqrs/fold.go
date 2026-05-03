package cqrs

import (
	"encoding/json"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/event"
)

// fold applies a single event to the SyncItemState, returning the new state.
// This is the pure fold function for the SyncItem Decider.
func fold(state SyncItemState, evt event.Event) (SyncItemState, error) {
	switch evt.Type() {
	case EventItemSynced:
		return foldItemSynced(state, evt)

	case EventItemConflictFound:
		return foldItemConflictFound(state, evt)

	case EventItemDeleted:
		return foldItemDeleted(state)

	default:
		return state, fmt.Errorf("unknown event type: %s", evt.Type())
	}
}

func foldItemSynced(state SyncItemState, evt event.Event) (SyncItemState, error) {
	var payload ItemSyncedPayload

	err := json.Unmarshal(evt.Payload(), &payload)
	if err != nil {
		return state, fmt.Errorf("unmarshal ItemSyncedPayload: %w", err)
	}

	return SyncItemState{
		Source:         payload.Source,
		SourceID:       payload.SourceID,
		Type:           payload.Type,
		ActorLogin:     payload.ActorLogin,
		ActorAvatarURL: payload.ActorAvatarURL,
		RepoName:       payload.RepoName,
		RepoURL:        payload.RepoURL,
		CreatedAt:      fromUnixNano(payload.CreatedAt),
		UpdatedAt:      fromUnixNano(payload.UpdatedAt),
		RawJSON:        payload.RawJSON,
		Deleted:        false,
	}, nil
}

func foldItemConflictFound(state SyncItemState, evt event.Event) (SyncItemState, error) {
	var payload ItemConflictFoundPayload

	err := json.Unmarshal(evt.Payload(), &payload)
	if err != nil {
		return state, fmt.Errorf("unmarshal ItemConflictFoundPayload: %w", err)
	}

	return state, nil
}

func foldItemDeleted(state SyncItemState) (SyncItemState, error) {
	state.Deleted = true

	return state, nil
}
