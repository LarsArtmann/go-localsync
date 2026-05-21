# Go-LocalSync Agent Configuration

**Updated:** 2026-05-21

## Project Overview

Go-LocalSync is a generic synchronization SDK with a pluggable provider-based architecture. It uses event-sourced CQRS via go-cqrs-lite for state management, timestamp-based conflict detection, and branded IDs from go-branded-id for compile-time type safety.

## Architecture

| Package                     | Purpose                                                                                                         |
| --------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `pkg/cqrs/`                 | CQRS integration layer using go-cqrs-lite (Decider, ReadModel, Projector, CQRSStack)                            |
| `pkg/provider/`             | Core interfaces (`Provider`, `Item`, `FetchResult`, `RateLimitConfig`, `RetryConfig`)                           |
| `pkg/providers/github/`     | GitHub provider implementation (only provider currently)                                                        |
| `pkg/sync/`                 | `Syncer` (basic), `ConflictAwareSyncer` (timestamp-based conflict detection)                                    |
| `pkg/types/`                | Branded phantom-type IDs (`ItemID` ULID, `ExternalID` string, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`) |
| `pkg/errors/`               | Sentinel errors using stdlib `fmt.Errorf` with `%w` wrapping                                                    |
| `pkg/testhelpers/`          | Shared test mocks and factories                                                                                 |
| `cmd/examples/github-sync/` | Example CLI entry point                                                                                         |

## CQRS Architecture

The entire storage layer is CQRS-based via go-cqrs-lite. There is **no legacy CRUD path**.

### Core Components

- `aggregate_id.go` — deterministic SHA256→ULID from (source, sourceID) with sync.Map cache
- `decider.go` — `SyncItemState{Item *provider.Item, Deleted bool}`, pure Fold + DecideSync/DecideDelete
- `events.go` — 3 event types: `ItemSynced`, `ItemConflictFound`, `ItemDeleted`
- `readmodel.go` — `ReadModel` interface + `ItemFilter`, stores `*provider.Item` directly
- `memory_readmodel.go` — concurrent-safe in-memory read model with filter/pagination
- `turso_readmodel.go` — SQLite/Turso-backed read model with DDL, filter/pagination
- `projection.go` — `Projector` implements `event.Handler`, wired to bus via `SubscribeAll`
- `stack.go` — `CQRSStack` with Store+Bus+Repo+ReadModel, automatic projection via bus subscription

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic aggregate IDs**: SHA256→ULID from (source, sourceID)
- **Delete + resurrect**: deleted items reappear with updated state
- **Conflict detection**: `DecideSync` compares fields and emits `ItemConflictFound` events
- **Remote wins**: on conflict, the incoming item always overwrites (remote-wins LWW)

### Known Architectural Issue

`ConflictAwareSyncer` and `DecideSync` both independently detect conflicts using the same `HasChanged()` function but different truth sources (read model vs event-sourced state). This is a split-brain that should be consolidated — the decider should be the single authority.

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

| Package                    | Tests | Status                                                        |
| -------------------------- | ----- | ------------------------------------------------------------- |
| `pkg/cqrs`                 | 51    | ✅ Decider, ReadModel, Projection, Stack, Turso RM, Push/Pull |
| `pkg/providers/github`     | 35    | ✅ Client, fetch, retry, error handling (19+16 BDD)           |
| `pkg/sync`                 | 11    | ✅ Syncer + ConflictAwareSyncer                               |
| `pkg/types`                | 10    | ✅ ID construction, roundtrip, zero, equal                    |
| `pkg/errors`               | 4     | ✅ Sentinel errors, wrapping                                  |
| `pkg/provider`             | 1     | ✅ Item validation                                            |
| `cmd/examples/github-sync` | 6     | ✅ exitCodeForError, LoadConfig, env defaults                 |
| `pkg/testhelpers`          | 0     | ⬜ Helper package                                             |

**~110 total test cases** across 7 test packages. Test:Code ratio 1.13:1. Overall coverage: 64.8%.

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

| Dependency                    | Purpose                                              |
| ----------------------------- | ---------------------------------------------------- |
| `go-cqrs-lite/core` | v1.3.0 | Decider, event types, branded IDs, error taxonomy (latest: v1.4.0) |
| `go-cqrs-lite/memory` | v1.1.0 | In-memory event store + bus (latest: v1.2.0) |
| `go-cqrs-lite/storage` | pseudo | SQLite/Turso event store with optimistic concurrency (latest: v0.2.0) |
| `go-branded-id`               | Branded phantom-type IDs for compile-time safety     |
| `go-github/v69`               | GitHub API client                                    |
| `turso.tech/database/tursogo` | Turso Go client — local + remote sync                |
| `charm.land/log/v2`           | Structured logging                                   |
| `caarlos0/env/v11`            | Environment variable config                          |

### Test Dependencies

| Dependency       | Purpose                                |
| ---------------- | -------------------------------------- |
| `onsi/ginkgo/v2` | Indirect only (via go-cqrs-lite tests) |
| `onsi/gomega`    | Indirect only (via go-cqrs-lite tests) |

## go-cqrs-lite Integration

| Area          | go-localsync                                                 | go-cqrs-lite                                           |
| ------------- | ------------------------------------------------------------ | ------------------------------------------------------ |
| IDs           | `id.ID[B, V]` via go-branded-id directly                     | `id.Of[T]` — same memory layout                        |
| Storage       | `CQRSStack` → `decider.Repository[SyncItemState]`            | `event.Store` + `event.Bus` via memory/storage modules |
| Conflict      | `DecideSync` produces ItemConflictFound events               | Error taxonomy with 5 families available               |
| Read Model    | `MemoryReadModel` + `TursoReadModel` with filter/pagination  | Projected from events via bus subscription             |
| Retry         | Hand-rolled in `github/client.go`                            | `middleware.CommandRetry` available but not yet wired  |
| SchemaVersion | Preserved in SQL + Pebble serialization (fixed in cqrs-lite) | Upcasting infrastructure available                     |

### Not Yet Adopted

- `projection.Runner` with replay + checkpointing — **HIGH priority** (no crash recovery for read model)
- Error taxonomy wiring (`event.Classify`, `event.IsRetryable`) — **HIGH priority** (smart retry + exit codes)
- `decider.WithOutbox` for Turso backend — **HIGH priority** (atomic save+publish)
- `sync.LWWResolver[T]` + `sync.VectorClock` — **MEDIUM** (replace split-brain conflict detection)
- `event.JSONCodec` + `DecodePayload[T]` + `NewEvents` — **MEDIUM** (eliminate boilerplate)
- `middleware.CommandRetry` for provider retry — **MEDIUM**
- `command.Dispatcher` for typed command dispatch — **LOW**
- `UpcasterRegistry` for schema evolution — **LOW**
- `SnapshotStore` + `EveryNEvents` — **LOW**
- `catalog/` for AsyncAPI/OpenAPI/D2 generation — **LOW**

## go-cqrs-lite Adoption Gap (as of 2026-05-21)

**Adoption: 3/12 modules (25%). API surface of used modules: ~40%.**

Modules used: `core/event`, `core/decider`, `core/pkg/id`, `memory`, `storage`.
Modules unused: `core/command`, `core/query`, `core/aggregate`, `projection`, `middleware`, `sync`, `catalog`, `testhelpers`.

Key anti-patterns:
- `event.Version` cast to `int()` in 3 places (`decider.go:119,143,170`) — bypasses phantom type safety
- `bus.SubscribeAll` without replay/checkpoint (`stack.go:50`) — events lost on restart
- Manual `json.Marshal`/`json.Unmarshal` everywhere vs `event.JSONCodec` + `DecodePayload[T]`
- `aggregate_id.go` SHA256→ULID→string round-trip unnecessary (AggregateID is string-backed)

Full audit: `docs/status/2026-05-21_01-30_COMPREHENSIVE_STATUS_AND_CQRS_AUDIT.md`

## Lint Status

golangci-lint v2 reports **0 issues**. Config is strict with 125+ linters enabled.
