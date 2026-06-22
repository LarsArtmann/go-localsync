# Go-LocalSync Agent Configuration

**Updated:** 2026-06-14

## Project Overview

Go-LocalSync is a generic synchronization SDK with a pluggable provider-based architecture. It uses event-sourced CQRS via go-cqrs-lite for state management, pluggable conflict resolution via CRDT (`pkg/crdt/`), and branded IDs from go-branded-id for compile-time type safety.

## Architecture

| Package         | Purpose                                                                                                                                                                                                                |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/crdt/`     | CRDT/sync primitives: VectorClock, Operation[T], ConflictResolver[T], LWWResolver[T] — **wired into DecideSync as pluggable conflict strategy**                                                                        |
| `pkg/api/`      | HTTP API server with Huma v2 + stdlib (`GET /items`, `GET /stats`, `POST /sync`, `GET /health`), split into server.go + dto.go + handlers.go                                                                           |
| `pkg/cqrs/`     | CQRS integration layer using go-cqrs-lite **v2.2** (Decider, ReadModel, Projector, CQRSStack, Runner, TypedHandler), split into focused files (middleware.go, commands.go, queries.go, sqlite\_\*.go)                  |
| `pkg/provider/` | Core interfaces (`Provider`, `Item`, `FetchResult`, `RateLimitConfig`, `RetryConfig`, `FetchConfig`) and `RateLimitCache`. The SDK defines the contract only — concrete providers (e.g. GitHub) live in consumer apps. |
| `pkg/sync/`     | `Syncer`, `ConflictAwareSyncer`, `SyncStore` interface (decoupled from `*cqrs.CQRSStack`), `SyncAction`, `ItemSyncResult`, `SyncSummary`                                                                               |
| `pkg/id/`       | Branded phantom-type IDs (`ItemID` ULID, `ExternalID` string, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`)                                                                                                        |
| `pkg/errors/`   | Structured errors via `go-error-family` constructors (Rejection, Transient, Infrastructure) with intrinsic classification, `IsRetryable`                                                                               |

### SyncStore Interface Seam

`pkg/sync/` defines `SyncStore` — a minimal interface decoupling sync logic from CQRS infrastructure. `*cqrs.CQRSStack` implements it via adapter methods. Dependency flows one way: `cqrs → sync → provider/types/errors`. No import cycles.

`SyncAction` constants and `ItemSyncResult` live in `pkg/sync/` — the architectural seam — not in `pkg/cqrs/`.

## CQRS Architecture

The entire storage layer is CQRS-based via go-cqrs-lite. There is **no legacy CRUD path**.

### Core Components

- `aggregate_id.go` — deterministic SHA256→hex from (source, sourceID) with sync.Map cache, shared `itemKey` helper
- `decider.go` — `SyncItemState{Item *provider.Item, Deleted bool}`, pure Fold + DecideSync/DecideDelete, `HasChanged` checks UpdatedAt/Type/ActorLogin/RepoName/RepoURL
- `events.go` — 3 event types: `ItemSynced`, `ItemConflictFound`, `ItemDeleted`
- `readmodel.go` — `ReadModel` interface + `ItemFilter`, stores `*provider.Item` directly
- `memory_readmodel.go` — concurrent-safe in-memory read model with filter/pagination
- `sqlite_readmodel.go` — SQLite-backed read model with DDL, filter/pagination
- `projection.go` — `Projector` implements `event.Projection`, wired via direct bus subscription + `projection.Runner` for replay
- `stack.go` — `CQRSStack` with Store+Bus+Repo+ReadModel+CommandDispatcher+QueryDispatcher, dual projection runner, SQL snapshots/checkpoints, event logging middleware, correlation IDs
- `runner.go` — Unified projection subscription: direct `bus.SubscribeAll` for synchronous event delivery, plus `projection.Runner` for journal replay (SQLite backend)
- `commands.go` + `queries.go` + `middleware.go` — typed `SyncItemCommand`/`DeleteItemCommand` via `command.Dispatcher`, typed queries (`ListItemsQuery`, `GetItemQuery`, `CountItemsQuery`, `GetTypesQuery`) via `query.Dispatcher`

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic aggregate IDs**: SHA256→hex from (source, sourceID)
- **Delete + resurrect**: deleted items reappear with updated state
- **Projection runner**: SQLite uses `projection.Runner` with global replay + live subscription. Memory uses direct `bus.SubscribeAll`. Both paths subscribe synchronously to avoid race conditions.
- **SQL persistence**: SQLite backend persists snapshots (`SQLSnapshotStore`), checkpoints (`SQLCheckpointStore`) via `snapshot/v2` and `storage/v2` modules.
- **Correlation IDs**: `SyncItems` generates a unique `CorrelationID` per sync run, passed via `event.WithCorrelationID` to all events.
- **Command dispatch**: `SyncItem`/`DeleteItem` dispatched through `command.Dispatcher` with typed commands. Enables logging, retry, validation middleware.
- **Query dispatch**: Read model queries dispatched through `query.Dispatcher` with typed queries. Enables logging, metrics middleware.
- **Remote wins (default)**: on conflict with no resolver configured, the incoming item always overwrites (remote-wins LWW)
- **Pluggable conflict resolution**: `CQRSConfig.ConflictResolver` accepts any `crdt.ConflictResolver[*provider.Item]` — `LWWResolver`, custom merge, etc.

