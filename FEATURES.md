# FEATURES.md — go-localsync

**Updated:** 2026-05-28 (session 5)

## Legend

| Status               | Meaning                                               |
| -------------------- | ----------------------------------------------------- |
| FULLY_FUNCTIONAL     | Works correctly, tested, production-ready             |
| PARTIALLY_FUNCTIONAL | Implemented but has known gaps or incomplete coverage |
| PLANNED              | Listed in roadmap/planning docs but no code exists    |
| NOT_APPLICABLE       | Explicitly out of scope or deprecated                 |

---

## Core Architecture

| #   | Feature                     | Status           | Package    | Description                                                                                                                                                                      |
| --- | --------------------------- | ---------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | CQRS Stack                  | FULLY_FUNCTIONAL | `pkg/cqrs` | Full event-sourced architecture: event store, bus, decider repository, read model, projection. Wired via `NewCQRSStack` with backend selection.                                  |
| 2   | Decider Pattern             | FULLY_FUNCTIONAL | `pkg/cqrs` | Pure `Fold`/`DecideSync`/`DecideDelete` with `SyncItemState`. Single authority for all state transitions.                                                                        |
| 3   | Event Sourcing              | FULLY_FUNCTIONAL | `pkg/cqrs` | 3 domain events: `ItemSynced`, `ItemConflictFound`, `ItemDeleted`. All state changes via events. No legacy CRUD.                                                                 |
| 4   | Projection                  | FULLY_FUNCTIONAL | `pkg/cqrs` | `Projector` implements `event.Projection`. Updates read model from events. Dual runner: `InMemoryRunner` (memory) + `projection.Runner` (Turso) with replay + live subscription. |
| 5   | Deterministic Aggregate IDs | FULLY_FUNCTIONAL | `pkg/cqrs` | SHA256→hex from (source, sourceID) with `sync.Map` cache. Same inputs always produce the same ID.                                                                                |
| 6   | JSON Codec                  | FULLY_FUNCTIONAL | `pkg/cqrs` | `event.JSONCodec` + `DecodePayload[T]` + `NewEvents` for type-safe event serialization. No manual json.Marshal/Unmarshal.                                                        |

## Storage Backends

| #   | Feature                      | Status           | Package    | Description                                                                                                                    |
| --- | ---------------------------- | ---------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------ |
| 7   | Memory Backend               | FULLY_FUNCTIONAL | `pkg/cqrs` | In-memory event store, bus, read model, snapshot store, checkpoint store. Default for testing and development.                 |
| 8   | Turso/SQLite Backend (Local) | FULLY_FUNCTIONAL | `pkg/cqrs` | Local SQLite file via `go-cqrs-lite/storage`. Event store + read model + outbox + snapshots + checkpoints in single `*sql.DB`. |
| 9   | Turso Backend (Remote Sync)  | FULLY_FUNCTIONAL | `pkg/cqrs` | Push/Pull to remote Turso database via `OpenTursoSync`. Multi-device sync with embedded replica pattern.                       |
| 10  | Backend Selection            | FULLY_FUNCTIONAL | `pkg/cqrs` | `CQRSConfig.Backend` selects memory or turso at construction time. Factory pattern in `store_factory.go`.                      |

## Sync Engine

| #   | Feature             | Status           | Package        | Description                                                                                                                                       |
| --- | ------------------- | ---------------- | -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| 11  | Full Sync           | FULLY_FUNCTIONAL | `pkg/sync`     | `Syncer.Sync()` fetches all pages from provider, validates items, syncs via CQRS stack.                                                           |
| 12  | Incremental Sync    | FULLY_FUNCTIONAL | `pkg/sync`     | `Syncer.SyncIncremental()` uses latest item's `CreatedAt` as cutoff. Falls back to full sync on empty database.                                   |
| 13  | Conflict-Aware Sync | FULLY_FUNCTIONAL | `pkg/sync`     | `ConflictAwareSyncer.SyncWithConflictDetection()` delegates to CQRS decider. Reports conflicts, upserts, skips, errors. Remote-wins LWW strategy. |
| 14  | Item Validation     | FULLY_FUNCTIONAL | `pkg/provider` | `Item.Validate()` checks required fields (ExternalID, Source, Type, CreatedAt). Invalid items counted in error metrics.                           |
| 15  | Progress Callbacks  | FULLY_FUNCTIONAL | `pkg/sync`     | `SyncOptions.OnProgress` callback for real-time progress reporting during sync.                                                                   |
| 16  | Stats Query         | FULLY_FUNCTIONAL | `pkg/sync`     | `Syncer.GetStats()` returns total count, type list, and per-type counts from read model.                                                          |

