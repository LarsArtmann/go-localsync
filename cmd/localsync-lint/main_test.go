package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

// TestAnalyze_CleanPackage pins the 0-exit contract: the real pkg/cqrs (the
// linter's own target) analyzes without error and without active findings.
func TestAnalyze_CleanPackage(t *testing.T) {
	pkg, findings, err := analyze("../../pkg/cqrs", cqrslint.RunOptions{})
	if err != nil {
		t.Fatalf("analyze pkg/cqrs: %v", err)
	}

	if pkg == nil || len(pkg.Files) == 0 {
		t.Fatal("expected parsed files for pkg/cqrs")
	}

	active := 0

	for _, f := range findings {
		if !f.Suppressed {
			active++
		}
	}

	if active != 0 {
		t.Errorf("pkg/cqrs must be clean under the internal linter, got %d active findings", active)
	}

	if code := exitCode(findings, outputOptions{strict: true}); code != 0 {
		t.Errorf("exitCode: want 0 for clean package, got %d", code)
	}
}

// TestAnalyze_BadTargetIsUsageError pins the 2-exit contract: an
// unparseable/missing target must return an error from analyze (main maps it
// to exit code 2).
func TestAnalyze_BadTargetIsUsageError(t *testing.T) {
	_, _, err := analyze(filepath.Join(t.TempDir(), "does-not-exist"), cqrslint.RunOptions{})
	if err == nil {
		t.Fatal("expected an error for a missing target")
	}
}

