# Comprehensive Status Report — 2026-04-21 19:39

**Author**: Crush (GLM 5.1) assisted session
**Scope**: go-cqrs-lite, go-localfirst, go-localsync (3 Go projects)

---

## A. FULLY DONE ✅

### A1. Pluggable Backend Architecture (all 3 projects)

**go-cqrs-lite** — `event/store_config.go` + `event/store_config_test.go`

- `Backend` type, `StoreConfig`, `StoreOption`, `NewStoreFromConfig` factory
- Switch-based dispatch for built-in `memory` backend
- External backends implement `Store` interface directly (Go idiom)
- 3 tests · Commit: `de7f0de`

**go-localfirst** — State + Event store pluggable

- `internal/storage/config.go` (76 lines) — `Backend` (`pebble`/`memory`), `Config`, `Option`, `NewStateStore` factory
- `internal/storage/memory_store.go` (167 lines) — Full `MemoryTodoStore` implementing `domain.TodoRepository`
- `internal/cqrs/store/config.go` (68 lines) — `Backend`, `EventStoreProvider` function type, `NewEventStore`
- `cmd/api/main.go` — Config-driven via `STATE_BACKEND` / `EVENT_BACKEND` env vars
- 17 tests · Commit: `c4913b2`

**go-localsync** — Storage pluggable

- `pkg/storage/config.go` (75 lines) — `Backend` (`sqlite`/`memory`), `Config`, `Option`, `NewStorage` factory
- `pkg/storage/memory_storage.go` (287 lines) — Full `MemoryStorage` implementing 16-method `Storage` interface
- `cmd/examples/github-sync/main.go` — `--backend` flag
- 27 tests · Commit: `a5484ae`

### A2. Dead Code Removal (go-localfirst)

- **Deleted**: `internal/storage/event_store.go` (367 lines) — `domain.EventStore` interface + `PebbleEventStore` implementation
- **Moved**: `PebbleMixin` to its own file (`pebble_mixin.go`) since `PebbleStore` still needs it
- **Removed**: `EventStore` interface from `internal/domain/todo.go`
- **Removed**: `eventStore domain.EventStore` field from `TodoService`, `NewTodoServiceWithEvents` function
- **Removed**: `eventStore domain.EventStore` field from `SSEHandler`, updated `NewSSEHandler()` signature
- **Fixed**: `todo_service_test.go` — removed `mockEventStore` (67 lines), fixed `setupTestService`
- **Fixed**: `sse_handler_test.go` — removed stale `NewSSEHandler(nil)` calls
- **Updated**: Stale comments in `pebble_adapter.go` and `pebble_store.go`
- Commit: `029a9db`

### A3. SSE Event Bus Fix (go-localfirst)

**Root cause**: Command handlers saved events to event store but never called `eventBus.Publish()`. SSE subscribers never received notifications.

**Fix**:

- Added `eventBus event.Bus` to `CommandHandlerMixin` in `mixin.go`
- Capture `uncommitted := todo.UncommittedChanges()` before `MarkChangesAsCommitted()`
- Call `m.eventBus.Publish(ctx, uncommitted...)` after `events.Save()` in both:
  - `executeCommand()` (used by update/delete/changeStatus handlers)
  - `CreateTodoHandler.Handle()` (direct save path)
- Updated all 4 handler constructors: `NewCreateTodoHandler`, `NewUpdateTodoHandler`, `NewDeleteTodoHandler`, `NewChangeStatusHandler` — now accept `eventBus` parameter
- Pass `eventBus` from `CQRSContainer` through `createCQRSContainer`
- Integration tests updated with `nil` eventBus
- Commit: `c12c738`

**Event flow now**: handler → event store → event bus → SSE bridge → clients ✅

### A4. Compliance Test Suites (2 projects)

**go-localfirst** — `internal/storage/compliance_test.go` (291 lines)

- 18 tests per backend × 2 backends = **36 tests**
- Covers: CRUD, NotFound errors, duplicate create, pagination, status/priority/tags/search filtering, case-insensitive search, clone safety, count with filter
- Both `PebbleStore` and `MemoryTodoStore` pass
- Commit: `d6a355f`

**go-localsync** — `pkg/storage/compliance_test.go` (342 lines)

