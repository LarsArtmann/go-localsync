package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

// runCLI drives the real CLI flow in-process (no binary build) and returns
// its exit code plus whatever it wrote to stdout and stderr.
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)

	return code, stdout.String(), stderr.String()
}

// violatingFixtureDir returns a package dir with a guaranteed C0002 error
// finding (an extra event.Type const), mirroring the process-level fixture.
func violatingFixtureDir(t *testing.T) string {
	t.Helper()

	original, err := os.ReadFile("../../pkg/cqrs/events.go")
	if err != nil {
		t.Fatalf("read events.go: %v", err)
	}

	violating := strings.Replace(
		string(original),
		"const (",
		"const (\nEventItemBogus event.Type = \"sync_item.bogus\"",
		1,
	)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "events.go"), []byte(violating), 0o600); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestRun_CleanPackageExitsZero(t *testing.T) {
	code, stdout, stderr := runCLI(t, "-pkg", "../../pkg/cqrs")
	if code != 0 {
		t.Errorf("clean package: exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stderr, "clean") && stdout != "" {
		t.Logf("clean run output: %q / %q", stdout, stderr)
	}
}

func TestRun_ViolatingPackageExitsOne(t *testing.T) {
	dir := violatingFixtureDir(t)

	code, stdout, _ := runCLI(t, "-pkg", dir)
	if code != 1 {
		t.Errorf("violating fixture: exit = %d, want 1\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "C0002") {
		t.Errorf("violating run must name C0002:\n%s", stdout)
	}
}

func TestRun_MissingTargetIsUsageError(t *testing.T) {
	code, _, stderr := runCLI(t, "-pkg", filepath.Join(t.TempDir(), "does-not-exist"))
	if code != 2 {
		t.Errorf("missing target: exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "localsync-lint:") {
		t.Errorf("usage error must be prefixed with the tool name:\n%s", stderr)
	}
}

func TestRun_FlagParseErrorIsUsageError(t *testing.T) {
	code, _, stderr := runCLI(t, "-definitely-not-a-flag")
	if code != 2 {
		t.Errorf("unknown flag: exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "Usage: localsync-lint") {
		t.Errorf("flag error must print usage:\n%s", stderr)
	}
}

func TestRun_DirectivesWithoutTarget(t *testing.T) {
	code, stdout, _ := runCLI(t, "-version")
	if code != 0 || !strings.HasPrefix(stdout, "localsync-lint ") {
		t.Errorf("-version: exit = %d, stdout = %q", code, stdout)
	}

	code, stdout, _ = runCLI(t, "-list")
	if code != 0 || !strings.Contains(stdout, "C0001") || !strings.Contains(stdout, "RATIONALE") {
		t.Errorf("-list: exit = %d, stdout = %q", code, stdout)
	}

	code, stdout, _ = runCLI(t, "-explain", "C0005")
	if code != 0 || !strings.Contains(stdout, "ADR-0007") {
		t.Errorf("-explain C0005: exit = %d, stdout = %q", code, stdout)
	}

	code, _, stderr := runCLI(t, "-explain", "C9999")
	if code != 2 || !strings.Contains(stderr, "unknown rule") {
		t.Errorf("-explain C9999: exit = %d, stderr = %q", code, stderr)
	}
}

func TestRun_JSONAliasMatchesFormatFlag(t *testing.T) {
	dir := violatingFixtureDir(t)

	_, aliasOut, _ := runCLI(t, "-pkg", dir, "-json")
	_, formatOut, _ := runCLI(t, "-pkg", dir, "-format=json")

	if aliasOut != formatOut {
		t.Errorf("-json and -format=json diverge:\n-json:        %q\n-format=json: %q", aliasOut, formatOut)
	}
}

func TestRun_BadFormatIsUsageError(t *testing.T) {
	dir := violatingFixtureDir(t)

	code, _, stderr := runCLI(t, "-pkg", dir, "-format=yaml")
	if code != 2 {
		t.Errorf("unknown -format: exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "text, json, github, or sarif") {
		t.Errorf("format error must list every accepted format:\n%s", stderr)
	}
}

func TestRun_QuietIsSilent(t *testing.T) {
	dir := violatingFixtureDir(t)

	code, stdout, stderr := runCLI(t, "-pkg", dir, "-quiet")
	if code != 1 {
		t.Errorf("-quiet on violations: exit = %d, want 1", code)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("-quiet wrote output: %q / %q", stdout, stderr)
	}
}

// TestRun_Sarif pins the --format=sarif contract: a single SARIF 2.1.0
// document whose results carry the finding, its level, and its location,
// with suppressed findings hidden unless --show-suppressed tags them
// inSource.
func TestRun_Sarif(t *testing.T) {
	dir := violatingFixtureDir(t)

	code, stdout, stderr := runCLI(t, "-pkg", dir, "-format=sarif")
	if code != 1 {
		t.Fatalf("sarif on violations: exit = %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	var log sarifReport
	if err := json.Unmarshal([]byte(stdout), &log); err != nil {
		t.Fatalf("-format=sarif stdout is not one JSON document: %v\n%s", err, stdout)
	}

	if log.Version != "2.1.0" || log.Schema == "" {
		t.Errorf("sarif version/schema = %q / %q", log.Version, log.Schema)
	}

	if len(log.Runs) != 1 {
		t.Fatalf("sarif log must have exactly one run, got %d", len(log.Runs))
	}

	run := log.Runs[0]
	if run.Tool.Driver.Name != "localsync-lint" || run.Tool.Driver.Version == "" {
		t.Errorf("sarif driver = %+v", run.Tool.Driver)
	}

	if len(run.Tool.Driver.Rules) != len(cqrslint.Rules()) {
		t.Errorf("sarif driver rules = %d, want the full catalog (%d)", len(run.Tool.Driver.Rules), len(cqrslint.Rules()))
	}

	sawC0002 := false
	for _, result := range run.Results {
		if result.RuleID != "C0002" {
			continue
		}
		sawC0002 = true
		if result.Level != "error" {
			t.Errorf("C0002 result level = %q, want error", result.Level)
		}
		if len(result.Locations) != 1 || result.Locations[0].PhysicalLocation.ArtifactLocation.URI == "" {
			t.Errorf("C0002 result must carry a file location: %+v", result.Locations)
		}
		if result.Locations[0].PhysicalLocation.Region.StartLine == 0 {
			t.Errorf("C0002 result must carry a 1-based start line: %+v", result.Locations)
		}
	}

	if !sawC0002 {
		t.Errorf("sarif results must include the C0002 violation:\n%s", stdout)
	}
}

// TestRun_SarifSuppressionTagging: with --show-suppressed a suppressed
// finding appears in the SARIF results tagged with an inSource suppression —
// SARIF's native spelling of a //cqrs-lint: directive.
func TestRun_SarifSuppressionTagging(t *testing.T) {
	suppressed := strings.Replace(
		mustReadEvents(t),
		"const (\nEventItemBogus event.Type = \"sync_item.bogus\"",
		"const (\n//cqrs-lint:ignore C0002 deliberate\nEventItemBogus event.Type = \"sync_item.bogus\"",
		1,
	)

	supDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(supDir, "events.go"), []byte(suppressed), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := runCLI(t, "-pkg", supDir, "-format=sarif", "-show-suppressed")
	if code != 0 {
		t.Fatalf("suppressed-only sarif: exit = %d, want 0\n%s", code, stdout)
	}

	var log sarifReport
	if err := json.Unmarshal([]byte(stdout), &log); err != nil {
		t.Fatalf("sarif output not valid JSON: %v\n%s", err, stdout)
	}

	sawSuppressed := false
	for _, result := range log.Runs[0].Results {
		if result.RuleID != "C0002" {
			continue
		}
		sawSuppressed = true
		if len(result.Suppressions) != 1 || result.Suppressions[0].Kind != "inSource" {
			t.Errorf("suppressed C0002 must carry an inSource suppression entry: %+v", result.Suppressions)
		}
	}

	if !sawSuppressed {
		t.Errorf("--show-suppressed sarif must include the suppressed C0002:\n%s", stdout)
	}
}

func TestRun_UnknownRuleSelectionIsUsageError(t *testing.T) {
	code, _, stderr := runCLI(t, "-pkg", "../../pkg/cqrs", "-rules", "C9999")
	if code != 2 || !strings.Contains(stderr, "C9999") {
		t.Errorf("-rules C9999: exit = %d, stderr = %q", code, stderr)
	}
}

func TestEmitSarif_WarningLevelAndUnpositionedFindings(t *testing.T) {
	var buf bytes.Buffer
	emitSarif(&buf, []cqrslint.Finding{
		{Rule: "C0009", Severity: cqrslint.SeverityWarning, Message: "no position"},
	}, outputOptions{})

	var log sarifReport
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatalf("sarif output not valid JSON: %v\n%s", err, buf.String())
	}

	result := log.Runs[0].Results[0]
	if result.Level != "warning" {
		t.Errorf("warning-severity result level = %q, want warning", result.Level)
	}
	if result.Locations != nil {
		t.Errorf("unpositioned finding must omit locations, got %+v", result.Locations)
	}
}

func TestSarifLevel(t *testing.T) {
	if got := sarifLevel(cqrslint.SeverityError); got != "error" {
		t.Errorf("sarifLevel(error) = %q", got)
	}
	if got := sarifLevel(cqrslint.SeverityWarning); got != "warning" {
		t.Errorf("sarifLevel(warning) = %q", got)
	}
	if got := sarifLevel(cqrslint.Severity("bogus")); got != "note" {
		t.Errorf("sarifLevel(bogus) = %q, want note", got)
	}
}

func mustReadEvents(t *testing.T) string {
	t.Helper()

	original, err := os.ReadFile("../../pkg/cqrs/events.go")
	if err != nil {
		t.Fatalf("read events.go: %v", err)
	}

	violating := strings.Replace(
		string(original),
		"const (",
		"const (\nEventItemBogus event.Type = \"sync_item.bogus\"",
		1,
	)

	return violating
}
