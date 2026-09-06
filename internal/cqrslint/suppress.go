package cqrslint

import (
	"go/ast"
	"strings"
)

// Directive markers. The canonical prefix is //cqrs-lint: followed by the
// action keyword (ignore, ignore-file, ignore-start, ignore-end), a space,
// and the rule list. Block comments carry the same directives after a
// /* decoration ("/*cqrs-lint:..." or "/* cqrs-lint:...").
const (
	directivePrefix      = "//cqrs-lint:"
	blockDirectiveMarker = "cqrs-lint:"
	directiveIgnore      = "ignore"
	directiveIgnoreF     = "ignore-file"
	directiveIgnoreStart = "ignore-start"
	directiveIgnoreEnd   = "ignore-end"
	suppressAllRules     = "all"
)

// unclosedRangeEnd is the sentinel end line for an ignore-start whose
// matching ignore-end never appears: the range suppresses to end of file
// (and a warning makes the missing close visible under --strict).
const unclosedRangeEnd = 1 << 30

// Suppressor decides whether a Finding is silenced by an inline
// //cqrs-lint: directive in the source. It is built once per package
// load and applied to every finding during Run.
type Suppressor struct {
	// fileRules maps relativized file path to the set of suppressed rule IDs
	// at file scope (//cqrs-lint:ignore-file).
	fileRules map[string]map[string]bool

	// lineRules maps relativized file path to line number to the set of
	// suppressed rule IDs at that line (//cqrs-lint:ignore).
	lineRules map[string]map[int]map[string]bool

	// rangeRules maps relativized file path to rule ID (or "all") to the
	// suppressed line ranges (//cqrs-lint:ignore-start/ignore-end).
	rangeRules map[string]map[string][]suppressedRange

	// openRanges tracks the ranges opened but not yet closed during the
	// scan: file path to rule ID to the range opened at its start line.
	openRanges map[string]map[string]suppressedRange

	// provenance records the directive kind and optional reason per
	// (file, rule) at file scope — the suppression audit trail.
	fileProvenance map[string]map[string]directiveInfo

	// lineProvenance records the directive kind and reason per (file, line, rule).
	lineProvenance map[string]map[int]map[string]directiveInfo

	// directiveFindings collects warnings about the directives themselves:
	// rule IDs that are not in the catalog (and not "all"), nested
	// ignore-start ranges, and ignore-end without a matching open range.
	// Surfaced as warnings so stale or misused suppressions cannot hide
	// forever.
	directiveFindings []Finding
}

// directiveInfo captures which directive silenced a finding and why.
type directiveInfo struct {
	kind   string // "ignore", "ignore-file", or "ignore-start"
	reason string // optional human reason following the rule list
}

// suppressedRange is one [start, end] line interval in which a rule is
// silenced. start is the ignore-start directive's line; end is the
// ignore-end's line, or unclosedRangeEnd when the range was never closed.
type suppressedRange struct {
	start, end int
	reason     string
}

// newSuppressor scans every comment in the package for //cqrs-lint:
// directives and builds the lookup tables. Line-level directives silence
// findings on the same line or the immediately following line; range
// directives (ignore-start/ignore-end) silence everything between them;
// block comments (/* cqrs-lint:... */) carry the same directives anywhere
// inside the comment.
func newSuppressor(pkg *Package) Suppressor {
	s := Suppressor{
		fileRules:      map[string]map[string]bool{},
		lineRules:      map[string]map[int]map[string]bool{},
		rangeRules:     map[string]map[string][]suppressedRange{},
		openRanges:     map[string]map[string]suppressedRange{},
		fileProvenance: map[string]map[string]directiveInfo{},
		lineProvenance: map[string]map[int]map[string]directiveInfo{},
	}

	for _, file := range pkg.Files {
		relFile := pkg.RelFile(pkg.Fset.Position(file.Pos()).Filename)

		for _, group := range file.Comments {
			for _, comment := range group.List {
				s.parseComment(pkg, relFile, comment)
			}
		}

		s.finalizeRanges(relFile)
	}

	return s
}

