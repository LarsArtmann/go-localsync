# Go-LocalSync Agent Configuration

**Updated:** 2026-05-25 (session 4)

## Project Overview

Go-LocalSync is a generic synchronization SDK with a pluggable provider-based architecture. It uses event-sourced CQRS via go-cqrs-lite for state management, timestamp-based conflict detection, and branded IDs from go-branded-id for compile-time type safety.

## Architecture

| Package                     | Purpose                                                                                                                                  |
| --------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/localsync/`            | CRDT/sync primitives: VectorClock, Operation[T], ConflictResolver[T], LWWResolver[T] (extracted from go-cqrs-lite/sync)                  |
| `pkg/cqrs/`                 | CQRS integration layer using go-cqrs-lite (Decider, ReadModel, Projector, CQRSStack, Runner)                                             |
| `pkg/provider/`             | Core interfaces (`Provider`, `Item`, `FetchResult`, `RateLimitConfig`, `RetryConfig`)                                                    |
| `pkg/providers/github/`     | GitHub provider implementation (only provider currently)                                                                                 |
| `pkg/sync/`                 | `Syncer`, `ConflictAwareSyncer`, `SyncStore` interface (decoupled from `*cqrs.CQRSStack`), `SyncAction`, `ItemSyncResult`, `SyncSummary` |
| `pkg/types/`                | Branded phantom-type IDs (`ItemID` ULID, `ExternalID` string, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`)                          |
| `pkg/errors/`               | Structured errors via `go-error-family` constructors (Rejection, Transient, Infrastructure) with intrinsic classification, `IsRetryable` |
| `cmd/examples/github-sync/` | Example CLI entry point                                                                                                                  |

### SyncStore Interface Seam

`pkg/sync/` defines `SyncStore` — a minimal interface that decouples the sync logic from concrete CQRS infrastructure:

```go
type SyncStore interface {
    SyncItems(ctx, items) *SyncSummary
    SyncItem(ctx, item) error
    ListItems(ctx, ItemFilter) ([]*provider.Item, error)
    CountItems(ctx, ItemFilter) (int64, error)
    GetItemTypes(ctx) ([]string, error)
    Count(ctx) (int64, error)
    Close() error
}
```

`*cqrs.CQRSStack` implements `SyncStore` via adapter methods (`ListItems`, `CountItems`, `GetItemTypes`) that convert `sync.ItemFilter` → `cqrs.ItemFilter` and delegate to the read model. The sync package has **zero imports on `pkg/cqrs`** — the dependency flows one way: `cqrs → sync`.

`SyncAction` constants (`ActionCreated`, `ActionUpdated`, `ActionConflictRemote`, `ActionUnchanged`, `ActionError`) and `ItemSyncResult` live in `pkg/sync/` — the architectural seam — not in `pkg/cqrs/`.

## CQRS Architecture

The entire storage layer is CQRS-based via go-cqrs-lite. There is **no legacy CRUD path**.

### Core Components

- `aggregate_id.go` — deterministic SHA256→hex from (source, sourceID) with sync.Map cache, shared `itemKey` helper
- `decider.go` — `SyncItemState{Item *provider.Item, Deleted bool}`, pure Fold + DecideSync/DecideDelete, `HasChanged` checks UpdatedAt/Type/ActorLogin/RepoName/RepoURL
- `events.go` — 3 event types: `ItemSynced`, `ItemConflictFound`, `ItemDeleted`
- `readmodel.go` — `ReadModel` interface + `ItemFilter`, stores `*provider.Item` directly
- `memory_readmodel.go` — concurrent-safe in-memory read model with filter/pagination
- `turso_readmodel.go` — SQLite/Turso-backed read model with DDL, filter/pagination
- `projection.go` — `Projector` implements `event.Projection`, wired via `event.InMemoryRunner` (memory) or `projection.Runner` (Turso)
- `stack.go` — `CQRSStack` with Store+Bus+Repo+ReadModel+CommandDispatcher+QueryDispatcher, dual projection runner, `event.OutboxPublisher`, SQL snapshots/checkpoints, event logging middleware, correlation IDs
- `commands_queries.go` — typed `SyncItemCommand`/`DeleteItemCommand` via `command.Dispatcher`, typed queries (`ListItemsQuery`, `GetItemQuery`, `CountItemsQuery`, `GetTypesQuery`) via `query.Dispatcher`

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic aggregate IDs**: SHA256→hex from (source, sourceID)
- **Delete + resurrect**: deleted items reappear with updated state
- **Outbox pattern**: Turso backend uses `decider.WithOutbox` for atomic save+publish. `event.OutboxPublisher` polls outbox and publishes events to bus (1s interval, configurable, with graceful shutdown, panic recovery, partial-batch safety).
- **Projection runner**: Turso uses `projection.Runner` with `GlobalLoader` replay + live subscription. Memory uses `InMemoryRunner`.
- **SQL persistence**: Turso backend persists snapshots (`SQLiteSnapshotStore`), checkpoints (`SQLiteCheckpointStore`), and outbox to the same `*sql.DB`.
- **Correlation IDs**: `SyncItems` generates a unique `CorrelationID` per sync run, passed via `event.WithCorrelationID` to all events.
- **Command dispatch**: `SyncItem`/`DeleteItem` dispatched through `command.Dispatcher` with typed commands. Enables logging, retry, validation middleware.
- **Query dispatch**: Read model queries dispatched through `query.Dispatcher` with typed queries. Enables logging, metrics middleware.
- **Remote wins**: on conflict, the incoming item always overwrites (remote-wins LWW)

