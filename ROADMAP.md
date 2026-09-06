# ROADMAP.md

**Project:** go-localsync
**Last Updated:** 2026-09-06

## Overview

Long-term direction and raw ideas not yet refined into actionable tasks. For short/mid-term work, see [TODO_LIST.md](TODO_LIST.md). For the feature inventory, see [FEATURES.md](FEATURES.md). For what shipped in each release, see [CHANGELOG.md](CHANGELOG.md).

---

## Future Themes

### Enhanced Features

- **TUI with Bubble Tea** — interactive terminal UI for browsing events, filtering, and real-time sync monitoring. Lives in a consumer app, not the SDK.
- **Multiple-source sync** — accept multiple sources in one sync run. Requires read model schema to track which source each event belongs to.
- **Daemon / background mode** — run as a cron job or systemd service for periodic sync. Consumer-app concern.
- **Second provider implementation** (GitLab? Jira?) — a second concrete provider would validate the `Provider` interface against a different API shape; today only `provider/github` exists. Follow the nested-module pattern.
- **Real-time sync protocol** — live multi-node sync. The former `SyncRequest`/`SyncResponse` types were removed when the CRDT machinery was deleted; this would need to be built from scratch and is out of scope per [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md).

### Data & Export

- ~~**Export to JSON/CSV**~~ — **SHIPPED**: `stack.ExportEvents` (NDJSON) + `ExportEventsCSV` (FEATURES row 65). Remaining raw idea, uncommitted: streaming export for very large journals.

### Recurring suggestions (raw ideas, unowned)

Ideas that recur across 2026 status reports but were never picked up; collected here so they stop masquerading as open tasks in old snapshots. None is committed to.

