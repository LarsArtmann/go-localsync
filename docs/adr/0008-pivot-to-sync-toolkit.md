# ADR-0008: Pivot go-localsync to a Sync Application Framework

**Status:** Proposed — dormant (as of 2026-07-19, never executed; the project continued within ADR-0004 scope. See Resolution note at end.)
**Date:** 2026-07-05
**Deciders:** Lars Artmann
**Supersedes:** None. Dissolves the constraint behind [ADR-0004](0004-multi-aggregate-generalisation-deferred.md) without reversing it.
**Full analysis:** [`docs/strategy/2026-07-05_localsync-v2-sync-toolkit-proposal.md`](../strategy/2026-07-05_localsync-v2-sync-toolkit-proposal.md)

## Context

A three-consumer adoption study (github-local-sync, bank-sync, DiscordSync) revealed three findings:

1. **Zero consumers use the core engine.** github-local-sync — the only consumer that imports go-localsync — uses `pkg/provider/`, `pkg/errors/`, `pkg/id/`, and `pkg/testutil/`. It already imports `go-cqrs-lite/stack/sqlite` directly, bypassing go-localsync's `pkg/cqrs/` (1,773 lines). bank-sync and DiscordSync don't import go-localsync at all.

2. **go-cqrs-lite v3.5's `stack/` layer makes `pkg/cqrs/` redundant.** `sqlite.New(dsn)` + `stack.Materialize` + `bundle.RunProjections` replace go-localsync's hand-rolled CQRS wiring.

3. **The three consumers hand-write ~3,270 lines of integration boilerplate** — stack wiring, bus subscription, projection registration, DLQ implementation, lifecycle management, sync loops, reconciliation. Each independently reinvents it, gets it slightly wrong (no checkpoint in 2/3 consumers, no DLQ in 2/3, no graceful shutdown in 2/3), and maintains it alone.

The integration is the hard part. go-cqrs-lite provides the primitives but explicitly refuses to own lifecycle or assembly ("not a framework"). That gap is go-localsync's job.

## Decision

Pivot go-localsync from a **single-aggregate engine** (4,333 lines) to an **opinionated sync application framework** (~1,050 lines) that provides a `Host` — the assembly point that makes wiring go-cqrs-lite + sync + reconciliation genuinely easy.

### What stays

| Package         | Purpose                               |
| --------------- | ------------------------------------- |
| `pkg/provider/` | Provider contract — pull + push       |
| `pkg/crdt/`     | Conflict resolution (already generic) |
| `pkg/errors/`   | Error taxonomy (already standalone)   |
| `pkg/id/`       | Sync-infrastructure branded IDs       |
| `pkg/testutil/` | Mock provider + test helpers          |

### What's new

| Package           | Purpose                                                                                   |
| ----------------- | ----------------------------------------------------------------------------------------- |
| `pkg/host/`       | The opinionated assembly point: `Host`, `NewHost`, lifecycle, health, graceful shutdown   |
| `pkg/sync/`       | Generic `PullSyncer[T]` with pluggable `ChangeDetector[T]` + `SyncSink[T]`, retry/backoff |
| `pkg/reconcile/`  | Reconciliation loop framework: interval scheduler + `Healer` interface                    |
| `pkg/projection/` | Built-in SQLite DLQ (promoted from DiscordSync), projection registration                  |

### What goes

| Package            | Lines | Replaced By                         |
| ------------------ | ----- | ----------------------------------- |
| `pkg/cqrs/`        | 1,773 | `go-cqrs-lite/stack/` + `pkg/host/` |
| `pkg/data/model/`  | ~400  | Consumer owns domain model          |
| `pkg/data/schema/` | ~50   | go-cqrs-lite `schema/` module       |
| `pkg/api/`         | ~320  | cqrs-htmx or consumer-owned         |

### The Host contract

The consumer provides domain types (aggregate state, decider, projections, provider) and gets back a running application with correct lifecycle, checkpoint, DLQ, retry, reconciliation, and graceful shutdown. ~30 lines of consumer code replace 700–1,520 lines of manual wiring.

### Execution: 4 phases

1. **Build the Host** (additive, zero breakage) — new packages alongside existing code
2. **Validate** against github-local-sync (simplest consumer, already uses stack/sqlite)
3. **Deprecate + remove** old packages, tag v2.0.0
4. **Adopt** bank-sync and DiscordSync

## Rationale

### Why framework, not loose library?

go-cqrs-lite is a library (primitives, no lifecycle). The three consumers prove that the integration gap is real: ~3,270 lines of structurally identical boilerplate, independently maintained, with correctness bugs (missing checkpoints, missing DLQs, missing graceful shutdown) in 2/3 consumers.

### Why not stay as single-aggregate engine?

The single-aggregate model (ADR-0004) serves zero consumers. Every consumer has a different aggregate shape. ADR-0007 de-GitHubified the vocabulary; this ADR removes the structural constraint.

### Why now?

go-cqrs-lite v3.5's `stack/` presets are mature. The one consumer that uses go-localsync already adopted them. The duplication is actively maintained dead code.

## Consequences

### Positive

- ~3,270 lines of consumer boilerplate eliminated
- Correctness baked in (checkpoint, DLQ, graceful shutdown — correct by default)
- All three consumers served
- 71% code reduction in go-localsync
- ADR-0004 generics concern dissolved

### Negative

- Breaking change for hypothetical `pkg/cqrs/` consumers (mitigated: github-local-sync already moved to stack/)
- Framework coupling (mitigated: Host is thin, consumers can drop to stack/ directly)

## References

- [Full strategy analysis](../strategy/2026-07-05_localsync-v2-sync-toolkit-proposal.md)
- [ADR-0004](0004-multi-aggregate-generalisation-deferred.md) — multi-aggregate deferral (constraint dissolved)
- [ADR-0007](0007-de-githubify-domain-model.md) — de-GitHubify (complementary)
- [Adoption feedback](../feedback/2026-06-23_discordsync-adoption-feedback.html) — 6 structural findings

---

## Resolution (2026-07-19)

**This ADR remains Proposed. It was neither accepted nor formally rejected; it was overtaken by continued work inside the ADR-0004 single-aggregate scope.** The packages this ADR proposes to drop (`pkg/cqrs`, `pkg/data`, `pkg/api`) all still exist and are actively maintained (14 commits across them since this ADR's date). The `pkg/host/`, `pkg/reconcile/`, and `pkg/projection/` framework packages this ADR proposes were never created.

In the same window, the project shipped the correctness properties this ADR argued for, but _within_ the existing engine rather than via a rewrite: tombstones + reconciliation (ADR-0005), `projectionhost.Host` with checkpoint + DLQ + crash-restart (ADR-0006), de-githubify (ADR-0007), and a static AST linter (`cqrs-lint`) that enforces ADR-0004's scope at CI time.

**Trigger to reopen:** a third consumer that actually adopts go-localsync and hits the integration-boilerplate wall described in the full analysis. Until then, treat this ADR as dormant, not pending.
