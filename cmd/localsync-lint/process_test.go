package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary compiles the localsync-lint CLI into a temp dir so tests exercise
// the real process surface: flag parsing, exit codes, and byte-level output.
func buildBinary(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "localsync-lint-process-bin")

	build := exec.CommandContext(context.Background(), "go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cqrs-lint binary: %v\n%s", err, out)
	}

	return bin
}

// writeFixtureDir copies pkg/cqrs/events.go into a fresh package dir and
// optionally rewrites it, mirroring the in-process fixture used by
// TestAnalyze_ViolatingFixture but for process-level runs.
func writeFixtureDir(t *testing.T, mutate func(string) string) string {
	t.Helper()

	original, err := os.ReadFile("../../pkg/cqrs/events.go")
	if err != nil {
		t.Fatalf("read events.go: %v", err)
	}

	src := string(original)
	if mutate != nil {
		src = mutate(src)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "events.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir
}

func runBinary(t *testing.T, bin string, args ...string) (int, string) {
	t.Helper()

	cmd := exec.CommandContext(context.Background(), bin, args...)

	out, err := cmd.CombinedOutput()

	exit := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exit = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, out)
	}

	return exit, string(out)
}

// extraEventConst turns the compliant fixture into a C0002 violation.
func extraEventConst(src string) string {
	return strings.Replace(
		src,
		"const (",
		"const (\nEventItemBogus event.Type = \"sync_item.bogus\"",
		1,
	)
}

// staleDirective adds a suppression naming a rule that does not exist — a
// WARNING per the suppression audit trail (fails only under --strict).
func staleDirective(src string) string {
	return src + "\n//cqrs-lint:ignore C9999 reason\n"
}

func TestProcess_ExitCode_CleanPackage(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, nil)

	exit, out := runBinary(t, bin, "-pkg", dir)
	if exit != 0 {
		t.Errorf("clean fixture: exit = %d, want 0\n%s", exit, out)
	}
}

func TestProcess_ExitCode_ViolatingPackage(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, extraEventConst)

	exit, out := runBinary(t, bin, "-pkg", dir)
	if exit != 1 {
		t.Errorf("violating fixture: exit = %d, want 1\n%s", exit, out)
	}

	if !strings.Contains(out, "C0002") {
		t.Errorf("violating fixture output must name C0002:\n%s", out)
	}
}

func TestProcess_ExitCode_MissingPackageIsUsageError(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	exit, out := runBinary(t, bin, "-pkg", filepath.Join(t.TempDir(), "does-not-exist"))
	if exit != 2 {
		t.Errorf("missing package: exit = %d, want 2\n%s", exit, out)
	}
}

func TestProcess_StrictFailsOnWarningOnly(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, staleDirective)

	if exit, out := runBinary(t, bin, "-pkg", dir); exit != 0 {
		t.Errorf("warning-only fixture without --strict: exit = %d, want 0\n%s", exit, out)
	}

	if exit, out := runBinary(t, bin, "-pkg", dir, "-strict"); exit != 1 {
		t.Errorf("warning-only fixture with --strict: exit = %d, want 1\n%s", exit, out)
	}

	if _, out := runBinary(t, bin, "-pkg", dir, "-strict"); !strings.Contains(out, "C9999") {
		t.Errorf("--strict output must surface the unknown-rule warning:\n%s", out)
	}
}

func TestProcess_JSONOutputShape(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, extraEventConst)

	exit, out := runBinary(t, bin, "-pkg", dir, "-json")
	if exit != 1 {
		t.Fatalf("violating fixture with -json: exit = %d, want 1\n%s", exit, out)
	}

	seen := 0

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if !strings.HasPrefix(line, "{") {
			continue // summary/progress lines share the stream
		}

		var finding struct {
			Rule       string `json:"rule"`
			Severity   string `json:"severity"`
			File       string `json:"file"`
			Line       int    `json:"line"`
			Message    string `json:"message"`
			Suppressed bool   `json:"suppressed"`
		}

		if err := json.Unmarshal([]byte(line), &finding); err != nil {
			t.Errorf("-json line is not a finding object: %q (%v)", line, err)

			continue
		}

		if finding.Rule == "" || finding.Message == "" {
			t.Errorf("JSON finding missing required fields: %s", line)

			continue
		}

		// Code-level findings carry position; package-level checks
		// (e.g. C0003 on a fixture without a fold) legitimately have
		// empty file/line.
		if finding.Rule == "C0002" && (finding.File == "" || finding.Line == 0) {
			t.Errorf("C0002 finding must be positioned: %s", line)

			continue
		}

		seen++
	}

	if seen == 0 {
		t.Error("-json run produced no finding objects")
	}
}

func TestProcess_VersionFlag(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	exit, out := runBinary(t, bin, "-version")
	if exit != 0 {
		t.Fatalf("-version: exit = %d, want 0", exit)
	}

	if !strings.HasPrefix(out, "cqrs-lint ") {
		t.Errorf("-version output = %q, want 'cqrs-lint <version>' prefix", out)
	}
}

func TestProcess_QuietSuppressesOutput(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, extraEventConst)

	exit, out := runBinary(t, bin, "-pkg", dir, "-quiet")
	if exit != 1 {
		t.Fatalf("-quiet on violations: exit = %d, want 1", exit)
	}

	if strings.TrimSpace(out) != "" {
		t.Errorf("-quiet must suppress all output, got:\n%s", out)
	}
}