- 22 tests per backend × 2 backends = **44 tests**
- Covers: all 16 `Storage` interface methods, idempotent upsert, batch operations, filter queries (type/actor/repo/source/since), pagination, NotFound edge cases, count, types
- Both `SQLiteStorage` and `MemoryStorage` pass
- Commit: `1c2a18f`

### A5. Documentation Updates

- **go-localfirst AGENTS.md** — Added pluggable backend architecture section, env vars, EventStoreProvider pattern, compliance test reference, SSE event flow, dead code removal notes · Commit: `76dae61`
- **go-localsync AGENTS.md** — Added pluggable storage section, CLI flag, compliance tests, how to add new backends · Commit: `d6414db`
- **go-localsync status report** — `docs/status/2026-04-21_19-18_PLUGGABLE_BACKEND_ARCHITECTURE.md` · Commit: `055e6c8`

---

## B. PARTIALLY DONE ⚠️

### B1. Deprecated `TodoService` still exists (go-localfirst)

- **Status**: Marked `Deprecated: Use CQRS command and query handlers instead` but still 249 lines with full CRUD + event handling
- **Still used by**: `setupTestService` in `todo_service_test.go` (8 test functions depend on it)
- **Still wired in**: `cmd/api/main.go` doesn't directly use it anymore but the DI container could still inject it
- **Risk**: Two parallel code paths (CQRS handlers + TodoService) create confusion about which is the "real" implementation
- **Next step**: Migrate remaining service tests to use CQRS handlers, then delete `TodoService` + `SyncService`

### B2. SSE bridge has suspicious code (go-localfirst)

- `sse_cqrs_bridge.go:20` — `const _ = "*"` appears to be dead code
- `convertTodoStatusChanged` maps to `domain.EventTodoUpdated` — likely should be a distinct event type
- The bridge itself works but has minor issues

---

## C. NOT STARTED ❌

### C1. NATS JetStream backend (original goal)

The pluggable architecture was designed so NATS JetStream (or any other backend) can be added as an event store/storage backend. The plumbing is in place but no NATS implementation exists yet.

### C2. Type model improvements

- `domain.TodoStatus` is a string alias — could use branded types like go-localsync's `types.EventTypeID`
- `domain.TodoID` uses `google/uuid` while go-localsync uses branded `id.ID[Brand, string]` from `go-composable-business-types`
- Inconsistency across projects in ID handling patterns

### C3. Error standardization

- go-localfirst uses `fmt.Errorf` + sentinel `domain.Err*`
- go-localsync uses `cockroachdb/errors` + `pkgerrors.Err*`
- go-cqrs-lite uses `pkg/errors` (different cockroachdb/errors fork?)
- No shared error package across the 3 projects

### C4. SyncService is a stub

`internal/service/todo_service.go:237` — `// TODO: Implement actual sync logic with CRDTs`. The `SyncService.Sync()` is a no-op placeholder.

### C5. Prometheus → OpenTelemetry migration

`internal/metrics/prometheus.go` is marked `DEPRECATED: prefer go.opentelemetry.io/otel` but still 276 lines of active code.

### C6. Integration/E2E test coverage for SSE event flow

The SSE event bus fix (eventBus.Publish in handlers) has no integration test proving the full round-trip: command → handler → event store → event bus → SSE bridge → client channel. The bridge unit tests exist but don't test the end-to-end flow.

### C7. go-cqrs-lite compliance test suite

go-cqrs-lite has `store_config_test.go` (3 tests) but no shared compliance test suite for `event.Store` implementations. Both `memory` and the Pebble adapter (in go-localfirst) should pass a shared suite.

### C8. CI/CD pipeline alignment

- go-localsync has known blockers (golangci-lint v1/v2 mismatch, Go toolchain mismatch)
- No shared CI config across the 3 projects
- `GONOSUMCHECK` workaround in CI but no documentation in go-localfirst/go-cqrs-lite

---

## D. TOTALLY FUCKED UP 💥

**Nothing is catastrophically broken.** All three projects build and pass all tests. But there are two near-misses worth documenting:

### D1. PebbleMixin was almost lost

When `internal/storage/event_store.go` was deleted (it contained both `PebbleEventStore` AND `PebbleMixin`), the previous session didn't extract `PebbleMixin` first. This caused 10+ build errors because `PebbleStore` embeds `PebbleMixin`. **Fixed in this session** by creating `pebble_mixin.go`.

