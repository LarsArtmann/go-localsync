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
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

const defaultTarget = "pkg/cqrs"

func main() {
	target := flag.String("pkg", defaultTarget, "path to the Go package to lint")
	listRules := flag.Bool("list", false, "list all rules and exit")
	strict := flag.Bool("strict", false, "exit non-zero when warnings are present (alias for -fail-on-warning)")
	failOnWarning := flag.Bool("fail-on-warning", false, "exit non-zero when warnings are present")
	jsonOut := flag.Bool("json", false, "emit findings as newline-delimited JSON (machine readable)")
	verbose := flag.Bool("verbose", false, "show package info, per-rule status, and timing on stderr")
	showSuppressed := flag.Bool("show-suppressed", false, "show findings silenced by //cqrs-lint:ignore directives")
	flag.Usage = printUsage

	flag.Parse()

	if *listRules {
		printRules()
		return
	}

	opts := outputOptions{
		strict:         *strict || *failOnWarning,
		verbose:        *verbose,
		showSuppressed: *showSuppressed,
		jsonOut:        *jsonOut,
	}

	start := time.Now()
	pkg, findings, err := analyze(*target)
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

type outputOptions struct {
	strict         bool
	verbose        bool
	showSuppressed bool
	jsonOut        bool
}

type report struct {
	findings  []cqrslint.Finding
	opts      outputOptions
	target    string
	fileCount int
	elapsed   time.Duration
}

func analyze(target string) (*cqrslint.Package, []cqrslint.Finding, error) {
	pkg, err := cqrslint.LoadPackage(target)
	if err != nil {
		return nil, nil, err
	}

	return pkg, cqrslint.Run(pkg), nil
}

func emit(stdout, stderr io.Writer, r report) {
	counts := countFindings(r.findings)

	if r.opts.verbose {
		emitVerboseHeader(stderr, r.target, r.fileCount)
		emitRuleStatus(stderr, r.findings)
	}

	emitFindings(stdout, r.findings, r.opts)
	emitSummary(stderr, counts, r.opts, r.elapsed)
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

		if opts.jsonOut {
			emitFindingJSON(w, finding)
			continue
		}

		fmt.Fprintln(w, finding)
	}
}

func emitFindingJSON(w io.Writer, f cqrslint.Finding) {
	fmt.Fprintf(
		w,
		`{"rule":%q,"severity":%q,"file":%q,"line":%d,"message":%q,"suppressed":%t}`+"\n",
		f.Rule, f.Severity, f.File, f.Line, f.Message, f.Suppressed,
	)
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
}