// parseComment inspects a single comment for suppression directives. Line
// comments hold at most one directive; block comments are scanned line by
// line so a directive may sit anywhere inside /* ... */.
func (s *Suppressor) parseComment(pkg *Package, relFile string, comment *ast.Comment) {
	if strings.HasPrefix(comment.Text, "/*") {
		baseLine := pkg.Fset.Position(comment.Pos()).Line

		for i, raw := range strings.Split(comment.Text, "\n") {
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), "/*"), "*/"))
			text = strings.TrimSpace(strings.TrimPrefix(text, "*"))
			if !strings.HasPrefix(text, blockDirectiveMarker) {
				continue
			}

			s.parseDirective(relFile, text, baseLine+i)
		}

		return
	}

	if !strings.HasPrefix(comment.Text, directivePrefix) {
		return
	}

	line := pkg.Fset.Position(comment.Pos()).Line
	s.parseDirective(relFile, strings.TrimPrefix(comment.Text, directivePrefix), line)
}

// parseDirective records one directive body (text after the marker) found on
// the given line. Malformed directives (empty rule list) are silently
// ignored — they are not lint findings themselves.
func (s *Suppressor) parseDirective(relFile, body string, line int) {
	switch {
	case strings.HasPrefix(body, directiveIgnoreStart):
		rules, reason := parseDirectiveRules(body, directiveIgnoreStart)
		s.recordUnknownRules(relFile, line, rules)
		s.openRange(relFile, rules, line, reason)

	case strings.HasPrefix(body, directiveIgnoreF):
		rules, reason := parseDirectiveRules(body, directiveIgnoreF)
		info := directiveInfo{kind: directiveIgnoreF, reason: reason}
		s.addFileRules(relFile, rules)
		s.recordFileProvenance(relFile, rules, info)
		s.recordUnknownRules(relFile, line, rules)

	case strings.HasPrefix(body, directiveIgnoreEnd):
		rules, _ := parseDirectiveRules(body, directiveIgnoreEnd)
		s.recordUnknownRules(relFile, line, rules)
		s.closeRange(relFile, rules, line)

	case strings.HasPrefix(body, directiveIgnore):
		rules, reason := parseDirectiveRules(body, directiveIgnore)
		info := directiveInfo{kind: directiveIgnore, reason: reason}
		s.addLineRules(relFile, line, rules)
		s.recordLineProvenance(relFile, line, rules, info)
		s.recordUnknownRules(relFile, line, rules)
	}
}

// openRange starts a suppressed interval for each named rule. A rule whose
// range is already open triggers the nesting guard: the inner start is
// ignored (the outer range already covers it) and a warning is recorded.
func (s *Suppressor) openRange(file string, rules []string, line int, reason string) {
	if len(rules) == 0 {
		return
	}

	if s.openRanges[file] == nil {
		s.openRanges[file] = map[string]suppressedRange{}
	}

	for _, rule := range rules {
		if _, open := s.openRanges[file][rule]; open {
			s.directiveFindings = append(s.directiveFindings, Finding{
				Rule:     rule,
				Severity: SeverityWarning,
				File:     file,
				Line:     line,
				Message:  "ignore-start nests inside an open range for the same rule; the inner start is ignored",
			})

			continue
		}

		s.openRanges[file][rule] = suppressedRange{start: line, end: unclosedRangeEnd, reason: reason}
	}
}

// closeRange ends the open interval(s) at line. An empty rule list closes
// every open range in the file; a named rule that is not open warns.
func (s *Suppressor) closeRange(file string, rules []string, line int) {
	open := s.openRanges[file]

	if len(rules) == 0 {
		if len(open) == 0 {
			s.directiveFindings = append(s.directiveFindings, Finding{
				Rule:     directiveIgnoreEnd,
				Severity: SeverityWarning,
				File:     file,
				Line:     line,
				Message:  "ignore-end closes no open range",
			})

			return
		}

		for rule := range open {
			s.endOpenRange(file, rule, line)
		}

		return
	}

	for _, rule := range rules {
		if _, isOpen := open[rule]; !isOpen {
			s.directiveFindings = append(s.directiveFindings, Finding{
				Rule:     rule,
				Severity: SeverityWarning,
				File:     file,
				Line:     line,
				Message:  "ignore-end without a matching ignore-start",
			})

			continue
		}

		s.endOpenRange(file, rule, line)
	}
}

// endOpenRange moves a rule's open range to the closed list with the given
// end line.
func (s *Suppressor) endOpenRange(file, rule string, line int) {
	started := s.openRanges[file][rule]
	delete(s.openRanges[file], rule)

	if s.rangeRules[file] == nil {
		s.rangeRules[file] = map[string][]suppressedRange{}
	}

	started.end = line
	s.rangeRules[file][rule] = append(s.rangeRules[file][rule], started)
}

