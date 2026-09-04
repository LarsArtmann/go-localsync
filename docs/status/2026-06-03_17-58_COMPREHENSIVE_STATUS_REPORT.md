# Go-LocalSync — Comprehensive Status Report

**Date:** 2026-06-03 17:58\
**Branch:** master (up to date with origin)\
**Reporter:** Session 9 — Post-v2 Correctness Sprint\
**Sessions completed:** 9 (since CQRS rewrite on 2026-05-03)

---

## Executive Summary

Go-LocalSync is a **generic synchronization SDK** with event-sourced CQRS, pluggable conflict resolution via CRDT, branded IDs, and a pluggable provider architecture. The project is in **active development** — architecture is solid, all tests green, zero lint issues. The codebase is **production-viable for single-provider local sync** but has known gaps in test coverage (CLI main flow), observability, and multi-user support.

**Key metrics:**

| Metric               | Value                                  |
| -------------------- | -------------------------------------- |
| Non-test Go source   | 4,250 lines across 28 files            |
| Test Go source       | 6,849 lines across 18 test files       |
| Total test functions | **240 passing, 0 failing**             |
| Packages             | 9 (all green)                          |
| Lint issues          | **0** (golangci-lint v2, 125+ linters) |
| BuildFlow steps      | **24/24 passing**                      |
| AGENTS.md            | 367 lines (under 377 limit)            |
| Go version           | 1.26.3                                 |
| Status reports       | 22 in `docs/status/` + 25 archived     |

---

## A) FULLY DONE

### Architecture (Production-Quality)

- **CQRS via go-cqrs-lite v2** — Full event-sourced architecture. Decider, Repository, Event Store, Bus, ReadModel, Projection, Snapshots, Checkpoints. No legacy CRUD.
- **Deterministic aggregate IDs** — SHA256→hex from (source, sourceID) with sync.Map cache. Idempotent by design.
- **Dual storage backend** — Memory (testing/dev) and SQLite (production) via `CQRSConfig.Backend` factory. Pure Go via `modernc.org/sqlite`. No CGo.
- **Projection** — `Projector` implements `event.Projection`. Direct `bus.SubscribeAll` (memory) + `projection.Runner` with global replay + live subscription (SQLite). SQL checkpoints for crash-safe replay.
- **Command & Query dispatch** — `command.Dispatcher` with typed `SyncItemCommand`/`DeleteItemCommand`. `query.Dispatcher` with typed queries. Logging + validation middleware wired.
- **Snapshots** — `SQLiteSnapshotStore` + `MemorySnapshotStore` with `snapshot.EveryNEvents(10)` strategy. Caps replay cost.
- **Correlation IDs** — Unique per sync run via `event.WithCorrelationID`. Cross-event tracing.
- **Event logging middleware** — `middleware.EventLogging` via charm log adapter. Structured logging of all domain events.

### Sync Engine

- **Full sync** — `Syncer.Sync()` fetches all pages, validates items, syncs via CQRS.
- **Incremental sync** — `Syncer.SyncIncremental()` uses latest `CreatedAt` as cutoff. Falls back to full sync on empty DB.
- **Conflict-aware sync** — `ConflictAwareSyncer` delegates to CQRS decider. Reports conflicts, upserts, skips, errors.
- **Pluggable CRDT resolution** — `CQRSConfig.ConflictResolver` accepts any `crdt.ConflictResolver[*provider.Item]`. Default nil = remote-wins. `LWWResolver` available.
- **Item validation** — `Item.Validate()` checks ExternalID, Source, Type, CreatedAt. Invalid items counted in error metrics.
- **`SyncStore` interface seam** — `pkg/sync/` defines the boundary. Zero imports on `pkg/cqrs/`. Dependency flows one way.

### Type System

- **Branded IDs** — 6 phantom types via go-branded-id: `ItemID` (ULID), `ExternalID`, `ProviderID`, `ActorID`, `RepoID`, `EventTypeID`. Compile-time type safety.
- **`ParseItemID`** — Non-panicking ULID parse (added session 9). `MustParseItemID` still available for tests.
- **Structured errors** — 9 sentinel errors via go-error-family with intrinsic classification. `IsRetryable`, `WithDetail`, `Wrap`.

### HTTP API

- **Huma v2** — 4 endpoints: `GET /items` (filterable), `GET /stats`, `POST /sync`, `GET /health`. Auto-generated OpenAPI 3 spec. Stdlib adapter.
- **8 tests** covering all endpoints including error paths.

