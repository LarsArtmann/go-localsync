package cqrs

import (
	"context"

	"github.com/larsartmann/go-cqrs-lite/command/v3"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

func commandValidationMiddleware() command.Middleware {
	return func(next command.Handler) command.Handler {
		return func(ctx context.Context, cmd command.Command) error {
			switch cmdTyped := cmd.(type) {
			case *SyncItemCommand:
				if cmdTyped.Item == nil {
					return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "sync item command: item is nil")
				}

				if cmdTyped.Item.Source.Get() == "" {
					return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "sync item command: source is empty")
				}
			case *TombstoneItemCommand:
				if cmdTyped.Source == "" {
					return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "tombstone item command: source is empty")
				}

				if cmdTyped.Reason == "" {
					return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "tombstone item command: reason is empty")
				}
			}

			return next(ctx, cmd)
		}
	}
}
