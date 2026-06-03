package errors

import (
	stderrors "errors"
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

// RegisterErrorTemplates registers user-facing message templates for all error codes.
// Call once at application startup.
func RegisterErrorTemplates() {
	errorfamily.RegisterTemplate("not_found", errorfamily.MessageTemplate{
		What:   "The requested resource was not found.",
		Why:    "The item or resource you requested does not exist in the system.",
		Fix:    "Verify the identifier and try again.",
		WayOut: "Check the logs for the exact resource path.",
	})
	errorfamily.RegisterTemplate("rate_limited", errorfamily.MessageTemplate{
		What:   "Too many requests — rate limit exceeded.",
		Why:    "The external API has received too many requests from this client.",
		Fix:    "Wait for the rate limit window to reset and retry.",
		WayOut: "Use --verbose to see the reset time.",
	})
	errorfamily.RegisterTemplate("invalid_token", errorfamily.MessageTemplate{
		What:   "The provided authentication token is invalid.",
		Why:    "The token is missing, expired, or does not have the required permissions.",
		Fix:    "Set a valid token via GITHUB_TOKEN or the -token flag.",
		WayOut: "Generate a new personal access token on GitHub.",
	})
	errorfamily.RegisterTemplate("user_not_found", errorfamily.MessageTemplate{
		What:   "The specified user was not found.",
		Why:    "The username does not exist on the provider platform.",
		Fix:    "Double-check the username spelling.",
		WayOut: "Try a different username or verify the account exists.",
	})
	errorfamily.RegisterTemplate("sync_failed", errorfamily.MessageTemplate{
		What:   "The synchronization operation failed.",
		Why:    "An unexpected error occurred while fetching or storing items.",
		Fix:    "Check network connectivity and provider status.",
		WayOut: "Run with --verbose for detailed error information.",
	})
	errorfamily.RegisterTemplate("database", errorfamily.MessageTemplate{
		What:   "A database error occurred.",
		Why:    "The storage backend returned an error during read or write.",
		Fix:    "Check the database path and permissions.",
		WayOut: "Verify the backend configuration and disk space.",
	})
	errorfamily.RegisterTemplate("invalid_input", errorfamily.MessageTemplate{
		What:   "The input provided is invalid.",
		Why:    "A required field is missing or has an unacceptable value.",
		Fix:    "Review the input and ensure all required fields are set.",
		WayOut: "See the error detail for the specific missing field.",
	})
	errorfamily.RegisterTemplate("unknown_backend", errorfamily.MessageTemplate{
		What:   "The specified storage backend is not supported.",
		Why:    "Only 'memory' and 'sqlite' backends are currently supported.",
		Fix:    "Use --backend memory or --backend sqlite.",
		WayOut: "Check the documentation for supported backends.",
	})
	errorfamily.RegisterTemplate("db_nil", errorfamily.MessageTemplate{
		What:   "The database connection is nil.",
		Why:    "The SQLite backend was selected but no database path was provided.",
		Fix:    "Set a database path via --db or DB_PATH.",
		WayOut: "Use --backend memory if you do not need persistence.",
	})
}

// IsRetryable reports whether the error is worth retrying.
// Delegates to errorfamily's intrinsic classification.
func IsRetryable(err error) bool {
	return errorfamily.IsRetryable(err)
}

// WithDetail wraps err with a detail string for debugging context.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func WithDetail(err error, detail string) error {
	e, ok := stderrors.AsType[*errorfamily.Error](err)
	if ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), detail)
	}

	return fmt.Errorf("%s: %w", detail, err)
}

// WithUserDetail is a convenience function to add username context.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func WithUserDetail(err error, username string) error {
	e, ok := stderrors.AsType[*errorfamily.Error](err)
	if ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), "username="+username)
	}

	return fmt.Errorf("username=%s: %w", username, err)
}

// Wrap wraps an error with additional context.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func Wrap(err error, message string) error {
	e, ok := stderrors.AsType[*errorfamily.Error](err)
	if ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), message)
	}

	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with a formatted message.
// Preserves errorfamily structure when wrapping an *errorfamily.Error.
func Wrapf(err error, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)

	e, ok := stderrors.AsType[*errorfamily.Error](err)
	if ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), msg)
	}

	return fmt.Errorf("%s: %w", msg, err)
}
