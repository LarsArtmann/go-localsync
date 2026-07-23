package cqrs

import (
	"context"
	"sync"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/projection/v4"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// Projector projects domain events onto the ReadModel.
type Projector struct {
	mu        sync.Mutex
	readModel ReadModel
	// lastVersions tracks the highest event version applied per aggregate ID.
	// This prevents stale events from background journal replay from
	// resurrecting rows that were already deleted via a newer live event.
	// Guarded by mu — the check-apply-store sequence in Handle must be atomic
	// to prevent a concurrent live event and stale replay from both applying.
	lastVersions map[string]event.Version
}

// newProjector creates a Projector for the given ReadModel.
func newProjector(rm ReadModel) *Projector {
	return &Projector{readModel: rm, lastVersions: make(map[string]event.Version)}
}

// Name returns the projection name.
func (p *Projector) Name() string {
	return "sync_item_projection"
}

// EventTypes returns the event types this projector handles.
func (p *Projector) EventTypes() []event.Type {
	return []event.Type{EventItemSynced, EventItemTombstoned, EventItemConflictFound}
}

// Handle processes a single event, updating the read model.
// Events with a version <= the last applied version for their aggregate are
// skipped — this prevents stale journal-replay events from resurrecting rows
// that were already deleted via a newer live event.
func (p *Projector) Handle(ctx context.Context, evt event.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	aggID := evt.StreamID().String()
	version := evt.Version()

	if last, ok := p.lastVersions[aggID]; ok && version <= last {
		return nil
	}

	switch evt.Type() {
	case EventItemSynced:
		if err := p.handleItemSynced(ctx, evt); err != nil {
			return err
		}
	case EventItemTombstoned:
		if err := p.handleItemTombstoned(ctx, evt); err != nil {
			return err
		}
	case EventItemConflictFound:
		// no-op — metadata event only
	}

	p.lastVersions[aggID] = version

	return nil
}

func (p *Projector) handleItemSynced(ctx context.Context, evt event.Event) error {
	item, err := decodeItemFromEvent(evt)
	if err != nil {
		return err
	}

	return p.readModel.Upsert(ctx, item)
}

func (p *Projector) handleItemTombstoned(ctx context.Context, evt event.Event) error {
	payload, err := event.DecodePayloadAuto[ItemTombstonedPayload](evt)
	if err != nil {
		return pkgerrors.Wrapf(err, "decode ItemTombstonedPayload for event %s", evt.ID())
	}

	tombstone := model.Tombstone{
		Reason: model.ParseTombstoneReason(payload.Reason),
		At:     fromUnixNano(payload.TombstonedAt),
	}

	return p.readModel.Tombstone(ctx, payload.Source, id.NewExternalID(payload.SourceID), tombstone)
}

var _ projection.Projection = (*Projector)(nil)
