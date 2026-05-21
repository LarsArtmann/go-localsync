package errors

import (
	"fmt"

	errorfamily "github.com/larsartmann/go-error-family"
)

var (
	ErrNotFound       = errorfamily.NewRejection("not_found", "not found")
	ErrRateLimited    = errorfamily.NewTransient("rate_limited", "rate limited")
	ErrInvalidToken   = errorfamily.NewRejection("invalid_token", "invalid token")
	ErrUserNotFound   = errorfamily.NewRejection("user_not_found", "user not found")
	ErrSyncFailed     = errorfamily.NewTransient("sync_failed", "sync failed")
	ErrDatabase       = errorfamily.NewInfrastructure("database", "database error")
	ErrInvalidInput   = errorfamily.NewRejection("invalid_input", "invalid input")
	ErrUnknownBackend = errorfamily.NewRejection("unknown_backend", "unknown backend")
	ErrDBNil          = errorfamily.NewRejection("db_nil", "database is nil")
)

// WithDetail wraps err with a detail string for debugging context.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func WithDetail(err error, detail string) error {
	if e, ok := err.(*errorfamily.Error); ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), detail)
	}

	return fmt.Errorf("%s: %w", detail, err)
}

// WithUserDetail is a convenience function to add username context.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func WithUserDetail(err error, username string) error {
	if e, ok := err.(*errorfamily.Error); ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), fmt.Sprintf("username=%s", username))
	}

	return fmt.Errorf("username=%s: %w", username, err)
}

// Wrap wraps an error with additional context.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func Wrap(err error, message string) error {
	if e, ok := err.(*errorfamily.Error); ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), message)
	}

	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with a formatted message.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func Wrapf(err error, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)

	if e, ok := err.(*errorfamily.Error); ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), msg)
	}

	return fmt.Errorf("%s: %w", msg, err)
}
