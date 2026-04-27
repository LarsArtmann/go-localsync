# Session Status Report — 2026-04-21 19:18

## Session Goal

Add pluggable backend architecture to go-cqrs-lite, go-localfirst, and go-localsync so that NATS JetStream (or any other backend) can be added as an event store / storage backend without modifying core code.

---

## A) FULLY DONE

### go-cqrs-lite

- **`event/store_config.go`** (49 lines) — `Backend` type, `StoreConfig`, `StoreOption`, `NewStoreFromConfig` factory. Clean switch-based dispatch for built-in `memory` backend. External backends implement `Store` interface directly (Go idiom, no global registries).
- **`event/store_config_test.go`** (45 lines) — 3 tests: memory backend, default backend, unknown backend error.
- All 393 tests pass. Committed `de7f0de`. Pushed.

### go-localfirst

- **`internal/storage/config.go`** (76 lines) — `Backend` type (`pebble`/`memory`), `Config`, `Option`, `NewStateStore` factory. Switch-based, no Pebble in the config package itself (only in the factory case). Passes `DataDir` through config.
- **`internal/storage/memory_store.go`** (167 lines) — `MemoryTodoStore` implementing `domain.TodoRepository` with full CRUD, filtering (status/priority/tags/search), pagination, and count. Uses `strings` stdlib (not hand-rolled).
- **`internal/storage/memory_store_test.go`** (221 lines) — 13 tests covering Create, CreateDuplicate, Get, GetNotFound, Update, UpdateNotFound, Delete, DeleteNotFound, List, ListWithStatusFilter, ListWithPagination, ListWithSearch, Count, CountWithFilter.
- **`internal/storage/config_test.go`** (46 lines) — 4 tests: memory backend, default is pebble, unknown backend, memory implements interface end-to-end.
- **`internal/cqrs/store/config.go`** (68 lines) — `Backend` type (`pebble`/`memory`), `EventStoreProvider` function type, `Config`, `Option`, `NewEventStore` factory. **Zero Pebble dependency** — external backends injected via `WithProvider`. This is the key architectural win.
- **`internal/cqrs/store/config_test.go`** (55 lines) — 5 tests: memory backend, default backend, unknown backend, custom provider, provider overrides backend.
- **`cmd/api/main.go`** — Refactored to config-driven construction via `STATE_BACKEND` / `EVENT_BACKEND` env vars. Uses `domain.TodoRepository` interface everywhere (not `*PebbleStore`). Sync manager gets `repo` interface. PebbleStore only type-asserted when needed for `*pebble.DB` sharing.
- All 269 tests pass. Committed `c4913b2`. Pushed.

### go-localsync

- **`pkg/storage/config.go`** (75 lines) — `Backend` type (`sqlite`/`memory`), `Config`, `Option`, `NewStorage` factory. Validates DBPath for sqlite backend.
- **`pkg/storage/memory_storage.go`** (287 lines) — `MemoryStorage` implementing full 16-method `Storage` interface with: Upsert, UpsertBatch, GetByID, GetLatest, GetItems, GetItemsByType/Actor/Repo/Source, GetItemsSince, BatchGetByIDs, Delete, DeleteAll, Count, CountByType, GetTypes, Close. Sorted pagination, proper `ErrNotFound` errors.
- **`pkg/storage/memory_storage_test.go`** (295 lines) — 22 tests covering every method on the Storage interface.
- **`pkg/storage/config_test.go`** (42 lines) — 5 tests: memory backend, default is sqlite, sqlite without path error, unknown backend, memory implements interface.
- **`cmd/examples/github-sync/main.go`** — Added `--backend` flag, config-driven store construction via `storage.NewStorage`. Removed direct `database.Open` dependency.
- All 160 tests pass. Committed `a5484ae`. Pushed.

### Architecture Summary

| Project                | Interface               | Backends               | Config Pattern                          | Env/Flag        |
| ---------------------- | ----------------------- | ---------------------- | --------------------------------------- | --------------- |
| go-cqrs-lite           | `event.Store`           | memory                 | `NewStoreConfig` + `NewStoreFromConfig` | —               |
| go-localfirst (state)  | `domain.TodoRepository` | pebble, memory         | `storage.NewConfig` + `NewStateStore`   | `STATE_BACKEND` |
| go-localfirst (events) | `event.Store`           | pebble, memory, custom | `cqrsStore.NewConfig` + `NewEventStore` | `EVENT_BACKEND` |
| go-localsync           | `storage.Storage`       | sqlite, memory         | `storage.NewConfig` + `NewStorage`      | `--backend`     |

