# FEATURES.md — go-localsync

**Updated:** 2026-07-22

> The SDK is a **pure contract library**. It defines the `Provider` interface, the CQRS sync engine, and CRDT primitives — but ships **no provider implementations and no CLI binary**. The reference consumer application — GitHub provider + CLI — lives in [`github.com/larsartmann/github-local-sync`](https://github.com/larsartmann/github-local-sync). New providers (GitLab, Jira, …) are built the same way, in their own consumer apps.
>
> **Scope boundary (see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md)):** go-localsync is a **single-aggregate, pull-only, flat-Item sync engine**. The domain model is provider-agnostic (`Attributes map[string]string` carries provider-specific content; see [ADR-0007](docs/adr/0007-de-githubify-domain-model.md)), the event vocabulary is fixed at three events, and there is one flat projection. It is **not** a generic multi-aggregate event-sourcing framework — generalising it was considered and deferred. For push-driven, multi-aggregate consumers (e.g. DiscordSync), share `go-cqrs-lite v4` directly instead.

## Legend

| Status               | Meaning                                                       |
| -------------------- | ------------------------------------------------------------- |
| FULLY_FUNCTIONAL     | Works correctly, tested, production-ready                     |
| PARTIALLY_FUNCTIONAL | Implemented but has known gaps or incomplete coverage         |
| BROKEN               | Code exists but does not work (references deleted code, etc.) |
| NOT_APPLICABLE       | Explicitly out of scope, removed, or superseded               |

---

## Core Architecture

| # | Feature                     | Status           | Package    | Description                                                                                                                                                                                                        |
| - | --------------------------- | ---------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1 | CQRS Stack                  | FULLY_FUNCTIONAL | `pkg/cqrs` | Full event-sourced architecture: event store, bus, decider repository, read model, projection. Wired via `NewCQRSStack` with backend selection.                                                                    |
| 2 | Decider Pattern             | FULLY_FUNCTIONAL | `pkg/cqrs` | Pure `Apply`/`DecideSync`/`DecideTombstone` with `SyncItemState{Item *model.Item}`. Single authority for all state transitions.                                                                                    |
| 3 | Event Sourcing              | FULLY_FUNCTIONAL | `pkg/cqrs` | 3 domain events: `ItemSynced`, `ItemConflictFound`, `ItemTombstoned`. All state changes via events. No legacy CRUD.                                                                                                |
| 4 | Projection                  | FULLY_FUNCTIONAL | `pkg/cqrs` | `Projector` implements `projection.Projection` (ADR-0037). Live delivery via `bus.SubscribeAll`; resilient SQLite catch-up via `projectionhost.Host` (checkpoint persistence, crash auto-restart, DLQ — ADR-0006). |
| 5 | Deterministic Aggregate IDs | FULLY_FUNCTIONAL | `pkg/cqrs` | SHA256→hex from (source, sourceID) with `sync.Map` cache. Same inputs always produce the same ID.                                                                                                                  |
| 6 | JSON Codec                  | FULLY_FUNCTIONAL | `pkg/cqrs` | `codec.JSONCodec` + `DecodePayload[T]` + `NewEvents` for type-safe event serialization. No manual json.Marshal/Unmarshal.                                                                                          |
| 7 | provider→model Adapter      | FULLY_FUNCTIONAL | `pkg/cqrs` | `item_adapter.go` converts `*provider.Item` (DTO) → `*model.Item` (domain entity) at the boundary. Decider, read model, events, and resolver all use `*model.Item`.                                                |

## Storage Backends

| #  | Feature           | Status           | Package    | Description                                                                                                                                             |
| -- | ----------------- | ---------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 8  | Memory Backend    | FULLY_FUNCTIONAL | `pkg/cqrs` | In-memory event store, read model, snapshot store via `storage/memory/v4`. Default for testing and development.                                         |
| 9  | SQLite Backend    | FULLY_FUNCTIONAL | `pkg/cqrs` | Local SQLite file via `go-cqrs-lite/storage/v4`. Event store + read model + snapshots in a single `*sql.DB`. Pure-Go via `modernc.org/sqlite` (no CGo). |
| 10 | Backend Selection | FULLY_FUNCTIONAL | `pkg/cqrs` | `CQRSConfig.Backend` selects `"memory"` or `"sqlite"` at construction time. Factory in `store_factory.go`.                                              |

