package cqrs

import (
	"context"
	"fmt"
	"sync"

	"github.com/larsartmann/go-cqrs-lite/codec/v3"
	"github.com/larsartmann/go-cqrs-lite/event/v3"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// Projector projects domain events onto the ReadModel.
type Projector struct {
	readModel ReadModel
	// lastVersions tracks the highest event version applied per aggregate ID.
	// This prevents stale events from background journal replay from
	// resurrecting rows that were already deleted via a newer live event.
	lastVersions sync.Map
}

// newProjector creates a Projector for the given ReadModel.
func newProjector(rm ReadModel) *Projector {
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
// Events with a version <= the last applied version for their aggregate are
// skipped — this prevents stale journal-replay events from resurrecting rows
// that were already deleted via a newer live event.
func (p *Projector) Handle(ctx context.Context, evt event.Event) error {
	aggID := evt.AggregateID().String()
	version := evt.Version()

	if last, ok := p.lastVersions.Load(aggID); ok && version <= last.(event.Version) {
		return nil
	}

	switch evt.Type() {
	case EventItemSynced:
		if err := p.handleItemSynced(ctx, evt); err != nil {
			return err
		}
	case EventItemDeleted:
		if err := p.handleItemDeleted(ctx, evt); err != nil {
			return err
		}
	case EventItemConflictFound:
		// no-op — metadata event only
	}

	p.lastVersions.Store(aggID, version)

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
