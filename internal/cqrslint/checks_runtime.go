package cqrslint

import (
	"go/ast"
)

// allowedHasChangedFields is the exhaustive set of model.Item fields hasChanged
// may read (ADR-0007: provider-agnostic change detection).
//
//nolint:gochecknoglobals // immutable declaration set, not mutable state
var allowedHasChangedFields = map[string]bool{
	"ContentHash": true,
	"UpdatedAt":   true,
	"Type":        true,
}

// checkFoldSwitchCoverage (C0003): the fold function's switch must case every
// declared event const, or replay will hit the default error path.
func checkFoldSwitchCoverage(pkg *Package) []Finding {
	fn := findFunc(pkg, "fold")
	if fn == nil || fn.Body == nil {
		return []Finding{{
			Rule: ruleFoldSwitchCoverage, Severity: SeverityWarning,
			Message: "fold function not found; cannot verify event coverage",
		}}
	}

	covered := collectSwitchCaseIdents(fn.Body)

	var findings []Finding

	for eventConst := range canonicalEventConsts {
		if !covered[eventConst] {
			findings = append(findings, Finding{
				Rule: ruleFoldSwitchCoverage, Severity: SeverityError,
				Message: "fold switch does not case event " + eventConst,
			})
		}
	}

	return findings
}

// checkProjectorEventTypes (C0004): Projector.EventTypes must reference every
// canonical event const so the projection subscribes to all of them.
func checkProjectorEventTypes(pkg *Package) []Finding {
	fn := findFunc(pkg, "EventTypes")
	if fn == nil || fn.Body == nil {
		return []Finding{{
			Rule: ruleProjectorEventTypes, Severity: SeverityWarning,
			Message: "EventTypes method not found; cannot verify projection subscriptions",
		}}
	}

	referenced := collectEventConstIdents(fn.Body)

	var findings []Finding

	for eventConst := range canonicalEventConsts {
		if !referenced[eventConst] {
			findings = append(findings, Finding{
				Rule: ruleProjectorEventTypes, Severity: SeverityError,
				Message: "Projector.EventTypes does not include event " + eventConst,
			})
		}
	}

	return findings
}

// checkHasChangedProviderAgn (C0005): hasChanged may only reference
// ContentHash, UpdatedAt, or Type on its local/remote operands (ADR-0007).
func checkHasChangedProviderAgn(pkg *Package) []Finding {
	fn := findFunc(pkg, "hasChanged")
	if fn == nil || fn.Body == nil {
		return []Finding{{
			Rule: ruleHasChangedProviderAgn, Severity: SeverityWarning,
			Message: "hasChanged function not found; cannot verify provider-agnostic invariant",
		}}
	}

	var findings []Finding

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		sel, ok := node.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}

		operand, ok := sel.X.(*ast.Ident)
		if !ok || (operand.Name != "local" && operand.Name != "remote") {
			return true
		}

		field := sel.Sel.Name

		if allowedHasChangedFields[field] {
			return true
		}

		file, line := pkg.PositionFor(sel)

		findings = append(findings, Finding{
			Rule: ruleHasChangedProviderAgn, Severity: SeverityError, File: file, Line: line,
			Message: "hasChanged must only read ContentHash/UpdatedAt/Type (ADR-0007); references " +
				operand.Name + "." + field,
		})

		return true
	})

	return findings
}

// checkProjectionLockGuard (C0008): Projector.Handle must acquire a mutex Lock
// before the version-gate check so concurrent live+replay delivery serializes.
func checkProjectionLockGuard(pkg *Package) []Finding {
	fn := findFunc(pkg, "Handle")
	if fn == nil || fn.Body == nil {
		return []Finding{{
			Rule: ruleProjectionLockGuard, Severity: SeverityWarning,
			Message: "Projector.Handle not found; cannot verify mutex guard",
		}}
	}

	if bodyContainsLockCall(fn.Body) {
		return nil
	}

	file, line := pkg.PositionFor(fn)

	return []Finding{{
		Rule: ruleProjectionLockGuard, Severity: SeverityError, File: file, Line: line,
		Message: "Projector.Handle must acquire a mutex Lock before the version-gate",
	}}
}

// collectSwitchCaseIdents returns the set of identifier names used as case
// expressions in any switch statement within body.
func collectSwitchCaseIdents(body *ast.BlockStmt) map[string]bool {
	covered := map[string]bool{}

	ast.Inspect(body, func(node ast.Node) bool {
		switchStmt, ok := node.(*ast.SwitchStmt)
		if !ok || switchStmt.Body == nil {
			return true
		}

		for _, stmt := range switchStmt.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}

			for _, expr := range clause.List {
				if ident, ok := expr.(*ast.Ident); ok {
					covered[ident.Name] = true
				}
			}
		}

		return true
	})

	return covered
}

// collectEventConstIdents returns the set of canonical event const names that
// appear as identifiers anywhere in body.
func collectEventConstIdents(body *ast.BlockStmt) map[string]bool {
	referenced := map[string]bool{}

	ast.Inspect(body, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}

		if _, canonical := canonicalEventConsts[ident.Name]; canonical {
			referenced[ident.Name] = true
		}

		return true
	})

	return referenced
}

// bodyContainsLockCall reports whether body contains a call to a method named
// Lock (e.g. p.mu.Lock()), the sync.Mutex acquisition primitive.
func bodyContainsLockCall(body *ast.BlockStmt) bool {
	found := false

	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}

		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return true
		}

		if sel.Sel.Name == "Lock" {
			found = true
		}

		return !found
	})

	return found
}
