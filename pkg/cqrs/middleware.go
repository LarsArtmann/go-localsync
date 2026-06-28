package cqrs

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/command/v3"
)

var errValidationFailed = stderrors.New("command validation failed")

func commandValidationMiddleware() command.Middleware {
	return func(next command.Handler) command.Handler {
		return func(ctx context.Context, cmd command.Command) error {
			switch cmdTyped := cmd.(type) {
			case *SyncItemCommand:
				if cmdTyped.Item == nil {
					return fmt.Errorf("sync item command: item is nil: %w", errValidationFailed)
				}

				if cmdTyped.Item.Source.Get() == "" {
					return fmt.Errorf(
						"sync item command: source is empty: %w",
						errValidationFailed,
					)
				}
			case *TombstoneItemCommand:
				if cmdTyped.Source == "" {
					return fmt.Errorf(
						"tombstone item command: source is empty: %w",
						errValidationFailed,
					)
				}

				if cmdTyped.Reason == "" {
					return fmt.Errorf(
						"tombstone item command: reason is empty: %w",
						errValidationFailed,
					)
				}
			}

			return next(ctx, cmd)
		}
	}
}