// TestExitCode_Contract covers the exit-code decision table with synthetic
// findings: error severity → 1, warning only + strict → 1, warning without
// strict → 0, suppressed findings never count.
func TestExitCode_Contract(t *testing.T) {
	makeFindings := func(severity cqrslint.Severity, suppressed bool) []cqrslint.Finding {
		return []cqrslint.Finding{{Rule: "C0001", Severity: severity, Suppressed: suppressed}}
	}

	cases := []struct {
		name     string
		findings []cqrslint.Finding
		strict   bool
		want     int
	}{
		{"error fails always", makeFindings(cqrslint.SeverityError, false), false, 1},
		{"warning fails only in strict", makeFindings(cqrslint.SeverityWarning, false), true, 1},
		{"warning passes without strict", makeFindings(cqrslint.SeverityWarning, false), false, 0},
		{"suppressed error passes", makeFindings(cqrslint.SeverityError, true), true, 0},
		{"no findings passes", nil, true, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCode(tc.findings, outputOptions{strict: tc.strict}); got != tc.want {
				t.Errorf("exitCode = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestCountFindings partitions findings into error/warning/suppressed buckets.
func TestCountFindings(t *testing.T) {
	findings := []cqrslint.Finding{
		{Rule: "C0001", Severity: cqrslint.SeverityError},
		{Rule: "C0002", Severity: cqrslint.SeverityWarning},
		{Rule: "C0003", Severity: cqrslint.SeverityError, Suppressed: true},
		{Rule: "C0004", Severity: cqrslint.SeverityWarning},
	}

	got := countFindings(findings)

	if got.errors != 1 || got.warnings != 2 || got.suppressed != 1 {
		t.Errorf("countFindings = %+v, want errors=1 warnings=2 suppressed=1", got)
	}
}

// TestEmitSummary_Variants asserts the summary lines for the clean, findings,
// and suppressed-only cases.
func TestEmitSummary_Variants(t *testing.T) {
	t.Run("clean", func(t *testing.T) {
		var buf bytes.Buffer
		emitSummary(&buf, findingCounts{}, outputOptions{}, time.Millisecond)
		if got := buf.String(); got != "cqrs-lint: clean\n" {
			t.Errorf("clean summary = %q", got)
		}
	})

	t.Run("mixed counts", func(t *testing.T) {
		var buf bytes.Buffer
		emitSummary(&buf, findingCounts{errors: 2, warnings: 1, suppressed: 3}, outputOptions{}, time.Millisecond)
		want := "cqrs-lint: 2 errors, 1 warning, 3 suppressed\n"
		if got := buf.String(); got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}
	})
}

// TestEmitFindings_JSONSchema pins the --json output contract: each line is a
// valid JSON object with rule/severity/file/line/message/suppressed keys.
func TestEmitFindings_JSONSchema(t *testing.T) {
	findings := []cqrslint.Finding{
		{Rule: "C0002", Severity: cqrslint.SeverityError, File: "events.go", Line: 18, Message: "unexpected event.Type const"},
		{Rule: "C0005", Severity: cqrslint.SeverityWarning, File: "decider.go", Line: 7, Message: "no fold coverage", Suppressed: true},
	}

	var buf bytes.Buffer
	emitFindings(&buf, findings, outputOptions{jsonOut: true})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("suppressed findings must be hidden without --show-suppressed, got %d lines", len(lines))
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("--json line is not valid JSON: %v\nline: %s", err, lines[0])
	}

	for _, key := range []string{"rule", "severity", "file", "line", "message", "suppressed"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("--json object missing key %q: %s", key, lines[0])
		}
	}

	var bufAll bytes.Buffer
	emitFindings(&bufAll, findings, outputOptions{jsonOut: true, showSuppressed: true})
	if lines := strings.Split(strings.TrimSpace(bufAll.String()), "\n"); len(lines) != 2 {
		t.Errorf("--show-suppressed must emit both findings, got %d lines", len(lines))
	}
}

// TestEmitVerboseHeaderAndRuleStatus asserts the --verbose stderr contract.
func TestEmitVerboseHeaderAndRuleStatus(t *testing.T) {
	var buf bytes.Buffer
	emitVerboseHeader(&buf, "pkg/cqrs", 9)
	if !strings.Contains(buf.String(), "analyzing pkg/cqrs (9 files,") {
		t.Errorf("verbose header = %q", buf.String())
	}

	buf.Reset()
	emitRuleStatus(&buf, []cqrslint.Finding{
		{Rule: "C0001", Severity: cqrslint.SeverityError, File: "x.go", Line: 1, Message: "m"},
		{Rule: "C0001", Severity: cqrslint.SeverityError, File: "x.go", Line: 2, Message: "m", Suppressed: true},
	})

	if !strings.Contains(buf.String(), "C0001") || !strings.Contains(buf.String(), "1 finding") {
		t.Errorf("rule status should list C0001 with 1 active finding, got:\n%s", buf.String())
	}
}

// TestAnalyze_ViolatingFixture proves a real violation is detected end to end:
// a copied events.go with an extra event.Type const must produce C0002 and a
// non-zero exit code, and a suppression directive must silence it again.
func TestAnalyze_ViolatingFixture(t *testing.T) {
	dir := t.TempDir()

	original, err := os.ReadFile("../../pkg/cqrs/events.go")
	if err != nil {
		t.Fatalf("read events.go: %v", err)
	}

	violating := strings.Replace(
		string(original),
		`const (`,
		`const (
	EventItemBogus event.Type = "sync_item.bogus"`,
		1,
	)

	// Keep the package name the linter expects.
	if err := os.WriteFile(filepath.Join(dir, "events.go"), []byte(violating), 0o600); err != nil {
		t.Fatal(err)
	}

	_, findings, err := analyze(dir, cqrslint.RunOptions{})
	if err != nil {
		t.Fatalf("analyze fixture: %v", err)
	}

	found := false

	for _, f := range findings {
		if f.Rule == "C0002" && !f.Suppressed {
			found = true
		}
	}

	if !found {
		t.Fatal("expected an unsuppressed C0002 finding for the extra event const")
	}

	if code := exitCode(findings, outputOptions{}); code != 1 {
		t.Errorf("exitCode = %d, want 1 for a violating package", code)
	}

	suppressed := strings.Replace(violating, `EventItemBogus event.Type = "sync_item.bogus"`,
		"//cqrs-lint:ignore C0002 fixture suppression\n\tEventItemBogus event.Type = \"sync_item.bogus\"", 1)

	supDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(supDir, "events.go"), []byte(suppressed), 0o600); err != nil {
		t.Fatal(err)
	}

	_, supFindings, err := analyze(supDir, cqrslint.RunOptions{})
	if err != nil {
		t.Fatalf("analyze suppressed fixture: %v", err)
	}

	if code := exitCode(supFindings, outputOptions{}); code != 0 {
		t.Errorf("exitCode after suppression = %d, want 0", code)
	}
}