func TestProcess_GitHubFormatAnnotations(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, extraEventConst)

	exit, out := runBinary(t, bin, "-pkg", dir, "-format=github")
	if exit != 1 {
		t.Fatalf("-format=github on violations: exit = %d, want 1", exit)
	}

	if !strings.Contains(out, "::error file=") {
		t.Errorf("github format must emit error annotations:\n%s", out)
	}

	if !strings.Contains(out, "title=C0002") {
		t.Errorf("github annotations must carry the rule title:\n%s", out)
	}
}

func TestProcess_BadFormatIsUsageError(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, nil)

	exit, _ := runBinary(t, bin, "-pkg", dir, "-format=yaml")
	if exit != 2 {
		t.Errorf("unknown -format: exit = %d, want 2", exit)
	}
}

func TestProcess_SuppressedCountsInVerbose(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	// A directive silencing the C0002 violation: --verbose must surface the
	// per-rule suppressed count.
	dir := writeFixtureDir(t, func(src string) string {
		return strings.Replace(
			src,
			`EventItemSynced event.Type = "sync_item.synced"`,
			"EventItemSynced event.Type = \"sync_item.synced\" //cqrs-lint:ignore C0002 test-silence",
			1,
		)
	})

	// The suppress directive must sit on the LINE the finding targets. The
	// C0002 finding targets the const block; add a file-level suppression
	// instead, which the parser maps reliably.
	dir2 := writeFixtureDir(t, func(src string) string {
		return extraEventConst(src) + "\n//cqrs-lint:ignore-file C0002 pinned for the verbose-count assertion\n"
	})

	if exit, out := runBinary(t, bin, "-pkg", dir2, "-verbose"); exit != 0 {
		t.Fatalf("suppressed-only fixture: exit = %d, want 0\n%s", exit, out)
	} else if !strings.Contains(out, "suppressed by rule:") || !strings.Contains(out, "C0002=1") {
		t.Errorf("--verbose must list per-rule suppressed counts:\n%s", out)
	}

	_ = dir
}

// suppressedEventConst turns the compliant fixture into a C0002 violation
// that an inline directive silences: the default run must exit 0, proving
// the --no-suppress contrast is real.
func suppressedEventConst(src string) string {
	return strings.Replace(
		src,
		"const (",
		"const (\n//cqrs-lint:ignore C0002 deliberate\nEventItemBogus event.Type = \"sync_item.bogus\"",
		1,
	)
}

func TestProcess_RulesSubset(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, extraEventConst)

	if exit, out := runBinary(t, bin, "-pkg", dir, "-rules", "C0002"); exit != 1 {
		t.Errorf("-rules C0002 on C0002 violation: exit = %d, want 1\n%s", exit, out)
	}

	if exit, out := runBinary(t, bin, "-pkg", dir, "-rules", "C0006"); exit != 0 {
		t.Errorf("-rules C0006 excludes the C0002 check: exit = %d, want 0\n%s", exit, out)
	}
}

func TestProcess_ExcludeRules(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, extraEventConst)

	if exit, out := runBinary(t, bin, "-pkg", dir, "-exclude-rules", "C0002"); exit != 0 {
		t.Errorf("-exclude-rules C0002: exit = %d, want 0\n%s", exit, out)
	}
}

func TestProcess_UnknownRuleSelectionIsUsageError(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, nil)

	if exit, out := runBinary(t, bin, "-pkg", dir, "-rules", "C9999"); exit != 2 {
		t.Errorf("-rules C9999: exit = %d, want 2\n%s", exit, out)
	}

	if exit, out := runBinary(t, bin, "-pkg", dir, "-exclude-rules", "C9999"); exit != 2 {
		t.Errorf("-exclude-rules C9999: exit = %d, want 2\n%s", exit, out)
	}
}

func TestProcess_NoSuppress(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)
	dir := writeFixtureDir(t, suppressedEventConst)

	if exit, out := runBinary(t, bin, "-pkg", dir); exit != 0 {
		t.Errorf("suppressed violation: exit = %d, want 0 without -no-suppress\n%s", exit, out)
	}

	if exit, out := runBinary(t, bin, "-pkg", dir, "-no-suppress"); exit != 1 {
		t.Errorf("suppressed violation with -no-suppress: exit = %d, want 1\n%s", exit, out)
	}

	if _, out := runBinary(t, bin, "-pkg", dir, "-no-suppress"); !strings.Contains(out, "C0002") {
		t.Errorf("-no-suppress output must surface the C0002 finding:\n%s", out)
	}
}

func TestProcess_Explain(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	if exit, out := runBinary(t, bin, "-explain", "C0005"); exit != 0 {
		t.Errorf("-explain C0005: exit = %d, want 0\n%s", exit, out)
	} else if !strings.Contains(out, "provider-agnostic") || !strings.Contains(out, "ADR-0007") {
		t.Errorf("-explain C0005 must print title and rationale:\n%s", out)
	}

	if exit, out := runBinary(t, bin, "-explain", "C9999"); exit != 2 {
		t.Errorf("-explain C9999: exit = %d, want 2\n%s", exit, out)
	}
}
