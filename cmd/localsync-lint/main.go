// Command localsync-lint statically verifies that a Go package conforms to
// the go-localsync CQRS architectural invariants (ADR-0004 + AGENTS.md).
//
// The directive vocabulary stays //cqrs-lint: on purpose: one inline comment
// targets this linter AND go-cqrs-lite's consumer cqrs-lint, whose rule-ID
// schemes this tool tolerates in directives.
//
// It parses the package with the standard-library go/parser — no type
// resolution, no third-party dependencies — and reports any rule violations.
//
// # Suppression Directives
//
// Findings can be silenced inline with directives:
//
//	//cqrs-lint:ignore C0005           line-level: silences next line or same line
//	//cqrs-lint:ignore C0005 reason    with optional human-readable reason
//	//cqrs-lint:ignore C0001,C0002     comma-separated rules
//	//cqrs-lint:ignore all             silence every rule at this position
//	//cqrs-lint:ignore-file C0005      file-level: silences the rule everywhere in the file
//
// Suppressed findings are hidden by default; use --show-suppressed to list them.
//
// Exit codes: 0 clean, 1 findings present, 2 usage/internal error.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

const (
	defaultTarget = "pkg/cqrs"
	cliVersion    = "0.1.0"

	formatText   = "text"
	formatJSON   = "json"
	formatGitHub = "github"
	formatSarif  = "sarif"

	// sarifSchema is the SARIF 2.1.0 schema URI every consumer validates
	// against; the version field below must stay in sync with it.
	sarifSchema = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"
	sarifFormat = "2.1.0"

	repoURI = "https://github.com/LarsArtmann/go-localsync"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the entire CLI flow minus process plumbing: flag parsing, dispatch,
// analysis, and output. It returns the process exit code (0 clean, 1 findings,
// 2 usage/internal error) so tests can drive the real flow in-process instead
// of paying a binary build per case.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("localsync-lint", flag.ContinueOnError)
	fs.SetOutput(stderr)

	target := fs.String("pkg", defaultTarget, "path to the Go package to lint")
	listRules := fs.Bool("list", false, "list all rules and exit")
	strict := fs.Bool("strict", false, "exit non-zero when warnings are present (alias for -fail-on-warning)")
	failOnWarning := fs.Bool("fail-on-warning", false, "exit non-zero when warnings are present")
	jsonOut := fs.Bool("json", false,
		"emit findings as newline-delimited JSON (machine readable; alias for -format=json)")
	format := fs.String(
		"format",
		"text",
		"output format: text, json (NDJSON), github (workflow annotations), or sarif",
	)
	quiet := fs.Bool("quiet", false, "suppress all output; communicate through the exit code only")
	verbose := fs.Bool("verbose", false, "show package info, per-rule status, and timing on stderr")
	showSuppressed := fs.Bool("show-suppressed", false, "show findings silenced by //cqrs-lint:ignore directives")
	rules := fs.String("rules", "", "comma-separated rule IDs to run (default: all)")
	excludeRules := fs.String("exclude-rules", "", "comma-separated rule IDs to skip")
	noSuppress := fs.Bool("no-suppress", false,
		"disable //cqrs-lint: directives; every violation counts (CI hardening)")
	explain := fs.String("explain", "", "print the full description of one rule and exit")
	showVersion := fs.Bool("version", false, "print the localsync-lint version and exit")
	fs.Usage = func() { printUsage(fs) }

	if err := fs.Parse(args); err != nil {
		return 2 // ContinueOnError already printed the reason and usage
	}

	if *showVersion {
		fmt.Fprintf(stdout, "localsync-lint %s\n", cliVersion)

		return 0
	}

	if *listRules {
		printRules(stdout)

		return 0
	}

	if *explain != "" {
		rule, ok := cqrslint.RuleByID(*explain)
		if !ok {
			fmt.Fprintf(stderr, "localsync-lint: unknown rule %q (see --list)\n", *explain)

			return 2
		}

		printRuleDetail(stdout, rule)

		return 0
	}

	ruleSelection, err := parseRuleSelection(*rules, *excludeRules)
	if err != nil {
		fmt.Fprintln(stderr, "localsync-lint:", err)

		return 2
	}

	resolvedFormat := *format
	if *jsonOut {
		resolvedFormat = formatJSON
	}

	switch resolvedFormat {
	case formatText, formatJSON, formatGitHub, formatSarif:
		// valid
	default:
		fmt.Fprintf(stderr, "localsync-lint: unknown -format %q (want text, json, github, or sarif)\n", resolvedFormat)

		return 2
	}

	opts := outputOptions{
		strict:         *strict || *failOnWarning,
		verbose:        *verbose,
		showSuppressed: *showSuppressed,
		jsonOut:        resolvedFormat == formatJSON,
		quiet:          *quiet,
		format:         resolvedFormat,
	}

	runOpts := cqrslint.RunOptions{
		Rules:        ruleSelection.include,
		ExcludeRules: ruleSelection.exclude,
		NoSuppress:   *noSuppress,
	}

	start := time.Now()
	pkg, findings, err := analyze(*target, runOpts)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Fprintln(stderr, "localsync-lint:", err)

		return 2
	}

	emit(stdout, stderr, report{
		findings:  findings,
		opts:      opts,
		target:    *target,
		fileCount: len(pkg.Files),
		elapsed:   elapsed,
	})

	return exitCode(findings, opts)
}

