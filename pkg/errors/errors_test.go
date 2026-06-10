package errors

import (
	"errors"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/event/v2"
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
		{"ErrSyncFailed", ErrSyncFailed, "[transient:sync_failed] sync failed"},
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

func TestWithUserDetail(t *testing.T) {
	t.Parallel()

	err := WithUserDetail(ErrUserNotFound, "octocat")
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if !errors.Is(err, ErrUserNotFound) {
		t.Error("expected err to match ErrUserNotFound via errors.Is")
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()

	err := Wrap(ErrSyncFailed, "sync interrupted")
	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if !errors.Is(err, ErrSyncFailed) {
		t.Error("expected err to match ErrSyncFailed via errors.Is")
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
		{ErrSyncFailed, event.Transient, true},
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

	wrappedSync := Wrapf(ErrSyncFailed, "attempt %d", 3)
	if got := event.Classify(wrappedSync); got != event.Transient {
		t.Errorf("Classify(wrapped ErrSyncFailed) = %v, want Transient", got)
	}

	if !event.IsRetryable(wrappedSync) {
		t.Error("IsRetryable(wrapped ErrSyncFailed) = false, want true")
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
		{"WithUserDetail", WithUserDetail(baseErr, "octocat"), "username=octocat: base error"},
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
		{"wrapped transient", Wrap(ErrSyncFailed, "context"), true},
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
		"sync_failed",
		"database",
		"invalid_input",
		"unknown_backend",
		"db_nil",
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