// finalizeRanges closes whatever ignore-start ranges a file left open at
// end of file: they suppress to EOF and each warns so the missing
// ignore-end stays visible under --strict.
func (s *Suppressor) finalizeRanges(file string) {
	for rule := range s.openRanges[file] {
		s.endOpenRange(file, rule, unclosedRangeEnd)

		s.directiveFindings = append(s.directiveFindings, Finding{
			Rule:     rule,
			Severity: SeverityWarning,
			File:     file,
			Line:     s.rangeRules[file][rule][len(s.rangeRules[file][rule])-1].start,
			Message:  "ignore-start without a matching ignore-end; the range extends to end of file",
		})
	}

	delete(s.openRanges, file)
}

// recordUnknownRules flags directives naming rule IDs that do not exist in
// the catalog. A typo like //cqrs-lint:ignore C9999 previously silenced
// nothing, silently — now it warns (and fails --strict).
func (s *Suppressor) recordUnknownRules(file string, line int, rules []string) {
	known := map[string]bool{}
	for _, rule := range Rules() {
		known[rule.ID] = true
	}

	for _, rule := range rules {
		if rule == suppressAllRules || known[rule] {
			continue
		}

		// Directives are shared vocabulary: one //cqrs-lint: comment can
		// target this linter OR go-cqrs-lite's consumer linter (rule IDs
		// like C017/E005/A001 — a disjoint 4-char scheme vs our C0ddd).
		// Only IDs that LOOK like ours but are not in the catalog are
		// typos/stale; foreign-scheme IDs belong to the other linter and
		// are left to it.
		if !looksLikeInternalRuleID(rule) {
			continue
		}

		s.directiveFindings = append(s.directiveFindings, Finding{
			Rule:     rule,
			Severity: SeverityWarning,
			File:     file,
			Line:     line,
			Message:  "suppression directive names an unknown rule; it silences nothing (typo or stale after a rule was removed?)",
		})
	}
}

