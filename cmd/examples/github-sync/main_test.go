package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
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

func TestLogFatalAndExit_ExtractsCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{"rate limited", pkgerrors.ErrRateLimited, exitTempFail},
		{"invalid token", pkgerrors.ErrInvalidToken, exitUsage},
		{"database error", pkgerrors.ErrDatabase, exitNoInput},
		{"sync failed", pkgerrors.ErrSyncFailed, exitSoftware},
		{"unknown error", errors.New("something"), exitUnavailable},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := exitCodeForError(tc.err)
			if got != tc.want {
				t.Errorf("exitCodeForError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestLoadConfig_InvalidMaxPages(t *testing.T) {
	t.Setenv("MAX_PAGES", "not-a-number")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid MAX_PAGES")
	}
}

func TestLoadConfig_InvalidBackend(t *testing.T) {
	t.Setenv("BACKEND", "invalid")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Backend != "invalid" {
		t.Errorf("expected Backend=invalid, got %s", cfg.Backend)
	}
}

func TestLoadConfig_Empty(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_USER", "")
	t.Setenv("BACKEND", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Backend != "memory" {
		t.Errorf("expected default Backend=memory, got %s", cfg.Backend)
	}
	if cfg.MaxPages != 10 {
		t.Errorf("expected default MaxPages=10, got %d", cfg.MaxPages)
	}
	if !cfg.Incremental {
		t.Error("expected default Incremental=true")
	}
}

func TestLoadConfig_ConflictAware(t *testing.T) {
	t.Setenv("CONFLICT_AWARE", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ConflictAware {
		t.Error("expected ConflictAware=true")
	}
}

func TestLoadConfig_ShowStats(t *testing.T) {
	t.Setenv("SHOW_STATS", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.ShowStats {
		t.Error("expected ShowStats=true")
	}
}

func TestLoadConfig_Verbose(t *testing.T) {
	t.Setenv("VERBOSE", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Verbose {
		t.Error("expected Verbose=true")
	}
}

func TestLoadConfig_JSONOutput(t *testing.T) {
	t.Setenv("JSON_OUTPUT", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.JSONOutput {
		t.Error("expected JSONOutput=true")
	}
}

func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	printVersion(&buf)

	out := buf.String()
	if !strings.HasPrefix(out, "gh-sync ") {
		t.Errorf("expected version prefix, got %q", out)
	}
	if !strings.Contains(out, "commit:") {
		t.Errorf("expected commit info, got %q", out)
	}
	if !strings.Contains(out, "built:") {
		t.Errorf("expected build date info, got %q", out)
	}
}

func TestPrintSyncResultJSON(t *testing.T) {
	var buf bytes.Buffer

	result := &synclib.SyncResult{
		Fetched: 10,
		Skipped: 2,
		Errors:  1,
	}

	printSyncResultJSONToWriter(result, &buf)

	out := buf.String()
	if !strings.Contains(out, `"fetched": 10`) {
		t.Errorf("expected fetched=10 in output, got %q", out)
	}
	if !strings.Contains(out, `"skipped": 2`) {
		t.Errorf("expected skipped=2 in output, got %q", out)
	}
	if !strings.Contains(out, `"errors": 1`) {
		t.Errorf("expected errors=1 in output, got %q", out)
	}
}