// ruleSelection is the parsed --rules / --exclude-rules flag pair.
type ruleSelection struct {
	include []string
	exclude []string
}

// parseRuleSelection splits the comma-separated flag values and validates
// every ID against the rule catalog — a typo in an explicit selection is a
// usage error, not a silent no-op.
func parseRuleSelection(rules, excludeRules string) (ruleSelection, error) {
	selection := ruleSelection{
		include: splitRuleList(rules),
		exclude: splitRuleList(excludeRules),
	}

	if err := cqrslint.ValidateRuleSelection(selection.include, selection.exclude); err != nil {
		return ruleSelection{}, err
	}

	return selection, nil
}

// splitRuleList parses a comma-separated flag value into trimmed, non-empty IDs.
func splitRuleList(value string) []string {
	var ids []string

	for id := range strings.SplitSeq(value, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}

	return ids
}

// printRuleDetail renders one rule for --explain: identity, severity, and
// the rationale that ties the rule back to its design decision.
func printRuleDetail(w io.Writer, rule cqrslint.Rule) {
	fmt.Fprintf(w, "%s — %s [%s]\n\n%s\n", rule.ID, rule.Title, rule.Severity, rule.Rationale)
}

type outputOptions struct {
	strict         bool
	verbose        bool
	showSuppressed bool
	jsonOut        bool
	quiet          bool
	format         string
}

type report struct {
	findings  []cqrslint.Finding
	opts      outputOptions
	target    string
	fileCount int
	elapsed   time.Duration
}

func analyze(target string, runOpts cqrslint.RunOptions) (*cqrslint.Package, []cqrslint.Finding, error) {
	pkg, err := cqrslint.LoadPackage(target)
	if err != nil {
		return nil, nil, err
	}

	return pkg, cqrslint.RunWithOptions(pkg, runOpts), nil
}

func emit(stdout, stderr io.Writer, r report) {
	counts := countFindings(r.findings)

	if r.opts.quiet {
		return // the exit code is the only channel
	}

	if r.opts.verbose {
		emitVerboseHeader(stderr, r.target, r.fileCount)
		emitRuleStatus(stderr, r.findings)
		emitSuppressedByRule(stderr, r.findings)
	}

	if r.opts.format == formatSarif {
		emitSarif(stdout, r.findings, r.opts)
	} else {
		emitFindings(stdout, r.findings, r.opts)
	}

	emitSummary(stderr, counts, r.opts, r.elapsed)
}

// emitSuppressedByRule lists how many findings each rule silenced — a
// suppressed-only rule is invisible in the active counts but may signal a
// stale directive worth cleaning up.
func emitSuppressedByRule(w io.Writer, findings []cqrslint.Finding) {
	suppressedByRule := map[string]int{}

	for _, f := range findings {
		if f.Suppressed {
			suppressedByRule[f.Rule]++
		}
	}

	if len(suppressedByRule) == 0 {
		return
	}

	fmt.Fprintf(w, "  suppressed by rule:")

	for _, rule := range cqrslint.Rules() {
		if n := suppressedByRule[rule.ID]; n > 0 {
			fmt.Fprintf(w, " %s=%d", rule.ID, n)
		}
	}

	fmt.Fprintln(w)
}

func countFindings(findings []cqrslint.Finding) findingCounts {
	var c findingCounts

	for _, f := range findings {
		if f.Suppressed {
			c.suppressed++
			continue
		}

		switch f.Severity {
		case cqrslint.SeverityError:
			c.errors++
		case cqrslint.SeverityWarning:
			c.warnings++
		}
	}

	return c
}

type findingCounts struct {
	errors     int
	warnings   int
	suppressed int
}

func emitVerboseHeader(w io.Writer, target string, fileCount int) {
	fmt.Fprintf(w, "localsync-lint: analyzing %s (%d files, %d rules)\n", target, fileCount, len(cqrslint.Rules()))
}

