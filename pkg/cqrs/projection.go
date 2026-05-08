package cqrs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/event"
)

type Projector struct {
	readModel ReadModel
}

func NewProjector(rm ReadModel) *Projector {
	return &Projector{readModel: rm}
}

func (p *Projector) Name() string {
	return "sync_item_projection"
}

func (p *Projector) EventTypes() []event.Type {
	return []event.Type{EventItemSynced, EventItemDeleted, EventItemConflictFound}
}

func (p *Projector) Handle(ctx context.Context, evt event.Event) error {
	return p.HandleEvent(ctx, evt)
}

func (p *Projector) HandleEvent(ctx context.Context, evt event.Event) error {
	switch evt.Type() {
	case EventItemSynced:
		return p.handleItemSynced(ctx, evt)
	case EventItemDeleted:
		return p.handleItemDeleted(ctx, evt)
	case EventItemConflictFound:
		return nil
	}

	return nil
}

func (p *Projector) handleItemSynced(ctx context.Context, evt event.Event) error {
	var payload ItemSyncedPayload

	err := json.Unmarshal(evt.Payload(), &payload)
	if err != nil {
		return fmt.Errorf("unmarshal ItemSyncedPayload: %w", err)
	}

	return p.readModel.Upsert(ctx, itemFromPayload(payload))
}

func (p *Projector) handleItemDeleted(ctx context.Context, evt event.Event) error {
	var payload ItemDeletedPayload

	err := json.Unmarshal(evt.Payload(), &payload)
	if err != nil {
		return fmt.Errorf("unmarshal ItemDeletedPayload: %w", err)
	}

	return p.readModel.Delete(ctx, payload.Source, payload.SourceID)
}

var _ event.Projection = (*Projector)(nil)
