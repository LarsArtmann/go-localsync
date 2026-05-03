package cqrs

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/memory"
	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// CQRSConfig holds configuration for the CQRS stack.
type CQRSConfig struct {
	// Backend selects the event store backend: "memory" (default).
	Backend string
}

// CQRSStack holds all CQRS components wired together.
type CQRSStack struct {
	Store      event.Store
	Bus        event.Bus
	Repository *decider.Repository[SyncItemState]
	ReadModel  ReadModel
	Projector  *Projector
	logger     *log.Logger
}

// NewCQRSStack creates a fully wired CQRS stack with the given configuration.
func NewCQRSStack(cfg CQRSConfig, logger *log.Logger) (*CQRSStack, error) {
	if logger == nil {
		logger = log.Default()
	}

	store, bus, err := createStoreAndBus(cfg)
	if err != nil {
		return nil, err
	}

	rm := NewMemoryReadModel()
	proj := NewProjector(rm)

	d := decider.Decider[SyncItemState]{
		Initial: InitialSyncItemState,
		Fold:    fold,
	}

	repo, err := decider.NewRepository[SyncItemState](store, bus, d)
	if err != nil {
		return nil, fmt.Errorf("create decider repository: %w", err)
	}

	return &CQRSStack{
		Store:      store,
		Bus:        bus,
		Repository: repo,
		ReadModel:  rm,
		Projector:  proj,
		logger:     logger,
	}, nil
}

// SyncItem syncs a single provider.Item through the CQRS decider path.
// The aggregate ID is derived from source:sourceID using the item's fields.
func (s *CQRSStack) SyncItem(ctx context.Context, item *provider.Item) error {
	aggID := aggregateID(item.Source.Get(), item.ExternalID.Get())

	err := s.Repository.Execute(ctx, aggID, aggregateType, DecideSync(item))
	if err != nil {
		return err
	}

	s.projectLatestState(ctx, aggID)

	return nil
}

// DeleteItem deletes an item through the CQRS decider path.
func (s *CQRSStack) DeleteItem(ctx context.Context, source, sourceID string) error {
	aggID := aggregateID(source, sourceID)

	err := s.Repository.Execute(ctx, aggID, aggregateType, DecideDelete())
	if err != nil {
		return err
	}

	s.projectLatestState(ctx, aggID)

	return nil
}

// SyncItems syncs multiple items and returns counts.
func (s *CQRSStack) SyncItems(ctx context.Context, items []*provider.Item) (synced, conflicts, errors int) {
	for _, item := range items {
		aggID := aggregateID(item.Source.Get(), item.ExternalID.Get())
		decide := DecideSync(item)

		var producedEvents []event.Event

		// We need to capture the events to count conflicts.
		// Use Load to get state, then decide, then Execute manually.
		state, version, err := s.Repository.Load(ctx, aggID, aggregateType)
		if err != nil {
			s.logger.Warn("Failed to load aggregate", "error", err)
			errors++

			continue
		}

		producedEvents, err = decide(state, version)
		if err != nil {
			s.logger.Warn("Decide failed", "error", err)
			errors++

			continue
		}

		if len(producedEvents) == 0 {
			continue
		}

		err = s.Repository.Execute(ctx, aggID, aggregateType, decide)
		if err != nil {
			s.logger.Warn("Execute failed", "error", err)
			errors++

			continue
		}

		synced++

		for _, evt := range producedEvents {
			if evt.Type() == EventItemConflictFound {
				conflicts++
			}

			if projErr := s.Projector.HandleEvent(ctx, evt); projErr != nil {
				s.logger.Warn("Projection failed", "error", projErr)
			}
		}
	}

	return synced, conflicts, errors
}

// Count returns the number of items in the read model.
func (s *CQRSStack) Count(ctx context.Context) (int64, error) {
	return s.ReadModel.Count(ctx, ItemFilter{})
}

// GetTypes returns all unique item types from the read model.
func (s *CQRSStack) GetTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

// Close releases all resources.
func (s *CQRSStack) Close() error {
	if err := s.ReadModel.Close(); err != nil {
		return err
	}

	return s.Store.Close()
}

// projectLatestState loads the current aggregate state and projects it to the read model.
func (s *CQRSStack) projectLatestState(ctx context.Context, aggID id.AggregateID) {
	state, _, err := s.Repository.Load(ctx, aggID, aggregateType)
	if err != nil {
		s.logger.Warn("Failed to load state for projection", "error", err)

		return
	}

	if state.IsNew() || state.Deleted {
		return
	}

	rmState := fromSyncItemState(state)

	if projErr := s.ReadModel.Upsert(ctx, rmState); projErr != nil {
		s.logger.Warn("Failed to project state", "error", projErr)
	}
}

func createStoreAndBus(cfg CQRSConfig) (event.Store, event.Bus, error) {
	switch cfg.Backend {
	case "memory", "":
		return cqrsmemory.NewMemoryStore(), cqrsmemory.NewMemoryBus(), nil
	default:
		return nil, nil, fmt.Errorf("unknown backend: %s", cfg.Backend)
	}
}
