package errors

import (
	"errors"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/event/v3"
	errorfamily "github.com/larsartmann/go-error-family"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	sentinels := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrNotFound", ErrNotFound, "[rejection:not_found] not found"},
		{"ErrRateLimited", ErrRateLimited, "[transient:rate_limited] rate limited"},
		{"ErrInvalidToken", ErrInvalidToken, "[rejection:invalid_token] invalid token"},
		{"ErrUserNotFound", ErrUserNotFound, "[rejection:user_not_found] user not found"},
		{"ErrPartialSync", ErrPartialSync, "[transient:partial_sync] sync completed with item errors"},
		{"ErrDatabase", ErrDatabase, "[infrastructure:database] database error"},
		{"ErrInvalidInput", ErrInvalidInput, "[rejection:invalid_input] invalid input"},
		{"ErrUnknownBackend", ErrUnknownBackend, "[rejection:unknown_backend] unknown backend"},
		{"ErrDBNil", ErrDBNil, "[rejection:db_nil] database is nil"},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			if s.err.Error() != s.msg {
				t.Errorf("expected %q, got %q", s.msg, s.err.Error())
			}
		})
	}
}

func TestWithDetail(t *testing.T) {
	t.Parallel()

	err := WithDetail(ErrNotFound, "resource=events")
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if !errors.Is(err, ErrNotFound) {
		t.Error("expected err to match ErrNotFound via errors.Is")
	}
}

func TestWithDetail_PreservesIs(t *testing.T) {
	t.Parallel()

	err := WithDetail(ErrUserNotFound, "username=octocat")
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if !errors.Is(err, ErrUserNotFound) {
		t.Error("expected err to match ErrUserNotFound via errors.Is")
	}
}

func TestWithCtx_AttachesStructuredContext(t *testing.T) {
	t.Parallel()

	err := WithCtx(ErrNotFound, "source", "github")

	// errors.Is identity is preserved through context attachment.
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("expected err to match ErrNotFound via errors.Is")
	}

	ef, ok := errors.AsType[*errorfamily.Error](err)
	if !ok {
		t.Fatal("expected an *errorfamily.Error to read context")
	}

	if got := ef.ErrorContext()["source"]; got != "github" {
		t.Errorf("expected context[source]=github, got %q", got)
	}

	// The original sentinel must not be mutated by attaching context.
	if len(ErrNotFound.ErrorContext()) != 0 {
		t.Error("attaching context mutated the shared ErrNotFound sentinel")
	}
}

func TestWithCtxf(t *testing.T) {
	t.Parallel()

	err := WithCtxf(ErrDatabase, "attempt", "%d of %d", 2, 3)
	ef, ok := errors.AsType[*errorfamily.Error](err)
	if !ok {
		t.Fatal("expected an *errorfamily.Error to read context")
	}

	if got := ef.ErrorContext()["attempt"]; got != "2 of 3" {
		t.Errorf("expected context[attempt]='2 of 3', got %q", got)
	}
}

func TestInvalidField(t *testing.T) {
	t.Parallel()

	err := InvalidField("externalId", "item.ExternalID is required")

	if !errors.Is(err, ErrInvalidInput) {
		t.Error("expected InvalidField to match ErrInvalidInput")
	}

	ef, ok := errors.AsType[*errorfamily.Error](err)
	if !ok {
		t.Fatal("expected an *errorfamily.Error to read context")
	}

	if got := ef.ErrorContext()["field"]; got != "externalId" {
		t.Errorf("expected context[field]=externalId, got %q", got)
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()

	err := Wrap(ErrPartialSync, "sync interrupted")
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if !errors.Is(err, ErrPartialSync) {
		t.Error("expected err to match ErrPartialSync via errors.Is")
	}
}

func TestErrorClassification(t *testing.T) {
	t.Parallel()

	classifications := []struct {
		err       error
		family    event.Family
		retryable bool
	}{
		{ErrNotFound, event.Rejection, false},
		{ErrRateLimited, event.Transient, true},
		{ErrInvalidToken, event.Rejection, false},
		{ErrUserNotFound, event.Rejection, false},
		{ErrPartialSync, event.Transient, true},
		{ErrDatabase, event.Infrastructure, false},
		{ErrInvalidInput, event.Rejection, false},
		{ErrUnknownBackend, event.Rejection, false},
		{ErrDBNil, event.Rejection, false},
	}

	for _, tc := range classifications {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()

			if got := event.Classify(tc.err); got != tc.family {
				t.Errorf("Classify(%v) = %v, want %v", tc.err, got, tc.family)
			}

			if got := event.IsRetryable(tc.err); got != tc.retryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tc.err, got, tc.retryable)
			}
		})
	}
}