### CLI / Example Application

- **Complete CLI** — Flag parsing, signal handling, graceful shutdown, domain-specific BSD exit codes.
- **Server mode** — `-server` flag runs HTTP API.
- **Environment config** — `caarlos0/env/v11`. All flags overridable by env vars.
- **JSON output** — `-json` flag for structured output.
- **Error templates** — `RegisterErrorTemplates()` for all 9 error codes with What/Why/Fix/WayOut.

### Build & CI

- **Nix flake** — `buildGoModule` package + devShell (Go 1.26, golangci-lint, ginkgo, gofumpt).
- **BuildFlow pre-commit** — 24 steps: build, test, lint, format, binary check, file size, TODO scan, gitleaks, doc age, AGENTS.md size, go-structure-linter.
- **GitHub Actions CI** — 4-job pipeline: test (race + coverage), lint, build (linux/darwin/arm64), release (on tags).
- **No CGo** — Pure Go via `modernc.org/sqlite`. `CGO_ENABLED=0` builds.

### Session 9 Fixes (This Session)

- ✅ **`scanItems` rows.Err() check** — Was silently returning partial results
- ✅ **`Fold(ItemDeleted)` nils Item** — Prevented stale data reads from deleted aggregates
- ✅ **SQLite indexes** — `idx_sync_items_repo_name`, `idx_sync_items_type_created` composite
- ✅ **`parseItemID` panic→error** — `id.ParseItemID()` for safe event replay
- ✅ **`itemFromPayload` validation** — Calls `Item.Validate()` on reconstructed items
- ✅ **`Item.String()`** — `fmt.Stringer` for structured logging
- ✅ **Projector error-path tests** — Corrupt ItemID, missing required fields
- ✅ **`TestItem_JSONRoundTrip`** — All fields through marshal/unmarshal
- ✅ **`id.ParseItemID` tests** — Success + error cases

### Session 8 (Previous Session)

- ✅ **go-cqrs-lite v2 migration** — All 11 modules updated to v2 sub-modules
- ✅ **turso→sqlite rename** — All internal references, dead config removed
- ✅ **Outbox/Turso sync removed** — Aligned with v2 upstream
- ✅ **`modernc.org/sqlite`** — Replaced `tursogo` driver
- ✅ **All 235 tests passing** after migration

---

## B) PARTIALLY DONE

### Test Coverage

| Package                    | Coverage | Gap                                                                               |
| -------------------------- | -------- | --------------------------------------------------------------------------------- |
| `pkg/errors`               | 100.0%   | —                                                                                 |
| `pkg/id`                   | 100.0%   | —                                                                                 |
| `pkg/provider`             | 95.8%    | Minor                                                                             |
| `pkg/crdt`                 | 97.6%    | Minor                                                                             |
| `pkg/sync`                 | 91.7%    | Minor                                                                             |
| `pkg/cqrs`                 | 85.7%    | `runner.go`, `store_factory.go`, `projection.go` have low/no direct test coverage |
| `pkg/providers/github`     | 84.7%    | Minor gaps in error paths                                                         |
| `pkg/api`                  | 76.3%    | Missing error path tests for store failures, malformed requests                   |
| `cmd/examples/github-sync` | 13.7%    | **Main sync/stats/server flows completely untested**. Only helpers tested.        |

### Zero-Coverage Files (No Direct Tests)

- `pkg/cqrs/runner.go` — Unified projection subscription. Tested indirectly via stack integration tests but no unit tests.
- `pkg/cqrs/store_factory.go` — Backend factory. Tested indirectly via stack tests.
- `pkg/cqrs/projection.go` — `Projector.Handle` tested (3 tests) but no replay/resubscription tests.

### Documentation

- `AGENTS.md` — 367 lines, under limit, session 9 updated ✅
- `TODO_LIST.md` — **Stale** — still references "go-cqrs-lite upstream WIP" (resolved in v2), test count says 241 (now 240), coverage table outdated
- `FEATURES.md` — **Stale** — test count says 235 (now 240), session 8 date but missing session 9 items
- `ROADMAP.md` — **Stale** — completed list missing session 9 items, open questions outdated

---

## C) NOT STARTED

### High-Impact Features

