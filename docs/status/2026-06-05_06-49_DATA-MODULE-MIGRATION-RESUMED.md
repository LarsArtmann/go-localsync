# Comprehensive Status Report — go-localsync

**Date:** 2026-06-05 06:49 CEST  
**Session:** Resumed from interrupted SUPERB data module migration  
**Branch:** master (clean, no uncommitted changes)  
**Latest Commit:** `a542818` — test(cqrs,api): add integration tests + benchmarks for data layer migration

---

## a) FULLY DONE

### Data Module Foundation (Phase A — 1% → 51%)

- `pkg/data/model/` — `Item`, `Key`, `ItemView`, `StatsView`, `ProviderItem`, validation, 10 tests
- `pkg/data/query/` — `Criterion[T]`, `Query[T]`, `Page[T]`, combinators (And/Or/Not), 17 tests
- `pkg/data/transform/` — `Mapper[From,To]`, `Compose`, 9 tests
- `pkg/data/schema/` — `Version` type, 4 tests
- `pkg/data/repo/` — `Reader[T]`, `Writer[T]`, `Repository[T]`, 2 tests
- `pkg/cqrs/item_adapter.go` — bridge functions: `ToDataItem`, `FromDataItem`, `DataItemFromPayload`, `DataItemToPayload`
- `ItemSyncedPayload.SchemaVersion` field added for forward compatibility

### CQRS Decider Migration (Phase B — 4% → 64%)

- `SyncItemState.Item` migrated from `*provider.Item` → `*model.Item`
- `Fold`, `DecideSync`, `HasChanged`, `resolveConflict`, `syncEvents` all use `*model.Item`
- `SyncItemCommand.Item` is `*model.Item` + `RawJSON []byte`
- `CQRSConfig.ConflictResolver` type updated to `crdt.ConflictResolver[*model.Item]`
- `CQRSStack.SyncItems` maps `provider.Item` → `model.Item` via `ToDataItem` before dispatch
- All `pkg/cqrs/` tests migrated (17 decider tests, 5 resolver tests, 11 dispatch tests, etc.)

### Read Model Migration (Phase C — 20% → 80%, COMPLETE)

- `ReadModel` interface uses `*model.Item` for Get/List/Upsert/Delete
- `MemoryReadModel` fully migrated (filter, pagination, concurrent-safe)
- `SQLiteReadModel` fully migrated (scan, upsert, DDL)
- `Projector.handleItemSynced` uses `DataItemFromPayload` + validates items
- `stack_adapters.go` `ListItems` returns `[]*model.Item` directly (removed temporary bridge)
- Read model integration test passes (event → projection → read model)

### Sync Engine + API Migration (Phase C continued)

- `SyncStore.ListItems` returns `[]*model.Item` (not `*provider.Item`)
- `Syncer.processIncrementalItems` accepts `*model.Item` for latestItem
- `api.Server` uses `ItemResponse` DTO — no longer leaks `*provider.Item`
- `toItemResponse` maps `*model.Item` → `*ItemResponse`
- All mock stores and tests updated across 7 files

### Integration Tests + Benchmarks (Phase E)

- **6 CQRS integration tests** (`pkg/cqrs/integration_test.go`):
  - SyncItemsPipeline — full CQRS roundtrip
  - SyncItemsIdempotent — same item synced twice = 1 entry
  - SyncItemsWithConflictResolver — LWW resolver end-to-end
  - DeleteAndResurrect — deleted item re-synced reappears
  - ReadModelFilter — filtering after pipeline sync
  - SQLiteBackend — full pipeline with SQLite
- **2 API integration tests** (`pkg/api/integration_test.go`):
  - APIListItemsRoundtrip — items via HTTP API
  - APIStatsRoundtrip — aggregate statistics via API
- **7 benchmarks**:
  - Adapter: ToDataItem ~0.2 ns/op, FromDataItem ~0.2 ns/op, Roundtrip ~3.5 ns/op
  - Payload: DataItemToPayload ~50 ns/op, DataItemFromPayload ~56 ns/op
  - ReadModel: Memory ~74 µs/op (1k items), SQLite ~269 µs/op (1k items)

### Verification

- `go build ./...` — 15 packages, all compile
- `go test ./... -count=1` — 290 test functions, all green
- `go test ./... -race` — clean, no races detected
- BuildFlow pre-commit hooks (24 steps) — all passing
- golangci-lint on changed files — clean (pre-existing issues in data/ untouched)

---

## b) PARTIALLY DONE

