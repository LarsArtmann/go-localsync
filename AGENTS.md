# Go-LocalSync Agent Configuration

## Project Overview

Go-LocalSync is a single-writer pull-mirror SDK with a pluggable provider-based architecture. It uses event-sourced CQRS via go-cqrs-lite for state management, pluggable conflict resolution (`pkg/crdt/`), tombstone-based soft-deletes with upstream reconciliation, and branded IDs from go-branded-id for compile-time type safety. There is no multi-writer/distributed CRDT machinery — the provider is the sole writer per aggregate.

> **Scope boundary (ADR-0004):** The SDK is deliberately a **single-aggregate, pull-only, flat-Item sync engine** — one `sync_item` aggregate, three fixed events, one projection. The domain model is GitHub-activity-feed-shaped (`ActorLogin`, `RepoName`, `RepoURL`). Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [`docs/adr/0004-multi-aggregate-generalisation-deferred.md`](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [`docs/feedback/`](docs/feedback/) adoption reviews. `go-cqrs-lite v3` is the cross-project sharing boundary. Do not widen the scope (multi-aggregate, push ingestion, consumer-defined events) without revisiting that ADR.

## Architecture

| Package              | Purpose                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/crdt/`          | Conflict resolution: `Conflict[T]`, `ConflictResolver[T]`, `LWWResolver[T]` (timestamp-only) — **wired into DecideSync as pluggable conflict strategy**. No vector clocks/operations (a single-writer pull mirror needs none).                                                                                                                                                                                                              |
| `pkg/api/`           | HTTP API server with Huma v2 + stdlib (`GET /items`, `GET /stats`, `POST /sync`, `GET /health`), split into server.go + dto.go + handlers.go                                                                                                                                                                                                                                                                                                |
| `pkg/cqrs/`          | CQRS integration layer using go-cqrs-lite **v3.4** (Decider, ReadModel, Projector, CQRSStack, TypedHandler), split into focused files (middleware.go, commands.go, sqlite\_\*.go)                                                                                                                                                                                                                                                           |
| `pkg/provider/`      | Core interfaces (`Provider`, `Item`, `FetchResult`, `RetryConfig`) and `RateLimitInfo`. The SDK defines the contract only — concrete providers (e.g. GitHub) live in consumer apps.                                                                                                                                                                                                                                                         |
| `pkg/sync/`          | `Syncer`, `ConflictAwareSyncer`, `SyncStore` interface (decoupled from `*cqrs.CQRSStack`), `SyncAction`, `ItemSyncResult`, `SyncSummary`, retry/backoff (`retry.go`), per-source mutex, opt-in reconciliation                                                                                                                                                                                                                               |
| `pkg/data/`          | Domain model: `model.Item` (persisted entity with `SchemaVersion` + optional `Tombstone`), `model.Key`, `model.ItemFilter` (`IncludeTombstoned`), `model.Tombstone`/`TombstoneReason`; `schema.Version` (V1/V2 versioning for event upcasting). Decider, read model, events, and conflict resolution all operate on `*model.Item`.                                                                                                          |
| `pkg/id/`            | Branded phantom-type IDs (`ItemID` ULID, `ExternalID` string, `ProviderID`, `EventTypeID`, `ActorLogin`, `RepoID`)                                                                                                                                                                                                                                                                                                                          |
| `pkg/errors/`        | Structured errors via `go-error-family` constructors (Rejection, Transient, Infrastructure) with intrinsic classification, `IsRetryable`                                                                                                                                                                                                                                                                                                    |
| `internal/cqrslint/` | Static architectural-invariant linter for `pkg/cqrs` (ADR-0004 enforcer). 10 AST checks (C0001-C0010): single aggregate type, three fixed events, fold coverage, projector subscriptions, provider-agnostic `hasChanged`, no query dispatcher, `SyncAction` stays in `pkg/sync`, projection mutex guard, payload json tags, `NewEvents` uses `aggregateType` const. CLI: `cmd/cqrs-lint/`. Zero third-party deps (stdlib `go/parser` only). |

### SyncStore Interface Seam

`pkg/sync/` defines `SyncStore` — a minimal interface decoupling sync logic from CQRS infrastructure. `*cqrs.CQRSStack` implements it via adapter methods. Dependency flows one way: `cqrs → sync → provider/types/errors`. No import cycles.

`SyncAction` constants and `ItemSyncResult` live in `pkg/sync/` — the architectural seam — not in `pkg/cqrs/`.

## CQRS Architecture

The entire storage layer is CQRS-based via go-cqrs-lite. There is **no legacy CRUD path**.

### Core Components

- `aggregate_id.go` — deterministic SHA256→hex from (source, sourceID) with sync.Map cache, shared `itemKey` helper
- `decider.go` — `SyncItemState{Item *model.Item}` (tombstone lives on `Item.Tombstone`; no separate Deleted flag), pure Apply (the event applier) + DecideSync/DecideTombstone, `IsTombstoned()`/`ShouldResurrect()`, `HasChanged` checks ContentHash/UpdatedAt/Type only (provider-agnostic since ADR-0007)
- `events.go` — 3 event types: `ItemSynced`, `ItemConflictFound`, `ItemTombstoned` (a sync event always means "live" → resurrects a tombstoned item automatically via projection upsert)
- `readmodel.go` — `ReadModel` interface (embeds `model.ItemReader`) + `model.ItemFilter`, stores `*model.Item` directly
- `memory_readmodel.go` — concurrent-safe in-memory read model with filter/pagination
- `sqlite_readmodel.go` — SQLite-backed read model with DDL, filter/pagination
- `projection.go` — `Projector` implements `projection.Projection` (moved from `event.Projection` in go-cqrs-lite v3.2), wired via direct bus subscription (live) + manual journal replay (persistence)
- `stack.go` — `CQRSStack` with Store+Bus+Repo+ReadModel+CommandDispatcher, SQL snapshots, event logging middleware, correlation IDs. `NewCQRSStack` uses named returns + a cleanup defer so any error path after store creation releases store/bus/db/goroutine resources. Public commands: `SyncItem`, `TombstoneItem`; `Reconcile(ctx, source, seenKeys)` tombstones upstream-gone items. (No query dispatcher — reads call the ReadModel directly; see ADR note in `stack_adapters.go`.)
- `runner.go` — Projection wiring: direct `bus.SubscribeAll` for synchronous live event delivery, plus `projectionhost.Host` (managed batch-drainer with checkpoint persistence, crash auto-restart, and dead-letter queue) for resilient SQLite catch-up. See ADR-0006.
- `commands.go` + `middleware.go` — typed `SyncItemCommand`/`TombstoneItemCommand` (carries `model.TombstoneReason`) via `command.Dispatcher`. (The read side has no dispatcher — `stack_adapters.go` calls the ReadModel directly; the command side stays dispatched for logging/retry/validation middleware.)

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic aggregate IDs**: SHA256→hex from (source, sourceID)
- **Tombstone + resurrect**: a tombstone is a soft-delete (keeps history on `Item.Tombstone`); re-syncing a tombstoned item overwrites the tombstone → it becomes live again (projection upsert resets the tombstone columns)
- **Reconciliation**: opt-in `SyncOptions.Reconcile` tombstones items for `Source` absent from a complete fetch with `ReasonUpstreamGone` (best-effort; only safe after a COMPLETE fetch)
- **Projection**: Live events delivered synchronously via `bus.SubscribeAll` (watermill `EventBus` with `BlockPublishUntilSubscriberAck` preserves read-your-writes). SQLite catch-up runs via `projectionhost.Host` (ADR-0006): a managed batch-drainer that reads from the last checkpoint (bounded replay), auto-restarts on crash with backoff, and captures poison messages to a dead-letter queue. The version-gate (skip events with version ≤ last applied per aggregate) is **mutex-guarded** so concurrent live+replay delivery for the same aggregate serializes — preventing stale events from resurrecting tombstoned rows. The checkpoint bounds work and survives failure.
- **SQL persistence**: SQLite backend persists snapshots (`SQLSnapshotStore`) via `snapshot/v3` and `storage/v3` modules.
- **Correlation IDs**: `SyncItems` generates a unique `CorrelationID` per sync run, passed via `event.WithCorrelationID` to all events.
- **Command dispatch**: `SyncItem`/`TombstoneItem` dispatched through `command.Dispatcher` with typed commands. Enables logging, retry, validation middleware.
- **Resilient fetch**: `fetchItems` retries transient errors with exponential backoff + ±25% jitter, consults `errors.IsRetryable`, and honors an optional `retryAfterer` (Retry-After) hook. Permanent errors surface immediately. Unclassified provider errors classify as Transient (fail-open) so they are retried — providers that know an error is permanent should return an errorfamily-classified (Rejection) error.
- **Partial sync**: when some items fail validation/persistence but the run completes, the sync returns a populated result **and** `pkgerrors.ErrPartialSync` (Transient); consumers detect it via `errors.Is`. The HTTP layer maps it to 200-with-result rather than discarding successfully-synced items.
- **Per-source serialization**: a per-source mutex orders concurrent syncs of the same source (TOCTOU guard on the latest-timestamp read); different sources run in parallel. Internals are split into lock-free `runSync`/`runSyncIncremental` to avoid re-entrant deadlock when incremental falls back to full.
- **Remote wins (default)**: on conflict with no resolver configured, the incoming item always overwrites (remote-wins LWW)
- **Pluggable conflict resolution**: `CQRSConfig.ConflictResolver` accepts any `crdt.ConflictResolver[*model.Item]` — `LWWResolver`, custom merge, etc.
- **Validated at boundary**: both `provider.Item.Validate()` and `model.Item.Validate()` require the same fields (including `UpdatedAt`) and route through `pkgerrors.InvalidField(field, reason)` (→ `ErrInvalidInput`) so classification is consistent **and** the offending field is attached as structured context (`ErrorContext()["field"]`) for programmatic handling.
- **Error-chain-preserving SQLite**: all read-model errors use multi-`%w` wrapping (`wrapDBErr`) so `errors.Is(err, ErrDatabase)` AND `errors.As(err, &driverErr)` both work.

- **Centralized HTTP mapping**: `pkgerrors.HTTPStatus(err)` is the single error→status translator (per-sentinel overrides where the family default is too coarse, then `errorfamily.Classify(err).HTTPStatus()`). `context.Canceled`→499, `context.DeadlineExceeded`→504. The API layer routes every handler through it; new sentinels inherit their family's status automatically — no brittle catch-all default.

### Conflict Flow

`ConflictAwareSyncer` delegates entirely to `SyncStore.SyncItems()` which uses `DecideSync` as the single authority. `DecideSync` calls `HasChanged()` and:

1. If no resolver configured (nil): emits `ItemConflictFound{Winner: ConflictWinnerRemote}` + `ItemSynced` with the incoming item (default remote-wins)
2. If resolver configured: calls `resolver.Resolve(&Conflict{Local, Remote, ...})` and uses the winner for `ItemSynced`. `ItemConflictFound{Winner}` records which side won (`ConflictWinnerRemote` or `ConflictWinnerLocal`)
3. On resolver error: falls back to remote-wins

The winner constants (`ConflictWinnerRemote`, `ConflictWinnerLocal`) are exported with `ParseConflictWinner` for safe payload→enum decoding (unknown values default to remote-wins). The conflict winner determines the `SyncAction`: `ActionConflictRemote` or `ActionConflictLocal`. No split-brain — the decider is the single source of truth for conflict detection. Invalid items from `filterValidItems` are properly counted in `ConflictResult.Errors`.

## Development Workflow

### Local Development

1. **Optional**: Create `go.work` in project root ONLY for live-editing local sibling checkouts. It is in `.gitignore` and **must never be committed or left on disk during `buildflow`** — buildflow detects go.work on disk and expands `go test ./...` to ALL workspace modules (including `../go-cqrs-lite/*`), causing sibling test failures. With the committed `vendor/` directory, builds and tests work offline without go.work (`go build ./...`, `go test ./...` use vendor mode automatically). Remove go.work before running buildflow:

   ```
   go 1.26.4

   use .

   use (
       ../go-branded-id
       ../go-cqrs-lite/codec
       ../go-cqrs-lite/command
       ../go-cqrs-lite/decider
       ../go-cqrs-lite/dispatcher
       ../go-cqrs-lite/event
       ../go-cqrs-lite/id
       ../go-cqrs-lite/kv
       ../go-cqrs-lite/listing
       ../go-cqrs-lite/middleware
       ../go-cqrs-lite/otel
       ../go-cqrs-lite/query
       ../go-cqrs-lite/schema
       ../go-cqrs-lite/snapshot
       ../go-cqrs-lite/storage
       ../go-cqrs-lite/storage/memory
       ../go-cqrs-lite/watermill
       ../go-error-family
   )
   ```

2. Build: `go build ./...`
3. Test: `go test ./... -count=1`
4. Lint: `golangci-lint run ./... --timeout=5m`
5. Format: `golangci-lint fmt ./...`
6. CQRS gate: `go run ./cmd/cqrs-lint -pkg pkg/cqrs` (or `nix run .#cqrs-lint`)
7. Full pipeline: `buildflow --build-mode full` (ensure go.work is removed first)

### CI (No go.work)

CI uses tagged versions from GitHub (no replace directives in `go.mod`). The `security` job runs **govulncheck** (dependency CVEs, reachability-based) and **gitleaks** (full-history secret scan, vendor/ excluded via `.gitleaks.toml`); **gosec** runs as part of the golangci-lint job. The build/release jobs gate on the security job.

```bash
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go build ./...
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go test ./... -count=1
```

### Pre-commit Hooks

Pre-commit hooks use `buildflow` (not testify-banning). Hooks are not set as executable and are skipped.

### Build & Lint Gotchas

- **Private dependency**: `go-cqrs-lite` is a **private** GitHub repo (siblings `go-branded-id` and `go-error-family` are public). The nix sandbox cannot fetch it, so `nix build` / `nix flake check` require **vendored deps**: a committed `vendor/` dir + `vendorHash = null` in `flake.nix` + a `vendor/**` exclusion in `treefmt`. Regenerate with `GOWORK=off go mod vendor`. Cleanest long-term fix: make `go-cqrs-lite` public, then switch to a real `vendorHash` and drop `vendor/`.
- **`go mod tidy` fails on nested eventtest module**: since `decider/v3` v3.3.0, its test imports pull in `event/v3/eventtest` (a nested Go module inside the event/ directory). The Go toolchain cannot resolve this nested module path via VCS because the tag `event/v3/eventtest/v3.3.0` exists but the module path lookup fails. **Workaround**: run `GOWORK=off go mod vendor` directly (it succeeds) instead of `go mod tidy`. The vendor directory is the source of truth for builds; `go mod tidy` is only needed when adding/removing top-level dependencies.
- **Pre-commit hook OOM on vendor dir**: the buildflow pre-commit hook runs gofumpt/goimports across the entire tree (including vendor/). With the large modernc.org/sqlite vendored sources (~400 generated .go files), these tools get OOM-killed within the 2-minute max timeout. **Workaround**: commit with `--no-verify` after manually verifying formatting on `pkg/` sources (`gofumpt -l pkg/ && goimports -l pkg/`). The hook budget should be increased or vendor/ should be excluded from the formatter steps.
- **go.work breaks buildflow**: buildflow's `ForEachGoModule` detects go.work **on disk** (not just tracked) and runs `go test ./...` in every workspace module — including sibling repos (`../go-cqrs-lite/*`) whose tests fail. Always **delete go.work before `buildflow`**; with `vendor/` committed, builds/tests work without it.
- **golangci-lint v2.12 `exhaustruct`**: the `settings.exhaustruct.exclude` list does **not** match local-package types in full runs (only stdlib full-path patterns work). For local domain structs with optional fields (`ItemFilter`, `FetchResult`), suppress via `issues.exclusions.rules` with a `text:` regex instead.
- **`SA5012` disabled**: staticcheck v0.7 panics ("can't set facts on objects belonging another package") on cross-package even-elements analysis (e.g. `testutil.BuildPairs` called from another package's tests). Disabled in `linters.settings.staticcheck.checks`.
- **nixpkgs Go lag blocks `nix flake check`**: `go.mod` requires `go 1.26.4` (deliberate — matches the active toolchain & silences the go.work warning; commit `a819eb6`), but nixpkgs unstable only packages `go_1_26` at 1.26.3 and `go_1_27` doesn't exist yet. The sandbox forces `GOTOOLCHAIN=local`, so `nix build` / `nix flake check` fail with `go.mod requires go >= 1.26.4 (running go 1.26.3)`. The native gate (`go build` / `go test` / `golangci-lint`) passes on the real 1.26.4 toolchain. This self-resolves once nixpkgs bumps `go_1_26` to 1.26.4; do **not** lower the directive to work around it (that reverts a deliberate bump). Check nixpkgs readiness with `nix eval --impure --expr 'let pkgs = import (builtins.getFlake "github:NixOS/nixpkgs/nixos-unstable") {}; in pkgs.go_1_26.version'`.

## Testing

| Package             | Tests | Coverage | Status                                                                                                         |
| ------------------- | ----- | -------- | -------------------------------------------------------------------------------------------------------------- |
| `pkg/cqrs`          | 88    | 82.0%    | ✅ Decider, ReadModel, Projection, Stack, SQLite RM, Replay, Correlation, tombstone, regression tests          |
| `pkg/sync`          | 32    | 84.5%    | ✅ Syncer + ConflictAwareSyncer + retry + reconcile + per-source lock + regression                             |
| `pkg/id`            | 12    | 100.0%   | ✅ ID construction, roundtrip, zero, equal                                                                     |
| `pkg/errors`        | 16    | 100.0%   | ✅ Sentinels, wrapping, classification, IsRetryable, HTTPStatus, WithCtx/InvalidField, templates, partial-sync |
| `pkg/provider`      | 2     | 90.9%    | ✅ Item validation                                                                                             |
| `pkg/api`           | 15    | 94.0%    | ✅ Server, routes, handlers, health/stats/items/sync endpoints, error mapping, partial-sync→200                |
| `pkg/crdt`          | 7     | 100.0%   | ✅ Conflict, ConflictResolver, LWWResolver, example test                                                       |
| `pkg/data/model`    | 10    | 80.5%    | ✅ Item, Key, Validate, ItemFilter, Tombstone                                                                  |
| `pkg/data/schema`   | 4     | 100.0%   | ✅ Schema Version (V1/V2), CurrentVersion, Valid                                                               |
| `internal/cqrslint` | 23    | —        | ✅ 10 architectural checks (C0001-C0010), loader, finding sort/format, rules catalog                           |

**217 total test functions** across 10 test packages.

Run: `go test ./... -count=1`

## Backend Selection

Storage backends are selected via `CQRSConfig.Backend` in `cqrs.NewCQRSStack()`.

| Backend  | Config value        | Use Case                                 |
| -------- | ------------------- | ---------------------------------------- |
| `memory` | `Backend: "memory"` | Testing, development (default)           |
| `sqlite` | `Backend: "sqlite"` | Local SQLite file via modernc.org/sqlite |

Event store + read model use the same backend.

## Provider Development

The SDK is a pure contract library — concrete providers live in consumer apps. To add a new provider:

1. Implement the `provider.Provider` interface (`Name`, `Fetch`, `FetchAll`, `GetRateLimit`)
2. Convert provider-specific data to `provider.Item` using branded types from `pkg/id/` (for identity) and `Attributes map[string]string` (for provider-specific content like actor, repo, etc.)
3. Add provider-specific tests
4. Update documentation with provider configuration

## Database Schema

Two tables managed by the CQRS stack:

### Events (via go-cqrs-lite/storage)

- `id`, `event_type`, `aggregate_type`, `aggregate_id`, `version`, `schema_version`
- `payload`, `metadata`, `occurred_at`, `created_at`
- Unique constraint on `(aggregate_type, aggregate_id, version)`

### Sync Items (read model projection)

- `item_id`, `source`, `source_id`, `type`, `attributes` (JSON map of provider-specific key-values)
- `content_hash`, `created_at`, `updated_at`, `schema_version`
- `tombstoned`, `tombstone_reason`, `tombstoned_at` (soft-delete columns; `migrateSyncItems` adds them idempotently)
- Primary key on `(source, source_id)`
- Legacy columns (`actor_login`, `actor_avatar_url`, `repo_name`, `repo_url`) exist in pre-V3 databases but are no longer read or written (ADR-0007)

## Dependencies

| Dependency                         | Version | Purpose                                                                                                         |
| ---------------------------------- | ------- | --------------------------------------------------------------------------------------------------------------- |
| `go-cqrs-lite/event/v3`            | v3.4.0  | Event types, Store, Bus, Journal, `Version` (uint64)                                                            |
| `go-cqrs-lite/command/v3`          | v3.4.0  | Command types, Dispatcher, TypedHandler[T], RegisterTyped[T], `ID()`                                            |
| `go-cqrs-lite/query/v3`            | v3.4.0  | Unused since QueryDispatcher removal; stays in go.mod until `go mod tidy` is unblocked (vendor mode unaffected) |
| `go-cqrs-lite/decider/v3`          | v3.3.0  | Decider (`Apply` field), Repository, snapshot/codec options                                                     |
| `go-cqrs-lite/id/v3`               | v3.3.0  | Branded phantom-type IDs (AggregateID, CorrelationID, etc.)                                                     |
| `go-cqrs-lite/codec/v3`            | v3.3.0  | Codec interface, JSONCodec                                                                                      |
| `go-cqrs-lite/projection/v3`       | v3.4.0  | Projection interface (moved from `event/` in v3.2, ADR-0037)                                                    |
| `go-cqrs-lite/projectionhost/v3`   | v3.4.0  | Managed projection host: checkpoint, crash-restart, DLQ (ADR-0006)                                              |
| `go-cqrs-lite/snapshot/v3`         | v3.4.0  | SnapshotStore, EveryNEvents strategy                                                                            |
| `go-cqrs-lite/storage/memory/v3`   | v3.4.0  | In-memory event store + snapshot store (bus deleted in v3)                                                      |
| `go-cqrs-lite/middleware/v3`       | v3.4.0  | EventLogging + CommandRetry middleware                                                                          |
| `go-cqrs-lite/watermill/v3`        | v3.3.0  | In-process `EventBus` (replaces deleted `memory.NewMemoryBus`)                                                  |
| `go-cqrs-lite/storage/v3`          | v3.4.0  | SQLite event store, snapshot, KV store                                                                          |
| `go-branded-id`                    | v0.3.1  | Branded phantom-type IDs for compile-time safety                                                                |
| `go-error-family`                  | v0.5.1  | Structured error classification + user-facing message templates                                                 |
| `modernc.org/sqlite`               | v1.53.0 | Pure-Go SQLite driver (no CGo)                                                                                  |
| `charm.land/log/v2`                | v2.0.0  | Structured logging                                                                                              |
| `github.com/danielgtaylor/huma/v2` | v2.38.0 | HTTP API framework with OpenAPI 3 generation + stdlib adapter                                                   |

### Test Dependencies

| Dependency       | Purpose                                |
| ---------------- | -------------------------------------- |
| `onsi/ginkgo/v2` | Indirect only (via go-cqrs-lite tests) |
| `onsi/gomega`    | Indirect only (via go-cqrs-lite tests) |

### Build System

| File        | Purpose                                            |
| ----------- | -------------------------------------------------- |
| `flake.nix` | Nix flake with Go devShell + buildGoModule package |

## go-cqrs-lite Integration

| Area           | go-localsync                                                                                                                                | go-cqrs-lite                                                                      |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| IDs            | `id.ID[B, V]` via go-branded-id directly                                                                                                    | `id.Of[T]` — same memory layout                                                   |
| Storage        | `CQRSStack` → `decider.Repository[SyncItemState]`                                                                                           | `event.Store` + `event.Bus` via storage/memory + watermill modules                |
| Conflict       | `DecideSync` produces ItemConflictFound events                                                                                              | Error taxonomy with 5 families                                                    |
| Read Model     | `MemoryReadModel` + `SQLiteReadModel` with filter/pagination                                                                                | Projected from events via custom `Projector` implementing `projection.Projection` |
| SyncStore      | `CQRSStack` implements `sync.SyncStore` via adapter methods (`List`, `Count`, `CountByType`)                                                | `sync.SyncStore` interface defined in consumer package                            |
| SyncActions    | `classifyAction` returns `synclib.SyncAction` (`ActionCreated`, etc.)                                                                       | Types defined in `pkg/sync/`, not `pkg/cqrs/`                                     |
| Codec          | `codec.JSONCodec` + `event.DecodePayload[T]` + `event.NewEvents`                                                                            | Eliminates all manual json.Marshal/Unmarshal                                      |
| Projection     | Direct `bus.SubscribeAll` (sync) + `projectionhost.Host` (managed catch-up with checkpoint + DLQ); see ADR-0006                             | `projectionhost/v3` adopted in v3.4; interface from `projection/v3` (ADR-0037)    |
| Snapshots      | `SQLiteSnapshotStore` (SQLite) + `MemorySnapshotStore` (memory) + `snapshot.EveryNEvents`                                                   | Caps replay cost, persists across restarts                                        |
| Correlation    | `event.WithCorrelationID` in `SyncItems`                                                                                                    | Unique per sync run for debugging                                                 |
| Logging        | `middleware.EventLogging` via charm log adapter                                                                                             | Structured logging of all domain events                                           |
| Error taxonomy | `go-error-family` constructors (intrinsic classification) + `pkgerrors.HTTPStatus` (status) + `WithCtx`/`InvalidField` (structured context) | Smart retry classification, error→HTTP, and field-addressable validation errors   |
| Version        | `event.Version` (uint64) with `Increment()`, `Add()`                                                                                        | `int` → `uint64` in v3; no `int()` casts needed                                   |
