package errors

import (
	"github.com/cockroachdb/errors"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrRateLimited  = errors.New("rate limited")
	ErrInvalidToken = errors.New("invalid token")
	ErrUserNotFound = errors.New("user not found")
	ErrSyncFailed   = errors.New("sync failed")
	ErrStorage      = errors.New("storage error")
	ErrDatabase     = errors.New("database error")
	ErrConflict     = errors.New("conflict detected")
	ErrInvalidInput = errors.New("invalid input")
)

// WithDetail wraps err with a detail string for debugging context.
func WithDetail(err error, detail string) error {
	return errors.WithDetail(err, detail)
}

// WithUserDetail is a convenience function to add username context.
func WithUserDetail(err error, username string) error {
	return errors.WithDetail(err, "username="+username)
}

// Wrap wraps an error with additional context.
func Wrap(err error, message string) error {
	return errors.Wrap(err, message)
}
