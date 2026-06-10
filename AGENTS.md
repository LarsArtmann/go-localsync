# Go-LocalSync Agent Configuration

**Updated:** 2026-06-10 (session 10 — architecture improvement plan execution)

## Project Overview

Go-LocalSync is a generic synchronization SDK with a pluggable provider-based architecture. It uses event-sourced CQRS via go-cqrs-lite for state management, pluggable conflict resolution via CRDT (`pkg/crdt/`), and branded IDs from go-branded-id for compile-time type safety.

## Architecture

| Package                     | Purpose                                                                                                                                         |
| --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/crdt/`                 | CRDT/sync primitives: VectorClock, Operation[T], ConflictResolver[T], LWWResolver[T] — **wired into DecideSync as pluggable conflict strategy** |
| `pkg/api/`                  | HTTP API server with Huma v2 + stdlib (`GET /items`, `GET /stats`, `POST /sync`, `GET /health`)                                                 |
| `pkg/cqrs/`                 | CQRS integration layer using go-cqrs-lite **v2** (Decider, ReadModel, Projector, CQRSStack, Runner)                                             |
| `pkg/provider/`             | Core interfaces (`Provider`, `Item`, `FetchResult`, `RateLimitConfig`, `RetryConfig`, `ItemFilter`)                                             |
| `pkg/providers/github/`     | GitHub provider implementation (only provider currently)                                                                                        |
| `pkg/sync/`                 | `Syncer`, `ConflictAwareSyncer`, `SyncStore` interface (decoupled from `*cqrs.CQRSStack`), `SyncAction`, `ItemSyncResult`, `SyncSummary`        |
| `pkg/id/`                   | Branded phantom-type IDs (`ItemID` ULID, `ExternalID` string, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`)                                 |
| `pkg/errors/`               | Structured errors via `go-error-family` constructors (Rejection, Transient, Infrastructure) with intrinsic classification, `IsRetryable`        |
| `cmd/examples/github-sync/` | Example CLI entry point (sync mode + HTTP server mode via `-server`)                                                                            |

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
- `turso_readmodel.go` — SQLite-backed read model with DDL, filter/pagination
- `projection.go` — `Projector` implements `event.Projection`, wired via direct bus subscription + `projection.Runner` for replay
- `stack.go` — `CQRSStack` with Store+Bus+Repo+ReadModel+CommandDispatcher+QueryDispatcher, dual projection runner, SQL snapshots/checkpoints, event logging middleware, correlation IDs
- `runner.go` — Unified projection subscription: direct `bus.SubscribeAll` for synchronous event delivery, plus `projection.Runner` for journal replay (Turso backend)
- `commands_queries.go` — typed `SyncItemCommand`/`DeleteItemCommand` via `command.Dispatcher`, typed queries (`ListItemsQuery`, `GetItemQuery`, `CountItemsQuery`, `GetTypesQuery`) via `query.Dispatcher`

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic aggregate IDs**: SHA256→hex from (source, sourceID)
- **Delete + resurrect**: deleted items reappear with updated state
- **Projection runner**: Turso uses `projection.Runner` with global replay + live subscription. Memory uses direct `bus.SubscribeAll`. Both paths subscribe synchronously to avoid race conditions.
- **SQL persistence**: Turso backend persists snapshots (`SQLSnapshotStore`), checkpoints (`SQLCheckpointStore`) via `snapshot/v2` and `storage/v2` modules.
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

### SyncStore Interface

`pkg/sync/` defines the `SyncStore` interface — the architectural boundary between sync logic and storage. `*cqrs.CQRSStack` satisfies it implicitly.

```go
type SyncStore interface {
    SyncItems(ctx context.Context, items []*provider.Item) *SyncSummary
    ListItems(ctx context.Context, filter provider.ItemFilter) ([]*provider.Item, error)
    CountItems(ctx context.Context, filter provider.ItemFilter) (int64, error)
    GetItemTypes(ctx context.Context) ([]string, error)
    Close() error
}
```

Dependency flows one way: `cqrs → sync → provider/types/errors`. No import cycles.

## Development Workflow

### Local Development

1. Create `go.work` in project root (already in `.gitignore`):
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

### CI (No go.work)