## Sync Engine

| #  | Feature             | Status           | Package        | Description                                                                                                                                               |
| -- | ------------------- | ---------------- | -------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 11 | Full Sync           | FULLY_FUNCTIONAL | `pkg/sync`     | `Syncer.Sync()` fetches all pages from provider, validates items, syncs via CQRS stack.                                                                   |
| 12 | Incremental Sync    | FULLY_FUNCTIONAL | `pkg/sync`     | `Syncer.SyncIncremental()` uses latest item's `CreatedAt` as cutoff. Falls back to full sync on empty database.                                           |
| 13 | Conflict-Aware Sync | FULLY_FUNCTIONAL | `pkg/sync`     | `ConflictAwareSyncer.SyncWithConflictDetection()` delegates to CQRS decider. Reports conflicts, upserts, skips, errors. Supports pluggable CRDT resolver. |
| 14 | Item Validation     | FULLY_FUNCTIONAL | `pkg/provider` | `Item.Validate()` checks required fields (ExternalID, Source, Type, CreatedAt). Invalid items counted in error metrics.                                   |
| 15 | Progress Callbacks  | FULLY_FUNCTIONAL | `pkg/sync`     | `SyncOptions.OnProgress` callback for real-time progress reporting during sync.                                                                           |
| 16 | Stats Query         | FULLY_FUNCTIONAL | `pkg/sync`     | `Syncer.GetStats()` returns total count, type list, and per-type counts from read model.                                                                  |

## Conflict Detection & Resolution

| #  | Feature                   | Status           | Package    | Description                                                                                                                                                                                                     |
| -- | ------------------------- | ---------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 17 | Change Detection          | FULLY_FUNCTIONAL | `pkg/cqrs` | `hasChanged()` is provider-agnostic (ADR-0007): `ContentHash` (SHA-256 of the provider raw payload) is the primary signal, with `UpdatedAt` and `Type` as fallbacks. Uses `.Equal()` for timestamps (not `!=`). |
| 18 | Conflict Events           | FULLY_FUNCTIONAL | `pkg/cqrs` | `DecideSync` emits `ItemConflictFound` + `ItemSynced` when item has changed. Conflict payload includes local/remote timestamps and winner.                                                                      |
| 19 | Remote-Wins LWW (Default) | FULLY_FUNCTIONAL | `pkg/cqrs` | Default strategy when no resolver configured. Incoming item always overwrites local state.                                                                                                                      |
| 20 | Pluggable CRDT Resolution | FULLY_FUNCTIONAL | `pkg/cqrs` | `CQRSConfig.ConflictResolver` accepts any `crdt.ConflictResolver[*model.Item]`. Injected into `DecideSync` via `resolveConflict` helper.                                                                        |
| 21 | Action Classification     | FULLY_FUNCTIONAL | `pkg/cqrs` | `classifyAction()` categorizes results as Created, Updated, ConflictRemote, ConflictLocal, Unchanged, or Error.                                                                                                 |
| 22 | Exported Winner Constants | FULLY_FUNCTIONAL | `pkg/cqrs` | `ConflictWinnerRemote`/`ConflictWinnerLocal` constants + `ParseConflictWinner` (unknown values default to remote-wins) for safe payload→enum decoding.                                                          |
| 23 | LWW Resolver              | FULLY_FUNCTIONAL | `pkg/crdt` | `LWWResolver[T]` picks item with later timestamp. Used for `*model.Item` conflict resolution.                                                                                                                   |
| 24 | Vector Clock              | NOT_APPLICABLE   | `pkg/crdt` | **Removed.** A single-writer pull mirror has no second writer and no causal ordering to track. Vector clocks were deleted in the tombstone/scope refactor.                                                      |
| 25 | CRDT Operations           | NOT_APPLICABLE   | `pkg/crdt` | **Removed.** `Operation[T]`, `SyncMessage`, and the distributed-sync protocol types were deleted — out of scope for a single-writer pull mirror.                                                                |
| 26 | Conflict Types            | FULLY_FUNCTIONAL | `pkg/crdt` | `Conflict[T]` with Local, Remote, Timestamp. Passed to `ConflictResolver.Resolve()`. No causal metadata (single-writer).                                                                                        |