| Area                    | What's Done                                                                       | What's Missing                                                                                           |
| ----------------------- | --------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| **Type Safety**         | Branded IDs for ItemID, ExternalID, ProviderID, ActorID, RepoID, EventTypeID      | Backend type still raw string; SyncAction still raw string; ConflictWinner still raw string              |
| **data/query**          | Beautiful generic DSL with Criterion[T], Query[T], Page[T]                        | Completely unused by read models (still use provider.ItemFilter); no integration with the actual system  |
| **data/transform**      | Mapper[From,To], Compose, 3 global vars                                           | Global vars are lint violations (gochecknoglobals); unused by CQRS layer (adapters live in cqrs package) |
| **Schema Evolution**    | SchemaVersion field on ItemSyncedPayload                                          | No upcaster infrastructure; no V1→V2 migration path; no registry                                         |
| **Conflict Resolution** | Wired into Decider; LWWResolver works; ActionConflictLocal exists                 | No CLI flag to configure resolver; ConflictAwareSyncer not tested with real CQRS stack for local-wins    |
| **API DTOs**            | ItemResponse for list endpoint                                                    | Stats endpoint still doesn't use typed DTOs for body; no error response DTOs                             |
| **Observability**       | Event logging middleware via charm log                                            | No OpenTelemetry spans; no metrics; no structured context fields in logs                                 |
| **Pagination**          | Limit/Offset in ItemFilter and API                                                | No cursor-based pagination; no pagination headers in API responses                                       |
| **CLI**                 | Config loaded from env; flags for backend, db, pages, incremental, conflict-aware | No graceful shutdown; no conflict strategy flag; no structured logging config                            |

---

## c) NOT STARTED

| #   | Item                                           | Why It Matters                                                                    |
| --- | ---------------------------------------------- | --------------------------------------------------------------------------------- |
| 1   | Cursor-based pagination (After + uint limit)   | Correct for append-only event stores; offset pagination is O(n) on large datasets |
| 2   | `schema.Upcaster` + registry for event replay  | Forward-compatible event migration; currently SchemaVersion is write-only         |
| 3   | `MetricsRecorder` middleware                   | Observability for command dispatch latency, event processing rates                |
| 4   | OpenTelemetry instrumentation                  | Production debugging; spans for Sync, CQRS, HTTP handlers                         |
| 5   | Graceful shutdown (`http.Server.Shutdown`)     | Current server hard-calls `http.ListenAndServe`; loses in-flight requests         |
| 6   | API authentication middleware                  | Anyone can POST /sync without credentials                                         |
| 7   | API rate limiting middleware                   | POST /sync is expensive; needs abuse protection                                   |
| 8   | Concurrent read model access tests             | `sync.RWMutex` is used but never tested under concurrent load                     |
| 9   | Real GitHub PAT smoke test                     | Zero end-to-end verification with real API                                        |
| 10  | `BatchMapper` utility in `pkg/data/transform/` | Pre-allocated batch transformations for 10k+ item syncs                           |
| 11  | `UnitOfWork` for atomic writes                 | Coordinate event store + read model writes atomically                             |
| 12  | SQLite indexes for query optimization          | `repo_name`, `type_created` composites exist but no performance analysis          |
| 13  | `govalid` struct tags on config structs        | Runtime validation of AppConfig, SyncOptions, CQRSConfig                          |
| 14  | ADRs (Architecture Decision Records)           | No documented rationale for CQRS, branded IDs, CRDT choices                       |
| 15  | `catalog/` from go-cqrs-lite                   | AsyncAPI/OpenAPI/D2 auto-generation from event types                              |
| 16  | `middleware.CommandRetry` from go-cqrs-lite    | Currently no retry on command dispatch failures                                   |

---

## d) TOTALLY FUCKED UP

Honestly? **Nothing is totally fucked up.** The build compiles. All 290 tests pass. The race detector is clean. Pre-commit hooks pass. The architecture is sound.

**However, these things are concerning:**

1. **Two filter systems exist side-by-side** — `provider.ItemFilter` (used everywhere) and `data/query.Criterion[T]` (used nowhere). The query DSL is beautiful dead code. This is architectural debt that will confuse future contributors.

2. **Three global Mapper variables** in `pkg/data/transform/` (`FromProviderItem`, `ToItemView`, `ProviderToView`) — these are lint violations and discourage testability. They exist but are unused by the CQRS layer.

3. **`ConfigureTursoPool` still in codebase** — `store_factory.go:71` calls `cqrsstorage.ConfigureTursoPool(db)`. Turso support was removed in the v2 migration. The function name is misleading and confusing. (Function itself does generic SQLite pool config, but the name is wrong.)

4. **`CQRSStack.GetTypes` duplicate** — `GetTypes()` and `GetItemTypes()` are identical. Two names for one method. One satisfies `SyncStore`, one is used by CLI. This is a naming smell.

5. **`pkg/data/` has 829 lines of production code** but only integrates at the edges. The query DSL, transform pipeline, and repo abstractions are not wired into the actual system. It's a "module in a box" — correct but isolated.

---

## e) WHAT WE SHOULD IMPROVE