CI uses pseudo-versions from GitHub (no replace directives in `go.mod`):

```bash
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go build ./...
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go test ./... -count=1
```

### Pre-commit Hooks

Pre-commit hooks use `buildflow` (not testify-banning). Hooks are not set as executable and are skipped.

## Testing

| Package                    | Tests | Coverage | Status                                                                                 |
| -------------------------- | ----- | -------- | -------------------------------------------------------------------------------------- |
| `pkg/cqrs`                 | ~80   | ~83%     | ✅ Decider, ReadModel, Projection, Stack, Turso RM, Runner, Correlation, CRDT Resolver |
| `pkg/providers/github`     | 32    | 84.6%    | ✅ Client, fetch, retry, error handling, rate limit, BDD                               |
| `pkg/sync`                 | 22    | 92.3%    | ✅ Syncer + ConflictAwareSyncer + reportProgress + invalid item error counting         |
| `pkg/id`                   | 10    | 100.0%   | ✅ ID construction, roundtrip, zero, equal                                             |
| `pkg/errors`               | 11    | 100.0%   | ✅ Sentinel errors, wrapping, classification, IsRetryable, registered templates        |
| `pkg/provider`             | 2     | 100.0%   | ✅ Item validation                                                                     |
| `pkg/api`                  | 8     | ~90%     | ✅ Server, routes, handlers, health/stats/items/sync endpoints                         |
| `pkg/crdt`                 | 52    | 97.6%    | ✅ VectorClock, Operation, LWWResolver, Conflict, SyncMessage roundtrip                |
| `cmd/examples/github-sync` | 14    | 10.3%    | ✅ exitCodeForError, LoadConfig, env defaults, printVersion, printSyncResultJSON       |

**220+ total test functions** across 9 test packages.

Run: `go test ./... -count=1`

## Backend Selection

Storage backends are selected via `CQRSConfig.Backend` in `cqrs.NewCQRSStack()`.

| Backend  | Flag/Config        | Use Case                                |
| -------- | ------------------ | --------------------------------------- |
| `memory` | `--backend memory` | Testing, development (default)          |
| `turso`  | `--backend turso`  | Local SQLite/Turso file or remote Turso |

Event store + read model use the same backend.

### CLI Usage

