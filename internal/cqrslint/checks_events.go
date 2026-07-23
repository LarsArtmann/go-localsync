package cqrslint

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// specHit pairs a const ValueSpec with the file it lives in, so a check can
// report positions without re-scanning.
type specHit struct {
	file *ast.File
	spec *ast.ValueSpec
}

// isSelectorType reports whether expr is the selector `pkgName.selName`
// (e.g. event.Type, event.StreamType).
func isSelectorType(expr ast.Expr, pkgName string, selNames ...string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || !slices.Contains(selNames, sel.Sel.Name) {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == pkgName
}

// collectConstSpecsByType returns every const ValueSpec whose declared type is
// `pkgName.<any selName>` (e.g. all `event.Type` consts).
func collectConstSpecsByType(pkg *Package, pkgName string, selNames ...string) []specHit {
	var hits []specHit

	visitGenDecls(pkg, func(_ *ast.File, decl *ast.GenDecl) {
		if decl.Tok != token.CONST {
			return
		}

		for _, spec := range decl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || !isSelectorType(valueSpec.Type, pkgName, selNames...) {
				continue
			}

			hits = append(hits, specHit{file: nil, spec: valueSpec})
		}
	})

	return hits
}

// checkAggregateTypeConst (C0001): there must be exactly one
// event.StreamType (or legacy event.AggregateType) const and it must equal
// "sync_item" (ADR-0004).
func checkAggregateTypeConst(pkg *Package) []Finding {
	hits := collectConstSpecsByType(pkg, "event", "StreamType", "AggregateType")

	if len(hits) == 0 {
		return []Finding{errorMsg(ruleAggregateTypeConst, fmt.Sprintf(
			"missing event.StreamType const; ADR-0004 requires exactly one valued %q",
			canonicalAggregateType,
		))}
	}

	var findings []Finding

	if len(hits) > 1 {
		findings = append(findings, errorMsg(ruleAggregateTypeConst, fmt.Sprintf(
			"only one event.StreamType const is allowed (ADR-0004 single-aggregate); found %d",
			len(hits),
		)))
	}

	for _, hit := range hits {
		if len(hit.spec.Values) == 0 {
			continue
		}

		value, ok := literalStringValue(hit.spec.Values[0])
		if !ok {
			continue
		}

		if value != canonicalAggregateType {
			findings = append(findings, errorAt(pkg, hit.spec, ruleAggregateTypeConst, fmt.Sprintf(
				"aggregate type must be %q (ADR-0004), got %q",
				canonicalAggregateType, value,
			)))
		}
	}

	return findings
}

// checkEventTypeConsts (C0002): exactly three event.Type consts with the
// canonical names must exist (ADR-0004: three fixed events).
func checkEventTypeConsts(pkg *Package) []Finding {
	hits := collectConstSpecsByType(pkg, "event", "Type")

	var findings []Finding

	declared := make(map[string]string, len(hits))

	for _, hit := range hits {
		name := ""
		if len(hit.spec.Names) > 0 {
			name = hit.spec.Names[0].Name
		}

		value, _ := literalStringValue(firstValue(hit.spec))

		declared[name] = value

		if _, canonical := canonicalEventConsts[name]; !canonical {
			findings = append(findings, errorAt(pkg, hit.spec, ruleEventTypeConsts, fmt.Sprintf(
				"unexpected event.Type const %q; adding events requires revisiting ADR-0004",
				name,
			)))
		}
	}

	for canonicalName := range canonicalEventConsts {
		if _, present := declared[canonicalName]; !present {
			findings = append(findings, errorMsg(ruleEventTypeConsts, fmt.Sprintf(
				"required event const %s is missing (ADR-0004)", canonicalName,
			)))
		}
	}

	return findings
}

// checkPayloadJSONTags (C0009): every named field in a canonical payload struct
// must carry an explicit json tag so the event wire format stays stable.
func checkPayloadJSONTags(pkg *Package) []Finding {
	var findings []Finding

	for _, structName := range canonicalPayloadStructs {
		_, st := findStructType(pkg, structName)
		if st == nil || st.Fields == nil {
			findings = append(findings, warningMsg(rulePayloadJSONTags, fmt.Sprintf(
				"payload struct %s not found; cannot verify json tags", structName,
			)))

			continue
		}

		for _, field := range st.Fields.List {
			if len(field.Names) == 0 {
				continue
			}

			if field.Tag != nil && strings.Contains(field.Tag.Value, "json:") {
				continue
			}

			names := joinFieldNames(field.Names)

			findings = append(findings, errorAt(pkg, field, rulePayloadJSONTags, fmt.Sprintf(
				"%s.%s is missing a json tag; event payloads are a wire contract",
				structName, names,
			)))
		}
	}

	return findings
}

// checkNewEventsUsesAggType (C0010): every event.NewEvents call must pass the
// `aggregateType` const as its second argument, never a literal or alias.
func checkNewEventsUsesAggType(pkg *Package) []Finding {
	var findings []Finding

	for _, file := range pkg.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			if !isEventPackageCall(call.Fun, "NewEvents") {
				return true
			}

			if len(call.Args) < 2 {
				return true
			}

			ident, ok := call.Args[1].(*ast.Ident)
			if ok && ident.Name == "aggregateType" {
				return true
			}

			findings = append(findings, errorAt(pkg, call, ruleNewEventsUsesAggType,
				"event.NewEvents must pass the aggregateType const as its 2nd argument"))

			return true
		})
	}

	return findings
}

// isEventPackageCall reports whether fun is `event.<methodName>`.
func isEventPackageCall(fun ast.Expr, methodName string) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != methodName {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == "event"
}

// firstValue returns the first value expression of a const spec, or nil.
func firstValue(spec *ast.ValueSpec) ast.Expr {
	if len(spec.Values) == 0 {
		return nil
	}

	return spec.Values[0]
}

// joinFieldNames renders a field's identifier list as "a, b".
func joinFieldNames(names []*ast.Ident) string {
	parts := make([]string, 0, len(names))

	for _, ident := range names {
		parts = append(parts, ident.Name)
	}

	return strings.Join(parts, ", ")
}