**Total new code**: 1,482 lines across 13 new files.
**Total new tests**: 52 new test functions (all passing).
**All three projects**: full test suites green, committed, pushed to origin/master.

---

## B) PARTIALLY DONE

### NATS JetStream backend itself

- The pluggable architecture is now in place to support NATS JetStream as a backend, but the actual NATS JetStream implementation has **not been written**. The factory/switch pattern makes this trivial to add — you'd implement the existing interface and add a case to the switch (or use `WithProvider`).

---

## C) NOT STARTED

1. **NATS JetStream event store implementation** for go-cqrs-lite (or go-localfirst)
2. **NATS JetStream storage implementation** for go-localsync
3. **PostgreSQL event store** for go-cqrs-lite (on their roadmap)
4. **SQLite event store** for go-cqrs-lite (low priority on their roadmap)
5. **Integration tests** that actually run with multiple backends (e.g., run same test suite against both pebble and memory)
6. **Benchmark tests** comparing backend performance
7. **Config validation** — deeper validation of config options before store creation
8. **Backend health checks** — standard interface for checking if a backend is alive
9. **Migration tools** — data migration between backends (e.g., sqlite → NATS)
10. **go.work file** for cross-project development (already in .gitignore for go-localsync)

---

## D) TOTALLY FUCKED UP (and fixed)

1. **Hand-rolled `toLower`/`containsIgnoreCase`** in `memory_store.go` — Initially wrote custom byte-level string functions that had a rune/byte type error. Replaced with `strings.ToLower`/`strings.Contains`. ✓ Fixed.
2. **Global mutable `storeFactories` map** in initial go-cqrs-lite design — Not goroutine-safe, overengineered. Replaced with simple switch. ✓ Fixed.
3. **`*pebble.DB` in `cqrs/store/config.go`** — Initially the config package imported Pebble directly, defeating pluggability. Replaced with `EventStoreProvider` function type. ✓ Fixed.
4. **Immediately-invoked function expression `func() *pebble.DB { ... }()`** — Unidiomatic Go anti-pattern in main.go. Replaced with clean conditional. ✓ Fixed.
5. **`stateStoreFactories` global map** in initial go-localfirst config.go — Same issue as #2. Replaced with switch. ✓ Fixed.
6. **Unused `i` in test loops** — Multiple `for i := range N` loops where `i` was unused. Fixed with `for range N` or keeping `i` when used. ✓ Fixed.
7. **Build artifact `api` binary** almost committed — Caught and excluded via `.gitignore`. ✓ Fixed.

---

## E) WHAT WE SHOULD IMPROVE

### Architecture

1. **Dual-write problem in go-localfirst** — Commands write to both event store (cqrs_event: prefix) AND state store (todo: prefix) with no transactional guarantee. If one fails, they're out of sync.
2. **Legacy `domain.EventStore` / `PebbleEventStore` is dead code** — Only the deprecated `TodoService` used it. Should be removed or officially deprecated.
3. **Event Bus is wired but never triggered** — `MemoryBus` + `SSEEventBusBridge` are set up but no code calls `eventBus.Publish()` from command handlers.
4. **go-localsync `MockStorage` in testhelpers vs `MemoryStorage`** — Mock has configurable stubs (returns fixed values); MemoryStorage has real behavior. Both exist but serve different purposes. Document the distinction.
5. **`github_id` column name** in go-localsync schema — Should be generalized to `source_id` for multi-provider support (noted in AGENTS.md as planned).

### Code Quality

6. **`strings` import inconsistency** — PebbleStore uses `strings.ToLower` for search filter but the filter logic is duplicated between PebbleStore and MemoryTodoStore.
7. **Test coverage gaps** — No tests for `cmd/api/main.go`, `internal/cqrs/commands/`, `internal/cqrs/queries/` in go-localfirst.
8. **No interface compliance test** — Should have a shared test suite that both PebbleStore and MemoryTodoStore pass (e.g., `TestTodoRepositoryCompliance`).

### Dependency Management

