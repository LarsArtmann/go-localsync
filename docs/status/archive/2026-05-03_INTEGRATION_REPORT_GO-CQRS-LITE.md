# Integration Report: go-localsync ↔ go-cqrs-lite

**Date:** 2026-05-03
**Scope:** Dependency alignment, ID type compatibility, storage/event patterns, integration blockers

---

## Executive Summary

**Integration level: 0% actual, ~80% architecturally ready.** go-localsync does not import go-cqrs-lite at all. The only connection is shared `go-branded-id v0.1.0` and documentation comments referencing alignment. But the ID types were deliberately converged in the last session (ItemID → ULID-backed), and go-localfirst already demonstrates a working integration pattern.

---

## 1. Dependency Alignment

| Dependency           | go-localsync     | go-cqrs-lite/core | go-localfirst        |
| -------------------- | ---------------- | ----------------- | -------------------- |
| `go-branded-id`      | `v0.1.0`         | `v0.1.0`          | `v0.1.0` (indirect)  |
| `oklog/ulid`         | **`v2.1.1`**     | `v2.1.0`          | `v2.1.0` (indirect)  |
| `go-cqrs-lite/core`  | **not imported** | —                 | `v0.1.1`             |
| `cockroachdb/errors` | `v1.13.0`        | `v1.12.0`         | `v1.12.0` (indirect) |

**Issue 1**: `oklog/ulid` version drift — localsync is on `v2.1.1`, cqrs-lite declares `v2.1.0`. Go modules will resolve this to `v2.1.1` (MVS picks highest), so functionally benign but should be aligned explicitly.

**Issue 2**: `cockroachdb/errors` drift — `v1.13.0` vs `v1.12.0`. Again MVS resolves, but indicates localsync is ahead.

**Issue 3**: go-cqrs-lite has **257 commits since `v0.1.1`** (the last published tag). go-localfirst is pinned to `v0.1.1` — significantly behind HEAD. New features like `Publisher`/`Subscriber` ISP split, `RegisterClassification`, `decider` package, error taxonomy, panic recovery are all unreleased.

---

## 2. ID Type Compatibility

This is the most critical integration dimension. Here's the exact type comparison:

| Concept                | go-localsync                           | go-cqrs-lite                                      |
| ---------------------- | -------------------------------------- | ------------------------------------------------- |
| **Core type**          | `id.ID[B, V]` (go-branded-id directly) | `id.Of[T]` = `cbid.ID[T, ulid.ULID]` (type alias) |
| **ULID-backed ID**     | `id.ID[ItemBrand, ulid.ULID]`          | `cbid.ID[T, ulid.ULID]`                           |
| **String-backed ID**   | `id.ID[ExternalBrand, string]`         | Not supported (`Of[T]` is ULID-only)              |
| **Internal structure** | `struct{ value ulid.ULID }`            | `struct{ value ulid.ULID }` (identical)           |

**Both are `cbid.ID[Brand, ulid.ULID]` with different phantom brands.** go-cqrs-lite's `id.Of[T]` is a type alias (`=`) to `cbid.ID[T, ulid.ULID]`, NOT a wrapper struct. This means the memory layout is identical — `struct{ value ulid.ULID }` everywhere.

**Conversion is trivial**: Extract the ULID via `.Get()`, re-wrap with the target brand:

```go
// localsync ItemID → cqrs-lite AggregateID
aggregateID := cbid.NewID[aggregateMarker](itemID.Get())

// cqrs-lite AggregateID → localsync ItemID
itemID := id.NewID[ItemBrand](aggregateID.Get())
```

**This is not free at the type level** (phantom brands prevent accidental mixing — by design), but it is zero-cost at runtime (just copying a `ulid.ULID`). The pattern is identical to go-localfirst's `domain.TodoID` ↔ `id.AggregateID` conversion at aggregate boundaries.

---

## 3. go-localfirst as Reference Integration

go-localfirst successfully integrates go-cqrs-lite and provides a proven pattern:

| Aspect          | go-localfirst approach                                    | Relevance to go-localsync                               |
| --------------- | --------------------------------------------------------- | ------------------------------------------------------- |
| **Aggregate**   | Embeds `*aggregate.Core`, implements `Root` interface     | Use `Decider[State]` instead (recommended by cqrs-lite) |
| **Event Store** | `CQRSAdapter` wraps Pebble (in `pkg/cqrs/store/`)         | Extract to shared package or copy                       |
| **Event Bus**   | `memory.NewMemoryBus()` from cqrs-lite                    | Direct reuse                                            |
| **Commands**    | `command.Dispatcher` + handler pattern                    | Direct reuse                                            |
| **Queries**     | `query.Dispatcher` + handler pattern                      | Direct reuse                                            |
| **Read model**  | Custom projection framework (`pkg/projection/`)           | Use cqrs-lite's `projection.Runner` instead             |
| **Branded IDs** | Converts `domain.TodoID` ↔ `id.AggregateID` at boundaries | Same pattern for `types.ItemID` ↔ `id.AggregateID`      |
| **Wiring**      | `samber/do/v2` DI container                               | Simpler — factory function or manual wiring             |

---

## 4. What Would Integration Look Like

Per the existing `CQRS_MIGRATION_PLAN.md`, the target architecture is:

