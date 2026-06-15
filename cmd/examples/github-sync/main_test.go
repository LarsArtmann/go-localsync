package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func assertContains(t *testing.T, out, want, label string) {
	t.Helper()

	if !strings.Contains(out, want) {
		t.Errorf("expected %s in output, got %q", label, out)
	}
}

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
	testutil.AssertEqual(t, cfg.Backend, "memory", "Backend")
	testutil.AssertEqual(t, cfg.MaxPages, 10, "MaxPages")
	if !cfg.Incremental {
		t.Error("expected Incremental=true")
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token-123")
	t.Setenv("GITHUB_USER", "testuser")
	t.Setenv("BACKEND", "sqlite")
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
	testutil.AssertEqual(t, cfg.Backend, "sqlite", "Backend")
	testutil.AssertEqual(t, cfg.MaxPages, 5, "MaxPages")
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
	testutil.AssertEqual(t, cfg.Backend, "invalid", "Backend")
}

func TestLoadConfig_Empty(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_USER", "")
	t.Setenv("BACKEND", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	testutil.AssertEqual(t, cfg.Backend, "memory", "Backend")
	testutil.AssertEqual(t, cfg.MaxPages, 10, "MaxPages")
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
	assertContains(t, out, "commit:", "commit info")
	assertContains(t, out, "built:", "build date info")
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
	assertContains(t, out, `"fetched": 10`, "fetched=10")
	assertContains(t, out, `"skipped": 2`, "skipped=2")
	assertContains(t, out, `"errors": 1`, "errors=1")
}
