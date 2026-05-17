package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrRateLimited    = errors.New("rate limited")
	ErrInvalidToken   = errors.New("invalid token")
	ErrUserNotFound   = errors.New("user not found")
	ErrSyncFailed     = errors.New("sync failed")
	ErrDatabase       = errors.New("database error")
	ErrInvalidInput   = errors.New("invalid input")
	ErrUnknownBackend = errors.New("unknown backend")
	ErrDBNil          = errors.New("database is nil")
)

// WithDetail wraps err with a detail string for debugging context.
func WithDetail(err error, detail string) error {
	return fmt.Errorf("%s: %w", detail, err)
}

// WithUserDetail is a convenience function to add username context.
func WithUserDetail(err error, username string) error {
	return fmt.Errorf("username=%s: %w", username, err)
}

// Wrap wraps an error with additional context.
func Wrap(err error, message string) error {
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with a formatted message.
func Wrapf(err error, format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
