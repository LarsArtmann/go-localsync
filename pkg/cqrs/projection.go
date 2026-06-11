package cqrs

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/codec/v2"
	"github.com/larsartmann/go-cqrs-lite/event/v2"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// Projector projects domain events onto the ReadModel.
type Projector struct {
	readModel ReadModel
}

// NewProjector creates a Projector for the given ReadModel.
func NewProjector(rm ReadModel) *Projector {
	return &Projector{readModel: rm}
}

// Name returns the projection name.
func (p *Projector) Name() string {
	return "sync_item_projection"
}

// EventTypes returns the event types this projector handles.
func (p *Projector) EventTypes() []event.Type {
	return []event.Type{EventItemSynced, EventItemDeleted, EventItemConflictFound}
}

// Handle processes a single event, updating the read model.
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
	item, err := decodeItemFromEvent(evt)
	if err != nil {
		return err
	}

	return p.readModel.Upsert(ctx, item)
}

func (p *Projector) handleItemDeleted(ctx context.Context, evt event.Event) error {
	payload, err := event.DecodePayload[ItemDeletedPayload](evt, codec.JSONCodec{})
	if err != nil {
		return fmt.Errorf("decode ItemDeletedPayload for event %s: %w", evt.ID(), err)
	}

	return p.readModel.Delete(ctx, payload.Source, id.NewExternalID(payload.SourceID))
}

var _ event.Projection = (*Projector)(nil)
