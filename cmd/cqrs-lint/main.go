// Command cqrs-lint statically verifies that a Go package conforms to the
// go-localsync CQRS architectural invariants (ADR-0004 + AGENTS.md).
//
// It parses the package with the standard-library go/parser — no type
// resolution, no third-party dependencies — and reports any rule violations.
// Exit codes: 0 clean, 1 findings present, 2 usage/internal error.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/larsartmann/go-localsync/internal/cqrslint"
)

const defaultTarget = "pkg/cqrs"

func main() {
	target := flag.String("pkg", defaultTarget, "path to the Go package to lint")
	listRules := flag.Bool("list", false, "list all rules and exit")
	failOnWarning := flag.Bool("fail-on-warning", false, "exit non-zero when warnings are present")
	jsonOut := flag.Bool("json", false, "emit findings as newline-delimited JSON (machine readable)")
	flag.Usage = printUsage

	flag.Parse()

	if *listRules {
		printRules()
		return
	}

	findings, err := analyze(*target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cqrs-lint:", err)
		os.Exit(2)
	}

	emit(findings, *jsonOut)

	os.Exit(exitCode(findings, *failOnWarning))
}

func analyze(target string) ([]cqrslint.Finding, error) {
	pkg, err := cqrslint.LoadPackage(target)
	if err != nil {
		return nil, err
	}

	return cqrslint.Run(pkg), nil
}

func emit(findings []cqrslint.Finding, jsonOut bool) {
	for _, finding := range findings {
		if jsonOut {
			fmt.Printf(
				`{"rule":%q,"severity":%q,"file":%q,"line":%d,"message":%q}`+"\n",
				finding.Rule, finding.Severity, finding.File, finding.Line, finding.Message,
			)

			continue
		}

		fmt.Println(finding)
	}
}

func exitCode(findings []cqrslint.Finding, failOnWarning bool) int {
	for _, finding := range findings {
		if finding.Severity == cqrslint.SeverityError {
			return 1
		}

		if failOnWarning && finding.Severity == cqrslint.SeverityWarning {
			return 1
		}
	}

	return 0
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

	var b strings.Builder

	fmt.Fprintf(&b, "Usage: cqrs-lint [flags] [package]\n\n")
	fmt.Fprintf(&b, "Statically verifies go-localsync CQRS architectural invariants.\n\n")
	fmt.Fprintf(&b, "Flags:\n")

	fmt.Fprint(out, b.String())

	flag.PrintDefaults()
}