### D2. SSE was silently broken before this session

The SSE handler was receiving zero events because command handlers never called `eventBus.Publish()`. This wasn't a crash — it was a silent failure. Clients connected via SSE but never got updates. **Fixed in this session** by wiring Publish calls.

---

## E. WHAT WE SHOULD IMPROVE 🔧

### E1. Architectural

1. **Eliminate dual code paths** — `TodoService` (deprecated) and CQRS handlers do the same job. Pick one, delete the other.
2. **Standardize ID types** — go-localfirst uses `google/uuid`, go-localsync uses branded phantom types. Should align.
3. **Standardize error handling** — `cockroachdb/errors` vs `fmt.Errorf` + sentinel vs `pkg/errors`. Pick one.
4. **Make `PebbleMixin` a real dependency, not an embedded struct** — Embedding exposes all fields; composition via explicit `db *pebble.DB` field would be cleaner.

### E2. Testing

5. **Add SSE end-to-end integration test** — Prove the full event flow works: command → store → bus → SSE bridge → channel
6. **Add go-cqrs-lite `event.Store` compliance suite** — Both memory and Pebble adapter should pass shared tests
7. **Add concurrent write safety tests** — Verify both backends handle concurrent Upsert/Create correctly
8. **Delete TodoService test suite AFTER migrating useful tests** — Don't lose coverage when deleting the deprecated service

### E3. Code Quality

9. **Remove `const _ = "*"` dead code in `sse_cqrs_bridge.go`**
10. **Fix `convertTodoStatusChanged` event type mapping** — Should emit a distinct event type, not `EventTodoUpdated`
11. **Remove `SyncService` stub or implement it** — Dead stub code is worse than no code
12. **Replace Prometheus with OpenTelemetry** — Already deprecated, still 276 lines
13. **Add `Delete_NotFound` consistency** — SQLite returns nil, Memory returns ErrNotFound. Interface contract should be explicit.

### E4. Documentation

14. **Document the interface contract for `Storage.Delete`** — Is ErrNotFound required on missing item? Or is it a no-op?
15. **Add CONTRIBUTING.md** with backend development guide
16. **Generate API docs from OpenAPI spec** — `openapi.json` exists but may be stale

---

## F. TOP 25 THINGS TO DO NEXT 📋

Sorted by **impact × effort** (high impact + low effort first):

| #  | Task                                                                | Project                    | Impact | Effort  | Status      |
| -- | ------------------------------------------------------------------- | -------------------------- | ------ | ------- | ----------- |
| 1  | Add SSE end-to-end integration test                                 | go-localfirst              | High   | Low     | Not started |
| 2  | Fix `convertTodoStatusChanged` event type mapping                   | go-localfirst              | High   | Low     | Not started |
| 3  | Remove dead `const _ = "*"` in sse_cqrs_bridge.go                   | go-localfirst              | Low    | Trivial | Not started |
| 4  | Clarify `Storage.Delete` NotFound contract in interface             | go-localsync               | High   | Low     | Not started |
| 5  | Add go-cqrs-lite `event.Store` compliance suite                     | go-cqrs-lite               | High   | Medium  | Not started |
| 6  | Migrate TodoService tests to CQRS handlers, then delete TodoService | go-localfirst              | High   | Medium  | Partial     |
| 7  | Delete `SyncService` stub (or implement it)                         | go-localfirst              | Medium | Trivial | Not started |
| 8  | Standardize ID types across projects                                | all                        | High   | Medium  | Not started |
| 9  | Standardize error handling (pick cockroachdb/errors everywhere)     | all                        | Medium | Medium  | Not started |
| 10 | Add NATS JetStream event store backend                              | go-localfirst/go-cqrs-lite | High   | High    | Not started |
| 11 | Add concurrent write safety tests for both backends                 | go-localfirst              | Medium | Low     | Not started |
| 12 | Add concurrent write safety tests for both backends                 | go-localsync               | Medium | Low     | Not started |
| 13 | Replace PebbleMixin embedding with explicit composition             | go-localfirst              | Medium | Low     | Not started |
| 14 | Migrate Prometheus → OpenTelemetry                                  | go-localfirst              | Medium | High    | Deprecated  |
| 15 | Add PostgreSQL storage backend for go-localsync                     | go-localsync               | High   | High    | Not started |
| 16 | Implement real CRDT sync in SyncService                             | go-localfirst              | High   | High    | Stub        |
| 17 | Document interface contracts (delete, upsert idempotency, etc.)     | all                        | Medium | Low     | Not started |
| 18 | Add versioned API migration path (v1 → v2)                          | go-localfirst              | Medium | Medium  | Not started |
| 19 | Fix go-localsync CI blockers (lint, toolchain)                      | go-localsync               | Medium | Medium  | Known       |
| 20 | Add shared CI config across projects                                | all                        | Medium | Medium  | Not started |
| 21 | Generalize `github_id` → `source_id` in SQLite schema               | go-localsync               | High   | Medium  | Not started |
| 22 | Add health check that verifies event bus is working                 | go-localfirst              | Medium | Low     | Not started |
| 23 | Add request tracing (correlation IDs through CQRS pipeline)         | go-localfirst              | Medium | Medium  | Not started |
| 24 | Extract shared test helpers across projects                         | all                        | Low    | Medium  | Not started |
| 25 | Add benchmark tests for storage backends                            | go-localfirst/go-localsync | Medium | Medium  | Not started |

