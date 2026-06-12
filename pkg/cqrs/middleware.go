package cqrs

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/command/v2"
	"github.com/larsartmann/go-cqrs-lite/query/v2"
)

var errValidationFailed = stderrors.New("command validation failed")

func commandLoggingMiddleware(logger *log.Logger) command.Middleware {
	return func(next command.Handler) command.Handler {
		return func(ctx context.Context, cmd command.Command) error {
			start := time.Now()
			err := next(ctx, cmd)
			duration := time.Since(start)

			if err != nil {
				logger.Error(
					"command dispatch failed",
					"type", cmd.Type(),
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info(
					"command dispatch succeeded",
					"type", cmd.Type(),
					"duration", duration,
				)
			}

			return err
		}
	}
}

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
			case *DeleteItemCommand:
				if cmdTyped.Source == "" {
					return fmt.Errorf(
						"delete item command: source is empty: %w",
						errValidationFailed,
					)
				}
			}

			return next(ctx, cmd)
		}
	}
}

func queryLoggingMiddleware(logger *log.Logger) query.Middleware {
	return func(next query.Handler) query.Handler {
		return func(ctx context.Context, queryArg query.Query) (any, error) {
			start := time.Now()
			result, err := next(ctx, queryArg)
			duration := time.Since(start)

			if err != nil {
				logger.Error(
					"query dispatch failed",
					"type", queryArg.Type(),
					"duration", duration,
					"error", err,
				)
			} else {
				logger.Info(
					"query dispatch succeeded",
					"type", queryArg.Type(),
					"duration", duration,
				)
			}

			return result, err
		}
	}
}
