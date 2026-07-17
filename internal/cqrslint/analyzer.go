package cqrslint

// Rule identifiers. Declared as constants so every check references the same
// spelling (satisfies goconst and prevents drift between the check and the
// Rules() catalog).
const (
	ruleAggregateTypeConst    = "C0001"
	ruleEventTypeConsts       = "C0002"
	ruleFoldSwitchCoverage    = "C0003"
	ruleProjectorEventTypes   = "C0004"
	ruleHasChangedProviderAgn = "C0005"
	ruleNoQueryDispatcher     = "C0006"
	ruleNoSyncActionInCQRS    = "C0007"
	ruleProjectionLockGuard   = "C0008"
	rulePayloadJSONTags       = "C0009"
	ruleNewEventsUsesAggType  = "C0010"
)

// canonicalAggregateType is the sole permitted aggregate type identifier
// (ADR-0004: single sync_item aggregate).
const canonicalAggregateType = "sync_item"

// canonicalEventConsts maps each required event const identifier to its wire
// string. A compliant package declares exactly these three (ADR-0004: three
// fixed events).
//
//nolint:gochecknoglobals // immutable declaration table, not mutable state
var canonicalEventConsts = map[string]string{
	"EventItemSynced":        "sync_item.synced",
	"EventItemConflictFound": "sync_item.conflict_found",
	"EventItemTombstoned":    "sync_item.tombstoned",
}

// canonicalPayloadStructs are the event payload types whose wire format must be
// stable; every named field needs an explicit json tag.
//
//nolint:gochecknoglobals // immutable declaration table, not mutable state
var canonicalPayloadStructs = []string{
	"ItemSyncedPayload",
	"ItemConflictFoundPayload",
	"ItemTombstonedPayload",
}

// Rule describes a single lint rule for the -list output.
type Rule struct {
	ID        string
	Severity  Severity
	Title     string
	Rationale string
}

// Rules returns the catalog of rules the analyzer enforces, in ID order.
// Each rationale cites the design decision the rule protects so that a finding
// always traces back to documentation, never to taste.
func Rules() []Rule {
	return []Rule{
		{
			ID: ruleAggregateTypeConst, Severity: SeverityError,
			Title:     "single aggregate type",
			Rationale: "ADR-0004: exactly one event.AggregateType const valued \"sync_item\".",
		},
		{
			ID: ruleEventTypeConsts, Severity: SeverityError,
			Title:     "three fixed event types",
			Rationale: "ADR-0004: exactly three event.Type consts (Synced, ConflictFound, Tombstoned).",
		},
		{
			ID: ruleFoldSwitchCoverage, Severity: SeverityError,
			Title:     "fold covers all events",
			Rationale: "Every declared event must be handled by the fold switch or the aggregate corrupts.",
		},
		{
			ID: ruleProjectorEventTypes, Severity: SeverityError,
			Title:     "projector subscribes to all events",
			Rationale: "Projector.EventTypes must return every event const or projections drift.",
		},
		{
			ID: ruleHasChangedProviderAgn, Severity: SeverityError,
			Title:     "hasChanged is provider-agnostic",
			Rationale: "ADR-0007: hasChanged may only read ContentHash/UpdatedAt/Type.",
		},
		{
			ID: ruleNoQueryDispatcher, Severity: SeverityError,
			Title:     "no query dispatcher",
			Rationale: "AGENTS.md: reads call the ReadModel directly; no query.Dispatcher.",
		},
		{
			ID: ruleNoSyncActionInCQRS, Severity: SeverityError,
			Title:     "SyncAction stays in pkg/sync",
			Rationale: "SyncAction/ItemSyncResult are the architectural seam in pkg/sync, not pkg/cqrs.",
		},
		{
			ID: ruleProjectionLockGuard, Severity: SeverityError,
			Title:     "projection version-gate is locked",
			Rationale: "Projector.Handle must hold a mutex before the version-gate (concurrent live+replay).",
		},
		{
			ID: rulePayloadJSONTags, Severity: SeverityError,
			Title:     "payload fields have json tags",
			Rationale: "Event payloads are a wire contract; every field needs an explicit json tag.",
		},
		{
			ID: ruleNewEventsUsesAggType, Severity: SeverityError,
			Title:     "NewEvents uses aggregateType const",
			Rationale: "All events must be tagged with the aggregateType const, never a literal.",
		},
	}
}

// Run executes every registered check against pkg and returns the findings,
// sorted by (file, line, rule) for stable, diff-friendly output.
func Run(pkg *Package) []Finding {
	findings := make([]Finding, 0, estimateFindings(pkg))

	for _, check := range allChecks {
		findings = append(findings, check(pkg)...)
	}

	SortFindings(findings)

	return findings
}

// estimateFindings returns a rough prealloc hint: one slot per file per check.
func estimateFindings(pkg *Package) int {
	return len(pkg.Files) * len(allChecks)
}
