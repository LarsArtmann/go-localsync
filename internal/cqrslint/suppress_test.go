package cqrslint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

// suppressCase is a single suppression expectation: a source snippet that
// triggers a known rule, plus a //cqrs-lint:ignore directive, and whether the
// finding should be suppressed or not.
type suppressCase struct {
	name       string
	source     string
	wantRule   string
	suppressed bool
	wantActive int // expected non-suppressed finding count (-1 = skip)
	wantTotal  int // expected total finding count including suppressed (-1 = skip)
}

func runSuppress(t *testing.T, tc suppressCase) {
	t.Helper()

	dir := t.TempDir()

	full := filepath.Join(dir, "cqrs.go")
	if err := os.WriteFile(full, []byte(tc.source), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	pkg, err := cqrslint.LoadPackage(dir)
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}

	findings := cqrslint.Run(pkg)

	found := false
	for _, f := range findings {
		if f.Rule == tc.wantRule {
			found = true
			if f.Suppressed != tc.suppressed {
				t.Errorf("rule %s: want Suppressed=%v, got %v (finding: %s)",
					tc.wantRule, tc.suppressed, f.Suppressed, f)
			}
		}
	}

	if !found {
		t.Fatalf("rule %s did not fire at all; findings: %+v", tc.wantRule, findings)
	}

	if tc.wantTotal >= 0 && len(findings) != tc.wantTotal {
		t.Errorf("want %d total findings, got %d: %+v", tc.wantTotal, len(findings), findings)
	}

	if tc.wantActive >= 0 {
		active := 0
		for _, f := range findings {
			if !f.Suppressed {
				active++
			}
		}

		if active != tc.wantActive {
			t.Errorf("want %d active findings, got %d", tc.wantActive, active)
		}
	}
}

// suppressibleSource returns compliant source with exactly two C0005 violations
// (hasChanged reads a banned field) on a known line, suitable for suppression tests.
func suppressibleSource() string {
	return `package cqrs

import "github.com/larsartmann/go-cqrs-lite/event/v4"

const aggregateType event.StreamType = "sync_item"

const (
	EventItemSynced        event.Type = "sync_item.synced"
	EventItemConflictFound event.Type = "sync_item.conflict_found"
	EventItemTombstoned    event.Type = "sync_item.tombstoned"
)

type ItemSyncedPayload struct {
	ItemID string ` + "`json:\"itemId\"`" + `
	Source string ` + "`json:\"source\"`" + `
}

type ItemConflictFoundPayload struct {
	Source string ` + "`json:\"source\"`" + `
}

type ItemTombstonedPayload struct {
	Source string ` + "`json:\"source\"`" + `
}

type State struct{}

func fold(state State, evt event.Event) (State, error) {
	switch evt.Type() {
	case EventItemSynced:
		return state, nil
	case EventItemConflictFound:
		return state, nil
	case EventItemTombstoned:
		return state, nil
	}
	return state, nil
}

type Projector struct {
	mu mutex
}

func (p *Projector) EventTypes() []event.Type {
	return []event.Type{EventItemSynced, EventItemConflictFound, EventItemTombstoned}
}

func (p *Projector) Handle(evt event.Event) error {
	p.mu.Lock()
	return nil
}

type mutex struct{}

func (m mutex) Lock() {}

type Item struct {
	ContentHash string
	UpdatedAt   int64
	Type        string
}

func hasChanged(local, remote *Item) bool {
	if local.Title != remote.Title {
		return true
	}
	return false
}

func makeEvents(aggID string) {
	event.NewEvents(aggID, aggregateType, 0, nil, nil)
}`
}