// looksLikeInternalRuleID reports whether id matches this linter's scheme
// (C0 + three digits, e.g. C0001..C0010) without being catalog-dependent.
func looksLikeInternalRuleID(id string) bool {
	if len(id) != 5 || id[0] != 'C' {
		return false
	}

	for _, c := range id[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

func (s *Suppressor) recordFileProvenance(file string, rules []string, info directiveInfo) {
	for _, rule := range rules {
		if s.fileProvenance[file] == nil {
			s.fileProvenance[file] = map[string]directiveInfo{}
		}

		s.fileProvenance[file][rule] = info
	}
}

func (s *Suppressor) recordLineProvenance(file string, line int, rules []string, info directiveInfo) {
	for _, rule := range rules {
		if s.lineProvenance[file] == nil {
			s.lineProvenance[file] = map[int]map[string]directiveInfo{}
		}

		if s.lineProvenance[file][line] == nil {
			s.lineProvenance[file][line] = map[string]directiveInfo{}
		}

		s.lineProvenance[file][line][rule] = info
	}
}

// DirectiveFindings returns the directive audit warnings collected during
// package scan: unknown rule IDs, nested ignore-start ranges, and
// ignore-end without a matching open range.
func (s *Suppressor) DirectiveFindings() []Finding {
	return s.directiveFindings
}

// Suppress reports whether the finding is silenced, and by which directive
// kind and reason (the audit trail). It is the provenance-aware successor to
// IsSuppressed.
func (s *Suppressor) Suppress(f Finding) (bool, string, string) {
	if rules, ok := s.fileRules[f.File]; ok {
		if rules[suppressAllRules] {
			by, reason := s.provenanceFor(f, suppressAllRules)

			return true, by, reason
		}

		if rules[f.Rule] {
			by, reason := s.provenanceFor(f, f.Rule)

			return true, by, reason
		}
	}

	if f.Line == 0 {
		return false, "", ""
	}

	for _, candidate := range []int{f.Line, f.Line - 1} {
		if matched, by, reason := s.matchLineRule(f.File, candidate, f.Rule); matched {
			return true, by, reason
		}
	}

	if matched, by, reason := s.matchRangeRule(f.File, f.Line, f.Rule); matched {
		return true, by, reason
	}

	return false, "", ""
}

// matchRangeRule reports whether a line falls inside an ignore-start/
// ignore-end interval covering the rule (directly or via "all"), with its
// provenance.
func (s *Suppressor) matchRangeRule(file string, line int, rule string) (bool, string, string) {
	byRule, ok := s.rangeRules[file]
	if !ok {
		return false, "", ""
	}

	for _, candidate := range []string{suppressAllRules, rule} {
		for _, r := range byRule[candidate] {
			if line >= r.start && line <= r.end {
				return true, directiveIgnoreStart, r.reason
			}
		}
	}

	return false, "", ""
}

// matchLineRule reports whether a line-scoped directive at (file, line)
// covers the rule (directly or via "all"), with its provenance.
func (s *Suppressor) matchLineRule(file string, line int, rule string) (bool, string, string) {
	if line <= 0 {
		return false, "", ""
	}

	lineMap, ok := s.lineRules[file]
	if !ok {
		return false, "", ""
	}

	rules, ok := lineMap[line]
	if !ok {
		return false, "", ""
	}

	if rules[suppressAllRules] {
		by, reason := s.provenanceAtLine(file, line, suppressAllRules)

		return true, by, reason
	}

	if rules[rule] {
		by, reason := s.provenanceAtLine(file, line, rule)

		return true, by, reason
	}

	return false, "", ""
}

func (s *Suppressor) provenanceFor(f Finding, rule string) (string, string) {
	if info, ok := s.fileProvenance[f.File][rule]; ok {
		return info.kind, info.reason
	}

	return directiveIgnoreF, ""
}

func (s *Suppressor) provenanceAtLine(file string, line int, rule string) (string, string) {
	if byLine, ok := s.lineProvenance[file]; ok {
		if info, ok := byLine[line][rule]; ok {
			return info.kind, info.reason
		}
	}

	return directiveIgnore, ""
}

// parseDirectiveRules extracts the comma-separated rule list from the text
// following the directive keyword. Example:
//
//	"ignore C0005,C0006 reason text" → ["C0005", "C0006"]
//
// parseDirectiveRules extracts the comma-separated rule list and the optional
// trailing reason. Two directive shapes are accepted so ONE comment can serve
// both this linter and go-cqrs-lite's consumer linter:
//
//	"ignore C0005,C0006 reason"      (internal form)
//	"ignore(C0005) reason text"      (go-cqrs-lite form, ADR-0112 style)
func parseDirectiveRules(body, keyword string) ([]string, string) {
	rest := strings.TrimPrefix(body, keyword)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, ""
	}

	// Parenthesized form: the rule list runs from "(" to the matching ")".
	if strings.HasPrefix(rest, "(") {
		end := strings.Index(rest, ")")
		if end < 0 {
			return nil, ""
		}

		inner := strings.TrimSpace(rest[1:end])
		reason := strings.TrimSpace(rest[end+1:])

		return splitRuleList(inner), reason
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil, ""
	}

	var rules []string

	for r := range strings.SplitSeq(fields[0], ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			rules = append(rules, r)
		}
	}

	reason := strings.TrimSpace(strings.Join(fields[1:], " "))

	return rules, reason
}

func splitRuleList(list string) []string {
	var rules []string

	for r := range strings.SplitSeq(list, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			rules = append(rules, r)
		}
	}

	return rules
}

func (s *Suppressor) addFileRules(file string, rules []string) {
	if len(rules) == 0 {
		return
	}

	if s.fileRules[file] == nil {
		s.fileRules[file] = map[string]bool{}
	}

	for _, rule := range rules {
		s.fileRules[file][rule] = true
	}
}

func (s *Suppressor) addLineRules(file string, line int, rules []string) {
	if len(rules) == 0 || line <= 0 {
		return
	}

	if s.lineRules[file] == nil {
		s.lineRules[file] = map[int]map[string]bool{}
	}

	if s.lineRules[file][line] == nil {
		s.lineRules[file][line] = map[string]bool{}
	}

	for _, rule := range rules {
		s.lineRules[file][line][rule] = true
	}
}

// IsSuppressed reports whether the finding is silenced by a directive.
// A finding at (file, line, rule) is suppressed when any of these hold:
//   - A //cqrs-lint:ignore-file directive covers the file and the rule (or all).
//   - A //cqrs-lint:ignore directive sits on the same line and covers the rule.
//   - A //cqrs-lint:ignore directive sits on the preceding line and covers the rule.
//
// Findings with Line == 0 (whole-package violations) can only be suppressed at
// file scope, since there is no line to anchor a line-level directive to.
func (s *Suppressor) IsSuppressed(f Finding) bool {
	suppressed, _, _ := s.Suppress(f)

	return suppressed
}
