package cqrslint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

// mixedViolationSource is a compliant package with one C0002 violation
// (extra event const) and one C0005 violation (hasChanged reads Title), so
// rule-selection options have two distinguishable findings to filter.
func mixedViolationSource() string {
	const events = `	EventItemTombstoned    event.Type = "sync_item.tombstoned"`

	return strings.Replace(suppressibleSource(), events,
		events+"\n	EventItemBogus         event.Type = \"sync_item.bogus\"", 1)
}

// loadOneFilePackage writes src into a temp dir and loads it.
func loadOneFilePackage(t *testing.T, src string) *cqrslint.Package {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cqrs.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	pkg, err := cqrslint.LoadPackage(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	return pkg
}

func ruleCounts(findings []cqrslint.Finding) map[string]int {
	counts := map[string]int{}

	for _, f := range findings {
		if !f.Suppressed {
			counts[f.Rule]++
		}
	}

	return counts
}

func TestRunWithOptions_RulesSubset(t *testing.T) {
	pkg := loadOneFilePackage(t, mixedViolationSource())

	findings := cqrslint.RunWithOptions(pkg, cqrslint.RunOptions{Rules: []string{"C0005"}})
	counts := ruleCounts(findings)

	if counts["C0005"] == 0 {
		t.Errorf("expected C0005 findings, got %+v", findings)
	}

	if counts["C0002"] != 0 {
		t.Errorf("--rules C0005 must not report C0002, got %+v", findings)
	}
}

func TestRunWithOptions_ExcludeRules(t *testing.T) {
	pkg := loadOneFilePackage(t, mixedViolationSource())

	findings := cqrslint.RunWithOptions(pkg, cqrslint.RunOptions{ExcludeRules: []string{"C0005"}})
	counts := ruleCounts(findings)

	if counts["C0005"] != 0 {
		t.Errorf("--exclude-rules C0005 must drop C0005, got %+v", findings)
	}

	if counts["C0002"] == 0 {
		t.Errorf("C0002 must still run, got %+v", findings)
	}
}

func TestRunWithOptions_RulesTakePrecedenceOverExclude(t *testing.T) {
	pkg := loadOneFilePackage(t, mixedViolationSource())

	findings := cqrslint.RunWithOptions(pkg, cqrslint.RunOptions{
		Rules:        []string{"C0002"},
		ExcludeRules: []string{"C0002"},
	})
	counts := ruleCounts(findings)

	if counts["C0002"] == 0 {
		t.Errorf("explicit --rules must win over --exclude-rules, got %+v", findings)
	}
}

func TestRunWithOptions_DefaultRunsAll(t *testing.T) {
	pkg := loadOneFilePackage(t, mixedViolationSource())

	counts := ruleCounts(cqrslint.Run(pkg))

	if counts["C0002"] == 0 || counts["C0005"] == 0 {
		t.Errorf("default run must report both violations, got %+v", counts)
	}
}

func TestRunWithOptions_NoSuppressReactivatesSuppressed(t *testing.T) {
	src := `package cqrs

import "example.com/event"

const (
	EventItemSynced event.Type = "sync_item.synced"
	EventItemConflictFound event.Type = "sync_item.conflict_found"
	EventItemTombstoned event.Type = "sync_item.tombstoned"
	//cqrs-lint:ignore C0002 deliberate
	EventItemBogus event.Type = "sync_item.bogus"
)
`
	pkg := loadOneFilePackage(t, src)

	defaultCounts := ruleCounts(cqrslint.Run(pkg))
	if defaultCounts["C0002"] != 0 {
		t.Fatalf("directive must suppress C0002 by default, got %+v", defaultCounts)
	}

	hardened := ruleCounts(cqrslint.RunWithOptions(pkg, cqrslint.RunOptions{NoSuppress: true}))
	if hardened["C0002"] == 0 {
		t.Errorf("--no-suppress must reactivate the suppressed C0002, got %+v", hardened)
	}
}

func TestRunWithOptions_NoSuppressKeepsDirectiveAudit(t *testing.T) {
	src := `package cqrs

import "example.com/event"

const (
	//cqrs-lint:ignore C9999 stale
	EventItemSynced event.Type = "sync_item.synced"
)
`
	pkg := loadOneFilePackage(t, src)

	findings := cqrslint.RunWithOptions(pkg, cqrslint.RunOptions{NoSuppress: true})

	staleWarned := false

	for _, f := range findings {
		if f.Rule == "C9999" && f.Severity == cqrslint.SeverityWarning {
			staleWarned = true
		}
	}

	if !staleWarned {
		t.Errorf("directive audit warnings must survive --no-suppress, got %+v", findings)
	}
}

func TestValidateRuleSelection(t *testing.T) {
	if err := cqrslint.ValidateRuleSelection([]string{"C0002", "C0005"}, []string{"C0006"}); err != nil {
		t.Errorf("known rule IDs must validate: %v", err)
	}

	if err := cqrslint.ValidateRuleSelection([]string{"C9999"}, nil); err == nil {
		t.Error("unknown include ID must be rejected")
	}

	if err := cqrslint.ValidateRuleSelection(nil, []string{"C9999"}); err == nil {
		t.Error("unknown exclude ID must be rejected")
	}

	if err := cqrslint.ValidateRuleSelection([]string{"C017"}, nil); err == nil {
		t.Error("foreign-scheme IDs are rejected in explicit selections (unlike directives)")
	}
}

func TestRuleByID(t *testing.T) {
	rule, ok := cqrslint.RuleByID("C0005")
	if !ok {
		t.Fatal("C0005 must be in the catalog")
	}

	if rule.Title == "" || rule.Rationale == "" {
		t.Errorf("C0005 catalog entry missing title/rationale: %+v", rule)
	}

	if _, ok := cqrslint.RuleByID("C9999"); ok {
		t.Error("C9999 must not resolve")
	}
}
