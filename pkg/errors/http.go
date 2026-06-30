package errors

import (
	"context"
	"errors"
	"net/http"

	errorfamily "github.com/larsartmann/go-error-family"
)

// StatusClientClosedRequest is the (nginx-origin) status used when the client
// disconnects before the response is written. It is non-standard but widely
// understood by proxies and observability tooling, and crucially signals "not a
// server fault" — unlike the 503 the Transient fallback would otherwise produce.
const StatusClientClosedRequest = 499

// httpStatusOverrides maps sentinels to HTTP statuses that are more precise than
// their family-level default. go-error-family's Family.HTTPStatus maps every
// Rejection to 400 and every Transient/Infrastructure to 503, but some sentinels
// carry finer-grained semantics (e.g. ErrNotFound is 404, not 400; ErrRateLimited
// is 429, not 503). Anything not listed here inherits its family's status via the
// fallback, so the mapping stays exhaustive by construction as new sentinels are
// added — there is no brittle catch-all default to forget.
//
//nolint:gochecknoglobals // Static read-only override table; consulted via errors.Is, never mutated.
var httpStatusOverrides = map[error]int{
	ErrNotFound:       http.StatusNotFound,            // 404 (Rejection default 400)
	ErrUserNotFound:   http.StatusNotFound,            // 404
	ErrInvalidToken:   http.StatusUnauthorized,        // 401
	ErrInvalidInput:   http.StatusBadRequest,          // 400
	ErrUnknownBackend: http.StatusInternalServerError, // 500 (misconfiguration, not a client error)
	ErrRateLimited:    http.StatusTooManyRequests,     // 429 (Transient default 503)
	ErrDatabase:       http.StatusInternalServerError, // 500 (Infrastructure default 503)
}

// HTTPStatus returns the recommended HTTP response status code for err. It applies
// precise per-sentinel overrides where the family default would be too coarse, then
// falls back to the error-family classification (go-error-family's
// Family.HTTPStatus). Unclassified errors classify as Transient (fail-open for
// retry) and thus map to 503. nil returns 200.
//
// This is the single source of truth for error-to-status translation, reusable by
// HTTP servers, CLI exit-code logic, and workers — instead of a hand-rolled switch
// duplicated per boundary.
func HTTPStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Client-driven cancellation is not a server fault: distinguish it from the
	// Transient(503) fallback so observability and clients read it correctly.
	switch {
	case errors.Is(err, context.Canceled):
		return StatusClientClosedRequest // 499
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout // 504
	}

	for sentinel, status := range httpStatusOverrides {
		if errors.Is(err, sentinel) {
			return status
		}
	}

	return errorfamily.Classify(err).HTTPStatus()
}
