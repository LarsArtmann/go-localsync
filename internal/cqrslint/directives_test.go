package cqrslint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

// directiveCase is one row of the suppression-directive matrix: a mutation of
// suppressibleSource() (2 C0005 findings on the hasChanged `if` line) plus
// the expected outcome for C0005.
type directiveCase struct {
	name           string
	source         string
	wantSuppressed bool
	wantTotal      int // total findings in the run (-1 = skip)
	wantWarning    int // expected directive-audit warnings (-1 = skip)
}

// runDirectiveCase loads the source as a one-file package and asserts the
// C0005 suppression state plus warning count.
func runDirectiveCase(t *testing.T, tc directiveCase) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cqrs.go"), []byte(tc.source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	pkg, err := cqrslint.LoadPackage(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	findings := cqrslint.Run(pkg)

	var c0005 *cqrslint.Finding

	warnings := 0

	for i := range findings {
		if findings[i].Rule == "C0005" {
			c0005 = &findings[i]
		}

		if findings[i].Severity == cqrslint.SeverityWarning {
			warnings++
		}
	}

	if c0005 == nil {
		t.Fatalf("C0005 did not fire; findings: %+v", findings)
	}

	if c0005.Suppressed != tc.wantSuppressed {
		t.Errorf("C0005: want suppressed=%v, got %v (%s)", tc.wantSuppressed, c0005.Suppressed, c0005)
	}

	if tc.wantWarning >= 0 && warnings != tc.wantWarning {
		t.Errorf("want %d directive warnings, got %d: %+v", tc.wantWarning, warnings, findings)
	}

	if tc.wantTotal >= 0 && len(findings) != tc.wantTotal {
		t.Errorf("want %d total findings, got %d: %+v", tc.wantTotal, len(findings), findings)
	}
}

// violationLine is the hasChanged guard line where both C0005 findings anchor.
const violationLine = `	if local.Title != remote.Title {`

func mutate(source, replacement string) string {
	return strings.Replace(suppressibleSource(), violationLine, replacement, 1)
}

func TestDirective_BlockComment_SameLine(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name:           "block-same-line",
		source:         mutate(suppressibleSource(), violationLine+` /* cqrs-lint:ignore C0005 */`),
		wantSuppressed: true,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_BlockComment_PreviousLine(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "block-previous-line",
		source: mutate(suppressibleSource(),
			`/* cqrs-lint:ignore C0005 */
`+violationLine),
		wantSuppressed: true,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_BlockComment_InnerLineAfterProse(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "block-inner-line",
		source: mutate(suppressibleSource(),
			`/*
	some prose that is not a directive
	cqrs-lint:ignore C0005 prose before the marker */
`+violationLine),
		wantSuppressed: true,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_BlockComment_NoSpaceAfterMarker(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name:           "block-tight-marker",
		source:         mutate(suppressibleSource(), violationLine+` /*cqrs-lint:ignore C0005*/`),
		wantSuppressed: true,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_Range_SuppressesInside(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-inside",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start C0005 historic provider fields
	if local.Title != remote.Title {
	//cqrs-lint:ignore-end C0005`),
		wantSuppressed: true,
		wantWarning:    0,
		wantTotal:      2,
	})
}

func TestDirective_Range_OutsideStaysActive(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-outside",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start C0005
	//cqrs-lint:ignore-end C0005
	if local.Title != remote.Title {`),
		wantSuppressed: false,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_Range_AllRules(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-all",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start all
	if local.Title != remote.Title {
	//cqrs-lint:ignore-end`),
		wantSuppressed: true,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_Range_DifferentRuleActive(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-wrong-rule",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start C0002
	if local.Title != remote.Title {
	//cqrs-lint:ignore-end C0002`),
		wantSuppressed: false,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_Range_NestedStartWarns(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-nested",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start C0005 outer
	//cqrs-lint:ignore-start C0005 inner
	if local.Title != remote.Title {
	//cqrs-lint:ignore-end C0005`),
		wantSuppressed: true,
		wantTotal:      -1,
		wantWarning:    1,
	})
}

func TestDirective_Range_UnmatchedEndWarns(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-unmatched-end",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-end C0005 stray
	if local.Title != remote.Title {`),
		wantSuppressed: false,
		wantTotal:      -1,
		wantWarning:    1,
	})
}

func TestDirective_Range_BareEndClosesAll(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-bare-end",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start C0005
	//cqrs-lint:ignore-start C0002
	//cqrs-lint:ignore-end
	if local.Title != remote.Title {`),
		wantSuppressed: false,
		wantTotal:      -1,
		wantWarning:    0,
	})
}

func TestDirective_Range_UnclosedWarnsAndSuppressesToEOF(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-unclosed",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start C0005 never closed
	if local.Title != remote.Title {`),
		wantSuppressed: true,
		wantTotal:      -1,
		wantWarning:    1,
	})
}

func TestDirective_Range_UnknownRuleWarns(t *testing.T) {
	runDirectiveCase(t, directiveCase{
		name: "range-unknown-rule",
		source: mutate(suppressibleSource(),
			`	//cqrs-lint:ignore-start C9999 stale
	if local.Title != remote.Title {
	//cqrs-lint:ignore-end C9999`),
		wantSuppressed: false,
		wantTotal:      -1,
		wantWarning:    2,
	})
}

func TestDirective_Range_ProvenanceRecordsKind(t *testing.T) {
	dir := t.TempDir()

	src := mutate(suppressibleSource(),
		`	//cqrs-lint:ignore-start C0005 deliberate window
	if local.Title != remote.Title {
	//cqrs-lint:ignore-end C0005`)
	if err := os.WriteFile(filepath.Join(dir, "cqrs.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	pkg, err := cqrslint.LoadPackage(dir)
	if err != nil {
		t.Fatal(err)
	}

	var found bool

	for _, f := range cqrslint.Run(pkg) {
		if f.Rule == "C0005" && f.Suppressed {
			found = true

			if f.SuppressedBy != "ignore-start" {
				t.Errorf("SuppressedBy = %q, want ignore-start", f.SuppressedBy)
			}

			if f.SuppressedReason != "deliberate window" {
				t.Errorf("SuppressedReason = %q, want the directive reason", f.SuppressedReason)
			}
		}
	}

	if !found {
		t.Fatal("expected a suppressed C0005 finding with range provenance")
	}
}
