# ROADMAP.md

**Project:** go-localsync
**Last Updated:** 2026-06-22

## Overview

Long-term direction and raw ideas not yet refined into actionable tasks. For short/mid-term work, see [TODO_LIST.md](TODO_LIST.md). For the feature inventory, see [FEATURES.md](FEATURES.md).

---

## ✅ COMPLETED

- [x] **CQRS migration** — Full event-sourced architecture via go-cqrs-lite. Legacy CRUD deleted.
- [x] **Deterministic aggregate IDs** — SHA256→hex from (source, sourceID) for idempotency.
- [x] **Conflict-aware sync** — `DecideSync` detects conflicts and emits `ItemConflictFound` events.
- [x] **Provider architecture** — Generic `Provider` interface; reference GitHub provider lives in the consumer app [`github-local-sync`](https://github.com/larsartmann/github-local-sync).
- [x] **Branded type IDs** — 6 phantom types for compile-time safety.
- [x] **SQLite backend** — SQLite event store + read model + snapshots (no checkpoint store in v3). Pure-Go via `modernc.org/sqlite`.
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
- [x] **225 tests passing** — 9 packages, 0 lint issues.

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

- [ ] **Real-time sync protocol** — `SyncRequest`/`SyncResponse` from `pkg/crdt/` for live multi-node sync.

---

## 🔧 TECHNICAL DEBT

### CI/CD

- [ ] **Rework CI build & release jobs for a pure library**
      The `build` job cross-compiles the deleted `./cmd/examples/github-sync` path (now in `github-local-sync`), and `release` depends on it — both fail. A pure library has no binary to build; rework to a library-appropriate release flow (or remove these jobs). **High priority.**

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