### Conflict Flow

`ConflictAwareSyncer` delegates entirely to `SyncStore.SyncItems()` which uses `DecideSync` as the single authority. `DecideSync` calls `HasChanged()` and:

1. If no resolver configured (nil): emits `ItemConflictFound{Winner: "remote"}` + `ItemSynced` with the incoming item (default remote-wins)
2. If resolver configured: calls `resolver.Resolve(&Conflict{Local, Remote, ...})` and uses the winner for `ItemSynced`. `ItemConflictFound{Winner}` records which side won ("remote" or "local")
3. On resolver error: falls back to remote-wins

The conflict winner determines the `SyncAction`: `ActionConflictRemote` or `ActionConflictLocal`. No split-brain — the decider is the single source of truth for conflict detection. Invalid items from `filterValidItems` are properly counted in `ConflictResult.Errors`.

## Development Workflow

### Local Development

1. **Optional**: Create `go.work` in project root ONLY for live-editing local sibling checkouts. It is in `.gitignore` and **must never be committed or left on disk during `buildflow`** — buildflow detects go.work on disk and expands `go test ./...` to ALL workspace modules (including `../go-cqrs-lite/*`), causing sibling test failures. With the committed `vendor/` directory, builds and tests work offline without go.work (`go build ./...`, `go test ./...` use vendor mode automatically). Remove go.work before running buildflow:

   ```
   go 1.26.3

   use .

   use (
       ../go-branded-id
       ../go-cqrs-lite/codec
       ../go-cqrs-lite/command
       ../go-cqrs-lite/decider
       ../go-cqrs-lite/dispatcher
       ../go-cqrs-lite/event
       ../go-cqrs-lite/id
       ../go-cqrs-lite/memory
       ../go-cqrs-lite/middleware
       ../go-cqrs-lite/projection
       ../go-cqrs-lite/snapshot
       ../go-cqrs-lite/storage
       ../go-error-family
   )
   ```

2. Build: `go build ./...`
3. Test: `go test ./... -count=1`
4. Lint: `golangci-lint run ./... --timeout=5m`
5. Format: `golangci-lint fmt ./...`
6. Full pipeline: `buildflow --build-mode full` (ensure go.work is removed first)

### CI (No go.work)

CI uses pseudo-versions from GitHub (no replace directives in `go.mod`):

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

| Package                    | Tests | Coverage | Status                                                                                                     |
| -------------------------- | ----- | -------- | ---------------------------------------------------------------------------------------------------------- |
| `pkg/cqrs`                 | ~85   | ~85%     | ✅ Decider, ReadModel, Projection, Stack, SQLite RM, Runner, Correlation, CRDT Resolver, Concurrent Access |
| `pkg/sync`                 | 22    | 91.0%    | ✅ Syncer + ConflictAwareSyncer + reportProgress + invalid item error counting                             |
| `pkg/id`                   | 10    | 100.0%   | ✅ ID construction, roundtrip, zero, equal                                                                 |
| `pkg/errors`               | 11    | 100.0%   | ✅ Sentinel errors, wrapping, classification, IsRetryable, registered templates                            |
| `pkg/provider`             | 2     | 95.8%    | ✅ Item validation                                                                                         |
| `pkg/api`                  | ~15   | 92.4%    | ✅ Server, routes, handlers, health/stats/items/sync endpoints, error path tests                           |
| `pkg/crdt`                 | ~55   | 97.6%    | ✅ VectorClock, Operation, LWWResolver, Conflict, SyncMessage, example test                                |
| `pkg/data/model`           | ~12   | 100%     | ✅ Item, Key, Validate, ItemFilter builder                                                                 |
| `cmd/examples/github-sync` | 14    | 12.3%    | ✅ exitCodeForError, LoadConfig, env defaults, printVersion, printSyncResultJSON                           |

**283 total test functions** across 11 test packages.

Run: `go test ./... -count=1`

## Backend Selection

Storage backends are selected via `CQRSConfig.Backend` in `cqrs.NewCQRSStack()`.

| Backend  | Flag/Config        | Use Case                                 |
| -------- | ------------------ | ---------------------------------------- |
| `memory` | `--backend memory` | Testing, development (default)           |
| `sqlite` | `--backend sqlite` | Local SQLite file via modernc.org/sqlite |

Event store + read model use the same backend.

### CLI Usage

