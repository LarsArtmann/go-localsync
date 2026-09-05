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
	"go/ast"
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
	Rule       string
	Severity   Severity
	File       string
	Line       int
	Message    string
	Suppressed bool // true when a //cqrs-lint:ignore directive silences this finding

	// SuppressedBy records which directive kind silenced this finding
	// ("ignore" or "ignore-file"); empty unless Suppressed is true. Together
	// with SuppressedReason it forms the suppression audit trail: silenced
	// findings stay attributable to the directive (and its stated reason)
	// instead of vanishing silently.
	SuppressedBy     string
	SuppressedReason string
}

// errorAt constructs a SeverityError Finding positioned at node. Centralizes the
// repeated `Rule, Severity, File, Line` literal so every check reports the same
// shape and a future field added to Finding only has to be threaded in one place.
func errorAt(pkg *Package, node ast.Node, rule, message string) Finding {
	file, line := pkg.PositionFor(node)

	return Finding{
		Rule: rule, Severity: SeverityError, File: file, Line: line, Message: message,
	}
}

// errorMsg constructs a SeverityError Finding without a position — used for
// whole-file or whole-package violations where no specific node applies.
func errorMsg(rule, message string) Finding {
	return Finding{Rule: rule, Severity: SeverityError, Message: message}
}

// warningMsg constructs a SeverityWarning Finding without a position.
func warningMsg(rule, message string) Finding {
	return Finding{Rule: rule, Severity: SeverityWarning, Message: message}
}

// String renders a finding in the "<file>:<line>: <rule> <severity>: <msg>" form
// understood by editors and CI log parsers (same shape as compiler diagnostics).
// Suppressed findings are suffixed with [suppressed].
func (f Finding) String() string {
	location := f.File

	if f.Line > 0 {
		location = fmt.Sprintf("%s:%d", location, f.Line)
	}

	suffix := ""
	if f.Suppressed {
		suffix = " [suppressed by " + f.SuppressedBy
		if f.SuppressedReason != "" {
			suffix += ": " + f.SuppressedReason
		}
		suffix += "]"
	}

	return fmt.Sprintf("%s: %s %s: %s%s", location, f.Rule, f.Severity, f.Message, suffix)
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