9. **go-localfirst `pebble` import in main.go** — Even though `cqrs/store/config.go` is Pebble-free, `main.go` still imports Pebble for the `DB()` accessor. Could use an interface.
10. **go-localsync still has `internal/database` package** — Example CLI no longer imports it, but it exists. Should it be public for custom SQLite usage?

---

## F) TOP 25 THINGS WE SHOULD GET DONE NEXT

### High Impact, Low Effort (do first)

1. ✏️ Remove dead `PebbleEventStore` / `domain.EventStore` from go-localfirst (unused in production)
2. ✏️ Wire `eventBus.Publish()` in go-localfirst command handlers (SSE is currently broken)
3. ✏️ Add shared `TodoRepositoryCompliance` test suite in go-localfirst (both backends pass same tests)
4. ✏️ Add shared `StorageCompliance` test suite in go-localsync (both backends pass same tests)
5. ✏️ Add `BackendMemory` to go-cqrs-lite's `StoreConfig` docs/README
6. ✏️ Update go-localfirst ARCHITECTURE.md with pluggable backend section
7. ✏️ Update go-localsync README.md with `--backend` flag docs

### High Impact, Medium Effort

8. 🔨 Implement NATS JetStream `event.Store` for go-cqrs-lite
9. 🔨 Implement NATS JetStream `Storage` for go-localsync
10. 🔨 Add PostgreSQL `event.Store` for go-cqrs-lite (was on their roadmap)
11. 🔨 Fix dual-write problem in go-localfirst (transactional event+state write)
12. 🔨 Add integration tests that run against both pebble and memory backends
13. 🔨 Add command/query handler tests in go-localfirst
14. 🔨 Rename `github_id` → `source_id` in go-localsync schema (migration 003)

### Medium Impact, Low Effort

15. ✏️ Add `WithProvider` example to go-localfirst cqrs/store docs
16. ✏️ Add backend health-check interface (`Ping(ctx) error`)
17. ✏️ Add `storage.BackendMemory` to go-localsync testhelpers (replace MockStorage in some tests)
18. ✏️ Document `MockStorage` vs `MemoryStorage` distinction in go-localsync
19. ✏️ Add config validation method (`cfg.Validate() error`)

### Medium Impact, Medium Effort

20. 🔨 Extract `DBProvider` interface in go-localfirst (decouple main.go from `*pebble.DB`)
21. 🔨 Add benchmark tests comparing PebbleStore vs MemoryTodoStore
22. 🔨 Add benchmark tests comparing SQLiteStorage vs MemoryStorage
23. 🔨 Add migration tool for backend switching (sqlite → memory, pebble → memory)

### Lower Priority

24. 🔨 Add `EventStreamer` implementation using NATS JetStream consumer
25. 🔨 Add `go.work` documentation for cross-project development workflow

---

## G) TOP #1 QUESTION I CANNOT FIGURE OUT MYSELF

**Should the NATS JetStream backend live in go-cqrs-lite itself, or in a separate module/repo?**

Arguments for keeping it in go-cqrs-lite:

- It's the canonical event store library; having official backends makes adoption easier
- The `Store` interface is already defined there

Arguments for separate module:

- go-cqrs-lite currently has **zero external dependencies** (only `cockroachdb/errors`, `google/uuid`, `go-json-experiment/json`). Adding `nats.go/nats.go` would break this principle
- The `WithProvider` / `EventStoreProvider` pattern already solves this — consumers inject the backend
- Separate repo allows independent versioning

**My recommendation**: Separate repo (e.g., `go-cqrs-lite-nats`), using `RegisterStoreBackend` or `WithProvider`. But this is a product decision I can't make alone.

---

## Test Results (All Green)

| Project       | Tests        | Status |
| ------------- | ------------ | ------ |
| go-cqrs-lite  | 393 PASS     | ✅     |
| go-localfirst | 269 PASS     | ✅     |
| go-localsync  | 160 PASS     | ✅     |
| **Total**     | **822 PASS** | ✅     |

## Commits Made (This Session)

| Project       | SHA       | Message                                         |
| ------------- | --------- | ----------------------------------------------- |
| go-cqrs-lite  | `de7f0de` | Add pluggable event store backend configuration |
| go-localfirst | `c4913b2` | Add pluggable storage backend architecture      |
| go-localsync  | `a5484ae` | Add pluggable storage backend architecture      |

All three pushed to `origin/master`.