### Conflict Flow

`ConflictAwareSyncer` delegates entirely to `SyncStore.SyncItems()` (which `CQRSStack` implements). The decider (`DecideSync`) is the single authority for conflict detection, producing `ItemSyncResult` with `SyncAction` classification. `ConflictAwareSyncer` iterates `summary.Results` using `sync.ActionCreated`/`sync.ActionConflictRemote` etc. constants. No split-brain — the decider is the single source of truth for conflict detection. Invalid items from `filterValidItems` are properly counted in `ConflictResult.Errors`.

## Development Workflow

### Local Development

1. Create `go.work` in project root (already in `.gitignore`):
   ```
   go 1.26.2
   use .
   use (
       ../go-branded-id
       ../go-cqrs-lite/core
       ../go-cqrs-lite/memory
       ../go-cqrs-lite/storage
       ../go-cqrs-lite/middleware
       ../go-cqrs-lite/projection
       ../go-error-family
   )
   ```
2. Build: `go build ./...`
3. Test: `go test ./... -count=1`
4. Lint: `golangci-lint run ./... --timeout=5m`
5. Format: `golangci-lint fmt ./...`

### CI (No go.work)

CI uses pseudo-versions from GitHub (no replace directives in `go.mod`):

```bash
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go build ./...
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go test ./... -count=1
```

### Pre-commit Hooks

Pre-commit hooks use `buildflow` (not testify-banning). Hooks are not set as executable and are skipped.

## Testing

| Package                    | Tests | Coverage | Status                                                                                     |
| -------------------------- | ----- | -------- | ------------------------------------------------------------------------------------------ |
| `pkg/cqrs`                 | 73    | 82.5%    | ✅ Decider, ReadModel, Projection, Stack, Turso RM, Push/Pull, Runner, Outbox, Correlation |
| `pkg/providers/github`     | 46    | 85.4%    | ✅ Client, fetch, retry, error handling, rate limit, BDD                                   |
| `pkg/sync`                 | 14    | ~85%     | ✅ Syncer + ConflictAwareSyncer + invalid item error counting (mock SyncStore)             |
| `pkg/types`                | 15    | 100.0%   | ✅ ID construction, roundtrip, zero, equal                                                 |
| `pkg/errors`               | 28    | 100.0%   | ✅ Sentinel errors, wrapping, classification, fallback paths                               |
| `pkg/provider`             | 6     | 100.0%   | ✅ Item validation                                                                         |
| `cmd/examples/github-sync` | 11    | 10.5%    | ✅ exitCodeForError, LoadConfig, env defaults                                              |
| `pkg/testhelpers`          | —     | —        | ⬜ Deleted — helpers moved to `pkg/providers/github/testhelpers_test.go`                   |

**153 total test functions** across 6 test packages.

Run: `go test ./... -count=1`

## Backend Selection

Storage backends are selected via `CQRSConfig.Backend` in `cqrs.NewCQRSStack()`.

| Backend  | Flag/Config        | Use Case                                |
| -------- | ------------------ | --------------------------------------- |
| `memory` | `--backend memory` | Testing, development (default)          |
| `turso`  | `--backend turso`  | Local SQLite/Turso file or remote Turso |

Event store + read model use the same backend. Turso backend adds remote sync via `Push()`/`Pull()`.

### CLI Usage

```bash
go run ./cmd/examples/github-sync --backend memory
go run ./cmd/examples/github-sync --backend turso --db ./data.db
go run ./cmd/examples/github-sync --backend turso --db ./data.db --remote-url https://... --auth-token ...
```

## Provider Development

When adding new providers:

