package cqrslint

import (
	"go/ast"
	"strings"
)

// Directive markers. The canonical prefix is //cqrs-lint: followed by the
// action keyword (ignore or ignore-file), a space, and the rule list.
const (
	directivePrefix  = "//cqrs-lint:"
	directiveIgnore  = "ignore"
	directiveIgnoreF = "ignore-file"
	suppressAllRules = "all"
)

// Suppressor decides whether a Finding is silenced by an inline
// //cqrs-lint:ignore directive in the source. It is built once per package
// load and applied to every finding during Run.
type Suppressor struct {
	// fileRules maps relativized file path to the set of suppressed rule IDs
	// at file scope (//cqrs-lint:ignore-file).
	fileRules map[string]map[string]bool

	// lineRules maps relativized file path to line number to the set of
	// suppressed rule IDs at that line (//cqrs-lint:ignore).
	lineRules map[string]map[int]map[string]bool

	// provenance records the directive kind and optional reason per
	// (file, rule) at file scope — the suppression audit trail.
	fileProvenance map[string]map[string]directiveInfo

	// lineProvenance records the directive kind and reason per (file, line, rule).
	lineProvenance map[string]map[int]map[string]directiveInfo

	// unknownRules collects directives naming rule IDs that are not in the
	// catalog (and not "all") — surfaced as warnings so stale suppressions
	// cannot hide forever.
	unknownRules []Finding
}

// directiveInfo captures which directive silenced a finding and why.
type directiveInfo struct {
	kind   string // "ignore" or "ignore-file"
	reason string // optional human reason following the rule list
}

// newSuppressor scans every comment in the package for //cqrs-lint:ignore and
// //cqrs-lint:ignore-file directives and builds the lookup tables. Comments
// are associated with their position so a line-level directive silences
// findings on the same line or the immediately following line.
func newSuppressor(pkg *Package) Suppressor {
	s := Suppressor{
		fileRules:       map[string]map[string]bool{},
		lineRules:       map[string]map[int]map[string]bool{},
		fileProvenance:  map[string]map[string]directiveInfo{},
		lineProvenance:  map[string]map[int]map[string]directiveInfo{},
	}

	for _, file := range pkg.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				s.parseDirective(pkg, comment)
			}
		}
	}

	return s
}

// parseDirective inspects a single comment for a suppression directive and
// records the rule(s) at the appropriate scope. Malformed directives (empty
// rule list) are silently ignored — they are not lint findings themselves.
func (s *Suppressor) parseDirective(pkg *Package, comment *ast.Comment) {
	text := comment.Text
	if !strings.HasPrefix(text, directivePrefix) {
		return
	}

	body := strings.TrimPrefix(text, directivePrefix)
	position := pkg.Fset.Position(comment.Pos())
	relFile := pkg.RelFile(position.Filename)

	switch {
	case strings.HasPrefix(body, directiveIgnoreF):
		rules, reason := parseDirectiveRules(body, directiveIgnoreF)
		info := directiveInfo{kind: directiveIgnoreF, reason: reason}
		s.addFileRules(relFile, rules)
		s.recordFileProvenance(relFile, rules, info)
		s.recordUnknownRules(pkg, relFile, position.Line, rules)

	case strings.HasPrefix(body, directiveIgnore):
		rules, reason := parseDirectiveRules(body, directiveIgnore)
		info := directiveInfo{kind: directiveIgnore, reason: reason}
		s.addLineRules(relFile, position.Line, rules)
		s.recordLineProvenance(relFile, position.Line, rules, info)
		s.recordUnknownRules(pkg, relFile, position.Line, rules)
	}
}

// recordUnknownRules flags directives naming rule IDs that do not exist in
// the catalog. A typo like //cqrs-lint:ignore C9999 previously silenced
// nothing, silently — now it warns (and fails --strict).
func (s *Suppressor) recordUnknownRules(pkg *Package, file string, line int, rules []string) {
	known := map[string]bool{}
	for _, rule := range Rules() {
		known[rule.ID] = true
	}

	for _, rule := range rules {
		if rule == suppressAllRules || known[rule] {
			continue
		}

		s.unknownRules = append(s.unknownRules, Finding{
			Rule:     rule,
			Severity: SeverityWarning,
			File:     file,
			Line:     line,
			Message:  "suppression directive names an unknown rule; it silences nothing (typo or stale after a rule was removed?)",
		})
	}
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

// UnknownRuleFindings returns the stale-directive warnings collected during
// package scan.
func (s *Suppressor) UnknownRuleFindings() []Finding {
	return s.unknownRules
}

// Suppress reports whether the finding is silenced, and by which directive
// kind and reason (the audit trail). It is the provenance-aware successor to
// IsSuppressed.
func (s *Suppressor) Suppress(f Finding) (bool, string, string) {
	if rules, ok := s.fileRules[f.File]; ok {
		if rules[suppressAllRules] {
			return true, s.provenanceFor(f, suppressAllRules)
		}

		if rules[f.Rule] {
			return true, s.provenanceFor(f, f.Rule)
		}
	}

	if f.Line == 0 {
		return false, "", ""
	}

	for _, candidate := range []int{f.Line, f.Line - 1} {
		if candidate <= 0 {
			continue
		}

		if lineMap, ok := s.lineRules[f.File]; ok {
			if rules, ok := lineMap[candidate]; ok {
				if rules[suppressAllRules] {
					return true, s.provenanceAtLine(f.File, candidate, suppressAllRules)
				}

				if rules[f.Rule] {
					return true, s.provenanceAtLine(f.File, candidate, f.Rule)
				}
			}
		}
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
func parseDirectiveRules(body, keyword string) ([]string, string) {
	rest := strings.TrimPrefix(body, keyword)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, ""
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