## Provider System

| #  | Feature            | Status           | Package        | Description                                                                                                                                                                                                                                                |
| -- | ------------------ | ---------------- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 27 | Provider Interface | FULLY_FUNCTIONAL | `pkg/provider` | Generic `Provider` interface: `Name()`, `Fetch()`, `FetchAll()`, `GetRateLimit()`, plus config types (`RateLimitConfig`, `RetryConfig`, `FetchConfig`) and `RateLimitCache`. The SDK defines the contract only — concrete providers live in consumer apps. |
| 28 | Rate Limit Cache   | FULLY_FUNCTIONAL | `pkg/provider` | `RateLimitCache` caches rate-limit info between calls to avoid redundant API requests. Concurrency-safe.                                                                                                                                                   |

## Read Model

| #  | Feature              | Status           | Package          | Description                                                                                                                                                              |
| -- | -------------------- | ---------------- | ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 29 | In-Memory Read Model | FULLY_FUNCTIONAL | `pkg/cqrs`       | `MemoryReadModel` with concurrent-safe `sync.RWMutex`. Filter by type, attribute key-value, source, since, include-tombstoned. Pagination support. Stores `*model.Item`. |
| 30 | SQLite Read Model    | FULLY_FUNCTIONAL | `pkg/cqrs`       | `SQLiteReadModel` with parameterized queries; indexes on `type`, `created_at`, and `(type, created_at)`. Same filter/pagination API as memory.                           |
| 31 | Read Model Interface | FULLY_FUNCTIONAL | `pkg/cqrs`       | `ReadModel` interface (embeds `model.ItemReader`): `Get`, `Upsert`, `Delete`, `Close`. Both backends implement it.                                                       |
| 32 | Item Filtering       | FULLY_FUNCTIONAL | `pkg/data/model` | `ItemFilter` with Type, Attributes (key-value), Source, Since, Limit, Offset, IncludeTombstoned. Builder pattern via `WithType`, `WithAttribute`, etc.                   |

## Command & Query Dispatch

| #  | Feature                       | Status           | Package    | Description                                                                                                                                                                                             |
| -- | ----------------------------- | ---------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 33 | Command Dispatcher            | FULLY_FUNCTIONAL | `pkg/cqrs` | `command.Dispatcher` with typed `SyncItemCommand`/`TombstoneItemCommand`. Dispatched through `CQRSStack.SyncItem()`/`TombstoneItem()`.                                                                  |
| 34 | Query Dispatcher (read side)  | NOT_APPLICABLE   | `pkg/cqrs` | **Removed by design.** There is no query dispatcher — reads call the `ReadModel` directly (see note in `stack_adapters.go`). The command side stays dispatched for logging/retry/validation middleware. |
| 35 | Command Validation Middleware | FULLY_FUNCTIONAL | `pkg/cqrs` | Validates commands before dispatch (nil item check, empty source check).                                                                                                                                |
| 36 | Command/Query Logging         | FULLY_FUNCTIONAL | `pkg/cqrs` | Logs all command/query dispatches with type, duration, and error status.                                                                                                                                |

## Event Infrastructure

