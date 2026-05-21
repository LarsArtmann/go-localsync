package cqrs

import (
	"context"
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
	payload, err := event.DecodePayload[ItemSyncedPayload](evt, event.JSONCodec{})
	if err != nil {
		return fmt.Errorf("decode ItemSyncedPayload for event %s: %w", evt.ID(), err)
	}

	return p.readModel.Upsert(ctx, itemFromPayload(payload))
}

func (p *Projector) handleItemDeleted(ctx context.Context, evt event.Event) error {
	payload, err := event.DecodePayload[ItemDeletedPayload](evt, event.JSONCodec{})
	if err != nil {
		return fmt.Errorf("decode ItemDeletedPayload for event %s: %w", evt.ID(), err)
	}

	return p.readModel.Delete(ctx, payload.Source, payload.SourceID)
}

var _ event.Projection = (*Projector)(nil)
