package main

import (
	"errors"
	"testing"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"rate limited", pkgerrors.ErrRateLimited, exitTempFail},
		{"invalid token", pkgerrors.ErrInvalidToken, exitUsage},
		{"user not found", pkgerrors.ErrUserNotFound, exitDataErr},
		{"database error", pkgerrors.ErrDatabase, exitNoInput},
		{"sync failed", pkgerrors.ErrSyncFailed, exitSoftware},
		{"unknown error", errors.New("something unexpected"), exitUnavailable},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := exitCodeForError(tc.err)
			if got != tc.wantCode {
				t.Errorf("exitCodeForError(%v) = %d, want %d", tc.err, got, tc.wantCode)
			}
		})
	}
}

func TestExitCodeForError_Wrapped(t *testing.T) {
	t.Parallel()

	wrapped := pkgerrors.WithDetail(pkgerrors.ErrRateLimited, "retry after 60s")
	got := exitCodeForError(wrapped)
	if got != exitTempFail {
		t.Errorf("expected %d for wrapped ErrRateLimited, got %d", exitTempFail, got)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "" {
		t.Errorf("expected empty Token, got %s", cfg.Token)
	}
	if cfg.Username != "" {
		t.Errorf("expected empty Username, got %s", cfg.Username)
	}
	if cfg.Backend != "memory" {
		t.Errorf("expected Backend=memory, got %s", cfg.Backend)
	}
	if cfg.MaxPages != 10 {
		t.Errorf("expected MaxPages=10, got %d", cfg.MaxPages)
	}
	if !cfg.Incremental {
		t.Error("expected Incremental=true")
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token-123")
	t.Setenv("GITHUB_USER", "testuser")
	t.Setenv("BACKEND", "turso")
	t.Setenv("MAX_PAGES", "5")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "test-token-123" {
		t.Errorf("expected Token=test-token-123, got %s", cfg.Token)
	}
	if cfg.Username != "testuser" {
		t.Errorf("expected Username=testuser, got %s", cfg.Username)
	}
	if cfg.Backend != "turso" {
		t.Errorf("expected Backend=turso, got %s", cfg.Backend)
	}
	if cfg.MaxPages != 5 {
		t.Errorf("expected MaxPages=5, got %d", cfg.MaxPages)
	}
}

func TestAppConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := AppConfig{}
	if cfg.Backend != "" {
		t.Errorf("expected empty Backend before parsing")
	}
	if cfg.MaxPages != 0 {
		t.Errorf("expected 0 MaxPages before parsing")
	}
}