| # | Task                              | Effort | Impact                                                                                          |
| - | --------------------------------- | ------ | ----------------------------------------------------------------------------------------------- |
| 1 | **OpenTelemetry instrumentation** | ~4h    | High — No observability. Production debugging impossible without log spelunking.                |
| 2 | **Second provider (GitLab/Jira)** | ~8h    | High — SDK is generic but only GitHub exists. No proof the abstraction works for other sources. |
| 3 | **Multi-user sync**               | ~4h    | Medium — CLI accepts one `-user`. No batch or file-based user lists.                            |
| 4 | **Daemon/cron mode**              | ~2h    | Medium — Must run manually or wrap in external scheduler.                                       |
| 5 | **API authentication middleware** | ~2h    | High — HTTP API has no auth. Not safe to expose on a network.                                   |

### Testing Gaps

| #  | Task                                                             | Effort | Impact                                                                           |
| -- | ---------------------------------------------------------------- | ------ | -------------------------------------------------------------------------------- |
| 6  | **Integration test for full sync pipeline**                      | ~2h    | High — Provider → CQRS → read model → API round-trip. All testing is mock-based. |
| 7  | **CLI main flow tests** (`runSync`, `runStats`, signal handling) | ~3h    | High — 13.7% coverage. Core flows untested.                                      |
| 8  | **Concurrent read model access tests**                           | ~1h    | Medium — MemoryReadModel uses RWMutex but never tested under concurrent load.    |
| 9  | **Real GitHub PAT smoke test**                                   | ~1h    | Medium — Never verified with real GitHub API.                                    |
| 10 | **Test SQLite read model with file persistence**                 | ~1h    | Medium — Only tested with `:memory:`. File I/O and locking untested.             |

### Code Quality

| #  | Task                                             | Effort | Impact                                                                               |
| -- | ------------------------------------------------ | ------ | ------------------------------------------------------------------------------------ |
| 11 | **Unify test framework**                         | ~3h    | Medium — 1 Ginkgo file + 6 testify files. Rest uses stdlib.                          |
| 12 | **Remove `GetTypes`/`GetItemTypes` duplication** | ~30min | Low — Two names for the same method in `stack_adapters.go`.                          |
| 13 | **Extract shared `testutil` package**            | ~1h    | Medium — `testItem()` helper duplicated across 4+ test files.                        |
| 14 | **Split `pkg/sync/sync.go`** (348 lines)         | ~30min | Low — Near 350-line soft limit. Extract types/constants to separate files.           |
| 15 | **Doc comments for exported types**              | ~1h    | Low — ~18 exported types missing godoc across `pkg/id/`, `pkg/errors/`, `pkg/crdt/`. |

### CLI / UX

| # | Task | Effort | Impact |
| --- | ------------------------------------ | ------ | -------------------------------------------------------------- | --------------------------- |
| 16 | **CLI flag for conflict resolver** | ~1h | Medium — `--conflict-strategy=remote-wins                      | lww` to expose CRDT wiring. |
| 17 | **Graceful shutdown for API server** | ~30min | Medium — No `http.Server.Shutdown(ctx)`. Hard close on SIGINT. |
| 18 | **Add structured logging fields** | ~1h | Medium — Inconsistent context fields across log statements. |

### API Enhancements

| #  | Task                             | Effort | Impact                                     |
| -- | -------------------------------- | ------ | ------------------------------------------ |
| 19 | **API pagination headers**       | ~1h    | Low — `X-Total-Count`, cursor-based.       |
| 20 | **API rate limiting middleware** | ~1h    | Medium — Prevent `POST /sync` abuse.       |
| 21 | **API OpenAPI error schemas**    | ~1h    | Low — Error response schemas per endpoint. |

### Documentation

| #  | Task                                             | Effort | Impact                                           |
| -- | ------------------------------------------------ | ------ | ------------------------------------------------ |
| 22 | **ADR: CQRS adoption**                           | ~30min | Low                                              |
| 23 | **ADR: Branded ID migration**                    | ~30min | Low                                              |
| 24 | **ADR: CRDT integration strategy**               | ~30min | Low                                              |
| 25 | **Update TODO_LIST.md, FEATURES.md, ROADMAP.md** | ~30min | Medium — All three are stale after sessions 8-9. |

---

## D) TOTALLY FUCKED UP

### Nothing is catastrophically broken.

The codebase is in **good shape**. All tests pass, zero lint issues, clean architecture. However, there are honest concerns:

1. **`cmd/examples/github-sync` at 13.7% coverage** — The main entry point has almost no tests. If someone breaks `runSync()` or `runAPIServer()`, we won't catch it in CI. This is the biggest quality gap.

2. **TODO_LIST.md is stale** — References resolved items (go-cqrs-lite upstream WIP), wrong test count (241→240), outdated coverage numbers. If someone reads it for next steps, they'll be misled.

