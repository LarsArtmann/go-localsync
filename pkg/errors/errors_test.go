package errors

import (
	"errors"
	"testing"

	"github.com/larsartmann/go-cqrs-lite/core/event"
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

func TestWithDetail_NonErrorFamily(t *testing.T) {
	t.Parallel()

	original := errors.New("base error")
	wrapped := WithDetail(original, "extra context")

	if !errors.Is(wrapped, original) {
		t.Error("expected wrapped to match original via errors.Is")
	}
	if wrapped.Error() != "extra context: base error" {
		t.Errorf("expected 'extra context: base error', got %q", wrapped.Error())
	}
}

func TestWithUserDetail_NonErrorFamily(t *testing.T) {
	t.Parallel()

	original := errors.New("base error")
	wrapped := WithUserDetail(original, "octocat")

	if !errors.Is(wrapped, original) {
		t.Error("expected wrapped to match original via errors.Is")
	}
	if wrapped.Error() != "username=octocat: base error" {
		t.Errorf("expected 'username=octocat: base error', got %q", wrapped.Error())
	}
}

func TestWrap_NonErrorFamily(t *testing.T) {
	t.Parallel()

	original := errors.New("base error")
	wrapped := Wrap(original, "context")

	if !errors.Is(wrapped, original) {
		t.Error("expected wrapped to match original via errors.Is")
	}
	if wrapped.Error() != "context: base error" {
		t.Errorf("expected 'context: base error', got %q", wrapped.Error())
	}
}

func TestWrapf_NonErrorFamily(t *testing.T) {
	t.Parallel()

	original := errors.New("base error")
	wrapped := Wrapf(original, "attempt %d", 3)

	if !errors.Is(wrapped, original) {
		t.Error("expected wrapped to match original via errors.Is")
	}
	if wrapped.Error() != "attempt 3: base error" {
		t.Errorf("expected 'attempt 3: base error', got %q", wrapped.Error())
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