### Immediate Wins (Next 1–2 Sessions)

**1. Type Safety — Backend, SyncAction, ConflictWinner**
These are the highest-leverage changes. They follow existing patterns (`crdt.NodeID`, `id.ProviderID`) and prevent entire classes of bugs.

**2. Move ItemFilter to data/query**
`provider.ItemFilter` is a query concern, not a provider concern. It's used by `sync`, `cqrs`, `api`, and test packages — all downstream of the provider boundary. Moving it to `data/query/` or its own package would clarify the dependency graph.

**3. Concurrent Read Model Tests**
`MemoryReadModel` uses `sync.RWMutex` but has zero concurrent access tests. Race conditions under load are a real risk.

**4. CLI Conflict Resolver Flag**
The CRDT resolver is fully wired but inaccessible. Adding `--conflict-strategy=remote-wins|lww|custom` would make the feature usable.

**5. Graceful Shutdown**
`http.Server.Shutdown(ctx)` with signal handling and request draining. Small change, big production impact.

### Medium-Term (Next 3–5 Sessions)

**6. Wire data/query into Read Models**
Either replace `provider.ItemFilter` with `data/query.Query[*model.Item]` in read models, or create a translation layer. Having two filter systems is worse than having one imperfect one.

**7. Replace Global Mapper Variables**
Change `var FromProviderItem Mapper[...]` to `func NewFromProviderItem() Mapper[...]`. Enables per-instance configuration and removes gochecknoglobals violations.

**8. OpenTelemetry Instrumentation**
Add spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, HTTP handlers. `go.opentelemetry.io/otel` is already an indirect dependency.

**9. API Error Path Tests**
Current API tests only cover happy paths. Store failures, malformed requests, and edge cases need coverage.

**10. Schema Upcasters**
Wire `schema.Upcaster` into the projection replay path. Currently `SchemaVersion` is written but never read during replay.

---

## f) Top #25 Things To Get Done Next

Sorted by **Impact / Effort** ratio (highest first):

| #   | Task                                                                                | Impact | Effort | Package                                                                  |
| --- | ----------------------------------------------------------------------------------- | ------ | ------ | ------------------------------------------------------------------------ |
| 1   | **Backend branded type** — replace raw string in CQRSConfig/AppConfig/store_factory | High   | Low    | `pkg/cqrs/`, `cmd/examples/`                                             |
| 2   | **SyncAction branded type** — prevent typos in switch statements                    | High   | Low    | `pkg/sync/`, `pkg/cqrs/`, `pkg/api/`                                     |
| 3   | **Concurrent read model tests** — test RWMutex under load                           | High   | Low    | `pkg/cqrs/`                                                              |
| 4   | **Move ItemFilter to data/query** — correct package boundary                        | High   | Medium | `pkg/provider/`, `pkg/data/query/`, `pkg/cqrs/`, `pkg/sync/`, `pkg/api/` |
| 5   | **ConflictWinner branded type** — replace raw string                                | Medium | Low    | `pkg/cqrs/`                                                              |
| 6   | **CLI conflict resolver flag** — make resolver configurable                         | Medium | Low    | `cmd/examples/`                                                          |
| 7   | **Graceful shutdown** — drain in-flight requests                                    | Medium | Low    | `cmd/examples/`                                                          |
| 8   | **mapSyncError() tests** — table-driven error→HTTP mapping                          | Medium | Low    | `pkg/api/`                                                               |
| 9   | **HasChanged table-driven tests** — edge cases in field comparison                  | Medium | Low    | `pkg/cqrs/`                                                              |
| 10  | **Extract shared testutil helpers** — DRY test item factories                       | Medium | Low    | `internal/testutil/`, multiple test files                                |
| 11  | **Replace global Mapper vars with factory funcs** — remove gochecknoglobals         | Medium | Low    | `pkg/data/transform/`                                                    |
| 12  | **Split sync.go** — extract interface, constants, types                             | Low    | Low    | `pkg/sync/`                                                              |
| 13  | **Consolidate GetTypes/GetItemTypes** — remove duplicate                            | Low    | Low    | `pkg/cqrs/`                                                              |
| 14  | **Rename ConfigureTursoPool** — remove misleading name                              | Low    | Low    | `pkg/cqrs/`                                                              |
| 15  | **OpenTelemetry spans** — production observability                                  | Medium | Medium | `pkg/sync/`, `pkg/cqrs/`, `pkg/api/`                                     |
| 16  | **API error path tests** — store failures, malformed requests                       | Medium | Medium | `pkg/api/`                                                               |
| 17  | **Cursor pagination** — replace limit/offset with After+uint                        | High   | High   | `pkg/cqrs/`, `pkg/api/`, `pkg/data/query/`                               |
| 18  | **Schema upcasters** — V1→V2 event replay migration                                 | Medium | High   | `pkg/data/schema/`, `pkg/cqrs/`                                          |
| 19  | **MetricsRecorder middleware** — command/event metrics                              | Medium | Medium | `pkg/cqrs/`                                                              |
| 20  | **Real GitHub smoke test** — end-to-end with PAT                                    | High   | High   | `cmd/examples/`                                                          |
| 21  | **BatchMapper utility** — pre-allocated batch transforms                            | Low    | Low    | `pkg/data/transform/`                                                    |
| 22  | **SQLite query optimization** — analyze slow queries, add indexes                   | Low    | Low    | `pkg/cqrs/`                                                              |
| 23  | **API authentication middleware** — API key or JWT                                  | Medium | Medium | `pkg/api/`                                                               |
| 24  | **API pagination headers** — X-Total-Count, Link header                             | Medium | Medium | `pkg/api/`                                                               |
| 25  | **ADRs** — document CQRS, branded ID, CRDT decisions                                | Low    | Low    | `docs/adr/`                                                              |

