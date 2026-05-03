package cqrs

import (
	"context"
	"encoding/json"

	"github.com/larsartmann/go-cqrs-lite/core/event"
)

// Projector subscribes to events and updates the read model.
type Projector struct {
	readModel ReadModel
}

// NewProjector creates a new Projector that updates the given ReadModel.
func NewProjector(rm ReadModel) *Projector {
	return &Projector{readModel: rm}
}

// HandleEvent processes a single event and updates the read model.
func (p *Projector) HandleEvent(_ context.Context, evt event.Event) error {
	switch evt.Type() {
	case EventItemSynced:
		return p.handleItemSynced(evt)

	case EventItemDeleted:
		return p.handleItemDeleted(evt)

	case EventItemConflictFound:
		return p.handleItemConflictFound(evt)
	}

	return nil
}

func (p *Projector) handleItemSynced(evt event.Event) error {
	var payload ItemSyncedPayload

	err := json.Unmarshal(evt.Payload(), &payload)
	if err != nil {
		return err
	}

	state := &itemState{
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
	}

	return p.readModel.Upsert(context.Background(), state)
}

func (p *Projector) handleItemDeleted(evt event.Event) error {
	var payload ItemDeletedPayload

	err := json.Unmarshal(evt.Payload(), &payload)
	if err != nil {
		return err
	}

	return p.readModel.Delete(context.Background(), payload.Source, payload.SourceID)
}

func (p *Projector) handleItemConflictFound(_ event.Event) error {
	return nil
}