```bash
go run ./cmd/examples/github-sync --backend memory
go run ./cmd/examples/github-sync --backend turso --db ./data.db
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
| `go-cqrs-lite/event/v2`            | v2.0.0  | Event types, Store, Bus, Journal, CheckpointStore, Codec, Projection |
| `go-cqrs-lite/command/v2`          | v2.0.0  | Command types, Dispatcher, typed commands                            |
| `go-cqrs-lite/query/v2`            | v2.0.0  | Query types, Dispatcher, typed queries                               |
| `go-cqrs-lite/decider/v2`          | v2.0.0  | Decider, Repository, snapshot/codec options                          |
| `go-cqrs-lite/id/v2`               | v2.0.0  | Branded phantom-type IDs (AggregateID, CorrelationID, etc.)          |
| `go-cqrs-lite/codec/v2`            | v2.0.0  | Codec interface, JSONCodec                                           |
| `go-cqrs-lite/snapshot/v2`         | v2.0.0  | SnapshotStore, EveryNEvents strategy                                 |
| `go-cqrs-lite/memory/v2`           | v2.0.0  | In-memory event store + bus + checkpoint store + snapshot store      |
| `go-cqrs-lite/middleware/v2`       | v2.0.0  | EventLogging middleware                                              |
| `go-cqrs-lite/projection/v2`       | v2.0.0  | Projection Runner with replay + live subscription                    |
| `go-cqrs-lite/storage/v2`          | v2.0.0  | SQLite event store, snapshot, checkpoint store (modernc.org/sqlite)  |
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
| Read Model     | `MemoryReadModel` + `TursoReadModel` with filter/pagination                                             | Projected from events via InMemoryRunner               |
| SyncStore      | `CQRSStack` implements `sync.SyncStore` via adapter methods (`ListItems`, `CountItems`, `GetItemTypes`) | `sync.SyncStore` interface defined in consumer package |
| SyncActions    | `classifyAction` returns `synclib.SyncAction` (`ActionCreated`, etc.)                                   | Types defined in `pkg/sync/`, not `pkg/cqrs/`          |
| Codec          | `codec.JSONCodec` + `event.DecodePayload[T]` + `event.NewEvents`                                        | Eliminates all manual json.Marshal/Unmarshal           |
| Projection     | Direct `bus.SubscribeAll` (sync) + `projection.Runner` (Turso replay), SQL checkpoints                  | Replay from store on restart + live subscription       |
| Snapshots      | `SQLiteSnapshotStore` (Turso) + `MemorySnapshotStore` (memory) + `snapshot.EveryNEvents`                | Caps replay cost, persists across restarts             |
| Correlation    | `event.WithCorrelationID` in `SyncItems`                                                                | Unique per sync run for debugging                      |
| Logging        | `middleware.EventLogging` via charm log adapter                                                         | Structured logging of all domain events                |
| Error taxonomy | `go-error-family` constructors (intrinsic classification) + `event.IsRetryable`                         | Smart retry classification for provider errors         |
| Version        | `event.Version` with `Increment()`, `Add()`                                                             | Phantom type safety — no `int()` casts                 |

## Session 8 — 2026-06-03: go-cqrs-lite v2 Migration

### go-cqrs-lite v2 Migration

- ✅ **go-cqrs-lite v2 migration**: All 11 module imports updated from v1 (`core/*`, `memory`, etc.) to v2 sub-modules (`event/v2`, `command/v2`, `decider/v2`, etc.)
- ✅ **go-error-family v0.2.0 → v0.3.0**: Required by go-cqrs-lite v2
- ✅ **go-branded-id v0.1.0 → v0.3.0**: Already at latest, now explicitly tracked
- ✅ **Outbox pattern removed**: `event.OutboxPublisher`, `decider.WithOutbox`, `cqrsstorage.NewSQLiteOutbox`, `cqrsstorage.NewSQLTransactionalStore` all removed — go-cqrs-lite v2 removed outbox support
- ✅ **Turso sync removed**: `cqrsstorage.OpenTursoSync`, `cqrsstorage.TursoSyncDB`, `Push()`/`Pull()` methods removed — remote sync is no longer in go-cqrs-lite storage
- ✅ **SQLite driver migrated**: `turso.tech/database/tursogo` → `modernc.org/sqlite` (pure-Go, no CGo)
- ✅ **`event.JSONCodec` → `codec.JSONCodec`**: Codec type moved to dedicated `codec/v2` module
- ✅ **`event.EveryNEvents` → `snapshot.EveryNEvents`**: Snapshot strategy moved to `snapshot/v2` module
- ✅ **`cqrsstorage.OpenTurso` → `cqrsstorage.OpenSQLite`**: Renamed in v2 storage module
- ✅ **`event.InMemoryRunner` removed**: Unified projection — direct `bus.SubscribeAll` for synchronous event delivery + `projection.Runner` for journal replay
- ✅ **CLI flags cleaned**: `--push` and `--pull` flags removed from github-sync example
- ✅ **Tests migrated**: All test files updated, `pushpull_test.go` removed, `RemoteStore_InvalidURL` test removed
- ✅ **go.work updated**: Removed `core` and `saga` entries, added all v2 sub-modules (`event`, `command`, `decider`, `query`, `id`, `dispatcher`, `snapshot`, `codec`, `storage`, `memory`, `middleware`, `projection`)
- ✅ **All 9 packages passing**: 235 tests green

### Naming Cleanup (Session 8 continued)

- ✅ **turso→sqlite rename**: All internal references renamed across 11 files. `TursoReadModel`→`SQLiteReadModel`, `backendTurso`→`backendSQLite`, all test names updated. Files renamed via `git mv`.
- ✅ **Dead config removed**: `CQRSConfig.RemoteURL`/`AuthToken` fields removed. `AppConfig.RemoteURL`/`AuthToken` env vars removed. CLI wiring simplified.
- ✅ **Error templates updated**: All error messages reference `sqlite` not `turso`.
- ✅ **Docs updated**: README, FEATURES, ROADMAP all reflect v2 migration and sqlite rename. Removed Outbox, Push/Pull, Turso references.
- ✅ **go-error-family v0.3 reviewed**: New `Compose`, `HandleErrorWithContext`, `HandleErrorDetailedWithConfig` APIs available. No breaking changes — our usage is forward-compatible.

### Not Yet Adopted

| Module                 | Reason                                        |
| ---------------------- | --------------------------------------------- |
| `otel/v2`              | Only `Name` constant — no real API            |
| `signing/v2`           | Local-first sync doesn't need Ed25519 yet     |
| `schema/v2`            | Only 1 schema version                         |
| `catalog/v2`           | AsyncAPI/D2 generation not critical           |
| `pebble/v2`            | Alternative storage — no immediate need       |
| `watermill/v2`         | Message broker — no immediate need            |
| `event.AggregateRef`   | Our type+id pattern is already clear          |
| `SyncItemState option` | `nil *provider.Item` + `IsNew()` is idiomatic |

### Breaking Changes

`core/*` → `*/v2` sub-modules. `event.JSONCodec` → `codec/v2`. `event.EveryNEvents` → `snapshot/v2`. `InMemoryRunner` → direct `bus.SubscribeAll`. `OutboxPublisher` removed. `OpenTurso` → `OpenSQLite`. `tursogo` → `modernc.org/sqlite`. `Push/Pull` removed.

## Session 9 — 2026-06-03: Correctness Improvements

- ✅ **`scanItems` rows.Err() check**: Could silently return partial results
- ✅ **`Fold(ItemDeleted)` nils Item**: Prevents stale data reads from deleted aggregates
- ✅ **SQLite indexes**: `idx_sync_items_repo_name`, `idx_sync_items_type_created` composite
- ✅ **`parseItemID` panic→error**: `id.ParseItemID` replaces `MustParseItemID` in event replay
- ✅ **`itemFromPayload` validation**: Calls `Item.Validate()` on reconstructed items
- ✅ **`Item.String()`**: `fmt.Stringer` for structured logging
- ✅ **Projector error-path tests**: Corrupt ItemID, missing required fields
- ✅ **`TestItem_JSONRoundTrip`**: All fields through marshal/unmarshal
- ✅ **`id.ParseItemID` tests**: Success + error cases

## Session 6 — 2026-05-29: CRDT Conflict Resolution Integration

### Completed Improvements

- ✅ **CRDT wired as pluggable conflict resolution strategy**: `crdt.ConflictResolver[*provider.Item]` is now injected into `DecideSync` via `CQRSConfig.ConflictResolver`. Default (nil) = remote-wins = backward compatible.
- ✅ **`ActionConflictLocal` added**: New `SyncAction` for when the resolver picks the local item over the remote. `ConflictAwareSyncer` handles both `ActionConflictRemote` and `ActionConflictLocal`.
- ✅ **`resolveConflict` helper**: Extracts conflict resolution logic from `DecideSync`. Creates `crdt.Conflict` with empty vector clocks (falls through to timestamp comparison in `LWWResolver`). Falls back to remote-wins on resolver error.
- ✅ **`conflictMeta` struct**: Replaces the boolean `isConflict` + `localUpdatedAt` parameters in `syncEvents` with a clean struct carrying `localUpdatedAt`, `remoteUpdatedAt`, and `winner`.
- ✅ **`classifyAction` updated**: Now accepts `conflictWinner string` to distinguish `ActionConflictLocal` from `ActionConflictRemote`.
- ✅ **`wireCommandDispatcher` passes resolver**: `handleSyncItem` closure captures the resolver and passes it to `DecideSync`.
- ✅ **13 new tests**: 5 decider-level tests (custom resolver remote/local/error, LWWResolver remote/local newer), 2 stack-level integration tests (LWW with local newer / remote newer), 1 classifyAction test (conflict_local case).
- ✅ **Backward compatible**: All existing callers pass `nil` resolver, which preserves the current remote-wins behavior.

### Architecture

```
CQRSConfig.ConflictResolver (nil = remote-wins default)
    ↓