```
Provider → SyncItemCommand → SyncItem Decider → event.Store (Pebble/Memory)
                                                    ↓
                                              event.Bus
                                                    ↓
                                          ReadModel Projection
                                                    ↓
                                          Query Dispatcher → Caller
```

### Components from go-cqrs-lite to reuse

| Component                                         | Import         | Status                         |
| ------------------------------------------------- | -------------- | ------------------------------ |
| `decider.Decider[State]` + `Repository[State]`    | `core/decider` | Production-ready (Session 37+) |
| `event.Store` interface                           | `core/event`   | Stable                         |
| `event.Bus` + `Publisher`/`Subscriber` ISP        | `core/event`   | Stable (Session 44)            |
| `memory.MemoryStore` + `MemoryBus`                | `memory`       | Production-ready               |
| `projection.Runner` + `HandlerRegistry`           | `projection`   | Production-ready               |
| `command.Dispatcher`                              | `core/command` | Stable                         |
| `query.Dispatcher` + `Pagination`                 | `core/query`   | Stable                         |
| `event.Error` taxonomy + `RegisterClassification` | `core/event`   | Production-ready (Session 31+) |
| `middleware.CommandRetry` + `CommandLogging`      | `middleware`   | Production-ready               |

### Components to build in go-localsync

| Component                             | Effort | Notes                                     |
| ------------------------------------- | ------ | ----------------------------------------- |
| `SyncItem` decider (fold + decide)    | Medium | Pure functions, ~100 lines                |
| Event types + payloads                | Easy   | ~80 lines                                 |
| Command types + handlers              | Medium | ~200 lines                                |
| Query types + handlers                | Medium | ~200 lines                                |
| ReadModel interface + implementations | Medium | ~300 lines                                |
| Pebble event store adapter            | Easy   | Copy from go-localfirst `pkg/cqrs/store/` |
| Syncer refactor to use CQRS           | Medium | Wire commands instead of storage directly |
| ID bridge functions                   | Easy   | ~30 lines                                 |

---

## 5. Concrete Blockers

| # | Blocker                                                                  | Severity | Resolution                                                                              |
| - | ------------------------------------------------------------------------ | -------- | --------------------------------------------------------------------------------------- |
| 1 | **go-localsync does not import go-cqrs-lite**                            | Critical | Add dependency to `go.mod`                                                              |
| 2 | **ID brand conversion** — `ItemBrand` vs `AggregateMarker` phantom types | Low      | Simple `cbid.NewID[Brand](id.Get())` at boundaries (identical to go-localfirst pattern) |
| 3 | **go-cqrs-lite v0.1.1 is 257 commits behind HEAD**                       | High     | Publish `v0.2.0` or use pseudo-version; local `go.work` handles dev                     |
| 4 | **Pebble adapter in go-localfirst is in `pkg/` but not extracted**       | Medium   | Extract to shared package or copy into localsync                                        |
| 5 | **`oklog/ulid` version drift** (v2.1.0 vs v2.1.1)                        | Low      | Align in go-cqrs-lite's `go.mod`                                                        |
| 6 | **No `event.Store` implementation for SQLite**                           | Low      | Use Pebble (migration target) or memory for tests                                       |

---

## 6. Recommended Next Steps (Pareto Order)

| Priority | Step                                                          | Impact                               | Effort |
| -------- | ------------------------------------------------------------- | ------------------------------------ | ------ |
| **P0**   | Publish go-cqrs-lite `v0.2.0` (257 commits of improvements)   | Unlocks all integration              | 30min  |
| **P0**   | Add `go-cqrs-lite/core` + `memory` to go-localsync `go.mod`   | Foundation for everything            | 5min   |
| **P1**   | Build ID bridge: `types.ItemID` ↔ `id.AggregateID` conversion | Unlocks aggregate usage              | 15min  |
| **P1**   | Copy Pebble `event.Store` adapter from go-localfirst          | Unlocks non-memory storage           | 1hr    |
| **P2**   | Build `SyncItem` decider + event types                        | Core CQRS logic                      | 2hr    |
| **P2**   | Build projection read model                                   | Replaces 16-method Storage interface | 2hr    |
| **P3**   | Refactor `Syncer` to use `command.Dispatcher`                 | Wire it together                     | 2hr    |
| **P3**   | Delete `internal/database/`, `internal/db/`, `sql/`           | Remove ~2000 lines of duplication    | 1hr    |

---

## 7. Verdict

**The integration is architecturally ready but not yet started.** The ULID migration in go-localsync was the critical prerequisite — it's done. The ID types share the same underlying `ulid.ULID` value, making conversion trivial. go-localfirst provides a proven reference pattern for the exact same integration.

The biggest risk is **go-cqrs-lite's unpublished changes** — 257 commits of improvements since `v0.1.1` including the `decider` package (recommended approach), error taxonomy, and ISP improvements. Publishing `v0.2.0` should be the first action.

---

## Resolution (2026-09-05)

Integration completed 2026-05-03 and deepened through v2/v3/v4 (now the v4.9 stack, v0.5.0). 'Integration level 0%' is historical. Pebble was never adopted (SQLite became the production backend); the go-cqrs-lite v0.2.0 publishing items were superseded by the v2-v4 major-version model. No live items remain.
