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
	return checkCanonicalEventCoverage(
		pkg, "fold",
		ruleFoldSwitchCoverage,
		"fold function not found; cannot verify event coverage",
		collectSwitchCaseIdents,
		"fold switch does not case event ",
	)
}

// checkProjectorEventTypes (C0004): Projector.EventTypes must reference every
// canonical event const so the projection subscribes to all of them.
func checkProjectorEventTypes(pkg *Package) []Finding {
	return checkCanonicalEventCoverage(
		pkg, "EventTypes",
		ruleProjectorEventTypes,
		"EventTypes method not found; cannot verify projection subscriptions",
		collectEventConstIdents,
		"Projector.EventTypes does not include event ",
	)
}

// checkCanonicalEventCoverage is the shared backbone for C0003 and C0004:
// locate the named function/method, collect the set of event-const identifiers
// it references (via extractIds), then emit one error finding per canonical
// event const that is missing. Returns a single warning finding if the
// function itself is absent so the user knows the rule did not run.
func checkCanonicalEventCoverage(
	pkg *Package,
	funcName, rule, missingMessage string,
	extractIds func(*ast.BlockStmt) map[string]bool,
	missingEventMessage string,
) []Finding {
	fn, missing := findFuncOrWarn(pkg, funcName, rule, missingMessage)
	if missing != nil {
		return missing
	}

	referenced := extractIds(fn.Body)

	var findings []Finding

	for eventConst := range canonicalEventConsts {
		if !referenced[eventConst] {
			findings = append(findings, errorMsg(rule, missingEventMessage+eventConst))
		}
	}

	return findings
}

// findFuncOrWarn looks up a package-level function or method by name. When the
// function is missing or has no body, it returns a non-nil warning finding
// slice so callers can early-return without re-typing the boilerplate. Used
// by every check that needs to inspect a named function or method.
func findFuncOrWarn(pkg *Package, funcName, rule, missingMessage string) (*ast.FuncDecl, []Finding) {
	fn := findFunc(pkg, funcName)
	if fn == nil || fn.Body == nil {
		return nil, []Finding{warningMsg(rule, missingMessage)}
	}

	return fn, nil
}

// checkDiffProviderAgn (C0005, ADR-0007): change detection must stay
// provider-agnostic. Since M17 (2026-11-21) the invariant lives in
// model.Baseline.Diff (pkg/data/model): if the analyzed package DECLARES a
// Diff method, its remote-operand field reads must stay in the allowed set;
// otherwise the package must delegate via the BaselineFromItem(...).Diff(...)
// seam — a package that neither owns nor delegates the seam gets a warning.
func checkDiffProviderAgn(pkg *Package) []Finding {
	if fn := findFunc(pkg, "Diff"); fn != nil && fn.Body != nil {
		var findings []Finding

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil {
				return true
			}

			operand, ok := sel.X.(*ast.Ident)
			if !ok || operand.Name != "remote" {
				return true
			}

			field := sel.Sel.Name

			if allowedHasChangedFields[field] {
				return true
			}

			findings = append(findings, errorAt(pkg, sel, ruleHasChangedProviderAgn,
				"Baseline.Diff must only read ContentHash/UpdatedAt/Type (ADR-0007); references "+
					operand.Name+"."+field))

			return true
		})

		return findings
	}

	for _, file := range pkg.Files {
		delegates := false

		ast.Inspect(file, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok &&
				(ident.Name == "BaselineFromItem" || ident.Name == "Diff") {
				delegates = true
			}

			return !delegates
		})

		if delegates {
			return nil
		}
	}

	return []Finding{warningMsg(ruleHasChangedProviderAgn,
		"change-detection seam not found (hasChanged was replaced by model.Baseline.Diff in M17); "+
			"cannot verify provider-agnostic invariant")}
}

// checkProjectionLockGuard (C0008): Projector.Handle must acquire a mutex Lock
// before the version-gate check so concurrent live+replay delivery serializes.
func checkProjectionLockGuard(pkg *Package) []Finding {
	fn, missing := findFuncOrWarn(pkg, "Handle", ruleProjectionLockGuard,
		"Projector.Handle not found; cannot verify mutex guard")
	if missing != nil {
		return missing
	}

	if bodyContainsLockCall(fn.Body) {
		return nil
	}

	return []Finding{errorAt(pkg, fn, ruleProjectionLockGuard,
		"Projector.Handle must acquire a mutex Lock before the version-gate")}
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
