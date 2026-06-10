// Package transform provides explicit, type-safe, and composable mappings
// between layers: Provider → Domain → View → API.
//
// Every mapping is explicit, tested, and versioned. No implicit struct reuse.
package transform

import (
	"context"
	"fmt"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// ---------------------------------------------------------------------------
// Generic Mapper interface and combinators — the core abstraction.
// ---------------------------------------------------------------------------

// Mapper transforms From → To. It is the fundamental building block
// for all layer-to-layer conversions in the data module.
type Mapper[From, To any] interface {
	Map(ctx context.Context, from From) (To, error)
}

// MapperFunc is a Mapper implemented as a function.
type MapperFunc[From, To any] struct {
	mapFn func(context.Context, From) (To, error)
}

// Map implements Mapper.
func (m MapperFunc[From, To]) Map(ctx context.Context, from From) (To, error) {
	return m.mapFn(ctx, from)
}

// NewMapper creates a Mapper from a function.
func NewMapper[From, To any](
	fn func(context.Context, From) (To, error),
) Mapper[From, To] {
	return MapperFunc[From, To]{mapFn: fn}
}

// Compose chains two Mappers: A → B → C.
// This is the key to building transformation pipelines without
// intermediate variables or boilerplate.
func Compose[A, B, C any](
	ab Mapper[A, B],
	bc Mapper[B, C],
) Mapper[A, C] {
	return NewMapper[A, C](func(ctx context.Context, a A) (C, error) {
		mapped, err := ab.Map(ctx, a)
		if err != nil {
			var zero C

			return zero, composeErr("A→B", err)
		}

		result, err := bc.Map(ctx, mapped)
		if err != nil {
			var zero C

			return zero, composeErr("B→C", err)
		}

		return result, nil
	})
}

func composeErr(stage string, err error) error {
	return fmt.Errorf("compose %s: %w", stage, err)
}

// AndThen is a fluent alias for Compose: m.AndThen(next) == Compose(m, next).
func AndThen[A, B, C any](
	m Mapper[A, B],
	next Mapper[B, C],
) Mapper[A, C] {
	return Compose(m, next)
}

// ---------------------------------------------------------------------------
// Concrete Mappers for the sync domain.
// ---------------------------------------------------------------------------

// NewFromProviderItem maps a ProviderItem (provider DTO) to a domain Item.
// This is the boundary between the provider layer and the domain layer.
func NewFromProviderItem() Mapper[*model.ProviderItem, *model.Item] {
	return NewMapper(
		func(_ context.Context, p *model.ProviderItem) (*model.Item, error) {
			if p == nil {
				return nil, fmt.Errorf("from provider item: %w", errNilInput)
			}

			err := p.Validate()
			if err != nil {
				return nil, fmt.Errorf("from provider item: invalid input: %w", err)
			}

			return &model.Item{
				ID:             id.NewItemID(),
				ExternalID:     p.ExternalID,
				Source:         p.Source,
				Type:           p.Type,
				ActorLogin:     p.ActorLogin,
				ActorAvatarURL: p.ActorAvatarURL,
				RepoName:       p.RepoName,
				RepoURL:        p.RepoURL,
				CreatedAt:      p.CreatedAt,
				UpdatedAt:      p.UpdatedAt,
				SchemaVersion:  schema.CurrentVersion(),
			}, nil
		},
	)
}

// NewToItemView maps a domain Item to a read-model ItemView.
// In production, this would accept sync metadata (counts, timestamps).
func NewToItemView() Mapper[*model.Item, *model.ItemView] {
	return NewMapper(
		func(_ context.Context, item *model.Item) (*model.ItemView, error) {
			if item == nil {
				return nil, fmt.Errorf("to item view: %w", errNilInput)
			}

			return &model.ItemView{
				Item:          *item,
				LastSyncedAt:  time.Time{},
				SyncCount:     0,
				ConflictCount: 0,
				IsDeleted:     false,
			}, nil
		},
	)
}

// NewProviderToView composes the full provider → view pipeline.
func NewProviderToView() Mapper[*model.ProviderItem, *model.ItemView] {
	return Compose(
		NewFromProviderItem(),
		NewToItemView(),
	)
}
