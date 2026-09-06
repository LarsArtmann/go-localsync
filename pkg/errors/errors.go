package errors

import (
	stderrors "errors"
	"fmt"
	"sync"

	errorfamily "github.com/larsartmann/go-error-family"
)

// Sentinel errors classified by error family.
// Rejection = permanent, Transient = retryable, Infrastructure = system-level.
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound error = errorfamily.NewRejection("not_found", "not found")
	// ErrRateLimited indicates the external API rate limit has been exceeded.
	ErrRateLimited error = errorfamily.NewTransient("rate_limited", "rate limited")
	// ErrInvalidToken indicates the authentication token is invalid.
	ErrInvalidToken error = errorfamily.NewRejection("invalid_token", "invalid token")
	// ErrUserNotFound indicates the specified user does not exist.
	ErrUserNotFound error = errorfamily.NewRejection("user_not_found", "user not found")
	// ErrPartialSync indicates a sync run completed but some individual items failed to persist.
	// Transient (retryable): re-running the sync retries the failed items. Consumers can detect
	// this outcome with errors.Is(err, ErrPartialSync).
	ErrPartialSync error = errorfamily.NewTransient("partial_sync", "sync completed with item errors")
	// ErrProviderUnavailable indicates the provider's API could not be reached
	// or kept failing (transport errors, exhausted 5xx retries). Transient
	// (retryable): the provider may recover without any change on our side.
	ErrProviderUnavailable error = errorfamily.NewTransient("provider_unavailable", "provider unavailable")
	// ErrDatabase indicates a storage backend error.
	ErrDatabase error = errorfamily.NewInfrastructure("database", "database error")
	// ErrInvalidInput indicates a required field is missing or invalid.
	ErrInvalidInput error = errorfamily.NewRejection("invalid_input", "invalid input")
	// ErrUnknownBackend indicates an unsupported storage backend was specified.
	ErrUnknownBackend error = errorfamily.NewRejection("unknown_backend", "unknown backend")
	// ErrDBNil indicates the database connection is nil.
	ErrDBNil error = errorfamily.NewRejection("db_nil", "database is nil")
)

// templatesRegistered guarantees RegisterErrorTemplates runs exactly once, so the
// global template table is populated exactly once even under concurrent first use.
//
//nolint:gochecknoglobals // Package-level idempotency guard; initialized once, never mutated thereafter.
var templatesRegistered sync.Once

// RegisterErrorTemplates registers user-facing message templates for all error codes.
// Safe to call any number of times. It is invoked from api.NewServer so every
// production binary populates the template table without any other caller action;
// tests and non-server consumers may call it directly.
func RegisterErrorTemplates() {
	templatesRegistered.Do(func() {
		for _, e := range errorEntries {
			errorfamily.RegisterTemplate(e.code, e.tmpl)
		}
	})
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
		"provider_unavailable",
		"The data source is temporarily unavailable.",
		"The external provider API could not be reached or kept failing.",
		"Wait briefly and retry; the provider usually recovers on its own.",
		"If it persists, check the provider's status page.",
	),
	makeEntry(
		"rate_limited",
		"Too many requests — rate limit exceeded.",
		"The external API has received too many requests from this client.",
		"Wait for the rate-limit window to reset and retry.",
		"Inspect the rate-limit reset time returned by the provider.",
	),
	makeEntry(
		"invalid_token",
		"The provided authentication token is invalid.",
		"The token is missing, expired, or does not have the required permissions.",
		"Set a valid authentication token via your provider's configuration.",
		"Generate a fresh access token for your provider.",
	),
	makeEntry(
		"user_not_found",
		"The specified user was not found.",
		"The username does not exist on the provider platform.",
		"Double-check the username spelling.",
		"Try a different username or verify the account exists.",
	),
	makeEntry(
		"partial_sync",
		"The synchronization finished, but some items could not be stored.",
		"One or more items failed validation or persistence during the sync run.",
		"Re-run the sync; only the failed items are retried.",
		"Inspect the per-item errors returned in the sync result.",
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
		"Use a supported backend name ('memory' or 'sqlite').",
		"Check the backend configuration.",
	),
	makeEntry(
		"db_nil",
		"The database connection is nil.",
		"The SQLite backend was selected but no database path was provided.",
		"Provide a database path when using the sqlite backend.",
		"Use the memory backend if persistence is not required.",
	),
	// Template for the crdt-owned sentinel crdt.ErrNilTimestampFunc (code below).
	// The sentinel stays in pkg/crdt to keep that package dependency-light; the
	// user-facing message lives here in the central catalog, per the errorfamily
	// "sentinel anywhere, messages centralized" pattern.
	makeEntry(
		"sync.resolver.nil_timestamp_func",
		"The conflict resolver was misconfigured.",
		"An LWW resolver was created without a timestamp function, so it cannot order conflicting versions.",
		"Provide a non-nil timestamp function when constructing the resolver.",
		"Use crdt.NewLWWResolver with a valid TimestampFunc.",
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
	return Wrap(err, detail)
}

// Wrap wraps an error with additional context.
func Wrap(err error, message string) error {
	return wrapPreservingFamily(err, message)
}

// Wrapf wraps an error with a formatted message.
func Wrapf(err error, format string, args ...any) error {
	return wrapPreservingFamily(err, fmt.Sprintf(format, args...))
}

// WithCtx attaches a structured key/value pair to err, preserving the error
// family, code, and errors.Is identity. Unlike WithDetail (which mashes context
// into the message string), WithCtx stores it as structured data reachable via
// (*errorfamily.Error).ErrorContext(), so it flows to structured logs and message
// templates. The underlying sentinel is never mutated — errorfamily clones on
// attach. Plain (non-errorfamily) errors fall back to a "key=value" message wrap.
func WithCtx(err error, key, value string) error {
	if e, ok := stderrors.AsType[*errorfamily.Error](err); ok {
		return e.WithContext(key, value)
	}

	return wrapPreservingFamily(err, key+"="+value)
}

// WithCtxf is the formatted variant of WithCtx.
func WithCtxf(err error, key, format string, args ...any) error {
	if e, ok := stderrors.AsType[*errorfamily.Error](err); ok {
		return e.WithContextf(key, format, args...)
	}

	return wrapPreservingFamily(err, key+"="+fmt.Sprintf(format, args...))
}

// InvalidField wraps ErrInvalidInput with a human-readable reason and a
// structured "field" key. Callers can display reason via Error() while handlers
// can programmatically locate the offending field via ErrorContext()["field"].
// Use it from model/provider Validate functions so every validation error is
// both human-readable and field-addressable.
func InvalidField(field, reason string) error {
	return WithCtx(WithDetail(ErrInvalidInput, reason), "field", field)
}