## Conflict Detection & Resolution

| #   | Feature               | Status           | Package    | Description                                                                                                                                |
| --- | --------------------- | ---------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| 17  | Change Detection      | FULLY_FUNCTIONAL | `pkg/cqrs` | `HasChanged()` compares UpdatedAt, Type, ActorLogin, RepoName, RepoURL. Uses `.Equal()` for timestamps (not `!=`).                         |
| 18  | Conflict Events       | FULLY_FUNCTIONAL | `pkg/cqrs` | `DecideSync` emits `ItemConflictFound` + `ItemSynced` when item has changed. Conflict payload includes local/remote timestamps and winner. |
| 19  | Remote-Wins LWW       | FULLY_FUNCTIONAL | `pkg/cqrs` | Hard-coded remote-wins strategy. On conflict, incoming item always overwrites local state.                                                 |
| 20  | Action Classification | FULLY_FUNCTIONAL | `pkg/cqrs` | `classifyAction()` categorizes results as Created, Updated, ConflictRemote, Unchanged, or Error.                                           |

## Provider System

| #   | Feature                     | Status           | Package                | Description                                                                                                          |
| --- | --------------------------- | ---------------- | ---------------------- | -------------------------------------------------------------------------------------------------------------------- |
| 21  | Provider Interface          | FULLY_FUNCTIONAL | `pkg/provider`         | Generic `Provider` interface: `Name()`, `Fetch()`, `FetchAll()`, `GetRateLimit()`. Any data source can implement it. |
| 22  | GitHub Provider             | FULLY_FUNCTIONAL | `pkg/providers/github` | Full implementation: paginated user events, rate limiting, retry, error classification, raw JSON preservation.       |
| 23  | Rate Limiting               | FULLY_FUNCTIONAL | `pkg/providers/github` | Pre-fetch rate limit check with configurable `MinRemaining` and `MaxWait`. Waits for reset if within limits.         |
| 24  | Retry with Backoff          | FULLY_FUNCTIONAL | `pkg/providers/github` | Exponential backoff retry for 5xx and 429 errors. Configurable `MaxRetries`, `InitialBackoff`, `MaxBackoff`.         |
| 25  | GitHub Error Classification | FULLY_FUNCTIONAL | `pkg/providers/github` | Maps HTTP status codes to typed errors: 401→`ErrInvalidToken`, 403→`ErrRateLimited`, 404→`ErrUserNotFound`.          |

## Read Model

| #   | Feature                 | Status           | Package    | Description                                                                                                                   |
| --- | ----------------------- | ---------------- | ---------- | ----------------------------------------------------------------------------------------------------------------------------- |
| 26  | In-Memory Read Model    | FULLY_FUNCTIONAL | `pkg/cqrs` | `MemoryReadModel` with concurrent-safe `sync.RWMutex`. Filter by type, actor, repo, source, since. Pagination support.        |
| 27  | Turso/SQLite Read Model | FULLY_FUNCTIONAL | `pkg/cqrs` | `TursoReadModel` with parameterized queries, indexes (type, created_at, actor_login). Same filter/pagination API as memory.   |
| 28  | Read Model Interface    | FULLY_FUNCTIONAL | `pkg/cqrs` | `ReadModel` interface: `Get`, `List`, `Count`, `GetTypes`, `Upsert`, `Delete`, `Close`. Both backends implement it.           |
| 29  | Item Filtering          | FULLY_FUNCTIONAL | `pkg/cqrs` | `ItemFilter` with Type, ActorLogin, RepoName, Source, Since, Limit, Offset. Builder pattern via `WithType`, `WithLimit`, etc. |

## Command & Query Dispatch

| #   | Feature                       | Status           | Package    | Description                                                                                                                      |
| --- | ----------------------------- | ---------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 30  | Command Dispatcher            | FULLY_FUNCTIONAL | `pkg/cqrs` | `command.Dispatcher` with typed `SyncItemCommand`/`DeleteItemCommand`. Dispatched through `CQRSStack.SyncItem()`/`DeleteItem()`. |
| 31  | Query Dispatcher              | FULLY_FUNCTIONAL | `pkg/cqrs` | `query.Dispatcher` with typed queries: `ListItemsQuery`, `GetItemQuery`, `CountItemsQuery`, `GetTypesQuery`.                     |
| 32  | Command Validation Middleware | FULLY_FUNCTIONAL | `pkg/cqrs` | Validates commands before dispatch (nil item check, empty source check).                                                         |
| 33  | Command Logging Middleware    | FULLY_FUNCTIONAL | `pkg/cqrs` | Logs all command dispatches with type, duration, and error status.                                                               |
| 34  | Query Logging Middleware      | FULLY_FUNCTIONAL | `pkg/cqrs` | Logs all query dispatches with type, duration, and error status.                                                                 |

