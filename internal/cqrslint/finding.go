// Package cqrslint statically enforces the go-localsync CQRS architectural
// invariants documented in AGENTS.md and ADR-0004 ("one sync_item aggregate,
// three fixed events, one projection").
//
// The analyzer parses the Go sources of a target package (pkg/cqrs by default)
// with the standard-library go/parser and walks the AST. It performs no type
// resolution and adds no dependencies, so it runs fast and hermetically.
//
// Each check maps to a documented invariant so that a violation is always
// traceable to a design decision, never to taste.
package cqrslint

import (
	"fmt"
	"sort"
)

// Severity ranks how seriously a finding should be treated.
type Severity string

const (
	// SeverityError fails the run (non-zero exit).
	SeverityError Severity = "error"
	// SeverityWarning is reported but does not fail the run unless -fail-on-warning is set.
	SeverityWarning Severity = "warning"
)

// Finding is a single rule violation reported by a check.
type Finding struct {
	Rule     string
	Severity Severity
	File     string
	Line     int
	Message  string
}

// String renders a finding in the "<file>:<line>: <rule> <severity>: <msg>" form
// understood by editors and CI log parsers (same shape as compiler diagnostics).
func (f Finding) String() string {
	location := f.File

	if f.Line > 0 {
		location = fmt.Sprintf("%s:%d", location, f.Line)
	}

	return fmt.Sprintf("%s: %s %s: %s", location, f.Rule, f.Severity, f.Message)
}

// SortFindings orders findings by file, then line, then rule for stable output.
func SortFindings(findings []Finding) {
	sort.SliceStable(findings, func(left, right int) bool {
		if findings[left].File != findings[right].File {
			return findings[left].File < findings[right].File
		}

		if findings[left].Line != findings[right].Line {
			return findings[left].Line < findings[right].Line
		}

		return findings[left].Rule < findings[right].Rule
	})
}