3. **No integration test** — Every test is mock-based. The full pipeline (Provider → CQRS → ReadModel → API) has never been tested end-to-end. If the wiring between packages breaks, no test catches it.

4. **`SyncStore.SyncItems()` returns `*SyncSummary` but not `error`** — Wholesale store failures (e.g., SQLite disk full) can't propagate to the caller. The sync just silently returns a partial summary. This is a design gap, not a bug — but it could bite in production.

5. **`GetStats` does N+1 queries** — `Syncer.GetStats()` calls `Count()` once per event type. With many types, this is O(n) queries. Should be a single `GROUP BY` query. Low priority for now (few types in practice).

---

## E) WHAT WE SHOULD IMPROVE

### Immediate (Next Session)

1. **Update TODO_LIST.md, FEATURES.md, ROADMAP.md** — All three are stale. Session 8-9 changes not reflected. Test counts, coverage numbers, completed items all need syncing.

2. **CLI main flow tests** — Get `cmd/examples/github-sync` from 13.7% to 50%+ by testing `runSync`, `runStats`, signal handling with mock store.

3. **Integration test** — One test that creates a CQRS stack, syncs items through a mock provider, and verifies the read model + API responses. Catches wiring bugs.

### Short-Term (Next 2 Sessions)

4. **OpenTelemetry instrumentation** — Add spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, HTTP middleware. `go.opentelemetry.io/otel` is already an indirect dependency.

5. **CLI conflict strategy flag** — `--conflict-strategy=remote-wins|lww` to expose the CRDT wiring that's already in place but hidden behind a nil default.

6. **Concurrent read model tests** — `MemoryReadModel` uses `sync.RWMutex` but has never been tested under concurrent load. Easy to add with `-race`.

### Medium-Term (Next Month)

7. **Second provider** — Prove the abstraction works. GitLab events or Jira issues. This validates the entire provider architecture.

8. **API authentication** — Without auth, the HTTP API is unsafe to expose. API key or JWT middleware.

9. **Daemon mode** — Periodic sync via cron/systemd integration. Required for any production use case.

### Architecture Improvements

10. **`SyncStore.SyncItems()` should return `error`** — Design gap. Wholesale store failures can't propagate. Requires updating the interface and all implementations.

11. **Consolidate `GetTypes`/`GetItemTypes`** — Two methods doing the same thing in `stack_adapters.go`. Keep `GetItemTypes` (satisfies `SyncStore`), remove `GetTypes`, update callers.

12. **Extract `internal/testutil`** — `testItem()` helper duplicated across 4+ test files. DRY violation.

---

## F) TOP 25 THINGS TO DO NEXT

Sorted by **impact × effort** (highest impact, lowest effort first):

| #  | Task                                                                  | Impact | Effort | Category      |
| -- | --------------------------------------------------------------------- | ------ | ------ | ------------- |
| 1  | Update TODO_LIST.md, FEATURES.md, ROADMAP.md with session 8-9 changes | Medium | 30min  | Docs          |
| 2  | CLI main flow tests (`runSync`, `runStats`, signal handling)          | High   | 3h     | Testing       |
| 3  | Full pipeline integration test (Provider → CQRS → ReadModel → API)    | High   | 2h     | Testing       |
| 4  | Remove `GetTypes`/`GetItemTypes` duplication                          | Low    | 30min  | Code Quality  |
| 5  | CLI flag for conflict resolver (`--conflict-strategy`)                | Medium | 1h     | CLI           |
| 6  | Graceful shutdown for API server                                      | Medium | 30min  | CLI           |
| 7  | Concurrent read model access tests                                    | Medium | 1h     | Testing       |
| 8  | Add structured logging fields (username, page, event_id)              | Medium | 1h     | Observability |
| 9  | Test SQLite read model with file persistence                          | Medium | 1h     | Testing       |
| 10 | Table-driven tests for `HasChanged` edge cases                        | Medium | 1h     | Testing       |
| 11 | Extract shared `internal/testutil` package                            | Medium | 1h     | Code Quality  |
| 12 | Split `pkg/sync/sync.go` (348 lines)                                  | Low    | 30min  | Code Quality  |
| 13 | `mapSyncError()` table-driven tests                                   | Medium | 30min  | Testing       |
| 14 | Performance benchmarks (1k/10k/100k items)                            | Medium | 2h     | Testing       |
| 15 | OpenTelemetry instrumentation (spans + HTTP middleware)               | High   | 4h     | Observability |
| 16 | API authentication middleware                                         | High   | 2h     | Security      |
| 17 | Real GitHub PAT smoke test                                            | Medium | 1h     | Testing       |
| 18 | `SyncStore.SyncItems()` return `error`                                | Medium | 1h     | Architecture  |
| 19 | Doc comments for exported types (~18 missing)                         | Low    | 1h     | Code Quality  |
| 20 | Unify test framework (remove testify + Ginkgo)                        | Medium | 3h     | Code Quality  |
| 21 | Second provider implementation (GitLab or Jira)                       | High   | 8h     | Features      |
| 22 | Multi-user sync support                                               | Medium | 4h     | Features      |
| 23 | API pagination headers + rate limiting                                | Medium | 2h     | API           |
| 24 | Daemon/cron mode for periodic sync                                    | Medium | 2h     | Features      |
| 25 | Data export (JSON/CSV)                                                | Low    | 1h     | Features      |

