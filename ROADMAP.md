# ROADMAP.md

**Project:** go-localsync
**Last Updated:** 2026-07-18

## Overview

Long-term direction and raw ideas not yet refined into actionable tasks. For short/mid-term work, see [TODO_LIST.md](TODO_LIST.md). For the feature inventory, see [FEATURES.md](FEATURES.md).

---

## ✅ COMPLETED

- [x] **CQRS migration** — Full event-sourced architecture via go-cqrs-lite. Legacy CRUD deleted.
- [x] **Deterministic aggregate IDs** — SHA256→hex from (source, sourceID) for idempotency.
- [x] **Conflict-aware sync** — `DecideSync` detects conflicts and emits `ItemConflictFound` events.
- [x] **Provider architecture** — Generic `Provider` interface; reference GitHub provider lives in the consumer app [`github-local-sync`](https://github.com/larsartmann/github-local-sync).
- [x] **Branded type IDs** — 6 phantom types for compile-time safety.
- [x] **SQLite backend** — SQLite event store + read model + snapshots. Pure-Go via `modernc.org/sqlite`. Catch-up projection now runs via `projectionhost.Host` (ADR-0006) with checkpoint persistence + DLQ.
- [x] **cockroachdb/errors removal** — Replaced with `go-error-family` constructors.
- [x] **Zero lint issues** — golangci-lint v2 with `enable-all`, 0 issues.
- [x] **HTTP API** — Huma v2 with 4 endpoints (`GET /items`, `GET /stats`, `POST /sync`, `GET /health`). OpenAPI 3 spec auto-generated.
- [x] **Error templates** — `RegisterErrorTemplates()` for all error codes with What/Why/Fix/WayOut.
- [x] **Nix flake** — `flake.nix` with devShell + `buildGoModule` (vendored private deps).
- [x] **CRDT conflict resolution** — `crdt.ConflictResolver[T]` wired into `DecideSync`. `LWWResolver` default. `ActionConflictLocal` support.
- [x] **Pluggable conflict strategy** — `CQRSConfig.ConflictResolver` accepts any resolver. Default nil = remote-wins.
- [x] **Pure contract library** — Removed all provider implementations and the example CLI. The SDK now defines the contract only; consumers implement providers. Reference consumer: [`github-local-sync`](https://github.com/larsartmann/github-local-sync).
- [x] **go-cqrs-lite v3 migration** — Migrated all modules to v3.0.0 paths. Adopted `watermill/v3` `EventBus` (replaces deleted `memory.NewMemoryBus`), `middleware.EventLogging` (replaces hand-rolled logging adapter), and `uint64` `event.Version`.
- [x] **projection.Runner removal** — go-cqrs-lite v3 dropped `projection/`. Replaced with direct `bus.SubscribeAll` (synchronous live delivery) + background `runner.replayJournal` (SQLite catch-up). Idempotent projection tolerates replay/live overlap, so no checkpoint store is needed.
- [x] **Exported ConflictWinner** — `ConflictWinnerRemote`/`ConflictWinnerLocal` constants + `ParseConflictWinner` for safe payload→enum decoding.
- [x] **DTO/domain boundary** — `provider.Item` (DTO) ↔ `model.Item` (domain entity) via `item_adapter.go`. Decider, read model, events, and resolver all use `*model.Item`.
- [x] **go-cqrs-lite v4 migration + JSON v2** — all modules moved to v4 paths; adopted `encoding/json/v2` (gated behind `GOEXPERIMENT=jsonv2` in Go 1.26). `watermill/v4` `EventBus`, `projection/v4` interface, `projectionhost/v4` managed host.
- [x] **Tombstone soft-delete (ADR-0005)** — tombstones replace hard-deletes; hidden items keep full history; re-syncing resurrects via projection upsert; opt-in upstream reconciliation (`ReasonUpstreamGone`).
- [x] **projectionhost adoption (ADR-0006)** — managed batch-drainer for resilient SQLite catch-up: checkpoint persistence, crash auto-restart with backoff, dead-letter queue for poison messages. Replaces the prior bare `replayJournal`.
- [x] **De-githubify domain model (ADR-0007)** — `ActorLogin`/`RepoName`/`RepoURL`/`ActorAvatarURL` removed from `provider.Item`/`model.Item`; provider-specific content now flows through `Attributes map[string]string`. `hasChanged` is ContentHash-first (provider-agnostic).
- [x] **De-githubify + scope re-affirmation (ADR-0007)** — `provider.Item`/`model.Item` stripped of GitHub-specific fields; SDK re-centred as a single-aggregate, pull-only sync engine. (The broader `Host` framework pivot proposed in ADR-0008 was **not** executed — the project stayed within ADR-0004 scope; ADR-0008 remains Proposed/dormant.)
- [x] **cqrs-lint static architectural linter** — `internal/cqrslint` enforces 10 invariants (C0001–C0010) for `pkg/cqrs` (ADR-0004 scope guard). CLI: `cmd/cqrs-lint`. Zero third-party deps.
- [x] **Error-handling overhaul** — `go-error-family` constructors with intrinsic classification, central `pkgerrors.HTTPStatus` mapping, `WithCtx`/`InvalidField` structured context, partial-sync surfacing (Transient `ErrPartialSync` → HTTP 200-with-result).
- [x] **190 tests passing** — 9 packages, 0 lint issues. _(Now 214 tests across 10 packages — see FEATURES.md.)_

---

## 🟢 FUTURE PHASES

### Enhanced Features

- [ ] **Build TUI with Bubble Tea**
      Interactive terminal UI for browsing events, filtering, and real-time sync. Lives in a consumer app, not the SDK.

- [ ] **Support multiple user sync**
      Accept multiple sources in one sync run. Requires read model schema to track which user each event belongs to.

- [ ] **Daemon/background mode**
      Run as cron job or systemd service for periodic sync. (Consumer-app concern.)

### Data & Export

- [ ] **Add export to JSON/CSV**
      Export stored events to file formats.

- [ ] **Conflict resolution per-sync override** — `SyncOptions.ConflictResolver` for per-sync strategy.

- [ ] **Real-time sync protocol** — live multi-node sync. The former `SyncRequest`/`SyncResponse` types were removed when the CRDT machinery was deleted; this would need to be built from scratch and is out of scope per [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md).

---

## 🔧 TECHNICAL DEBT

### CI/CD

- [x] **Rework CI build & release jobs for a pure library**
      The `build` job cross-compiles the deleted `./cmd/examples/github-sync` path (now in `github-local-sync`), and `release` depends on it — both fail. A pure library has no binary to build; rework to a library-appropriate release flow (or remove these jobs). **High priority.** — **Done (2026-06-29):** `build` now verifies cross-platform library compilation; `release` creates a binary-free GitHub release.

### Code Quality

- [ ] **OpenTelemetry instrumentation** — spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, HTTP middleware.

- [ ] **API authentication middleware** — API key or JWT; the HTTP API is currently unauthenticated.

- [ ] **API pagination headers** — `X-Total-Count`, cursor-based.

- [ ] **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution (the `schema.Version` foundation is already in place).

---

## ❓ OPEN QUESTIONS

1. **Multi-user sync** — Should the read model track which user each event belongs to?
2. **Event retention/TTL** — Automatic cleanup of old events? Configurable?
3. **Conflict resolution policy** — Configurable via `CQRSConfig.ConflictResolver`. A per-sync override (`SyncOptions.ConflictResolver`) is pending.
4. **Library release flow** — What does a tagged release of a pure Go module look like without a binary? (Governs the CI rework above.)
5. **Multi-aggregate generalisation** — Should go-localsync generalise beyond a single `sync_item` aggregate into a multi-aggregate event-sourcing framework? **Decided: deferred.** See [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). Revisit only if a third+ consumer needs it or `go-cqrs-lite` can't evolve the ergonomics. `go-cqrs-lite v4` remains the cross-project sharing boundary.

---

## 📑 RECORDED DECISIONS

| ADR                                                                  | Decision                                                                                                                                                                                                                                                                      | Status   |
| -------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------- |
| [ADR-0001](docs/adr/0001-cqrs-adoption.md)                           | Adopt event-sourced CQRS via go-cqrs-lite (no legacy CRUD)                                                                                                                                                                                                                    | Accepted |
| [ADR-0002](docs/adr/0002-branded-ids.md)                             | Branded phantom-type IDs for compile-time safety                                                                                                                                                                                                                              | Accepted |
| [ADR-0003](docs/adr/0003-crdt-integration.md)                        | Pluggable CRDT conflict resolution (`ConflictResolver[T]`)                                                                                                                                                                                                                    | Accepted |
| [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) | Defer multi-aggregate generalisation; go-cqrs-lite stays the sharing boundary                                                                                                                                                                                                 | Accepted |
| [ADR-0005](docs/adr/0005-tombstone-over-delete.md)                   | Tombstone-based soft-delete with upstream reconciliation                                                                                                                                                                                                                      | Accepted |
| [ADR-0006](docs/adr/0006-projectionhost-adoption.md)                 | Adopt `projectionhost.Host` for resilient managed catch-up projection                                                                                                                                                                                                         | Accepted |
| [ADR-0007](docs/adr/0007-de-githubify-domain-model.md)               | Provider-agnostic domain model (`Attributes` map; ContentHash-first diff)                                                                                                                                                                                                     | Accepted |
| [ADR-0008](docs/adr/0008-pivot-to-sync-toolkit.md)                   | **Proposed — dormant.** Pivot to a `Host` sync-application-framework (drop `pkg/cqrs`/`pkg/data`/`pkg/api`). Never executed; project continued in the ADR-0004 single-aggregate direction (tombstones, projectionhost, de-githubify, v4, cqrs-lint all shipped within scope). | Proposed |