CQRSStack.conflictResolver
    ↓
DecideSync(item, resolver, opts...)
    ↓ HasChanged(state.Item, item) = true
resolveConflict(resolver, state.Item, item)
    ↓ crdt.ConflictResolver[*provider.Item].Resolve()
syncEvents(winner, ..., conflictMeta{winner: "local"|"remote"}, ...)
    ↓
classifyAction(err, count, wasNew, conflictWinner)
    ↓
ActionConflictLocal | ActionConflictRemote
```

### Dependency Flow

```
cqrs → crdt (for ConflictResolver[T], Conflict[T], VectorClock)
cqrs → sync (for SyncAction, SyncSummary)
crdt → go-error-family (for error types only)
```

No circular dependencies. The CRDT package is now a real dependency of the CQRS layer.

## Unix-Style Modularity (Session 4 — 2026-05-25)

### Completed Improvements

- ✅ **`SyncStore` interface seam**: `pkg/sync/` defines `SyncStore` interface; `*cqrs.CQRSStack` implements it via adapter methods. `pkg/sync/` has **zero imports on `pkg/cqrs/`**.
- ✅ **`SyncAction`/`ItemSyncResult` moved to seam**: These types now live in `pkg/sync/` (the architectural boundary), not `pkg/cqrs/`. The `classifyAction` function in `cqrs` returns `synclib.SyncAction`.
- ✅ **`IsRetryable` moved to `pkg/errors/`**: Retry-classification is a generic concern, not event-sourcing. Delegates to `errorfamily.IsRetryable()`.
- ✅ **`pkg/testhelpers/` deleted**: Only used by github provider tests. Helpers moved to `pkg/providers/github/testhelpers_test.go` as unexported test helpers.
- ✅ **`sync_test.go` uses mock `SyncStore`**: No import cycle — sync tests use a `mockSyncStore` struct, not `*cqrs.CQRSStack`. Integration tests for conflict-aware sync live in `pkg/cqrs/`.
- ✅ **go-cqrs-lite API drift fixed**: `command.Core→BasicCommand`, `query.Core→BasicQuery`, `event.Core→ImmutableEvent`, `NewCheckpointStore→NewMemoryCheckpointStore`.

## Session 10 — 2026-06-10: Architecture Improvement Plan

Executed all 8 items from `docs/planning/2026-06-10_ARCHITECTURE_IMPROVEMENT_PLAN.html`:

### Critical

- ✅ **LS-1: SyncItems through command pipeline**: `SyncItems` now dispatches per-item through `CommandDispatcher` instead of calling `Repo.Execute` directly. New `SyncOutcome` type + `decideWithOutcome` captures domain semantics. `SyncItemCommand.Options` field passes correlation IDs through the pipeline.
- ✅ **LS-2: Compile-time SyncStore assertion**: `var _ synclib.SyncStore = (*CQRSStack)(nil)` added to `stack.go`.

### Structural

- ✅ **LS-3: CRDT doc.go updated**: `pkg/crdt/doc.go` now clearly marks "Active Types" vs "Future Types (planned for multi-node sync)".
- ✅ **LS-4: Consistent not-found semantics**: `MemoryReadModel.Get()` now returns `(nil, pkgerrors.ErrNotFound)` instead of `(nil, nil)`, matching `SQLiteReadModel.Get()`.

### Placement

- ✅ **LS-5: NewServer simplified**: `api.NewServer(syncer, logger)` — no longer takes redundant `SyncStore` param. `Syncer.Store()` getter added.
- ✅ **LS-6: Duplicate GetTypes removed**: `CQRSStack.GetTypes()` removed; callers use `GetItemTypes()`.
- ✅ **LS-7: Dead raw_json removed**: `raw_json` column, `scannedItem.rawJSON` field, dead `ToItemView` function all removed from SQLite read model.
- ✅ **LS-8: Runner errors logged**: `runner.Run(ctx)` errors logged via `slog.Error` instead of silently discarded.

### Upstream API Fixes (go-cqrs-lite)

- `cqrsid.MustParseAggregateID` → `cqrsid.ParseAggregateID` (MustParse removed)
- `command.MustNew` → `command.New` with `mustNewCommand` helper (MustNew removed)
- `query.MustNew` → `query.New` with `mustNewQuery` helper (MustNew removed)

### New Files

- `pkg/cqrs/sync_outcome.go` — `SyncOutcome` type, context helpers, `decideWithOutcome` wrapper

## Lint Status

golangci-lint v2 reports **0 issues** across all 9 packages. Config is strict with 125+ linters enabled.