| #  | Feature          | Status           | Package    | Description                                                                                                                                                          |
| -- | ---------------- | ---------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 37 | Event Logging MW | FULLY_FUNCTIONAL | `pkg/cqrs` | `middleware.EventLogging` (from go-cqrs-lite v4) via charm log adapter. Structured logging of all domain events on the bus. Replaces the former hand-rolled adapter. |
| 38 | Correlation IDs  | FULLY_FUNCTIONAL | `pkg/cqrs` | `SyncItems()` generates unique `CorrelationID` per sync run via `event.WithCorrelationID`. Cross-event tracing.                                                      |
| 39 | Snapshots        | FULLY_FUNCTIONAL | `pkg/cqrs` | `SQLiteSnapshotStore` (sqlite) + `MemorySnapshotStore` (memory). `EveryNEvents(10)` strategy. Caps replay cost.                                                      |

## Type System

| #  | Feature           | Status           | Package           | Description                                                                                                                                                                 |
| -- | ----------------- | ---------------- | ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 40 | Branded IDs       | FULLY_FUNCTIONAL | `pkg/id`          | 6 phantom-type IDs: `ItemID` (ULID), `ExternalID`, `ProviderID`, `ActorLogin`, `RepoID`, `EventTypeID` (all string). Compile-time type safety via go-branded-id.            |
| 41 | Item ID (ULID)    | FULLY_FUNCTIONAL | `pkg/id`          | `ItemID` uses ULID for sortable, unique internal identifiers. `NewItemID()` generates with crypto/rand.                                                                     |
| 42 | Structured Errors | FULLY_FUNCTIONAL | `pkg/errors`      | Sentinel errors via `go-error-family` constructors with intrinsic classification (Rejection, Transient, Infrastructure). `WithDetail`, `Wrap`, `Wrapf` preserve family.     |
| 43 | Domain Model      | FULLY_FUNCTIONAL | `pkg/data/model`  | `Item` (domain entity with `SchemaVersion`), `Key`, `ItemFilter`, `ItemReader` interface, `Validate()`. The single canonical model used across decider, read model, events. |
| 44 | Schema Versioning | FULLY_FUNCTIONAL | `pkg/data/schema` | `Version` (V1/V2/V3; V3 = de-githubify) with `CurrentVersion()`, `Valid()`, `Int()`. Carried on every item for forward event migration (upcasting).                         |

## HTTP API

| #  | Feature     | Status           | Package   | Description                                                                                                                                                           |
| -- | ----------- | ---------------- | --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 45 | API Server  | FULLY_FUNCTIONAL | `pkg/api` | Huma v2 with `humago` stdlib adapter. Auto-generated OpenAPI 3 spec. Split into server.go + dto.go + handlers.go.                                                     |
| 46 | GET /items  | FULLY_FUNCTIONAL | `pkg/api` | Filterable list with query params: `type`, `source`, `since`, `limit`, `offset`. Returns JSON array. (Provider-specific attributes are not first-class query params.) |
| 47 | GET /stats  | FULLY_FUNCTIONAL | `pkg/api` | Returns total count, distinct types, and per-type breakdown.                                                                                                          |
| 48 | POST /sync  | FULLY_FUNCTIONAL | `pkg/api` | Triggers a sync run. Supports `user`, `pages`, `incremental` body params. Returns `SyncSummary`.                                                                      |
| 49 | GET /health | FULLY_FUNCTIONAL | `pkg/api` | Health check endpoint. Returns 200 with `status: "ok"`.                                                                                                               |

## Build System

| #  | Feature   | Status           | Package     | Description                                                                                                           |
| -- | --------- | ---------------- | ----------- | --------------------------------------------------------------------------------------------------------------------- |
| 50 | Nix Flake | FULLY_FUNCTIONAL | `flake.nix` | Dev shell (Go 1.26, golangci-lint) + `buildGoModule` package derivation. Vendored private deps (`vendorHash = null`). |
| 51 | No CGO    | FULLY_FUNCTIONAL | project     | Pure Go via `modernc.org/sqlite`. Builds with `CGO_ENABLED=0`.                                                        |

## CI/CD