func TestErrorClassification_ThroughWrapping(t *testing.T) {
	t.Parallel()

	wrapped := WithDetail(ErrNotFound, "resource=events")
	if got := event.Classify(wrapped); got != event.Rejection {
		t.Errorf("Classify(wrapped ErrNotFound) = %v, want Rejection", got)
	}

	wrappedSync := Wrapf(ErrPartialSync, "attempt %d", 3)
	if got := event.Classify(wrappedSync); got != event.Transient {
		t.Errorf("Classify(wrapped ErrPartialSync) = %v, want Transient", got)
	}

	if !event.IsRetryable(wrappedSync) {
		t.Error("IsRetryable(wrapped ErrPartialSync) = false, want true")
	}
}

func TestWrapping_NonErrorFamily(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("base error")

	tests := []struct {
		name    string
		wrapped error
		wantMsg string
	}{
		{"WithDetail", WithDetail(baseErr, "extra context"), "extra context: base error"},
		{"Wrap", Wrap(baseErr, "context"), "context: base error"},
		{"Wrapf", Wrapf(baseErr, "attempt %d", 3), "attempt 3: base error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(tc.wrapped, baseErr) {
				t.Error("expected wrapped to match original via errors.Is")
			}
			if tc.wrapped.Error() != tc.wantMsg {
				t.Errorf("expected %q, got %q", tc.wantMsg, tc.wrapped.Error())
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"transient error", ErrRateLimited, true},
		{"rejection error", ErrNotFound, false},
		{"infrastructure error", ErrDatabase, false},
		{"wrapped transient", Wrap(ErrPartialSync, "context"), true},
		{"wrapped rejection", WithDetail(ErrInvalidToken, "detail"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := IsRetryable(tc.err); got != tc.retryable {
				t.Errorf("IsRetryable(%v) = %v, want %v", tc.err, got, tc.retryable)
			}
		})
	}
}

func TestRegisteredTemplates(t *testing.T) {
	t.Parallel()

	RegisterErrorTemplates()
	codes := []string{
		"not_found",
		"rate_limited",
		"invalid_token",
		"user_not_found",
		"partial_sync",
		"database",
		"invalid_input",
		"unknown_backend",
		"db_nil",
		"sync.resolver.nil_timestamp_func",
	}

	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			t.Parallel()

			result := errorfamily.HandleErrorDetailed(errorfamily.NewRejection(code, "test"))
			if result == nil {
				t.Fatal("expected non-nil HandleResult")
			}

			if result.Message == "" {
				t.Errorf("expected non-empty Message for code %q", code)
			}
		})
	}
}

// TestRegisterErrorTemplatesIsIdempotent guards the sync.Once guard: calling
// RegisterErrorTemplates repeatedly must be safe and must populate templates
// (production wiring is api.NewServer, which calls this once).
func TestRegisterErrorTemplatesIsIdempotent(t *testing.T) {
	t.Parallel()

	// Must not panic on the second call.
	RegisterErrorTemplates()
	RegisterErrorTemplates()

	result := errorfamily.HandleErrorDetailed(ErrNotFound)
	if result == nil || result.Message == "" {
		t.Error("expected templates to remain populated after repeated registration")
	}
}