---

## G) TOP #1 QUESTION I CANNOT FIGURE OUT MYSELF

**What is the target use case?**

The codebase is a generic sync SDK with a provider abstraction, event-sourced storage, and HTTP API. But:

- Is this meant to be a **library** (imported by other Go programs) or a **service** (deployed and called via API)?
- The CLI is in `cmd/examples/` — suggesting it's example code, not the primary interface. But the HTTP API is in `pkg/api/` — suggesting the API is the primary interface.
- If it's a library, the HTTP API and CLI are unnecessary baggage. If it's a service, the provider abstraction needs to support runtime registration (not just compile-time).
- The `SyncStore` interface returns `*SyncSummary` without `error` — this makes sense for a CLI tool (best-effort sync) but is wrong for a library (callers need to know about failures).

**Why it matters:** The answer determines whether we should invest in:

- Library: Better public API, godoc, stability guarantees, provider registration API
- Service: Auth, rate limiting, multi-tenancy, deployment story, observability
- Both: Clear separation between SDK core and service layer

---

## Test Coverage Detail

| Package                    | Tests | Coverage | Status                                         |
| -------------------------- | ----- | -------- | ---------------------------------------------- |
| `pkg/errors`               | 11    | 100.0%   | ✅                                             |
| `pkg/id`                   | 12    | 100.0%   | ✅                                             |
| `pkg/provider`             | 4     | 95.8%    | ✅                                             |
| `pkg/crdt`                 | 52    | 97.6%    | ✅                                             |
| `pkg/sync`                 | 22    | 91.7%    | ✅                                             |
| `pkg/cqrs`                 | ~80   | 85.7%    | ⚠️ `runner.go`, `store_factory.go` low coverage |
| `pkg/providers/github`     | 32    | 84.7%    | ✅                                             |
| `pkg/api`                  | 8     | 76.3%    | ⚠️ Error paths untested                         |
| `cmd/examples/github-sync` | 14    | 13.7%    | ❌ Main flows untested                         |

**Total: 240 test functions, 0 failures.**

---

## Package Dependency Graph

```
cmd/examples/github-sync
    → pkg/api → pkg/cqrs → pkg/sync → pkg/provider → pkg/id
    → pkg/providers/github → pkg/provider → pkg/id
    → pkg/cqrs → pkg/crdt → pkg/errors
    → pkg/cqrs → pkg/errors
    → pkg/sync → pkg/errors
```

No circular dependencies. Clean one-way flow: `cqrs → sync → provider/types/errors`.

---

## Session History

| Session | Date          | Focus                                          | Commits   |
| ------- | ------------- | ---------------------------------------------- | --------- |
| 1-2     | 2026-05-03/04 | CQRS rewrite, go-cqrs-lite integration         | Multiple  |
| 3       | 2026-05-21    | CQRS audit, best-use sprint                    | Multiple  |
| 4       | 2026-05-25    | Unix-style modularity, interface seams         | Multiple  |
| 5       | 2026-05-28    | HTTP API, error templates, build system        | Multiple  |
| 6       | 2026-05-29    | CRDT conflict resolution integration           | Multiple  |
| 7       | 2026-05-29    | File size refactoring, helpers extraction      | Multiple  |
| 8       | 2026-06-03    | go-cqrs-lite v2 migration, turso→sqlite rename | 6 commits |
| 9       | 2026-06-03    | Correctness fixes, validation, String(), tests | 5 commits |

**Total commits on master:** 12 since last push to origin (sessions 8-9).

---

_Generated at 2026-06-03 17:58 by Session 9_