---

## G. TOP #1 QUESTION ❓

**What is the intended fate of `TodoService` (go-localfirst)?**

It's marked `Deprecated: Use CQRS command and query handlers instead` but still has 8 active test functions, a working `SyncService` wrapper, and is the only place that emits `domain.TodoEvent` events through the `RegisterEventHandler` mechanism. Meanwhile, the CQRS handlers now emit events through `eventBus.Publish()` which flows through the SSE bridge.

**Can I delete TodoService entirely, or is something still depending on its `RegisterEventHandler` / `emitEvent` mechanism that the CQRS path doesn't cover?** Specifically: is the `executeWithEvent` pattern in `TodoService` redundant with the CQRS `eventBus.Publish()` + SSE bridge, or does some consumer still register handlers on `TodoService` directly?

---

## Test Results (Current)

| Project       | Packages        | Test Functions | Status               |
| ------------- | --------------- | -------------- | -------------------- |
| go-cqrs-lite  | 13 ok           | 241 PASS       | 🟢 All green         |
| go-localfirst | 12 ok           | 156 PASS       | 🟢 All green         |
| go-localsync  | 7 ok            | 77 PASS        | 🟢 All green         |
| **Total**     | **32 packages** | **474 tests**  | **🟢 Zero failures** |

## Code Stats

| Project       | Non-test .go files | Lines of Go code |
| ------------- | ------------------ | ---------------- |
| go-cqrs-lite  | ~30                | 5,963            |
| go-localfirst | 42                 | 6,625            |
| go-localsync  | 16 (+4 sqlc)       | 2,834 (+sqlc)    |
| **Total**     | ~92                | ~15,422          |

## Commits This Session (6 total)

| # | Commit    | Project       | Description                                            |
| - | --------- | ------------- | ------------------------------------------------------ |
| 1 | `029a9db` | go-localfirst | Remove dead domain.EventStore + PebbleEventStore       |
| 2 | `c12c738` | go-localfirst | Wire eventBus.Publish() in command handlers to fix SSE |
| 3 | `d6a355f` | go-localfirst | Add TodoRepository compliance test suite               |
| 4 | `76dae61` | go-localfirst | Update AGENTS.md with pluggable backend docs           |
| 5 | `1c2a18f` | go-localsync  | Add Storage compliance test suite                      |
| 6 | `d6414db` | go-localsync  | Update AGENTS.md with pluggable storage docs           |

## Git Status

All three repos: **clean working tree**, **all changes pushed to origin/master**.

---

## Resolution (2026-09-06 docs-health sweep)

Era-closed: SSE end-to-end tests, `convertTodoStatusChanged`, and the Storage.Delete contract all belong to the deleted pre-rewrite stack; live-update streaming remains an uncommitted ROADMAP idea. No live items remain here; the living trackers are [TODO_LIST.md](../../../TODO_LIST.md) and [ROADMAP.md](../../../ROADMAP.md).