1. Implement the `provider.Provider` interface (`Name`, `Fetch`, `FetchAll`, `GetRateLimit`)
2. Convert provider-specific data to `provider.Item` using branded types from `pkg/types/`
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

| Dependency                    | Version | Purpose                                                                  |
| ----------------------------- | ------- | ------------------------------------------------------------------------ |
| `go-cqrs-lite/core`           | v1.4.0  | Decider, event types, branded IDs, error taxonomy, codec, InMemoryRunner |
| `go-cqrs-lite/memory`         | v1.2.0  | In-memory event store + bus + checkpoint store + snapshot store          |
| `go-cqrs-lite/storage`        | pseudo  | SQLite/Turso event store, outbox, snapshot, checkpoint store             |
| `go-cqrs-lite/middleware`     | v1.0.0  | EventLogging middleware                                                  |
| `go-cqrs-lite/projection`     | v1.1.0  | Projection Runner with replay + live subscription                        |
| `go-branded-id`               | v0.1.0  | Branded phantom-type IDs for compile-time safety                         |
| `go-error-family`             | v0.1.1  | Structured error classification (Rejection, Transient, Infrastructure)   |
| `go-github/v69`               | v69.2.0 | GitHub API client                                                        |
| `turso.tech/database/tursogo` | v0.6.0  | Turso Go client — local + remote sync                                    |
| `charm.land/log/v2`           | v2.0.0  | Structured logging                                                       |
| `caarlos0/env/v11`            | v11.4.1 | Environment variable config                                              |

### Test Dependencies

| Dependency       | Purpose                                |
| ---------------- | -------------------------------------- |
| `onsi/ginkgo/v2` | Indirect only (via go-cqrs-lite tests) |
| `onsi/gomega`    | Indirect only (via go-cqrs-lite tests) |

## go-cqrs-lite Integration

| Area           | go-localsync                                                                                            | go-cqrs-lite                                           |
| -------------- | ------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| IDs            | `id.ID[B, V]` via go-branded-id directly                                                                | `id.Of[T]` — same memory layout                        |
| Storage        | `CQRSStack` → `decider.Repository[SyncItemState]`                                                       | `event.Store` + `event.Bus` via memory/storage modules |
| Conflict       | `DecideSync` produces ItemConflictFound events                                                          | Error taxonomy with 5 families                         |
| Read Model     | `MemoryReadModel` + `TursoReadModel` with filter/pagination                                             | Projected from events via InMemoryRunner               |
| SyncStore      | `CQRSStack` implements `sync.SyncStore` via adapter methods (`ListItems`, `CountItems`, `GetItemTypes`) | `sync.SyncStore` interface defined in consumer package |
| SyncActions    | `classifyAction` returns `synclib.SyncAction` (`ActionCreated`, etc.)                                   | Types defined in `pkg/sync/`, not `pkg/cqrs/`          |
| Codec          | `event.JSONCodec` + `DecodePayload[T]` + `NewEvents`                                                    | Eliminates all manual json.Marshal/Unmarshal           |
| Outbox         | `SQLTransactionalStore.SaveWithOutbox` for Turso, outbox poller goroutine                               | Atomic save+publish, crash-safe event delivery         |
| Projection     | `projection.Runner` (Turso) + `InMemoryRunner` (memory), SQL checkpoints                                | Replay from store on restart + live subscription       |
| Snapshots      | `SQLiteSnapshotStore` (Turso) + `MemorySnapshotStore` (memory) + `EveryNEvents`                         | Caps replay cost, persists across restarts             |
| Correlation    | `event.WithCorrelationID` in `SyncItems`                                                                | Unique per sync run for debugging                      |
| Logging        | `middleware.EventLogging` via charm log adapter                                                         | Structured logging of all domain events                |
| Error taxonomy | `go-error-family` constructors (intrinsic classification) + `event.IsRetryable`                         | Smart retry classification for provider errors         |
| Version        | `event.Version` with `Increment()`, `Add()`                                                             | Phantom type safety — no `int()` casts                 |

### Not Yet Adopted

- `sync.LWWResolver[T]` + `sync.VectorClock` — **MEDIUM** (formalize conflict resolution)
- `middleware.CommandRetry` for provider retry — **LOW** (API mismatch: wraps `command.Handler`, not `func() error`)
- `UpcasterRegistry` for schema evolution — **LOW** (only 1 schema version)
- `catalog/` for AsyncAPI/OpenAPI/D2 generation — **LOW**
- `core/aggregate` — **LOW** (we use `decider.Decider` directly — correct, no benefit)

## go-cqrs-lite Adoption (as of 2026-05-22)

**Adoption: 9/12 modules (75%). API surface of used modules: ~85%.**

