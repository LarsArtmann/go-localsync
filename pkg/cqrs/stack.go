package cqrs

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/core/decider"
	"github.com/larsartmann/go-cqrs-lite/core/event"
	cqrsmemory "github.com/larsartmann/go-cqrs-lite/memory"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

type CQRSConfig struct {
	Backend string
}

type CQRSStack struct {
	Store     event.Store
	Bus       event.Bus
	Repo      *decider.Repository[SyncItemState]
	ReadModel ReadModel
}

func NewCQRSStack(cfg CQRSConfig) (*CQRSStack, error) {
	store, bus, err := createStoreAndBus(cfg)
	if err != nil {
		return nil, err
	}

	rm := NewMemoryReadModel()
	proj := NewProjector(rm)

	err = bus.SubscribeAll(proj.HandleEvent)
	if err != nil {
		return nil, fmt.Errorf("subscribe projector: %w", err)
	}

	d := decider.Decider[SyncItemState]{
		Initial: InitialState,
		Fold:    Fold,
	}

	repo, err := decider.NewRepository[SyncItemState](store, bus, d)
	if err != nil {
		return nil, fmt.Errorf("create decider repository: %w", err)
	}

	return &CQRSStack{
		Store:     store,
		Bus:       bus,
		Repo:      repo,
		ReadModel: rm,
	}, nil
}

func (s *CQRSStack) SyncItem(ctx context.Context, item *provider.Item) error {
	aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

	return s.Repo.Execute(ctx, aggID, aggregateType, DecideSync(item))
}

func (s *CQRSStack) DeleteItem(ctx context.Context, source, sourceID string) error {
	aggID := AggregateID(source, sourceID)

	return s.Repo.Execute(ctx, aggID, aggregateType, DecideDelete(source, sourceID))
}

//nolint:nonamedreturns
func (s *CQRSStack) SyncItems(
	ctx context.Context,
	items []*provider.Item,
) (synced, conflicts, errs int) {
	for _, item := range items {
		aggID := AggregateID(item.Source.Get(), item.ExternalID.Get())

		_, beforeVer, _ := s.Repo.Load(ctx, aggID, aggregateType)

		err := s.Repo.Execute(ctx, aggID, aggregateType, DecideSync(item))
		if err != nil {
			errs++

			continue
		}

		_, afterVer, _ := s.Repo.Load(ctx, aggID, aggregateType)

		delta := int(afterVer) - int(beforeVer)
		if delta > 0 {
			synced++

			if delta > 1 {
				conflicts += delta - 1
			}
		}
	}

	return synced, conflicts, errs
}

func (s *CQRSStack) Count(ctx context.Context) (int64, error) {
	return s.ReadModel.Count(ctx, ItemFilter{
		Type:       nil,
		ActorLogin: nil,
		RepoName:   nil,
		Source:     nil,
		Since:      nil,
		Limit:      0,
		Offset:     0,
	})
}

func (s *CQRSStack) GetTypes(ctx context.Context) ([]string, error) {
	return s.ReadModel.GetTypes(ctx)
}

func (s *CQRSStack) Close() error {
	err := s.ReadModel.Close()
	if err != nil {
		return err
	}

	return s.Store.Close()
}

//nolint:ireturn
func createStoreAndBus(cfg CQRSConfig) (event.Store, event.Bus, error) {
	switch cfg.Backend {
	case "memory", "":
		return cqrsmemory.NewMemoryStore(), cqrsmemory.NewMemoryBus(), nil
	default:
		//nolint:err113 // error is specific to input, not a generic failure
		return nil, nil, fmt.Errorf(
			"unknown backend: %s",
			cfg.Backend,
		)
	}
}
