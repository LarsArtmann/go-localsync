package cqrslint_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

// fixtureCase is a single (source → expected findings) expectation.
type fixtureCase struct {
	name          string
	files         map[string]string // filename → source
	wantRules     []string          // rules that must be present (order-independent)
	wantNoRules   []string          // rules that must be absent
	wantRuleCount int               // exact total count (0 means none); -1 skips check
}

// ruleIDs extracts the sorted set of rule IDs from findings.
func ruleIDs(findings []cqrslint.Finding) map[string]bool {
	set := map[string]bool{}

	for _, finding := range findings {
		set[finding.Rule] = true
	}

	return set
}

// runFixture writes the source map to a temp dir, loads it, runs the analyzer,
// and asserts the expectation.
func runFixture(t *testing.T, tc fixtureCase) {
	t.Helper()

	dir := t.TempDir()

	for name, source := range tc.files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}

		if err := os.WriteFile(full, []byte(source), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	pkg, err := cqrslint.LoadPackage(dir)
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}

	findings := cqrslint.Run(pkg)

	if tc.wantRuleCount >= 0 && len(findings) != tc.wantRuleCount {
		t.Fatalf("want %d findings, got %d: %+v", tc.wantRuleCount, len(findings), findings)
	}

	got := ruleIDs(findings)

	for _, rule := range tc.wantRules {
		if !got[rule] {
			t.Errorf("expected finding %s to be present; got %v", rule, got)
		}
	}

	for _, rule := range tc.wantNoRules {
		if got[rule] {
			t.Errorf("expected finding %s to be absent; got %v", rule, got)
		}
	}
}

func TestLoadPackage_RejectsMissingDir(t *testing.T) {
	_, err := cqrslint.LoadPackage(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected a fs.ErrNotExist-wrapped error, got %v", err)
	}
}

func TestLoadPackage_RejectsNonGoSources(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := cqrslint.LoadPackage(dir)
	if err == nil {
		t.Fatal("expected error for directory with no .go files")
	}

	if !errors.Is(err, cqrslint.ErrNoGoSources) {
		t.Fatalf("expected ErrNoGoSources, got %v", err)
	}
}

func TestLoadPackage_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()

	// Only a _test.go file → should be treated as empty.
	err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte("package cqrs\n"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	_, loadErr := cqrslint.LoadPackage(dir)
	if !errors.Is(loadErr, cqrslint.ErrNoGoSources) {
		t.Fatalf("expected ErrNoGoSources for test-only dir, got %v", loadErr)
	}
}

func TestSortFindings_Stable(t *testing.T) {
	findings := []cqrslint.Finding{
		{Rule: "C0002", File: "a.go", Line: 10},
		{Rule: "C0001", File: "a.go", Line: 10},
		{Rule: "C0001", File: "a.go", Line: 5},
		{Rule: "C0001", File: "b.go", Line: 1},
	}

	cqrslint.SortFindings(findings)

	want := []cqrslint.Finding{
		{Rule: "C0001", File: "a.go", Line: 5},
		{Rule: "C0001", File: "a.go", Line: 10},
		{Rule: "C0002", File: "a.go", Line: 10},
		{Rule: "C0001", File: "b.go", Line: 1},
	}

	for i, expected := range want {
		if findings[i] != expected {
			t.Errorf("index %d: want %+v, got %+v", i, expected, findings[i])
		}
	}
}

func TestFinding_String_NoLine(t *testing.T) {
	f := cqrslint.Finding{Rule: "C0001", Severity: cqrslint.SeverityError, File: "a.go", Message: "bad"}

	want := "a.go: C0001 error: bad"
	if got := f.String(); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestFinding_String_WithLine(t *testing.T) {
	f := cqrslint.Finding{
		Rule: "C0001", Severity: cqrslint.SeverityWarning,
		File: "a.go", Line: 42, Message: "meh",
	}

	want := "a.go:42: C0001 warning: meh"
	if got := f.String(); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestRules_CountAndOrder(t *testing.T) {
	rules := cqrslint.Rules()

	wantCount := 15
	if len(rules) != wantCount {
		t.Fatalf("want %d rules, got %d", wantCount, len(rules))
	}

	wantIDs := []string{
		"C0001", "C0002", "C0003", "C0004", "C0005",
		"C0006", "C0007", "C0008", "C0009", "C0010",
		"C0011", "C0012", "C0013", "C0014", "C0015",
	}

	for i, expected := range wantIDs {
		if rules[i].ID != expected {
			t.Errorf("rule %d: want ID %s, got %s", i, expected, rules[i].ID)
		}
	}
}
