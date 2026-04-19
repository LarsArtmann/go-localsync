package errors

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	sentinels := []struct {
		name  string
		err   error
		msg   string
	}{
		{"ErrNotFound", ErrNotFound, "not found"},
		{"ErrRateLimited", ErrRateLimited, "rate limited"},
		{"ErrInvalidToken", ErrInvalidToken, "invalid token"},
		{"ErrUserNotFound", ErrUserNotFound, "user not found"},
		{"ErrSyncFailed", ErrSyncFailed, "sync failed"},
		{"ErrDatabase", ErrDatabase, "database error"},
		{"ErrConflict", ErrConflict, "conflict detected"},
		{"ErrInvalidInput", ErrInvalidInput, "invalid input"},
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