| #  | Feature               | Status           | Package             | Description                                                                                                                               |
| -- | --------------------- | ---------------- | ------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| 52 | GitHub Actions CI     | FULLY_FUNCTIONAL | `.github/workflows` | `test` (race + coverage) + `lint` (vet + golangci-lint) + `build` (cross-platform compile verify) jobs. All pass on master.               |
| 53 | Cross-Platform Builds | FULLY_FUNCTIONAL | `.github/workflows` | Build matrix (linux/darwin × amd64/arm64) verifies the library compiles on all target platforms. A pure library has no binary to ship.    |
| 54 | Automated Releases    | FULLY_FUNCTIONAL | `.github/workflows` | Tag-triggered release via `softprops/action-gh-release` generates release notes. Library releases ship source only (no binary artifacts). |

## Testing

| #  | Feature      | Status           | Package        | Description                                                                                                                                  |
| -- | ------------ | ---------------- | -------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| 55 | Test Suite   | FULLY_FUNCTIONAL | all            | 216 test functions across 10 packages, all passing. Run: `go test ./... -count=1`.                                                           |
| 56 | Test Helpers | FULLY_FUNCTIONAL | `pkg/testutil` | Shared test utilities: `MockProvider`, `SyncStore` test double, `BuildPairs`, assertions. (Provider-specific helpers live in consumer apps.) |

## Quality

| #  | Feature                | Status               | Package           | Description                                                                                                                                                                                                                                                                                                               |
| -- | ---------------------- | -------------------- | ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 57 | Lint (Zero Issues)     | FULLY_FUNCTIONAL     | project           | golangci-lint v2 with `enable-all` (+ targeted `disable`/exclusions), 0 issues. Strict `.golangci.yml`.                                                                                                                                                                                                                   |
| 58 | GitHub Events Provider | PARTIALLY_FUNCTIONAL | `provider/github` | Optional nested module: `provider.Provider` over `go-github-kit` v0.2.0 (auth, rate gate, retry; `Fetch`/`FetchAll`/`GetRateLimit`; error-family mapping incl. `ErrProviderUnavailable`). Extracted from github-local-sync, suite ported. Unreleased: parent pin is a master pseudo-version and no module tag exists yet. |

---

## Honest Assessment

### What Works Well

- CQRS architecture is clean and complete — no legacy CRUD, no split brains
- Dual backend (memory/sqlite) with identical `ReadModel` API
- Provider abstraction is genuinely pluggable — new providers only implement the interface
- Error taxonomy gives smart retry classification
- Idempotent sync — deterministic aggregate IDs prevent duplicates
- Projection via synchronous `bus.SubscribeAll` (live) + `projectionhost.Host` (managed catch-up with checkpoint, crash-restart, DLQ — ADR-0006)
- 216 tests with good coverage across all packages
- Pluggable CRDT conflict resolution — `LWWResolver` is default, any `ConflictResolver[T]` works
- Clear DTO/domain boundary: `provider.Item` (DTO) → `model.Item` (domain entity) via `item_adapter.go`

### Known Gaps

| Area                   | Issue                                                                                                                                                                                                                | Impact                                                                                                                                      |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| Contract-only SDK      | No provider or CLI shipped in-repo. GitHub provider + CLI live in [`github-local-sync`](https://github.com/larsartmann/github-local-sync).                                                                           | Consumers implement their own provider against the interface.                                                                               |
| No observability       | No OpenTelemetry, no metrics, no tracing                                                                                                                                                                             | Production debugging requires log spelunking                                                                                                |
| API has no auth        | HTTP API has no authentication middleware                                                                                                                                                                            | Not safe to expose on a network                                                                                                             |
| No data export         | No JSON/CSV export of stored events                                                                                                                                                                                  | Cannot export for analysis in external tools                                                                                                |
| Single-aggregate scope | One `sync_item` aggregate, three fixed events, one flat projection, pull-only `Provider`+`Syncer`. Domain model is provider-agnostic (`Attributes` map; see [ADR-0007](docs/adr/0007-de-githubify-domain-model.md)). | Not suitable for multi-aggregate / push consumers. Accepted scope per [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md). |
