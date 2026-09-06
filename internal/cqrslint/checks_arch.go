package cqrslint

import (
	"go/ast"
	"strings"
)

// Architectural checks C0011-C0015. Each encodes one documented invariant of
// the single-aggregate pull-mirror design (ADR-0004 + AGENTS.md) that the
// C0001-C0010 set does not yet pin.

// eventStoreWriteSelectors are method names that write events to a store.
// They must never appear inside the projector (read-side only).
//
//nolint:gochecknoglobals // immutable name set, not mutable state
var eventStoreWriteSelectors = map[string]bool{
	"Append": true,
	"Save":   true,
}

// checkSingleProjection (C0011): exactly one type may implement the
// projection — enforced as exactly one EventTypes method in the package
// (ADR-0004: "one projection").
func checkSingleProjection(pkg *Package) []Finding {
	var findings []Finding

	seen := false

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "EventTypes" {
				continue
			}

			if seen {
				findings = append(findings, errorAt(pkg, fn, ruleSingleProjection,
					"second projection discovered; ADR-0004 allows exactly one projection"))
			}

			seen = true
		}
	}

	return findings
}

// checkFoldPurity (C0012): the fold functions must be deterministic — they
// replay historical events, so any time.Now/time.Since call would make
// replays diverge from live folds.
func checkFoldPurity(pkg *Package) []Finding {
	var findings []Finding

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !strings.HasPrefix(fn.Name.Name, "fold") || fn.Body == nil {
				continue
			}

			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.SelectorExpr)
				if !ok || call.Sel == nil {
					return true
				}

				if ident, ok := call.X.(*ast.Ident); ok && ident.Name == "time" &&
					(call.Sel.Name == "Now" || call.Sel.Name == "Since") {
					findings = append(findings, errorAt(pkg, call, ruleFoldPurity,
						"fold must be deterministic; time."+call.Sel.Name+" makes replays diverge"))
				}

				return true
			})
		}
	}

	return findings
}

// checkProjectorReadOnly (C0013): the projector consumes events; it must
// never write to the event store. Any Append/Save call inside a Projector
// method inverts the read-side contract.
func checkProjectorReadOnly(pkg *Package) []Finding {
	var findings []Finding

	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil || receiverTypeName(fn) != "Projector" {
				continue
			}

			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.SelectorExpr)
				if !ok || call.Sel == nil || !eventStoreWriteSelectors[call.Sel.Name] {
					return true
				}

				findings = append(findings, errorAt(pkg, call, ruleProjectorReadOnly,
					"the projector must not write events ("+call.Sel.Name+"); it is read-side only"))

				return true
			})
		}
	}

	return findings
}

// checkWireValueLiterals (C0014): the canonical event wire values are the
// const declarations in the file that owns them; string literals repeating
// those values anywhere else are drift waiting to happen.
func checkWireValueLiterals(pkg *Package) []Finding {
	ownerFile := make(map[string]string, len(canonicalEventConsts)+1)
	ownerFile[canonicalAggregateType] = declaringFile(pkg, func(value string) bool {
		return value == canonicalAggregateType
	})

	for _, wire := range canonicalEventConsts {
		ownerFile[wire] = declaringFile(pkg, func(value string) bool {
			return value == wire
		})
	}

	var findings []Finding

	for _, file := range pkg.Files {
		relFile := pkg.RelFile(pkg.Fset.Position(file.Pos()).Filename)

		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok {
				return true
			}

			value, isString := literalStringValue(lit)
			if !isString {
				return true
			}

			owner, pinned := ownerFile[value]
			if !pinned || owner == "" || owner == relFile {
				return true
			}

			findings = append(findings, errorAt(pkg, lit, ruleWireValueLiterals,
				"wire value "+value+" must be referenced via its const; only "+owner+" may spell it"))

			return true
		})
	}

	return findings
}

// declaringFile returns the relativized path of the file containing a string
// const whose value satisfies match, or "" when none declares it.
func declaringFile(pkg *Package, match func(string) bool) string {
	for _, file := range pkg.Files {
		hit := false

		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok || len(valueSpec.Values) != 1 {
					continue
				}

				if value, isString := literalStringValue(valueSpec.Values[0]); isString && match(value) {
					hit = true
				}
			}
		}

		if hit {
			return pkg.RelFile(pkg.Fset.Position(file.Pos()).Filename)
		}
	}

	return ""
}

// receiverTypeName returns the type name a method is declared on ("" for
// plain functions).
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}

	expr := fn.Recv.List[0].Type

	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}

	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}

	return ""
}

// checkNewEventsTypeLiterals (C0015): event types passed to NewEvents must
// come from the declared consts; an inline string literal in the event-type
// slice bypasses the single source of truth C0002 establishes.
func checkNewEventsTypeLiterals(pkg *Package) []Finding {
	var findings []Finding

	for _, file := range pkg.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "NewEvents" {
				return true
			}

			for _, arg := range call.Args {
				composite, ok := arg.(*ast.CompositeLit)
				if !ok {
					continue
				}

				for _, elt := range composite.Elts {
					lit, ok := elt.(*ast.BasicLit)
					if !ok {
						continue
					}

					findings = append(findings, errorAt(pkg, lit, ruleNewEventsTypeLiterals,
						"NewEvents event types must use the declared consts, not inline literals"))
				}
			}

			return true
		})
	}

	return findings
}