## Event Infrastructure

| #   | Feature                  | Status           | Package    | Description                                                                                                                                                |
| --- | ------------------------ | ---------------- | ---------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 35  | Outbox Pattern           | FULLY_FUNCTIONAL | `pkg/cqrs` | `decider.WithOutbox` for Turso backend. `SQLTransactionalStore` for atomic save+publish. `event.OutboxPublisher` polls at 1s interval with batch size 100. |
| 36  | Event Logging Middleware | FULLY_FUNCTIONAL | `pkg/cqrs` | `middleware.EventLogging` via `charmLogAdapter`. Structured logging of all domain events on the bus.                                                       |
| 37  | Correlation IDs          | FULLY_FUNCTIONAL | `pkg/cqrs` | `SyncItems()` generates unique `CorrelationID` per sync run via `event.WithCorrelationID`. Cross-event tracing.                                            |
| 38  | Snapshots                | FULLY_FUNCTIONAL | `pkg/cqrs` | `SQLiteSnapshotStore` (Turso) + `MemorySnapshotStore` (memory). `EveryNEvents(10)` strategy. Caps replay cost.                                             |
| 39  | Checkpoints              | FULLY_FUNCTIONAL | `pkg/cqrs` | `SQLiteCheckpointStore` (Turso) + `MemoryCheckpointStore` (memory). Tracks projection position for replay.                                                 |

## Type System

| #   | Feature           | Status           | Package      | Description                                                                                                                                                                                       |
| --- | ----------------- | ---------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 40  | Branded IDs       | FULLY_FUNCTIONAL | `pkg/id`     | 6 phantom-type IDs: `ItemID` (ULID), `ExternalID`, `ProviderID`, `ActorID`, `RepoID`, `EventTypeID` (all string). Compile-time type safety via go-branded-id.                                     |
| 41  | Item ID (ULID)    | FULLY_FUNCTIONAL | `pkg/id`     | `ItemID` uses ULID for sortable, unique internal identifiers. `NewItemID()` generates with crypto/rand.                                                                                           |
| 42  | Structured Errors | FULLY_FUNCTIONAL | `pkg/errors` | 9 sentinel errors via `go-error-family` constructors with intrinsic classification (Rejection, Transient, Infrastructure). `WithDetail`, `WithUserDetail`, `Wrap`, `Wrapf` preserve error family. |

## CLI / Example Application

| #   | Feature            | Status           | Package                    | Description                                                                                                             |
| --- | ------------------ | ---------------- | -------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| 43  | Example CLI        | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | Complete CLI with flag parsing, signal handling, graceful shutdown, domain-specific exit codes.                         |
| 44  | Environment Config | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | `AppConfig` via `caarlos0/env/v11`. All flags overridable by env vars. Defaults for backend, max pages, incremental.    |
| 45  | Exit Code Mapping  | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | BSD-style exit codes mapped from error taxonomy: rate-limited→75, invalid-token→64, not-found→65, etc.                  |
| 46  | Version Info       | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | `-version` flag prints version, commit, build date (injected via ldflags).                                              |
| 47  | Stats Display      | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | `-stats` flag shows total items and distinct event types, then exits.                                                   |
| 48  | Verbose Logging    | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | `-verbose` flag enables debug-level logging via `charm.land/log/v2`.                                                    |
| 49  | Signal Handling    | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | Catches SIGINT/SIGTERM, cancels context, logs shutdown.                                                                 |
| 50  | Push/Pull Flags    | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | `-push` and `-pull` flags trigger Turso remote sync before/after local sync.                                            |
| 51  | JSON Output        | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | `-json` flag outputs stats and sync results as structured JSON. Supports `-stats`, `-conflict-aware`, and regular sync. |
| 52  | HTTP Server Mode   | FULLY_FUNCTIONAL | `cmd/examples/github-sync` | `-server` flag runs HTTP API instead of one-off sync. `-port` configures listen address (default 8080).                |

## HTTP API