func TestSuppress_LineLevel_SameLine(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	if local.Title != remote.Title { //cqrs-lint:ignore C0005`, 1)

	runSuppress(t, suppressCase{
		name:       "same-line",
		source:     source,
		wantRule:   "C0005",
		suppressed: true,
		wantActive: 0,
		wantTotal:  2,
	})
}

func TestSuppress_LineLevel_PreviousLine(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	//cqrs-lint:ignore C0005
	if local.Title != remote.Title {`, 1)

	runSuppress(t, suppressCase{
		name:       "previous-line",
		source:     source,
		wantRule:   "C0005",
		suppressed: true,
		wantActive: 0,
		wantTotal:  2,
	})
}

func TestSuppress_DifferentRule_NotSuppressed(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	if local.Title != remote.Title { //cqrs-lint:ignore C0001`, 1)

	runSuppress(t, suppressCase{
		name:       "wrong-rule",
		source:     source,
		wantRule:   "C0005",
		suppressed: false,
		wantActive: 2,
		wantTotal:  2,
	})
}

func TestSuppress_AllRules(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	if local.Title != remote.Title { //cqrs-lint:ignore all`, 1)

	runSuppress(t, suppressCase{
		name:       "all-rules",
		source:     source,
		wantRule:   "C0005",
		suppressed: true,
		wantActive: 0,
		wantTotal:  2,
	})
}

func TestSuppress_CommaSeparatedRules(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	if local.Title != remote.Title { //cqrs-lint:ignore C0001,C0005`, 1)

	runSuppress(t, suppressCase{
		name:       "comma-separated",
		source:     source,
		wantRule:   "C0005",
		suppressed: true,
		wantActive: 0,
		wantTotal:  2,
	})
}

func TestSuppress_WithReason(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	if local.Title != remote.Title { //cqrs-lint:ignore C0005 temporary provider field`, 1)

	runSuppress(t, suppressCase{
		name:       "with-reason",
		source:     source,
		wantRule:   "C0005",
		suppressed: true,
		wantActive: 0,
		wantTotal:  2,
	})
}

func TestSuppress_FileLevel(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`package cqrs`,
		`package cqrs //cqrs-lint:ignore-file C0005`, 1)

	runSuppress(t, suppressCase{
		name:       "file-level",
		source:     source,
		wantRule:   "C0005",
		suppressed: true,
		wantActive: 0,
		wantTotal:  2,
	})
}

func TestSuppress_FileLevel_AllRules(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`package cqrs`,
		`package cqrs //cqrs-lint:ignore-file all`, 1)

	runSuppress(t, suppressCase{
		name:       "file-level-all",
		source:     source,
		wantRule:   "C0005",
		suppressed: true,
		wantActive: 0,
		wantTotal:  2,
	})
}

func TestSuppress_NoDirective_NotSuppressed(t *testing.T) {
	runSuppress(t, suppressCase{
		name:       "no-directive",
		source:     suppressibleSource(),
		wantRule:   "C0005",
		suppressed: false,
		wantActive: 2,
		wantTotal:  2,
	})
}

func TestSuppress_LineLevel_TooFarAbove_NotSuppressed(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	//cqrs-lint:ignore C0005

	if local.Title != remote.Title {`, 1)

	runSuppress(t, suppressCase{
		name:       "too-far-above",
		source:     source,
		wantRule:   "C0005",
		suppressed: false,
		wantActive: 2,
		wantTotal:  2,
	})
}

func TestSuppress_MalformedDirective_NotSuppressed(t *testing.T) {
	source := strings.Replace(suppressibleSource(),
		`	if local.Title != remote.Title {`,
		`	if local.Title != remote.Title { //cqrs-lint:ignore`, 1)

	runSuppress(t, suppressCase{
		name:       "malformed",
		source:     source,
		wantRule:   "C0005",
		suppressed: false,
		wantActive: 2,
		wantTotal:  2,
	})
}

func TestFinding_SuppressedString(t *testing.T) {
	f := cqrslint.Finding{
		Rule: "C0005", Severity: cqrslint.SeverityError,
		File: "a.go", Line: 42, Message: "bad",
		Suppressed: true,
	}

	want := "a.go:42: C0005 error: bad [suppressed]"
	if got := f.String(); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	f.SuppressedBy = "ignore-file"
	f.SuppressedReason = "historical"

	wantProvenance := "a.go:42: C0005 error: bad [suppressed by ignore-file: historical]"
	if got := f.String(); got != wantProvenance {
		t.Fatalf("want %q, got %q", wantProvenance, got)
	}
}

func TestFinding_NotSuppressedString(t *testing.T) {
	f := cqrslint.Finding{
		Rule: "C0005", Severity: cqrslint.SeverityError,
		File: "a.go", Line: 42, Message: "bad",
		Suppressed: false,
	}

	want := "a.go:42: C0005 error: bad"
	if got := f.String(); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// TestRun_UnknownInternalRuleWarns proves the audit trail end to end: a
// directive naming an internal-scheme rule that does not exist produces a
// warning, while a library-scheme rule (C017) is left alone.
func TestRun_UnknownInternalRuleWarns(t *testing.T) {
	dir := t.TempDir()

	src := `package cqrs

import "example.com/event"

const (
	//cqrs-lint:ignore C9999 stale directive
	//cqrs-lint:ignore(C017) targets the library linter
	EventItemSynced event.Type = "sync_item.synced"
)
`
	if err := os.WriteFile(filepath.Join(dir, "events.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	pkg, err := cqrslint.LoadPackage(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var unknownWarned, libraryFlagged bool

	for _, f := range cqrslint.Run(pkg) {
		if f.Rule == "C9999" && !f.Suppressed && strings.Contains(f.Message, "unknown rule") {
			unknownWarned = true
		}

		if f.Rule == "C017" {
			libraryFlagged = true
		}
	}

	if !unknownWarned {
		t.Error("expected a warning for the unknown internal-scheme rule C9999")
	}

	if libraryFlagged {
		t.Error("library-scheme rule IDs (C017) must not be flagged by the internal linter")
	}
}

// TestRun_SuppressionProvenance asserts SuppressedBy/SuppressedReason are
// populated from the directive that silenced a finding.
func TestRun_SuppressionProvenance(t *testing.T) {
	dir := t.TempDir()

	src := `package cqrs

import "example.com/event"

const (
	EventItemSynced event.Type = "sync_item.synced"
	//cqrs-lint:ignore C0002 deliberate extra event
	EventItemBogus event.Type = "sync_item.bogus"
)
`

	if err := os.WriteFile(filepath.Join(dir, "events.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	pkg, err := cqrslint.LoadPackage(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var found bool

	for _, f := range cqrslint.Run(pkg) {
		if f.Rule == "C0002" && f.Suppressed {
			found = true

			if f.SuppressedBy != "ignore" {
				t.Errorf("SuppressedBy = %q, want ignore", f.SuppressedBy)
			}

			if f.SuppressedReason != "deliberate extra event" {
				t.Errorf("SuppressedReason = %q, want the directive text", f.SuppressedReason)
			}
		}
	}

	if !found {
		t.Fatal("expected a suppressed C0002 finding with provenance")
	}
}
