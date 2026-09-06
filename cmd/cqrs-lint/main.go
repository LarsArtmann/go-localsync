// Command cqrs-lint statically verifies that a Go package conforms to the
// go-localsync CQRS architectural invariants (ADR-0004 + AGENTS.md).
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
)

func main() {
	target := flag.String("pkg", defaultTarget, "path to the Go package to lint")
	listRules := flag.Bool("list", false, "list all rules and exit")
	strict := flag.Bool("strict", false, "exit non-zero when warnings are present (alias for -fail-on-warning)")
	failOnWarning := flag.Bool("fail-on-warning", false, "exit non-zero when warnings are present")
	jsonOut := flag.Bool("json", false,
		"emit findings as newline-delimited JSON (machine readable; alias for -format=json)")
	format := flag.String("format", "text", "output format: text, json (NDJSON), github (workflow annotations), or sarif")
	quiet := flag.Bool("quiet", false, "suppress all output; communicate through the exit code only")
	verbose := flag.Bool("verbose", false, "show package info, per-rule status, and timing on stderr")
	showSuppressed := flag.Bool("show-suppressed", false, "show findings silenced by //cqrs-lint:ignore directives")
	rules := flag.String("rules", "", "comma-separated rule IDs to run (default: all)")
	excludeRules := flag.String("exclude-rules", "", "comma-separated rule IDs to skip")
	noSuppress := flag.Bool("no-suppress", false,
		"disable //cqrs-lint: directives; every violation counts (CI hardening)")
	explain := flag.String("explain", "", "print the full description of one rule and exit")
	showVersion := flag.Bool("version", false, "print the cqrs-lint version and exit")
	flag.Usage = printUsage

	flag.Parse()

	if *showVersion {
		fmt.Printf("cqrs-lint %s\n", cliVersion)

		return
	}

	if *listRules {
		printRules()

		return
	}

	if *explain != "" {
		rule, ok := cqrslint.RuleByID(*explain)
		if !ok {
			fmt.Fprintf(os.Stderr, "cqrs-lint: unknown rule %q (see --list)\n", *explain)
			os.Exit(2)
		}

		printRuleDetail(rule)

		return
	}

	ruleSelection, err := parseRuleSelection(*rules, *excludeRules)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cqrs-lint:", err)
		os.Exit(2)
	}

	resolvedFormat := *format
	if *jsonOut {
		resolvedFormat = formatJSON
	}

	switch resolvedFormat {
	case formatText, formatJSON, formatGitHub:
		// valid
	default:
		fmt.Fprintf(os.Stderr, "cqrs-lint: unknown -format %q (want text, json, or github)\n", resolvedFormat)
		os.Exit(2)
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
		fmt.Fprintln(os.Stderr, "cqrs-lint:", err)
		os.Exit(2)
	}

	emit(os.Stdout, os.Stderr, report{
		findings:  findings,
		opts:      opts,
		target:    *target,
		fileCount: len(pkg.Files),
		elapsed:   elapsed,
	})

	os.Exit(exitCode(findings, opts))
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
func printRuleDetail(rule cqrslint.Rule) {
	fmt.Printf("%s — %s [%s]\n\n%s\n", rule.ID, rule.Title, rule.Severity, rule.Rationale)
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

	emitFindings(stdout, r.findings, r.opts)
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
	fmt.Fprintf(w, "cqrs-lint: analyzing %s (%d files, %d rules)\n", target, fileCount, len(cqrslint.Rules()))
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
		panic(fmt.Sprintf("cqrs-lint: marshal finding: %v", err))
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

func emitSummary(w io.Writer, counts findingCounts, opts outputOptions, elapsed time.Duration) {
	total := counts.errors + counts.warnings

	if total == 0 && counts.suppressed == 0 {
		if opts.verbose {
			fmt.Fprintf(w, "cqrs-lint: clean (%s)\n", elapsed.Round(time.Microsecond))
		} else {
			fmt.Fprintln(w, "cqrs-lint: clean")
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
		fmt.Fprintf(w, "cqrs-lint: %s (%s)\n", strings.Join(parts, ", "), elapsed.Round(time.Microsecond))
	} else {
		fmt.Fprintf(w, "cqrs-lint: %s\n", strings.Join(parts, ", "))
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

func printRules() {
	rules := cqrslint.Rules()

	fmt.Printf("%-7s  %-8s  %-32s  %s\n", "RULE", "SEVERITY", "TITLE", "RATIONALE")

	for _, rule := range rules {
		fmt.Printf("%-7s  %-8s  %-32s  %s\n", rule.ID, rule.Severity, rule.Title, rule.Rationale)
	}
}

func printUsage() {
	out := flag.CommandLine.Output()

	fmt.Fprintf(out, "Usage: cqrs-lint [flags] [package]\n\n")
	fmt.Fprintf(out, "Statically verifies go-localsync CQRS architectural invariants.\n\n")
	fmt.Fprintf(out, "Flags:\n")

	flag.PrintDefaults()

	fmt.Fprintf(out, "\nSuppression directives:\n")
	fmt.Fprintf(out, "  //cqrs-lint:ignore C0005         silence a rule on the next/same line\n")
	fmt.Fprintf(out, "  //cqrs-lint:ignore C0005 reason  with an optional reason\n")
	fmt.Fprintf(out, "  //cqrs-lint:ignore all           silence every rule at this position\n")
	fmt.Fprintf(out, "  //cqrs-lint:ignore-file C0005    silence a rule for the entire file\n")
	fmt.Fprintf(out, "  /* cqrs-lint:ignore C0005 */     block-comment form of any directive\n")
	fmt.Fprintf(out, "  //cqrs-lint:ignore-start C0005   begin a suppressed interval\n")
	fmt.Fprintf(out, "  //cqrs-lint:ignore-end C0005     end the interval (bare ignore-end closes all)\n")
	fmt.Fprintf(out, "\nRun -no-suppress to ignore every directive (CI hardening).\n")
}