| #   | Feature           | Status           | Package    | Description                                                                                                              |
| --- | ----------------- | ---------------- | ---------- | ------------------------------------------------------------------------------------------------------------------------ |
| 53  | API Server        | FULLY_FUNCTIONAL | `pkg/api`  | Huma v2 with `humago` stdlib adapter. Auto-generated OpenAPI 3 spec.                                                     |
| 54  | GET /items        | FULLY_FUNCTIONAL | `pkg/api`  | Filterable list with query params: `type`, `actor`, `repo`, `source`, `since`, `limit`, `offset`. Returns JSON array.    |
| 55  | GET /stats        | FULLY_FUNCTIONAL | `pkg/api`  | Returns total count, distinct types, and per-type breakdown.                                                             |
| 56  | POST /sync        | FULLY_FUNCTIONAL | `pkg/api`  | Triggers a sync run. Supports `user`, `pages`, `incremental` body params. Returns `SyncSummary`.                         |
| 57  | GET /health       | FULLY_FUNCTIONAL | `pkg/api`  | Health check endpoint. Returns 200 with `status: "ok"`.                                                                  |

## CI/CD

| #   | Feature               | Status           | Package             | Description                                                                                                                  |
| --- | --------------------- | ---------------- | ------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| 51  | GitHub Actions CI     | FULLY_FUNCTIONAL | `.github/workflows` | 4-job pipeline: test (race + coverage), lint (go vet + golangci-lint), build (linux/darwin, amd64/arm64), release (on tags). |
| 52  | Cross-Platform Builds | FULLY_FUNCTIONAL | `.github/workflows` | Builds for linux/amd64, linux/arm64, darwin/arm64. No CGO required.                                                          |
| 53  | Automated Releases    | FULLY_FUNCTIONAL | `.github/workflows` | Tag-triggered releases via `softprops/action-gh-release` with auto-generated release notes.                                  |

## Testing

| #   | Feature      | Status           | Package           | Description                                                                                                                       |
| --- | ------------ | ---------------- | ----------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| 54  | Test Suite   | FULLY_FUNCTIONAL | all               | 222 test functions across 8 packages, ~82% overall coverage.                                                                      |
| 55  | Test Helpers | FULLY_FUNCTIONAL | `pkg/providers/github` | Unexported test helpers: `NewTestEvent`, `NewErrorTestServer`, `NewFailingThenSucceedingTestServer`, `TestRetryConfig`.        |

## Quality

| #   | Feature            | Status           | Package | Description                                                                          |
| --- | ------------------ | ---------------- | ------- | ------------------------------------------------------------------------------------ |
| 56  | Lint (Zero Issues) | FULLY_FUNCTIONAL | project | golangci-lint v2 with 125+ linters enabled, 0 issues. Strict `.golangci.yml` config. |
| 57  | No CGO             | FULLY_FUNCTIONAL | project | Pure Go via `modernc.org/sqlite`. Builds with `CGO_ENABLED=0`.                       |

---

## Honest Assessment

### What Works Well

- CQRS architecture is clean and complete — no legacy CRUD, no split brains
- Dual backend (memory/turso) with identical `ReadModel` API
- Provider abstraction is genuinely pluggable — new providers only implement the interface
- Error taxonomy gives meaningful CLI exit codes and smart retry classification
- Idempotent sync — deterministic aggregate IDs prevent duplicates
- Outbox pattern with atomic save+publish for crash-safe Turso persistence
- 222 tests with good coverage across all packages

### Known Gaps

| Area              | Issue                                                                                     | Impact                                                         |
| ----------------- | ----------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Conflict policy   | Hard-coded remote-wins. No configurable strategy.                                         | Users who need local-wins or manual resolution cannot use this |
| Single provider   | Only GitHub exists. No GitLab, Jira, etc.                                                 | SDK is generic but only one real provider                      |
| Multi-user sync   | CLI accepts one `-user` flag. No multi-user support.                                      | Cannot sync events for multiple users in one run               |
| Push/Pull testing | `CQRSStack.Push()` and `Pull()` have tests but no integration test with real Turso remote | Remote sync correctness relies on go-cqrs-lite's tests         |
| No daemon mode    | No cron/systemd integration for periodic sync                                             | Must run manually or wrap in external scheduler                |
| No data export    | No JSON/CSV export of stored events                                                       | Cannot export for analysis in external tools                   |
| Domain Language   | `docs/DOMAIN_LANGUAGE.md` is a template with no actual terms defined                      | New contributors lack domain vocabulary                        |
