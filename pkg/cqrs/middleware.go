package cqrs

import (
	"github.com/larsartmann/go-cqrs-lite/command/v4"
	"github.com/larsartmann/go-cqrs-lite/middleware/v4"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// validateSyncCommand enforces the command contracts at dispatch time. It
// returns errors wrapping pkgerrors.ErrInvalidInput so consumers can classify
// them; the library middleware additionally stamps the Rejection family and
// logs failures via WithLogger.
func validateSyncCommand(cmd command.Command) error {
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

	return nil
}

// commandValidationMiddleware delegates to the library's validation
// middleware: same checks as before, plus failure logging and consistent
// Rejection classification (middleware.command_validation).
func commandValidationMiddleware() command.Middleware {
	return middleware.CommandValidation(validateSyncCommand, middleware.WithLogger(newSlogLogger()))
}