```bash
go run ./cmd/examples/github-sync --backend memory
go run ./cmd/examples/github-sync --backend sqlite --db ./data.db
```

## Provider Development

When adding new providers:

1. Implement the `provider.Provider` interface (`Name`, `Fetch`, `FetchAll`, `GetRateLimit`)
2. Convert provider-specific data to `provider.Item` using branded types from `pkg/id/`
3. Add provider-specific tests
4. Update documentation with provider configuration
5. Add example in `cmd/examples/`

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

| Dependency                         | Version | Purpose                                                              |
| ---------------------------------- | ------- | -------------------------------------------------------------------- |
| `go-cqrs-lite/event/v2`            | v2.2.0  | Event types, Store, Bus, Journal, CheckpointStore, Codec, Projection |
| `go-cqrs-lite/command/v2`          | v2.2.0  | Command types, Dispatcher, TypedHandler[T], RegisterTyped[T]         |
| `go-cqrs-lite/query/v2`            | v2.2.0  | Query types, Dispatcher, TypedHandler[Q,R], RegisterTyped[Q,R]       |
| `go-cqrs-lite/decider/v2`          | v2.2.0  | Decider, Repository, snapshot/codec options                          |
| `go-cqrs-lite/id/v2`               | v2.2.0  | Branded phantom-type IDs (AggregateID, CorrelationID, etc.)          |
| `go-cqrs-lite/codec/v2`            | v2.2.0  | Codec interface, JSONCodec                                           |
| `go-cqrs-lite/snapshot/v2`         | v2.2.0  | SnapshotStore, EveryNEvents strategy                                 |
| `go-cqrs-lite/memory/v2`           | v2.2.0  | In-memory event store + bus + checkpoint store + snapshot store      |
| `go-cqrs-lite/middleware/v2`       | v2.2.0  | EventLogging middleware                                              |
| `go-cqrs-lite/projection/v2`       | v2.2.0  | Projection Runner with replay + live subscription                    |
| `go-cqrs-lite/storage/v2`          | v2.2.0  | SQLite event store, snapshot, checkpoint store                       |
| `go-branded-id`                    | v0.3.0  | Branded phantom-type IDs for compile-time safety                     |
| `go-error-family`                  | v0.3.0  | Structured error classification + user-facing message templates      |
| `go-github/v69`                    | v69.2.0 | GitHub API client                                                    |
| `modernc.org/sqlite`               | v1.51.0 | Pure-Go SQLite driver (replaces tursogo for local SQLite)            |
| `charm.land/log/v2`                | v2.0.0  | Structured logging                                                   |
| `caarlos0/env/v11`                 | v11.4.1 | Environment variable config                                          |
| `github.com/danielgtaylor/huma/v2` | v2.38.0 | HTTP API framework with OpenAPI 3 generation + stdlib adapter        |

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

| Area           | go-localsync                                                                                            | go-cqrs-lite                                           |
| -------------- | ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| IDs            | `id.ID[B, V]` via go-branded-id directly                                                                | `id.Of[T]` — same memory layout                        |
| Storage        | `CQRSStack` → `decider.Repository[SyncItemState]`                                                       | `event.Store` + `event.Bus` via memory/storage modules |
| Conflict       | `DecideSync` produces ItemConflictFound events                                                          | Error taxonomy with 5 families                         |
| Read Model     | `MemoryReadModel` + `SQLiteReadModel` with filter/pagination                                            | Projected from events via InMemoryRunner               |
| SyncStore      | `CQRSStack` implements `sync.SyncStore` via adapter methods (`ListItems`, `CountItems`, `GetItemTypes`) | `sync.SyncStore` interface defined in consumer package |
| SyncActions    | `classifyAction` returns `synclib.SyncAction` (`ActionCreated`, etc.)                                   | Types defined in `pkg/sync/`, not `pkg/cqrs/`          |
| Codec          | `codec.JSONCodec` + `event.DecodePayload[T]` + `event.NewEvents`                                        | Eliminates all manual json.Marshal/Unmarshal           |
| Projection     | Direct `bus.SubscribeAll` (sync) + `projection.Runner` (SQLite replay), SQL checkpoints                 | Replay from store on restart + live subscription       |
| Snapshots      | `SQLiteSnapshotStore` (Turso) + `MemorySnapshotStore` (memory) + `snapshot.EveryNEvents`                | Caps replay cost, persists across restarts             |
| Correlation    | `event.WithCorrelationID` in `SyncItems`                                                                | Unique per sync run for debugging                      |
| Logging        | `middleware.EventLogging` via charm log adapter                                                         | Structured logging of all domain events                |
| Error taxonomy | `go-error-family` constructors (intrinsic classification) + `event.IsRetryable`                         | Smart retry classification for provider errors         |
| Version        | `event.Version` with `Increment()`, `Add()`                                                             | Phantom type safety — no `int()` casts                 |
