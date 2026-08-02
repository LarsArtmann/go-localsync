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
}

// newSuppressor scans every comment in the package for //cqrs-lint:ignore and
// //cqrs-lint:ignore-file directives and builds the lookup tables. Comments
// are associated with their position so a line-level directive silences
// findings on the same line or the immediately following line.
func newSuppressor(pkg *Package) Suppressor {
	s := Suppressor{
		fileRules: map[string]map[string]bool{},
		lineRules: map[string]map[int]map[string]bool{},
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
		rules := parseDirectiveRules(body, directiveIgnoreF)
		s.addFileRules(relFile, rules)

	case strings.HasPrefix(body, directiveIgnore):
		rules := parseDirectiveRules(body, directiveIgnore)
		s.addLineRules(relFile, position.Line, rules)
	}
}

// parseDirectiveRules extracts the comma-separated rule list from the text
// following the directive keyword. Example:
//
//	"ignore C0005,C0006 reason text" → ["C0005", "C0006"]
func parseDirectiveRules(body, keyword string) []string {
	rest := strings.TrimPrefix(body, keyword)
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return nil
	}

	var rules []string

	for r := range strings.SplitSeq(fields[0], ",") {
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
	if rules, ok := s.fileRules[f.File]; ok {
		if rules[suppressAllRules] || rules[f.Rule] {
			return true
		}
	}

	if f.Line == 0 {
		return false
	}

	for _, candidate := range []int{f.Line, f.Line - 1} {
		if candidate <= 0 {
			continue
		}

		if lineMap, ok := s.lineRules[f.File]; ok {
			if rules, ok := lineMap[candidate]; ok {
				if rules[suppressAllRules] || rules[f.Rule] {
					return true
				}
			}
		}
	}

	return false
}
