# Go-LocalSync Agent Configuration

**Updated:** 2026-06-22

## Project Overview

Go-LocalSync is a generic synchronization SDK with a pluggable provider-based architecture. It uses event-sourced CQRS via go-cqrs-lite for state management, pluggable conflict resolution via CRDT (`pkg/crdt/`), and branded IDs from go-branded-id for compile-time type safety.

## Architecture

| Package         | Purpose                                                                                                                                                                                                                                          |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `pkg/crdt/`     | CRDT/sync primitives: VectorClock, Operation[T], ConflictResolver[T], LWWResolver[T] — **wired into DecideSync as pluggable conflict strategy**                                                                                                  |
| `pkg/api/`      | HTTP API server with Huma v2 + stdlib (`GET /items`, `GET /stats`, `POST /sync`, `GET /health`), split into server.go + dto.go + handlers.go                                                                                                     |
| `pkg/cqrs/`     | CQRS integration layer using go-cqrs-lite **v3.0** (Decider, ReadModel, Projector, CQRSStack, TypedHandler), split into focused files (middleware.go, commands.go, queries.go, sqlite\_\*.go)                                                    |
| `pkg/provider/` | Core interfaces (`Provider`, `Item`, `FetchResult`, `RateLimitConfig`, `RetryConfig`, `FetchConfig`) and `RateLimitCache`. The SDK defines the contract only — concrete providers (e.g. GitHub) live in consumer apps.                           |
| `pkg/sync/`     | `Syncer`, `ConflictAwareSyncer`, `SyncStore` interface (decoupled from `*cqrs.CQRSStack`), `SyncAction`, `ItemSyncResult`, `SyncSummary`                                                                                                         |
| `pkg/data/`     | Domain model: `model.Item` (persisted entity with `SchemaVersion`), `model.Key`, `model.ItemFilter`; `schema.Version` (V1/V2 versioning for event upcasting). Decider, read model, events, and conflict resolution all operate on `*model.Item`. |
| `pkg/id/`       | Branded phantom-type IDs (`ItemID` ULID, `ExternalID` string, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`)                                                                                                                                  |
| `pkg/errors/`   | Structured errors via `go-error-family` constructors (Rejection, Transient, Infrastructure) with intrinsic classification, `IsRetryable`                                                                                                         |

### SyncStore Interface Seam

`pkg/sync/` defines `SyncStore` — a minimal interface decoupling sync logic from CQRS infrastructure. `*cqrs.CQRSStack` implements it via adapter methods. Dependency flows one way: `cqrs → sync → provider/types/errors`. No import cycles.

`SyncAction` constants and `ItemSyncResult` live in `pkg/sync/` — the architectural seam — not in `pkg/cqrs/`.

## CQRS Architecture

The entire storage layer is CQRS-based via go-cqrs-lite. There is **no legacy CRUD path**.

### Core Components

- `aggregate_id.go` — deterministic SHA256→hex from (source, sourceID) with sync.Map cache, shared `itemKey` helper
- `decider.go` — `SyncItemState{Item *model.Item, Deleted bool}`, pure Apply (the event applier) + DecideSync/DecideDelete, `HasChanged` checks UpdatedAt/Type/ActorLogin/RepoName/RepoURL
- `events.go` — 3 event types: `ItemSynced`, `ItemConflictFound`, `ItemDeleted`
- `readmodel.go` — `ReadModel` interface (embeds `model.ItemReader`) + `model.ItemFilter`, stores `*model.Item` directly
- `memory_readmodel.go` — concurrent-safe in-memory read model with filter/pagination
- `sqlite_readmodel.go` — SQLite-backed read model with DDL, filter/pagination
- `projection.go` — `Projector` implements `event.Projection`, wired via direct bus subscription (live) + manual journal replay (persistence)
- `stack.go` — `CQRSStack` with Store+Bus+Repo+ReadModel+CommandDispatcher+QueryDispatcher, SQL snapshots, event logging middleware, correlation IDs
- `runner.go` — Projection wiring: direct `bus.SubscribeAll` for synchronous live event delivery, plus background `replayJournal` (reads all persisted events via `Journal.ReadAll`) for SQLite catch-up. Replaces the deleted `projection.Runner` (go-cqrs-lite v3 dropped `projection/` per ADR-0030).
- `commands.go` + `queries.go` + `middleware.go` — typed `SyncItemCommand`/`DeleteItemCommand` via `command.Dispatcher`, typed queries (`ListItemsQuery`, `GetItemQuery`, `CountItemsQuery`, `GetTypesQuery`) via `query.Dispatcher`

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic aggregate IDs**: SHA256→hex from (source, sourceID)
- **Delete + resurrect**: deleted items reappear with updated state
- **Projection**: Live events delivered synchronously via `bus.SubscribeAll` (watermill `EventBus` with `BlockPublishUntilSubscriberAck` preserves read-your-writes). SQLite catch-up replays the full journal in a background goroutine (`runner.replayJournal`); the idempotent projection tolerates replay/live overlap, so no checkpoint store is needed.
- **SQL persistence**: SQLite backend persists snapshots (`SQLSnapshotStore`) via `snapshot/v3` and `storage/v3` modules.
- **Correlation IDs**: `SyncItems` generates a unique `CorrelationID` per sync run, passed via `event.WithCorrelationID` to all events.
- **Command dispatch**: `SyncItem`/`DeleteItem` dispatched through `command.Dispatcher` with typed commands. Enables logging, retry, validation middleware.
- **Query dispatch**: Read model queries dispatched through `query.Dispatcher` with typed queries. Enables logging, metrics middleware.
- **Remote wins (default)**: on conflict with no resolver configured, the incoming item always overwrites (remote-wins LWW)
- **Pluggable conflict resolution**: `CQRSConfig.ConflictResolver` accepts any `crdt.ConflictResolver[*model.Item]` — `LWWResolver`, custom merge, etc.

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
6. Full pipeline: `buildflow --build-mode full` (ensure go.work is removed first)

### CI (No go.work)

CI uses tagged versions from GitHub (no replace directives in `go.mod`):

```bash
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go build ./...
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go test ./... -count=1
```

### Pre-commit Hooks

Pre-commit hooks use `buildflow` (not testify-banning). Hooks are not set as executable and are skipped.

### Build & Lint Gotchas

- **Private dependency**: `go-cqrs-lite` is a **private** GitHub repo (siblings `go-branded-id` and `go-error-family` are public). The nix sandbox cannot fetch it, so `nix build` / `nix flake check` require **vendored deps**: a committed `vendor/` dir + `vendorHash = null` in `flake.nix` + a `vendor/**` exclusion in `treefmt`. Regenerate with `GOWORK=off go mod vendor`. Cleanest long-term fix: make `go-cqrs-lite` public, then switch to a real `vendorHash` and drop `vendor/`.
- **go.work breaks buildflow**: buildflow's `ForEachGoModule` detects go.work **on disk** (not just tracked) and runs `go test ./...` in every workspace module — including sibling repos (`../go-cqrs-lite/*`) whose tests fail. Always **delete go.work before `buildflow`**; with `vendor/` committed, builds/tests work without it.
- **golangci-lint v2.12 `exhaustruct`**: the `settings.exhaustruct.exclude` list does **not** match local-package types in full runs (only stdlib full-path patterns work). For local domain structs with optional fields (`ItemFilter`, `FetchConfig`, `FetchResult`, `RateLimitCache`), suppress via `issues.exclusions.rules` with a `text:` regex instead.
- **`SA5012` disabled**: staticcheck v0.7 panics ("can't set facts on objects belonging another package") on cross-package even-elements analysis (e.g. `testutil.BuildPairs` called from another package's tests). Disabled in `linters.settings.staticcheck.checks`.

## Testing

| Package           | Tests | Coverage | Status                                                                                                     |
| ----------------- | ----- | -------- | ---------------------------------------------------------------------------------------------------------- |
| `pkg/cqrs`        | 89    | 81.4%    | ✅ Decider, ReadModel, Projection, Stack, SQLite RM, Replay, Correlation, CRDT Resolver, Concurrent Access |
| `pkg/sync`        | 24    | 85.5%    | ✅ Syncer + ConflictAwareSyncer + reportProgress + invalid item error counting                             |
| `pkg/id`          | 12    | 100.0%   | ✅ ID construction, roundtrip, zero, equal                                                                 |
| `pkg/errors`      | 9     | 100.0%   | ✅ Sentinel errors, wrapping, classification, IsRetryable, registered templates                            |
| `pkg/provider`    | 10    | 96.7%    | ✅ Item validation, RateLimitCache                                                                         |
| `pkg/api`         | 14    | 94.0%    | ✅ Server, routes, handlers, health/stats/items/sync endpoints, error path tests                           |
| `pkg/crdt`        | 53    | 96.2%    | ✅ VectorClock, Operation, LWWResolver, Conflict, SyncMessage, example test                                |
| `pkg/data/model`  | 10    | 100.0%   | ✅ Item, Key, Validate, ItemFilter builder                                                                 |
| `pkg/data/schema` | 4     | 100.0%   | ✅ Schema Version (V1/V2), CurrentVersion, Valid                                                           |

**225 total test functions** across 9 test packages.

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
2. Convert provider-specific data to `provider.Item` using branded types from `pkg/id/`
3. Add provider-specific tests
4. Update documentation with provider configuration

## Database Schema

Two tables managed by the CQRS stack:

### Events (via go-cqrs-lite/storage)

- `id`, `event_type`, `aggregate_type`, `aggregate_id`, `version`, `schema_version`
- `payload`, `metadata`, `occurred_at`, `created_at`
- Unique constraint on `(aggregate_type, aggregate_id, version)`

### Sync Items (read model projection)

- `item_id`, `source`, `source_id`, `type`, `actor_login`, `actor_avatar_url`
- `repo_name`, `repo_url`, `created_at`, `updated_at`, `raw_json`
- Primary key on `(source, source_id)`

## Dependencies

| Dependency                         | Version | Purpose                                                          |
| ---------------------------------- | ------- | ---------------------------------------------------------------- |
| `go-cqrs-lite/event/v3`            | v3.0.0  | Event types, Store, Bus, Journal, Projection, `Version` (uint64) |
| `go-cqrs-lite/command/v3`          | v3.0.0  | Command types, Dispatcher, TypedHandler[T], RegisterTyped[T]     |
| `go-cqrs-lite/query/v3`            | v3.0.0  | Query types, Dispatcher, TypedHandler[Q,R], RegisterTyped[Q,R]   |
| `go-cqrs-lite/decider/v3`          | v3.0.0  | Decider (`Apply` field), Repository, snapshot/codec options      |
| `go-cqrs-lite/id/v3`               | v3.0.0  | Branded phantom-type IDs (AggregateID, CorrelationID, etc.)      |
| `go-cqrs-lite/codec/v3`            | v3.0.0  | Codec interface, JSONCodec                                       |
| `go-cqrs-lite/snapshot/v3`         | v3.0.0  | SnapshotStore, EveryNEvents strategy                             |
| `go-cqrs-lite/storage/memory/v3`   | v3.0.0  | In-memory event store + snapshot store (bus deleted in v3)       |
| `go-cqrs-lite/middleware/v3`       | v3.0.0  | EventLogging + CommandRetry middleware                           |
| `go-cqrs-lite/watermill/v3`        | v3.0.0  | In-process `EventBus` (replaces deleted `memory.NewMemoryBus`)   |
| `go-cqrs-lite/storage/v3`          | v3.0.0  | SQLite event store, snapshot, KV store                           |
| `go-branded-id`                    | v0.3.1  | Branded phantom-type IDs for compile-time safety                 |
| `go-error-family`                  | v0.4.0  | Structured error classification + user-facing message templates  |
| `modernc.org/sqlite`               | v1.52.0 | Pure-Go SQLite driver (no CGo)                                   |
| `charm.land/log/v2`                | v2.0.0  | Structured logging                                               |
| `github.com/danielgtaylor/huma/v2` | v2.38.0 | HTTP API framework with OpenAPI 3 generation + stdlib adapter    |

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

| Area           | go-localsync                                                                                             | go-cqrs-lite                                                                 |
| -------------- | -------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| IDs            | `id.ID[B, V]` via go-branded-id directly                                                                 | `id.Of[T]` — same memory layout                                              |
| Storage        | `CQRSStack` → `decider.Repository[SyncItemState]`                                                        | `event.Store` + `event.Bus` via storage/memory + watermill modules           |
| Conflict       | `DecideSync` produces ItemConflictFound events                                                           | Error taxonomy with 5 families                                               |
| Read Model     | `MemoryReadModel` + `SQLiteReadModel` with filter/pagination                                             | Projected from events via custom `Projector` (no `projection/` module in v3) |
| SyncStore      | `CQRSStack` implements `sync.SyncStore` via adapter methods (`List`, `Count`, `CountByType`, `GetTypes`) | `sync.SyncStore` interface defined in consumer package                       |
| SyncActions    | `classifyAction` returns `synclib.SyncAction` (`ActionCreated`, etc.)                                    | Types defined in `pkg/sync/`, not `pkg/cqrs/`                                |
| Codec          | `codec.JSONCodec` + `event.DecodePayload[T]` + `event.NewEvents`                                         | Eliminates all manual json.Marshal/Unmarshal                                 |
| Projection     | Direct `bus.SubscribeAll` (sync) + background `replayJournal` (SQLite catch-up); no checkpoint store     | `projection/` deleted in v3 — manual replay replaces it                      |
| Snapshots      | `SQLiteSnapshotStore` (SQLite) + `MemorySnapshotStore` (memory) + `snapshot.EveryNEvents`                | Caps replay cost, persists across restarts                                   |
| Correlation    | `event.WithCorrelationID` in `SyncItems`                                                                 | Unique per sync run for debugging                                            |
| Logging        | `middleware.EventLogging` via charm log adapter                                                          | Structured logging of all domain events                                      |
| Error taxonomy | `go-error-family` constructors (intrinsic classification) + `event.IsRetryable`                          | Smart retry classification for provider errors                               |
| Version        | `event.Version` (uint64) with `Increment()`, `Add()`                                                     | `int` → `uint64` in v3; no `int()` casts needed                              |
