# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Release dates are reconciled against the actual git tags (`v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.4.1`).

## [Unreleased]

## [0.4.1] - 2026-07-23

A maintenance release: go-cqrs-lite v4.1 dependency bump with full deprecation cleanup, build-system migration from committed `vendor/` to Nix `mkPreparedSource`, and internal refactoring.

### Changed

- **go-cqrs-lite v4.1.0** — all modules bumped from v4.0.x to v4.1.0 (`codec`, `command`, `decider`, `dispatcher`, `event`, `id`, `middleware`, `projection`, `projectionhost`, `snapshot`, `storage`, `storage/memory`, `watermill`). Adopted the upstream `AggregateID`→`StreamID` and `AggregateType`→`StreamType` vocabulary: migrated every deprecated type reference (`cqrsid.AggregateID`→`cqrsid.StreamID`, `ParseAggregateID`→`ParseStreamID`, `NewAggregateID`→`NewStreamID`, `event.AggregateType`→`event.StreamType`). Source-compatible — the old names are retained as type aliases.
- **go-error-family v0.8.0** — bumped from v0.7.0.
- **Build system: `vendor/` → `mkPreparedSource`** — replaced the committed `vendor/` directory with Nix `mkPreparedSource` for hermetic builds. Eliminates the force-add workaround for private deps and shrinks the repository. CI configured with SSH agent for private repo access.
- **cqrslint refactoring** — extracted `Finding` helper constructors applied consistently across all 10 checks; extracted `queryRows` helper; consolidated `AssertInt`→`AssertEqual`; shared per-source lock across sync entry points; simplified `isSelectorType` with `slices.Contains`.
- **cqrs-lint C0001** — updated to recognize `event.StreamType` (the new canonical type name) while still accepting the legacy `event.AggregateType` alias.
- **Dependency refresh** — `golang.org/x/exp` refreshed; nix inputs updated to latest nixpkgs; `huma/v2` bumped to v2.39.0.

### Fixed

- **devShell `GOFLAGS` propagation** — `GOFLAGS=-tags=goexperiment.jsonv2` was not inherited by buildflow's native go subcommands (`test-race`, `go-fix`, `go-auto-upgrade`, `govalid-generate`), causing misleading partial-green results. Now documented and wired consistently.
- **`slices.Contains` migration bug** — masked iterator-semantics regression from the Go 1.26 `slices` package migration.

### Removed

- **Unused `warningAt` helper** — dead code in `internal/cqrslint/finding.go`.

## [0.4.0] - 2026-07-18

A major release: tombstone soft-deletes, resilient managed projection (`projectionhost.Host`), a static architectural linter (`cqrs-lint`), a full error-handling overhaul, de-githubification of the domain model, and the go-cqrs-lite v4 + JSON v2 migration.

### Added

- **Tombstone soft-delete (ADR-0005)** — tombstones replace hard-deletes; tombstoned items keep full history on `Item.Tombstone`; re-syncing a tombstoned item resurrects it via projection upsert; opt-in `SyncOptions.Reconcile` tombstones upstream-gone items (`ReasonUpstreamGone`). New `DecideTombstone` + `TombstoneItemCommand`.
- **`projectionhost.Host` catch-up (ADR-0006)** — managed batch-drainer for resilient SQLite projection: checkpoint persistence, crash auto-restart with backoff, dead-letter queue for poison messages. Replaces the prior bare `replayJournal`. Mutex-guarded version-gate prevents stale replay events from resurrecting tombstoned rows.
- **`cqrs-lint` architectural linter** — `internal/cqrslint` enforces 10 invariants (C0001–C0010) for `pkg/cqrs` (ADR-0004 scope guard). CLI: `cmd/cqrs-lint`. Zero third-party deps (stdlib `go/parser` only).
- **Error-handling overhaul** — `go-error-family` constructors with intrinsic classification (Rejection, Transient, Infrastructure); central `pkgerrors.HTTPStatus` translator (per-sentinel overrides + family defaults; `context.Canceled`→499, `context.DeadlineExceeded`→504); `WithCtx`/`WithCtxf`/`InvalidField` structured context; partial-sync surfacing (Transient `ErrPartialSync` → HTTP 200-with-result rather than discarding synced items).
- **Schema V3 (de-githubify)** — `pkg/data/schema` `Version` extended to V3 (removing GitHub-specific fields from the model). Carried on every `model.Item` for forward event migration (upcasting). V1/V2 were introduced in v0.2.0.
- **govulncheck + gitleaks** in CI — reachability-based dependency CVE scanning and full-history secret scanning (vendor/ excluded).