---

## g) Top #1 Question I Cannot Figure Out Myself

### The Filter Dilemma: Two Systems, One Domain

We have **two filter/query systems** in the codebase:

1. **`provider.ItemFilter`** — concrete struct with pointer fields (`Type *id.EventTypeID`, `Since *time.Time`, `Limit int`, `Offset int`). Used by `ReadModel.List()`, `SyncStore.ListItems()`, API handlers, and all test mocks. Simple, imperative, works.

2. **`data/query.Criterion[T]` + `Query[T]`** — beautiful generic DSL with `HasSource`, `HasType`, `And`, `Or`, `Not`, `Sort`, `Limit`, `Build()`. Has both in-memory `Match()` and SQL `ToSQL()` generation. Zero production usage.

**The question:** What is the migration strategy?

**Option A: Replace `provider.ItemFilter` entirely**

- Update `ReadModel` interface to accept `query.Query[*model.Item]`
- Update all callers (sync, cqrs, api, tests)
- Breaks all existing code at once; big-bang refactor
- The generic DSL is more powerful but also more verbose for simple cases

**Option B: Keep `provider.ItemFilter` as the public API, add internal translation**

- Read models still accept `provider.ItemFilter`
- Internally, build a `query.Query` from the filter fields
- Zero breaking changes to callers
- But then why have the DSL at all? It's just an internal implementation detail.

**Option C: Hybrid — `provider.ItemFilter` for simple cases, `query.Query` for advanced**

- `ReadModel.List(filter provider.ItemFilter)` for basic filtering
- Add `ReadModel.Query(q query.Query[*model.Item])` for complex queries
- Two entry points = API surface bloat

**Option D: Deprecate data/query and remove it**

- The DSL is beautiful but YAGNI for this project
- Simple struct filters are idiomatic Go and sufficient
- But it feels wrong to delete well-tested, well-designed code

I genuinely don't know which path is correct without understanding the product vision:

- Will we need complex AND/OR queries across multiple fields?
- Will we support multiple storage backends (PostgreSQL, Elasticsearch) where a generic query compiler pays off?
- Is the query DSL premature abstraction, or is it laying groundwork?

**This is a product/architecture question, not a technical one.** The code works either way. The choice depends on where the project is going.

---

## Metrics Snapshot

| Metric                     | Value                                                                                  |
| -------------------------- | -------------------------------------------------------------------------------------- |
| Production Go files        | 89 files, ~4,817 lines                                                                 |
| Test Go files              | 41 files, ~6,894 lines                                                                 |
| Test functions             | 290                                                                                    |
| Packages                   | 15 (14 testable, all green)                                                            |
| Lint issues (full project) | 13 (3 exhaustruct, 3 gochecknoglobals, 2 gosec, 1 nilnil, 1 noinlineerr, 3 varnamelen) |
| Race detector              | Clean                                                                                  |
| BuildFlow pre-commit       | 24/24 steps passing                                                                    |

---

## Commit History (Last 5)

```
a542818 test(cqrs,api): add integration tests + benchmarks for data layer migration
6d92be1 feat(cqrs): complete Phase C — migrate SyncStore.ListItems + API to data.Item
1865389 feat(cqrs): migrate read models from provider.Item to data.Item
9f8a78d feat(cqrs): migrate decider core from provider.Item to data.Item
94d0659 docs(planning): add SUPERB data module execution plan with Pareto breakdown
```

---

## Next Actions (Pending User Decision)

1. **Resolve the Filter Dilemma** — Need product/architecture guidance on data/query fate
2. **Pick type safety improvements** — Backend branded type is the simplest starting point
3. **Decide on OpenTelemetry vs structured logging vs both** — Observability strategy
4. **Prioritize cursor pagination** — Only if we expect >10k items per user

---

_Report generated automatically. All data verified against current working tree._
