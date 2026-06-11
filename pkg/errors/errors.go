package errors

import (
	stderrors "errors"
	"fmt"

	errorfamily "github.com/larsartmann/go-error-family"
)

// Sentinel errors classified by error family.
// Rejection = permanent, Transient = retryable, Infrastructure = system-level.
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errorfamily.NewRejection("not_found", "not found")
	// ErrRateLimited indicates the external API rate limit has been exceeded.
	ErrRateLimited = errorfamily.NewTransient("rate_limited", "rate limited")
	// ErrInvalidToken indicates the authentication token is invalid.
	ErrInvalidToken = errorfamily.NewRejection("invalid_token", "invalid token")
	// ErrUserNotFound indicates the specified user does not exist.
	ErrUserNotFound = errorfamily.NewRejection("user_not_found", "user not found")
	// ErrSyncFailed indicates a sync operation failed.
	ErrSyncFailed = errorfamily.NewTransient("sync_failed", "sync failed")
	// ErrDatabase indicates a storage backend error.
	ErrDatabase = errorfamily.NewInfrastructure("database", "database error")
	// ErrInvalidInput indicates a required field is missing or invalid.
	ErrInvalidInput = errorfamily.NewRejection("invalid_input", "invalid input")
	// ErrUnknownBackend indicates an unsupported storage backend was specified.
	ErrUnknownBackend = errorfamily.NewRejection("unknown_backend", "unknown backend")
	// ErrDBNil indicates the database connection is nil.
	ErrDBNil = errorfamily.NewRejection("db_nil", "database is nil")
)

// RegisterErrorTemplates registers user-facing message templates for all error codes.
// Call once at application startup.
func RegisterErrorTemplates() {
	for _, e := range errorEntries {
		errorfamily.RegisterTemplate(e.code, e.tmpl)
	}
}

type errorEntry struct {
	code string
	tmpl errorfamily.MessageTemplate
}

// makeEntry constructs an errorEntry from raw template fields.
// Used internally to keep the static table flat.
func makeEntry(code, what, why, fix, out string) errorEntry {
	return errorEntry{code: code, tmpl: errorfamily.MessageTemplate{What: what, Why: why, Fix: fix, WayOut: out}}
}

//nolint:gochecknoglobals // Static template table; initialized once at startup.
var errorEntries = []errorEntry{
	makeEntry(
		"not_found",
		"The requested resource was not found.",
		"The item or resource you requested does not exist in the system.",
		"Verify the identifier and try again.",
		"Check the logs for the exact resource path.",
	),
	makeEntry(
		"rate_limited",
		"Too many requests — rate limit exceeded.",
		"The external API has received too many requests from this client.",
		"Wait for the rate limit window to reset and retry.",
		"Use --verbose to see the reset time.",
	),
	makeEntry(
		"invalid_token",
		"The provided authentication token is invalid.",
		"The token is missing, expired, or does not have the required permissions.",
		"Set a valid token via GITHUB_TOKEN or the -token flag.",
		"Generate a new personal access token on GitHub.",
	),
	makeEntry(
		"user_not_found",
		"The specified user was not found.",
		"The username does not exist on the provider platform.",
		"Double-check the username spelling.",
		"Try a different username or verify the account exists.",
	),
	makeEntry(
		"sync_failed",
		"The synchronization operation failed.",
		"An unexpected error occurred while fetching or storing items.",
		"Check network connectivity and provider status.",
		"Run with --verbose for detailed error information.",
	),
	makeEntry(
		"database",
		"A database error occurred.",
		"The storage backend returned an error during read or write.",
		"Check the database path and permissions.",
		"Verify the backend configuration and disk space.",
	),
	makeEntry(
		"invalid_input",
		"The input provided is invalid.",
		"A required field is missing or has an unacceptable value.",
		"Review the input and ensure all required fields are set.",
		"See the error detail for the specific missing field.",
	),
	makeEntry(
		"unknown_backend",
		"The specified storage backend is not supported.",
		"Only 'memory' and 'sqlite' backends are currently supported.",
		"Use --backend memory or --backend sqlite.",
		"Check the documentation for supported backends.",
	),
	makeEntry(
		"db_nil",
		"The database connection is nil.",
		"The SQLite backend was selected but no database path was provided.",
		"Set a database path via --db or DB_PATH.",
		"Use --backend memory if you do not need persistence.",
	),
}

// IsRetryable reports whether the error is worth retrying.
// Delegates to errorfamily's intrinsic classification.
func IsRetryable(err error) bool {
	return errorfamily.IsRetryable(err)
}

// wrapPreservingFamily wraps an error with detail, preserving errorfamily
// structure when wrapping an *errorfamily.Error. Falls back to fmt.Errorf
// for plain errors.
func wrapPreservingFamily(err error, detail string) error {
	e, ok := stderrors.AsType[*errorfamily.Error](err)
	if ok {
		return errorfamily.Wrap(e, e.ErrorFamily(), e.Code(), detail)
	}

	return fmt.Errorf("%s: %w", detail, err)
}

// WithDetail wraps err with a detail string for debugging context.
func WithDetail(err error, detail string) error {
	return wrapPreservingFamily(err, detail)
}

// WithUserDetail is a convenience function to add username context.
func WithUserDetail(err error, username string) error {
	return wrapPreservingFamily(err, "username="+username)
}

// Wrap wraps an error with additional context.
func Wrap(err error, message string) error {
	return wrapPreservingFamily(err, message)
}

// Wrapf wraps an error with a formatted message.
func Wrapf(err error, format string, args ...any) error {
	return wrapPreservingFamily(err, fmt.Sprintf(format, args...))
}