### Changed

- **Breaking: provider-agnostic domain model (ADR-0007)** — removed `ActorLogin`, `ActorAvatarURL`, `RepoName`, `RepoURL` from `provider.Item` and `model.Item`. Provider-specific content flows through `Attributes map[string]string`. `hasChanged` is ContentHash-first (with `UpdatedAt`/`Type` fallbacks). SQLite `actor_login` index dropped; indexes now on `type`, `created_at`, `(type, created_at)`. `GET /items` no longer accepts `actor`/`repo` query params.
- **Breaking: go-cqrs-lite v4 migration** — all modules moved to v4 paths (`event/v4`, `command/v4`, `decider/v4`, `id/v4`, `codec/v4`, `projection/v4`, `projectionhost/v4`, `snapshot/v4`, `storage/v4`, `storage/memory/v4`, `middleware/v4`, `watermill/v4`). Adopted `encoding/json/v2` (gated behind `GOEXPERIMENT=jsonv2` build tag in Go 1.26).
- **Strategic pivot (ADR-0008)** — re-centred the SDK as a single-aggregate, pull-only sync toolkit; explicit scope boundary against multi-aggregate generalisation (ADR-0004). The broader `Host` framework pivot proposed in ADR-0008 was **not** executed — the project stayed within ADR-0004 scope.
- **Per-source serialization** split into lock-free `runSync`/`runSyncIncremental` to avoid re-entrant deadlock when incremental falls back to full.
- **CI hardening** — `cancel-in-progress`, `paths-ignore`, per-job `timeout-minutes`; cross-platform build matrix verifies library compilation (linux/darwin × amd64/arm64).
- **Dependencies** — `go-error-family` v0.7.0, `huma/v2` v2.39.0, `go-branded-id` v0.3.2, `modernc.org/sqlite` v1.54.0, `charm.land/log/v2` v2.0.0.
- `flake.nix` now derives `version` from git revision; `CONTRIBUTING.md` streamlined.

### Removed

- **CRDT distributed-sync types** — `VectorClock`, `Operation[T]`, `SyncMessage`, and the multi-writer protocol types deleted. A single-writer pull mirror has no second writer and no causal ordering to track (see ADR-0004).
- **QueryDispatcher** — removed by design. Reads call the `ReadModel` directly (see note in `stack_adapters.go`); the command side stays dispatched for logging/retry/validation middleware.

### Fixed

- **Projection version-gate TOCTOU race** — concurrent live + replay delivery for the same aggregate could let a stale event resurrect a tombstoned row. Now mutex-guarded so deliveries serialize per aggregate.
- **`NewCQRSStack` resource leak** — error paths after store creation could leak store/bus/db/goroutine resources. Now uses named returns + cleanup defer.
- **Aggregate-ID collision and `hasChanged` data loss** — content-hash comparison was silently dropping items; aggregate-ID parsing was swallowing errors and caching zero values.
- **Partial-sync error dropping** — `ConflictAwareSyncer` silently dropped item-level errors when some items failed validation/persistence but the run completed. Now surfaces `ErrPartialSync` (Transient).
- **SQLite read model dropping columns** — `ContentHash` and `SchemaVersion` were silently dropped on upsert. SQLite error chains now preserved via multi-`%w` wrapping so `errors.Is` and `errors.As` both work.
- **`Since` filter boundary** — SQLite exclusive `>` corrected to inclusive `>=` to match the memory read model.

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