Modules used: `core/event`, `core/decider`, `core/command`, `core/query`, `core/pkg/id`, `memory`, `storage`, `middleware`, `projection`.
Modules unused: `core/aggregate`, `sync`, `catalog`.

All anti-patterns from the previous audit have been resolved:

- ✅ `event.Version` uses `.Increment()` / `.Add()` — no `int()` casts
- ✅ `event.InMemoryRunner` with checkpointing — events tracked on restart
- ✅ `event.JSONCodec` + `DecodePayload[T]` + `NewEvents` — no manual json.Marshal/Unmarshal
- ✅ `aggregate_id.go` uses `hex.EncodeToString` — no ULID dependency in this file
- ✅ Error taxonomy wired — `go-error-family` constructors with intrinsic classification (no `init()`, no `RegisterClassification`)
- ✅ `SyncItems` distinguishes `ActionCreated` vs `ActionUpdated` via `state.IsNew()`
- ✅ `HasChanged()` checks `RepoURL` in addition to UpdatedAt/Type/ActorLogin/RepoName
- ✅ DRY: `itemKey` helper consolidates `source+":"sourceID` pattern (4→1)
- ✅ DRY: `initSchema` consolidates `initTursoSyncDB`/`initTursoDB` via `dbExecContext` interface
- ✅ `classifyAction` helper reduces nesting complexity in `SyncItems`
- ✅ Snapshot support with `EveryNEvents(10)` — caps replay cost
- ✅ `middleware.EventLogging` — structured logging of all domain events
- ✅ `Projector` implements `event.Projection` directly — no Handle/HandleEvent duplication
- ✅ `decider.WithOutbox` for Turso — atomic save+publish via `SQLTransactionalStore`
- ✅ `projection.Runner` for Turso — replay from `GlobalLoader` + live subscription + retry
- ✅ `SQLiteSnapshotStore` + `SQLiteCheckpointStore` for Turso — persists across restarts
- ✅ Single `*sql.DB` for Turso local — shared by event store, read model, snapshots, checkpoints
- ✅ `SQLiteInitSchema` + `ConfigureTursoPool` — replaces hand-rolled schema/pool config
- ✅ Correlation IDs via `event.WithCorrelationID` — unique per `SyncItems` run
- ✅ `DecideSync`/`DecideDelete` accept `...event.Option` — extensible event metadata
- ✅ `event.OutboxPublisher` replaces hand-rolled `startOutboxPoller` — graceful shutdown, panic recovery, partial-batch safety, 1s configurable poll interval
- ✅ `command.Dispatcher` with `SyncItemCommand`/`DeleteItemCommand` — typed command dispatch, enables middleware pipeline
- ✅ `query.Dispatcher` with typed queries — `ListItemsQuery`, `GetItemQuery`, `CountItemsQuery`, `GetTypesQuery` dispatched through query pipeline
- ✅ `event.NewOutboxPublisher` exported from go-cqrs-lite — was previously unexported `outboxPublisher`
- ✅ `event.NewEvents`/`event.MustNewEvents`/`event.DecodePayloads` exported from go-cqrs-lite — were previously unexported

## Unix-Style Modularity (Session 4 — 2026-05-25)

### Completed Improvements

- ✅ **`SyncStore` interface seam**: `pkg/sync/` defines `SyncStore` interface; `*cqrs.CQRSStack` implements it via adapter methods. `pkg/sync/` has **zero imports on `pkg/cqrs/`**.
- ✅ **`SyncAction`/`ItemSyncResult` moved to seam**: These types now live in `pkg/sync/` (the architectural boundary), not `pkg/cqrs/`. The `classifyAction` function in `cqrs` returns `synclib.SyncAction`.
- ✅ **`IsRetryable` moved to `pkg/errors/`**: Retry-classification is a generic concern, not event-sourcing. Delegates to `errorfamily.IsRetryable()`.
- ✅ **`pkg/testhelpers/` deleted**: Only used by github provider tests. Helpers moved to `pkg/providers/github/testhelpers_test.go` as unexported test helpers.
- ✅ **`sync_test.go` uses mock `SyncStore`**: No import cycle — sync tests use a `mockSyncStore` struct, not `*cqrs.CQRSStack`. Integration tests for conflict-aware sync live in `pkg/cqrs/`.
- ✅ **go-cqrs-lite API drift fixed**: `command.Core→BasicCommand`, `query.Core→BasicQuery`, `event.Core→ImmutableEvent`, `NewCheckpointStore→NewMemoryCheckpointStore`.

## Lint Status

golangci-lint v2 reports **0 issues**. Config is strict with 125+ linters enabled.