- BDD/Ginkgo suite for the sync flow; fuzz tests (`DecideSync`, `StreamID`, the localsync-lint directive parser); property-based tests
- ~~Full-pipeline benchmarks under load (SQLite growth, 10k-event replay)~~ — **SHIPPED**: `docs/benchmarks.md` + protocol runner, from-zero replay, conflict-heavy, upcast-tax benchmarks
- ~~Prometheus `/metrics` endpoint on the HTTP API~~ — **SHIPPED as the generic hook**: `api.WithMetricsHandler` mounts any exporter (promhttp included) under `GET /metrics`; a bundled Prometheus exporter remains an uncommitted idea
- Live updates via WebSocket/SSE from the read model
- Config-file support and provider auto-detection/plugin registry
- NixOS module for consumer deployments
- `govalid` struct tags — revisit only if govalid is published as a proxy-resolvable module with a stable tag format (pivoted 2026-09-05 to real `Validate()` methods; decision in CHANGELOG)
- DLQ HTTP admin endpoints (`GET /dead-letters`, `POST /dead-letters/replay`) — the SDK surface shipped; endpoints stay optional pending owner buy-in on API surface growth
- Upstream contributions to go-cqrs-lite: PR for [#21](https://github.com/LarsArtmann/go-cqrs-lite/issues/21) (bus metadata mapping); watermill `MessageToEvent` reconstructing typed `Causation`; projectionhost auto-delete-on-successful-replay option
- Dedicated per-server/per-stack loggers instead of `WithLogLevel` mutating a shared logger (API hygiene for a future minor)
- Multi-error aggregation for partial sync (`errors.Join` of per-item failures)
- Rename the internal package `internal/cqrslint` → `internal/localsynclint` for full vocabulary alignment (owner-optional; the command rename was the user-visible fix)
- Per-page ETag reuse in provider pagination (upstream go-github-kit feature request if useful)
- `encoding/json` v1→v2 source migration + dropping `GOEXPERIMENT=jsonv2` — blocked until Go 1.27 graduates `encoding/json/v2`; revisit then (deferred by three reviews: session-24/26 plans, 17:52 brutal review)
- v0.7+ breaking-change candidates (bundle with the deprecated-shim removal, Open Question 6): rename `pkg/sync` → `pkg/synclib` (stdlib collision, `stdsync "sync"` alias today); prune the provider interface (`Fetch`/`FetchOptions`/`GetRateLimit`); widen `source string` → `id.ProviderID` across read-model/API signatures; keep-or-trim `ConflictAwareSyncer` (public API, zero in-repo production callers)

---

## Open Questions

1. **Multi-source sync** — Should the read model track which source each event belongs to?
2. **Event retention / TTL** — Automatic cleanup of old events (and purged tombstones)? Configurable? [ADR-0005](docs/adr/0005-tombstone-over-delete.md) defers tombstone purge/TTL.
3. **Conflict timestamp trust (LWW clock-skew)** — provider-supplied clocks decide LWW conflicts; does a skewed provider clock need a skew guard or server-time fallback? Raised by [is-it-what-it-claims-to-be](docs/brainstorming/is-it-what-it-claims-to-be.html); no guard exists today.
4. **Upstream watch: `eventtest`** — adopt `go-cqrs-lite/eventtest` for stack tests once the module has a released version (never tagged as of 2026-09-05).
5. **Multi-aggregate generalisation** — Should go-localsync generalise beyond a single `sync_item` aggregate into a multi-aggregate event-sourcing framework? **Decided: deferred.** See [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). Revisit only if a third+ consumer needs it or `go-cqrs-lite` can't evolve the ergonomics. `go-cqrs-lite v4` remains the cross-project sharing boundary.
6. **Deprecated-alias lifetime** — `id.ExternalID`/`NewExternalID`, `cqrs.AggregateID`, `Syncer.GetStats` shims ship in v0.6.0 for the migration window; drop in v0.7 or hold longer for consumer comfort? (ADR-0009 implies one cycle; owner call at v0.7 planning.)

---

## Non-Goals (Deliberate Scope Boundaries)

- **Multi-writer / distributed sync** — the provider is the sole source of truth per aggregate. No vector clocks, no operation-based CRDTs, no multi-node merge (see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md)).
- **Push ingestion** — go-localsync is pull-only. Push-driven consumers share `go-cqrs-lite v4` directly.
- **Provider sprawl in the core module** — the SDK core stays a pure contract library. Concrete providers live in optional nested modules (reference: [`provider/github`](provider/github), released as `provider/github/v0.1.0`) or in consumer apps (reference: [`github-local-sync`](https://github.com/larsartmann/github-local-sync)).
- **Multi-aggregate framework** — one `sync_item` aggregate, three fixed events, one flat projection. Widening this requires revisiting ADR-0004.

---

## Recorded Decisions

| ADR                                                                  | Decision                                                                                                                                                                                                                                                                                   | Status   |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------- |
| [ADR-0001](docs/adr/0001-cqrs-adoption.md)                           | Adopt event-sourced CQRS via go-cqrs-lite (no legacy CRUD)                                                                                                                                                                                                                                 | Accepted |
| [ADR-0002](docs/adr/0002-branded-ids.md)                             | Branded phantom-type IDs for compile-time safety                                                                                                                                                                                                                                           | Accepted |
| [ADR-0003](docs/adr/0003-crdt-integration.md)                        | Pluggable CRDT conflict resolution (`ConflictResolver[T]`)                                                                                                                                                                                                                                 | Accepted |
| [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) | Defer multi-aggregate generalisation; go-cqrs-lite stays the sharing boundary                                                                                                                                                                                                              | Accepted |
| [ADR-0005](docs/adr/0005-tombstone-over-delete.md)                   | Tombstone-based soft-delete with upstream reconciliation                                                                                                                                                                                                                                   | Accepted |
| [ADR-0006](docs/adr/0006-projectionhost-adoption.md)                 | Adopt `projectionhost.Host` for resilient managed catch-up projection                                                                                                                                                                                                                      | Accepted |
| [ADR-0007](docs/adr/0007-de-githubify-domain-model.md)               | Provider-agnostic domain model (`Attributes` map; ContentHash-first diff)                                                                                                                                                                                                                  | Accepted |
| [ADR-0008](docs/adr/0008-pivot-to-sync-toolkit.md)                   | **Proposed — dormant.** Pivot to a `Host` sync-application-framework (drop `pkg/cqrs`/`pkg/data`/`pkg/api`). Never executed; project continued in the ADR-0004 single-aggregate direction (tombstones, projectionhost, de-githubify, v4, cqrs-lint all shipped within scope).              | Proposed |
| [ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md)                | v0.6 vocabulary alignment — **ENACTED 2026-09-06, untagged** (ships with v0.6.0): `AggregateID()`→`StreamID()`, exactly one user-facing result type (`SyncResult.Batch *BatchOutcome`), `GetStats`→`Stats`, ExternalID→SourceID. Our deliberate `DeriveStreamID` encoding divergence kept. | Accepted |
