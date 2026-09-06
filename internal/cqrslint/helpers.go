package cqrslint

import (
	"go/ast"
	"go/token"
	"strings"
)

// Check is a single invariant verifier. It inspects a parsed package and
// returns zero or more findings. Checks must be pure: they neither mutate the
// AST nor perform I/O.
type Check func(pkg *Package) []Finding

// ruleCheck pairs a check with the rule ID it enforces, so a run can execute
// a selected subset (--rules / --exclude-rules) without post-filtering
// findings — an excluded check never runs at all.
type ruleCheck struct {
	rule  string
	check Check
}

// allChecks is the ordered registry of architectural checks. Order matters
// only for deterministic reporting when two checks share a line.
//
//nolint:gochecknoglobals // immutable ordered registry, not mutable state
var allChecks = []ruleCheck{
	{ruleAggregateTypeConst, checkAggregateTypeConst},       // C0001
	{ruleEventTypeConsts, checkEventTypeConsts},             // C0002
	{ruleFoldSwitchCoverage, checkFoldSwitchCoverage},       // C0003
	{ruleProjectorEventTypes, checkProjectorEventTypes},     // C0004
	{ruleHasChangedProviderAgn, checkHasChangedProviderAgn}, // C0005
	{ruleNoQueryDispatcher, checkNoQueryDispatcher},         // C0006
	{ruleNoSyncActionInCQRS, checkNoSyncActionInCQRS},       // C0007
	{ruleProjectionLockGuard, checkProjectionLockGuard},     // C0008
	{rulePayloadJSONTags, checkPayloadJSONTags},             // C0009
	{ruleNewEventsUsesAggType, checkNewEventsUsesAggType},   // C0010
	{ruleSingleProjection, checkSingleProjection},           // C0011
	{ruleFoldPurity, checkFoldPurity},                       // C0012
	{ruleProjectorReadOnly, checkProjectorReadOnly},         // C0013
	{ruleWireValueLiterals, checkWireValueLiterals},         // C0014
	{ruleNewEventsTypeLiterals, checkNewEventsTypeLiterals}, // C0015
}

// literalStringValue extracts the unquoted value of a string BasicLit, returning
// ok=false for anything that is not a string literal.
func literalStringValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	return strings.Trim(lit.Value, "\"`"), true
}

// visitGenDecls walks every general declaration (const/var/type) in the
// package, invoking fn for each. Useful for type/const discovery.
func visitGenDecls(pkg *Package, fn func(file *ast.File, decl *ast.GenDecl)) {
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				fn(file, gen)
			}
		}
	}
}

// findFunc returns the first package-level or method FuncDecl whose name
// matches. Methods are matched on the function name alone (e.g. "fold",
// "Handle"); receiver type is not constrained because the package is small.
func findFunc(pkg *Package, name string) *ast.FuncDecl {
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			if fn.Name.Name == name {
				return fn
			}
		}
	}

	return nil
}

// findStructType returns the file and struct type for a top-level type named
// name, or nil if the type is missing or not a struct.
func findStructType(pkg *Package, name string) (*ast.File, *ast.StructType) {
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}

			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != name {
					continue
				}

				st, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				return file, st
			}
		}
	}

	return nil, nil
}