func emitRuleStatus(w io.Writer, findings []cqrslint.Finding) {
	activeByRule := map[string]int{}

	for _, f := range findings {
		if f.Suppressed {
			continue
		}

		activeByRule[f.Rule]++
	}

	for _, rule := range cqrslint.Rules() {
		count := activeByRule[rule.ID]

		status := "ok"
		if count > 0 {
			status = fmt.Sprintf("%d %s", count, plural("finding", count))
		}

		fmt.Fprintf(w, "  %-7s %-40s  %s\n", rule.ID, rule.Title, status)
	}
}

func emitFindings(w io.Writer, findings []cqrslint.Finding, opts outputOptions) {
	for _, finding := range findings {
		if finding.Suppressed && !opts.showSuppressed {
			continue
		}

		switch {
		case opts.jsonOut:
			emitFindingJSON(w, finding)
		case opts.format == "github":
			emitFindingGitHub(w, finding)
		default:
			fmt.Fprintln(w, finding)
		}
	}
}

// findingJSON mirrors cqrslint.Finding for stable wire output: the JSON keys
// are a consumed contract, so marshaling goes through an explicitly tagged
// struct instead of hand-built formatting that drifts from the schema.
type findingJSON struct {
	Rule       string `json:"rule"`
	Severity   string `json:"severity"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Message    string `json:"message"`
	Suppressed bool   `json:"suppressed"`
}

func emitFindingJSON(w io.Writer, f cqrslint.Finding) {
	encoded, err := json.Marshal(findingJSON{
		Rule:       f.Rule,
		Severity:   string(f.Severity),
		File:       f.File,
		Line:       f.Line,
		Message:    f.Message,
		Suppressed: f.Suppressed,
	})
	if err != nil {
		// json.Marshal on a flat struct of primitives cannot fail; be loud
		// anyway rather than emitting a torn NDJSON line.
		panic(fmt.Sprintf("localsync-lint: marshal finding: %v", err))
	}

	fmt.Fprintf(w, "%s\n", encoded)
}

// emitFindingGitHub prints GitHub Actions workflow annotations so findings
// appear inline in the PR files-changed view when the CLI runs in CI.
func emitFindingGitHub(w io.Writer, f cqrslint.Finding) {
	annotation := "::error"

	if f.Severity == cqrslint.SeverityWarning {
		annotation = "::warning"
	}

	fmt.Fprintf(w, "%s file=%s,line=%d,title=%s::%s\n", annotation, f.File, f.Line, f.Rule, f.Message)
}

// SARIF 2.1.0 emission (https://json.sarifspec.com/): one run, one result per
// visible finding, and the full rule catalog in tool.driver.rules so any
// ruleId resolves for consumers even when it has no findings. Suppressed
// findings reuse the same visibility rule as every other format (hidden
// unless --show-suppressed) and are additionally tagged with an inSource
// suppression entry, which is SARIF's native spelling of a //cqrs-lint:
// directive.
type sarifReport struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	InformationURI string          `json:"informationUri"`
	Rules          []sarifRuleDesc `json:"rules"`
}

type sarifRuleDesc struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
}

type sarifResult struct {
	RuleID       string             `json:"ruleId"`
	Level        string             `json:"level"`
	Message      sarifMessage       `json:"message"`
	Locations    []sarifLocation    `json:"locations,omitempty"`
	Suppressions []sarifSuppression `json:"suppressions,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           *sarifRegion          `json:"region,omitempty"` // pointer: nested omitempty is otherwise a no-op
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

type sarifSuppression struct {
	Kind          string `json:"kind"`
	Status        string `json:"status"`
	Justification string `json:"justification"`
}

// emitSarif writes the whole log as a single JSON document (not NDJSON):
// SARIF consumers expect one object per file.
func emitSarif(w io.Writer, findings []cqrslint.Finding, opts outputOptions) {
	driverRules := make([]sarifRuleDesc, 0, len(cqrslint.Rules()))

	for _, rule := range cqrslint.Rules() {
		driverRules = append(driverRules, sarifRuleDesc{
			ID:               rule.ID,
			ShortDescription: sarifMessage{Text: rule.Title},
		})
	}

	results := make([]sarifResult, 0, len(findings))

	for _, f := range findings {
		if f.Suppressed && !opts.showSuppressed {
			continue
		}

		result := sarifResult{
			RuleID:  f.Rule,
			Level:   sarifLevel(f.Severity),
			Message: sarifMessage{Text: f.Message},
		}

		if f.File != "" {
			location := sarifLocation{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: f.File},
				},
			}

			// SARIF regions are 1-based; package-level findings carry no
			// position and omit the region rather than lying with a 0.
			if f.Line > 0 {
				location.PhysicalLocation.Region = &sarifRegion{StartLine: f.Line}
			}

			result.Locations = []sarifLocation{location}
		}

		if f.Suppressed {
			result.Suppressions = []sarifSuppression{{
				Kind:          "inSource",
				Status:        "accepted",
				Justification: "//cqrs-lint: directive",
			}}
		}

		results = append(results, result)
	}

	log := sarifReport{
		Schema:  sarifSchema,
		Version: sarifFormat,
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "localsync-lint",
					Version:        cliVersion,
					InformationURI: repoURI,
					Rules:          driverRules,
				},
			},
			Results: results,
		}},
	}

	encoded, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		// Same contract as the NDJSON path: a marshal failure on this shape
		// is a programming error, not a runtime condition.
		panic(fmt.Sprintf("localsync-lint: marshal sarif log: %v", err))
	}

	fmt.Fprintf(w, "%s\n", encoded)
}

