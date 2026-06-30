package errors

import (
	"context"
	"errors"
	"net/http"
	"testing"

	errorfamily "github.com/larsartmann/go-error-family"
)

func TestHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		// Precise overrides (finer than family default).
		{"not found", ErrNotFound, http.StatusNotFound},
		{"user not found", ErrUserNotFound, http.StatusNotFound},
		{"invalid token", ErrInvalidToken, http.StatusUnauthorized},
		{"invalid input", ErrInvalidInput, http.StatusBadRequest},
		{"unknown backend", ErrUnknownBackend, http.StatusInternalServerError},
		{"rate limited", ErrRateLimited, http.StatusTooManyRequests},
		{"database", ErrDatabase, http.StatusInternalServerError},
		// Override preserved through wrapping.
		{"wrapped not found", Wrap(ErrNotFound, "source=github"), http.StatusNotFound},
		// Family-level fallback (no override registered).
		{"partial sync -> family Transient default 503", ErrPartialSync, http.StatusServiceUnavailable},
		{"db nil -> family Rejection default 400", ErrDBNil, http.StatusBadRequest},
		// Unclassified errors default to Transient (fail-open) -> 503.
		{"plain error -> 503", errors.New("boom"), http.StatusServiceUnavailable},
		// Client cancellation is not a server fault.
		{"context canceled -> 499", context.Canceled, StatusClientClosedRequest},
		{"context deadline -> 504", context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"wrapped cancel keeps 499", Wrap(context.Canceled, "during list"), StatusClientClosedRequest},
		{"nil -> 200", nil, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := HTTPStatus(tc.err); got != tc.want {
				t.Errorf("HTTPStatus(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

// TestHTTPStatus_MatchesAllSentinels is an exhaustiveness guard: every exported
// sentinel must resolve to a concrete status (never 0) so no error is accidentally
// unmapped at the HTTP boundary.
func TestHTTPStatus_MatchesAllSentinels(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		ErrNotFound, ErrRateLimited, ErrInvalidToken, ErrUserNotFound,
		ErrPartialSync, ErrDatabase, ErrInvalidInput, ErrUnknownBackend, ErrDBNil,
	}

	for _, s := range sentinels {
		t.Run(s.Error(), func(t *testing.T) {
			t.Parallel()

			if got := HTTPStatus(s); got == 0 {
				t.Errorf("sentinel %v resolved to status 0", s)
			}
		})
	}
}

// TestHTTPStatus_FamilyFallbackConsistency verifies the fallback path actually
// consults the library's classification: a raw errorfamily error of a family with
// no override must return that family's HTTPStatus, not a hardcoded number.
func TestHTTPStatus_FamilyFallbackConsistency(t *testing.T) {
	t.Parallel()

	// Conflict family is unused by this SDK but the fallback must honor it (409).
	conflict := errorfamily.NewConflict("test", "test")
	if got := HTTPStatus(conflict); got != http.StatusConflict {
		t.Errorf("HTTPStatus(conflict) = %d, want 409", got)
	}
}
