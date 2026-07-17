package cqrslint

import (
	"go/ast"
)

// bannedScopeTypes are types that must NOT be (re)defined inside pkg/cqrs;
// they live in pkg/sync as the architectural seam (AGENTS.md).
//
//nolint:gochecknoglobals // immutable declaration set, not mutable state
var bannedScopeTypes = map[string]bool{
	"SyncAction":     true,
	"ItemSyncResult": true,
}

// checkNoQueryDispatcher (C0006): no reference to query.Dispatcher or a
// QueryDispatcher identifier may appear (reads go through the ReadModel).
func checkNoQueryDispatcher(pkg *Package) []Finding {
	var findings []Finding

	for _, file := range pkg.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.SelectorExpr:
				if typed.Sel == nil || typed.Sel.Name != "Dispatcher" {
					return true
				}

				ident, ok := typed.X.(*ast.Ident)
				if !ok || ident.Name != "query" {
					return true
				}

				outFile, line := pkg.PositionFor(typed)

				findings = append(findings, Finding{
					Rule: ruleNoQueryDispatcher, Severity: SeverityError, File: outFile, Line: line,
					Message: "query.Dispatcher is banned; reads must call the ReadModel directly (AGENTS.md)",
				})

			case *ast.Ident:
				if typed.Name != "QueryDispatcher" {
					return true
				}

				outFile, line := pkg.PositionFor(typed)

				findings = append(findings, Finding{
					Rule: ruleNoQueryDispatcher, Severity: SeverityError, File: outFile, Line: line,
					Message: "QueryDispatcher identifier is banned; reads must call the ReadModel directly (AGENTS.md)",
				})
			}

			return true
		})
	}

	return findings
}

// checkNoSyncActionInCQRS (C0007): pkg/cqrs must not define the SyncAction or
// ItemSyncResult types — they belong to pkg/sync.
func checkNoSyncActionInCQRS(pkg *Package) []Finding {
	var findings []Finding

	visitGenDecls(pkg, func(_ *ast.File, decl *ast.GenDecl) {
		for _, spec := range decl.Specs {
			switch typed := spec.(type) {
			case *ast.TypeSpec:
				if !bannedScopeTypes[typed.Name.Name] {
					continue
				}

				file, line := pkg.PositionFor(typed)

				findings = append(findings, Finding{
					Rule: ruleNoSyncActionInCQRS, Severity: SeverityError, File: file, Line: line,
					Message: "type " + typed.Name.Name +
						" must live in pkg/sync, not pkg/cqrs (architectural seam)",
				})

			case *ast.ValueSpec:
				for _, name := range typed.Names {
					if !bannedScopeTypes[name.Name] {
						continue
					}

					file, line := pkg.PositionFor(typed)

					findings = append(findings, Finding{
						Rule: ruleNoSyncActionInCQRS, Severity: SeverityError, File: file, Line: line,
						Message: name.Name + " must live in pkg/sync, not pkg/cqrs (architectural seam)",
					})
				}
			}
		}
	})

	return findings
}
