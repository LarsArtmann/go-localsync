# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Release dates and entries are reconciled against the actual git tags (`v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`).

## [Unreleased]

## [0.3.0] - 2026-06-23

### Changed

- **Breaking: `ActorID` renamed to `ActorLogin`** (`pkg/id`). The type previously called `ActorID` actually represents an external provider actor login (e.g. a GitHub username like `"octocat"`), and every field using it was already named `ActorLogin` — the type name didn't match its purpose. The rename also resolves a P0 seam violation where three incompatible types across sibling repos were all named `ActorID`. Affected public API:
  - `ActorBrand` → `ActorLoginBrand` (phantom type)
  - `ActorID` → `ActorLogin` (type alias)
  - `NewActorID(v string)` → `NewActorLogin(v string)` (constructor)
  - Field types updated across `pkg/data/model` (`Item.ActorLogin`, `ItemFilter.ActorLogin`, `WithActorLogin`), `pkg/provider` (`Item.ActorLogin`), `pkg/api`, and `pkg/cqrs`. Consumers referencing `id.ActorID` or `id.NewActorID` must update to `id.ActorLogin` / `id.NewActorLogin`.
- **go-error-family upgraded from v0.4.0 to v0.5.0.** Vendored dependencies re-synced. No code changes needed — `RegisterTemplate`, `NewRejection`, `Wrap`, and `IsRetryable` APIs are backward-compatible (global functions now delegate to `DefaultRegistry`).

## [0.2.0] - 2026-06-22

### Added

- **go-cqrs-lite v3 migration** — all modules moved to v3.0.0 paths (`event/v3`, `command/v3`, `query/v3`, `decider/v3`, `id/v3`, `codec/v3`, `snapshot/v3`, `storage/v3`, `storage/memory/v3`, `middleware/v3`, `watermill/v3`).
- **`watermill/v3` EventBus** — replaces the deleted `memory.NewMemoryBus` for in-process event delivery. `BlockPublishUntilSubscriberAck` preserves read-your-writes on synchronous projection.
- **Exported `ConflictWinner` constants** — `ConflictWinnerRemote` / `ConflictWinnerLocal` plus `ParseConflictWinner` for safe payload→enum decoding (unknown values default to remote-wins).
- **`runner.go` projection wiring** — direct `bus.SubscribeAll` for synchronous live event delivery, plus a background `replayJournal` (reads all persisted events via `Journal.ReadAll`) for SQLite catch-up. The idempotent projection tolerates replay/live overlap, so no checkpoint store is needed.
- **DTO/domain boundary** — `item_adapter.go` converts `provider.Item` (DTO) → `model.Item` (domain entity with `SchemaVersion`). Decider, read model, events, and conflict resolver now all operate on `*model.Item`.
- **`pkg/data/schema`** — `Version` (V1/V2) with `CurrentVersion()`, `Valid()`, carried on every item for forward event migration (upcasting).

### Changed

- **Logging middleware** — replaced the hand-rolled logging adapter with `middleware.EventLogging` from go-cqrs-lite v3.
- **`event.Version`** — migrated from `int` to `uint64` (`Increment()`, `Add()`); no `int()` casts needed.
- **`uint64` conflict winner** — winner decoded via `ParseConflictWinner` rather than raw string compares.
- **go directive** bumped to `1.26.4`.
- **Dependencies** — `go-branded-id` v0.3.1, `go-error-family` v0.4.0, `modernc.org/sqlite` v1.52.0, `huma/v2` v2.38.0.

### Removed

- **All provider implementations and the example CLI** — the SDK is now a **pure contract library**. `pkg/providers/`, `cmd/examples/github-sync`, the `go-github` dependency, and `caarlos0/env` were removed. The reference GitHub provider + CLI moved to the consumer app [`github-local-sync`](https://github.com/larsartmann/github-local-sync).
- **Checkpoint stores** — `SQLiteCheckpointStore` / `MemoryCheckpointStore` removed; the v3 projection is idempotent and needs no checkpoint.
- **`projection.Runner`** — go-cqrs-lite v3 dropped `projection/`; replaced by `runner.go` (see Added).
- **Dead config** — `RemoteURL`, `AuthToken`, Push/Pull flags, and the Turso backend removed in favor of local SQLite.

### Performance

- **Rate limit cache** — `RateLimitCache` caches rate-limit info between calls to avoid redundant API requests; concurrency-safe.
- **Concurrent `FetchAll`** — pages fetched in parallel (`MaxConcurrentFetches`, default 3).
- **SQLite optimizations** — WAL mode, aggregate-ID `sync.Map` cache, scan-path query optimization.
- **`CountByType`** — fixes the `GetStats` N+1 query; `TypeCounts` added to the API `StatsOutput`.

### Fixed

- **`flake.nix` release coherence** — bumped package `version` to 0.2.0 (was stuck at 0.1.0), removed stale `mainProgram = "go-localsync"` and the broken `apps.default` (it used `getExe` on a library, so `nix run` failed). The SDK is a pure contract library with no binaries as of this release.

## [0.1.1] - 2026-06-14

### Changed

- Upgraded go-cqrs-lite sub-modules to v2.3.0 (`schema/v2` v2.2.0 → v2.3.0; `event/v2` → v2.3.1).
- Performance sprint: rate limit cache, concurrent `FetchAll`, SQLite WAL mode, aggregate-ID cache, `CountByType`, `TypeCounts` in API stats.
- Vendored private dependencies for offline nix builds (`vendor/` + `vendorHash = null`).

## [0.1.0] - 2026-05-28

### Added

- **CQRS architecture** — full event-sourced storage via go-cqrs-lite (Decider, ReadModel, Projection, Stack). No legacy CRUD path.
- **Deterministic aggregate IDs** — SHA256→hex from (source, sourceID) for idempotency.
- **Conflict-aware sync** — `DecideSync` detects conflicts and emits `ItemConflictFound` events; pluggable `crdt.ConflictResolver[T]` with `LWWResolver` default.
- **CRDT primitives** — `VectorClock`, `Operation[T]`, `Conflict[T]`, `SyncMessage` in `pkg/crdt`.
- **Dual backends** — in-memory and SQLite (`modernc.org/sqlite`, no CGo) with snapshots.
- **Branded IDs** — 6 phantom-type IDs via go-branded-id.
- **HTTP API** — Huma v2 with `GET /items`, `GET /stats`, `POST /sync`, `GET /health`; OpenAPI 3 spec.
- **Error taxonomy** — `go-error-family` constructors with intrinsic classification (Rejection, Transient, Infrastructure).
- **Nix flake** — devShell + `buildGoModule`.
- **GitHub Actions CI** — test, lint, build, release jobs.
