package main

import (
	"bytes"
	"encoding/json"
	"flag"
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
		if got := buf.String(); got != "localsync-lint: clean\n" {
			t.Errorf("clean summary = %q", got)
		}
	})

	t.Run("mixed counts", func(t *testing.T) {
		var buf bytes.Buffer
		emitSummary(&buf, findingCounts{errors: 2, warnings: 1, suppressed: 3}, outputOptions{}, time.Millisecond)
		want := "localsync-lint: 2 errors, 1 warning, 3 suppressed\n"
		if got := buf.String(); got != want {
			t.Errorf("summary = %q, want %q", got, want)
		}
	})
}

// TestEmitFindings_JSONSchema pins the --json output contract: each line is a
// valid JSON object with rule/severity/file/line/message/suppressed keys.
func TestEmitFindings_JSONSchema(t *testing.T) {
	findings := []cqrslint.Finding{
		{
			Rule:     "C0002",
			Severity: cqrslint.SeverityError,
			File:     "events.go",
			Line:     18,
			Message:  "unexpected event.Type const",
		},
		{
			Rule:       "C0005",
			Severity:   cqrslint.SeverityWarning,
			File:       "decider.go",
			Line:       7,
			Message:    "no fold coverage",
			Suppressed: true,
		},
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

func TestSplitRuleList(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{"empty", "", nil},
		{"single", "C0001", []string{"C0001"}},
		{"multiple", "C0001, C0005 ,,C0009", []string{"C0001", "C0005", "C0009"}},
		{"only separators", " , , ", nil},
	}

	for _, tt := range tests {
		if got := splitRuleList(tt.value); !equalStrings(got, tt.want) {
			t.Errorf("%s: splitRuleList(%q) = %v, want %v", tt.name, tt.value, got, tt.want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestParseRuleSelection_Valid(t *testing.T) {
	selection, err := parseRuleSelection("C0001,C0005", "C0009")
	if err != nil {
		t.Fatalf("expected valid selection, got %v", err)
	}
	if !equalStrings(selection.include, []string{"C0001", "C0005"}) {
		t.Errorf("include = %v", selection.include)
	}
	if !equalStrings(selection.exclude, []string{"C0009"}) {
		t.Errorf("exclude = %v", selection.exclude)
	}
}

func TestParseRuleSelection_UnknownRuleIsUsageError(t *testing.T) {
	if _, err := parseRuleSelection("C9999", ""); err == nil {
		t.Fatal("expected unknown rule ID to be rejected")
	}
}

func TestPrintRuleDetail(t *testing.T) {
	rules := cqrslint.Rules()
	printRuleDetail(&bytes.Buffer{}, rules[0])
}

func TestPrintRulesAndUsage(t *testing.T) {
	fs := flag.NewFlagSet("localsync-lint", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	printRules(&bytes.Buffer{})
	printUsage(fs)
}

func TestEmitFindingGitHub(t *testing.T) {
	var buf bytes.Buffer
	emitFindingGitHub(&buf, cqrslint.Finding{
		Rule: "C0005", Severity: cqrslint.SeverityWarning,
		File: "a.go", Line: 12, Message: "boom",
	})

	want := "::warning file=a.go,line=12,title=C0005::boom\n"
	if buf.String() != want {
		t.Errorf("github annotation = %q, want %q", buf.String(), want)
	}
}

func TestEmitFindings_Formats(t *testing.T) {
	findings := []cqrslint.Finding{
		{Rule: "C0001", Severity: cqrslint.SeverityError, File: "a.go", Line: 1, Message: "one"},
		{Rule: "C0005", Severity: cqrslint.SeverityWarning, File: "b.go", Line: 2, Message: "two", Suppressed: true},
	}

	var out bytes.Buffer
	emitFindings(&out, findings, outputOptions{jsonOut: true, showSuppressed: true})
	if !strings.Contains(out.String(), `"rule":"C0005"`) || !strings.Contains(out.String(), `"suppressed":true`) {
		t.Errorf("json output missing suppressed finding: %s", out.String())
	}

	out.Reset()
	emitFindings(&out, findings, outputOptions{format: "github"})
	if strings.Contains(out.String(), "C0005") {
		t.Error("suppressed finding must be skipped without showSuppressed")
	}
	if !strings.Contains(out.String(), "::error file=a.go,line=1") {
		t.Errorf("github output missing active finding: %s", out.String())
	}

	out.Reset()
	emitFindings(&out, findings, outputOptions{showSuppressed: true})
	if !strings.Contains(out.String(), "two") {
		t.Errorf("text output with showSuppressed must include suppressed: %s", out.String())
	}
}

func TestEmit_QuietIsSilent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	emit(&stdout, &stderr, report{
		findings: []cqrslint.Finding{{Rule: "C0001", Severity: cqrslint.SeverityError, File: "a.go", Line: 1}},
		opts:     outputOptions{quiet: true},
	})

	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("quiet mode wrote to stdout/stderr: %q / %q", stdout.String(), stderr.String())
	}
}

func TestEmitSuppressedByRule(t *testing.T) {
	var buf bytes.Buffer
	emitSuppressedByRule(&buf, []cqrslint.Finding{
		{Rule: "C0005", Suppressed: true},
		{Rule: "C0005", Suppressed: true},
	})

	if !strings.Contains(buf.String(), "C0005=2") {
		t.Errorf("suppressed-by-rule line missing counts: %q", buf.String())
	}

	buf.Reset()
	emitSuppressedByRule(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("no suppressed findings must print nothing, got %q", buf.String())
	}
}

func TestEmitRuleStatusAndHeader(t *testing.T) {
	var buf bytes.Buffer
	emitVerboseHeader(&buf, "./pkg", 3)
	if !strings.Contains(buf.String(), "analyzing ./pkg (3 files") {
		t.Errorf("header: %q", buf.String())
	}

	buf.Reset()
	emitRuleStatus(&buf, []cqrslint.Finding{
		{Rule: "C0001", Severity: cqrslint.SeverityError, File: "a.go", Line: 1},
	})
	if !strings.Contains(buf.String(), "C0001") {
		t.Errorf("rule status missing rule id: %q", buf.String())
	}
}