// sarifLevel maps the two severities onto SARIF's level vocabulary; anything
// unknown degrades to a note rather than inventing a level.
func sarifLevel(severity cqrslint.Severity) string {
	if severity == cqrslint.SeverityError {
		return "error"
	}

	if severity == cqrslint.SeverityWarning {
		return "warning"
	}

	return "note"
}

func emitSummary(w io.Writer, counts findingCounts, opts outputOptions, elapsed time.Duration) {
	total := counts.errors + counts.warnings

	if total == 0 && counts.suppressed == 0 {
		if opts.verbose {
			fmt.Fprintf(w, "localsync-lint: clean (%s)\n", elapsed.Round(time.Microsecond))
		} else {
			fmt.Fprintln(w, "localsync-lint: clean")
		}

		return
	}

	parts := make([]string, 0, 3)

	if counts.errors > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", counts.errors, plural("error", counts.errors)))
	}

	if counts.warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", counts.warnings, plural("warning", counts.warnings)))
	}

	if counts.suppressed > 0 {
		parts = append(parts, fmt.Sprintf("%d suppressed", counts.suppressed))
	}

	if opts.verbose {
		fmt.Fprintf(w, "localsync-lint: %s (%s)\n", strings.Join(parts, ", "), elapsed.Round(time.Microsecond))
	} else {
		fmt.Fprintf(w, "localsync-lint: %s\n", strings.Join(parts, ", "))
	}
}

func exitCode(findings []cqrslint.Finding, opts outputOptions) int {
	for _, finding := range findings {
		if finding.Suppressed {
			continue
		}

		if finding.Severity == cqrslint.SeverityError {
			return 1
		}

		if opts.strict && finding.Severity == cqrslint.SeverityWarning {
			return 1
		}
	}

	return 0
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}

	return word + "s"
}

func printRules(w io.Writer) {
	rules := cqrslint.Rules()

	fmt.Fprintf(w, "%-7s  %-8s  %-32s  %s\n", "RULE", "SEVERITY", "TITLE", "RATIONALE")

	for _, rule := range rules {
		fmt.Fprintf(w, "%-7s  %-8s  %-32s  %s\n", rule.ID, rule.Severity, rule.Title, rule.Rationale)
	}
}

func printUsage(fs *flag.FlagSet) {
	w := fs.Output()

	fmt.Fprintf(w, "Usage: localsync-lint [flags] [package]\n\n")
	fmt.Fprintf(w, "Statically verifies go-localsync CQRS architectural invariants.\n\n")
	fmt.Fprintf(w, "Flags:\n")

	fs.PrintDefaults()

	fmt.Fprintf(w, "\nSuppression directives:\n")
	fmt.Fprintf(w, "  //cqrs-lint:ignore C0005         silence a rule on the next/same line\n")
	fmt.Fprintf(w, "  //cqrs-lint:ignore C0005 reason  with an optional reason\n")
	fmt.Fprintf(w, "  //cqrs-lint:ignore all           silence every rule at this position\n")
	fmt.Fprintf(w, "  //cqrs-lint:ignore-file C0005    silence a rule for the entire file\n")
	fmt.Fprintf(w, "  /* cqrs-lint:ignore C0005 */     block-comment form of any directive\n")
	fmt.Fprintf(w, "  //cqrs-lint:ignore-start C0005   begin a suppressed interval\n")
	fmt.Fprintf(w, "  //cqrs-lint:ignore-end C0005     end the interval (bare ignore-end closes all)\n")
	fmt.Fprintf(w, "\nRun -no-suppress to ignore every directive (CI hardening).\n")
}
