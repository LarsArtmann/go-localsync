# Strong ID Migration & NewEvents Batch API — Status Report

**Date:** 2026-05-23 00:48
**Session Focus:** Branded ID enforcement for `sourceID`/`SourceID` fields (branching-flow strong-id remediation)

---

## A) FULLY DONE

### 1. `event.NewEvents` / `event.MustNewEvents` — Batch Event Creation (go-cqrs-lite)

- **Created** `core/event/batch.go` in go-cqrs-lite with `NewEvents()` and `MustNewEvents()`
- Accepts `[]event.Type` + `[]any` payloads, increments version per event
- Added `ErrMismatchedEventCount` sentinel error
- All 197 tests pass — the two `undefined: event.NewEvents` compilation errors in `decider.go:118,168` are **resolved**

### 2. Strong ID Migration — 10 of 16 violations fixed

The `branching-flow strong-id` tool originally reported **16 violations** where raw `string` was used for IDs that should use branded types. We fixed **10 of 16**:

**Before:** 16 violations
**After:** 6 violations (62.5% reduction)

| Area                           | Change                                            | Files                 |
| ------------------------------ | ------------------------------------------------- | --------------------- |
| `AggregateID()` signature      | `string` → `types.ExternalID`                     | `aggregate_id.go`     |
| `itemKey()` signature          | `string` → `types.ExternalID`                     | `aggregate_id.go`     |
| `DecideDelete()` signature     | `string` → `types.ExternalID`                     | `decider.go`          |
| `ReadModel.Get()` interface    | `string` → `types.ExternalID`                     | `readmodel.go`        |
| `ReadModel.Delete()` interface | `string` → `types.ExternalID`                     | `readmodel.go`        |
| `MemoryReadModel` impl         | Updated to match interface                        | `memory_readmodel.go` |
| `TursoReadModel` impl          | Updated to match interface                        | `turso_readmodel.go`  |
| `DeleteItemCommand.SourceID`   | `string` → `types.ExternalID`                     | `commands_queries.go` |
| `GetItemQuery.SourceID`        | `string` → `types.ExternalID`                     | `commands_queries.go` |
| `CQRSStack.DeleteItem()`       | `string` → `types.ExternalID`                     | `stack.go`            |
| `CQRSStack.SyncItem()`         | Pass `item.ExternalID` directly                   | `stack.go`            |
| `handleSyncItem()`             | Pass `item.ExternalID` directly                   | `commands_queries.go` |
| `Projector.Handle()` delete    | Wrap with `types.NewExternalID()`                 | `projection.go`       |
| All test files                 | Wrap string literals with `types.NewExternalID()` | 7 test files          |

### 3. All Tests Green

```
ok  github.com/larsartmann/go-localsync/cmd/examples/github-sync  0.004s
ok  github.com/larsartmann/go-localsync/pkg/cqrs                   3.079s
ok  github.com/larsartmann/go-localsync/pkg/errors                 0.002s
ok  github.com/larsartmann/go-localsync/pkg/provider               0.001s
ok  github.com/larsartmann/go-localsync/pkg/providers/github       0.016s
ok  github.com/larsartmann/go-localsync/pkg/sync                   0.007s
ok  github.com/larsartmann/go-localsync/pkg/types                  0.002s
```

197 total tests, all passing. Build compiles clean.

---

## B) PARTIALLY DONE

### 1. Strong ID — 6 remaining violations

The remaining 6 violations are in **event payloads** and **internal scan structs**:

| # | Location                 | Field                               | Issue                           |
| - | ------------------------ | ----------------------------------- | ------------------------------- |
| 1 | `events.go:19`           | `ItemSyncedPayload.ItemID`          | `string` → should be branded    |
| 2 | `events.go:21`           | `ItemSyncedPayload.SourceID`        | `string` → should be branded    |
| 3 | `events.go:34`           | `ItemConflictFoundPayload.SourceID` | `string` → should be branded    |
| 4 | `events.go:42`           | `ItemDeletedPayload.SourceID`       | `string` → should be branded    |
| 5 | `stack.go:213`           | `ItemSyncResult.SourceID`           | `string` → should be branded    |
| 6 | `turso_readmodel.go:235` | `scannedItem.sourceID`              | `string` → internal scan struct |

**Why not done:** These are JSON-serialized event payload structs. Changing them requires careful consideration of:

- JSON tag compatibility (the serialized form must stay `string`)
- go-cqrs-lite codec integration (`event.JSONCodec` must still work)
- Backward compatibility with stored events

### 2. `types.SourceID` brand added but unused

We added `SourceBrand` / `SourceID` / `NewSourceID` to `pkg/types/ids.go` but ended up using `types.ExternalID` instead (which already existed and semantically matches the "external ID from provider" concept). The `SourceID` brand is dead code and should be removed.

---

## C) NOT STARTED

1. **Event payload branded types** — `ItemSyncedPayload`, `ItemConflictFoundPayload`, `ItemDeletedPayload` still use raw `string` for `ItemID` and `SourceID` fields
2. **`ItemSyncResult.SourceID`** in `stack.go:213` — still raw `string`
3. **`scannedItem.sourceID`** in `turso_readmodel.go:235` — internal scan struct, lower priority
4. **FEATURES.md update** — Branded IDs row needs update (now 7 phantom-type IDs, not 6)
5. **AGENTS.md update** — Needs to reflect the new `ExternalID` parameter changes and the `NewEvents` batch API
6. **go-cqrs-lite upstream release** — `batch.go` was created locally but not tagged/released

---

## D) TOTALLY FUCKED UP

1. **Dead `SourceID` brand** — We added `SourceBrand`, `SourceID`, and `NewSourceID` to `ids.go` but never used them. They should be removed. This is noise in the type system.
2. **LSP still shows stale errors** — `gopls` and `golangci_lint_ls` still report `undefined: event.NewEvents` from `decider.go` even though `go build` and `go test` succeed. This is because the LSP is using the published `v1.4.0` tag of go-cqrs-lite, not the local workspace. The local `go.work` has the fix, but the LSP doesn't respect `go.work` for the storage module's transitive dependency on core. **Annoying but not blocking.**

---

## E) WHAT WE SHOULD IMPROVE

1. **Remove dead `SourceID` brand** — Clean up `ids.go`, remove `SourceBrand`, `SourceID`, `NewSourceID`
2. **Decide on event payload strategy** — Should event payloads use branded types with custom JSON marshaling, or stay as raw strings for serialization simplicity? This is an architectural decision.
3. **Update AGENTS.md** — Reflect the `ExternalID` parameter changes across the CQRS layer
4. **Tag go-cqrs-lite release** — `batch.go` needs to be released so CI works without `go.work`
5. **Fix LSP stale diagnostics** — Either tag a new core release or configure LSP to use workspace mode
6. **Consistent naming** — `SourceID` in event payloads vs `ExternalID` in domain types. The event payload field is called `SourceID` (maps to JSON `"sourceId"`) but the domain type is `ExternalID`. This split-brain naming should be reconciled.

---

## F) Top #25 Things We Should Get Done Next

### High Priority (Type Safety & Correctness)

| # | Task                                                                    | Impact       | Effort |
| - | ----------------------------------------------------------------------- | ------------ | ------ |
| 1 | Remove dead `SourceID` brand from `ids.go`                              | Cleanup      | 5min   |
| 2 | Decide: branded types in event payloads vs raw strings                  | Architecture | 30min  |
| 3 | If branded payloads: add `MarshalJSON`/`UnmarshalJSON` to branded types | Type safety  | 1hr    |
| 4 | If branded payloads: update all 3 payload structs                       | Type safety  | 1hr    |
| 5 | Tag go-cqrs-lite `core/v1.5.0` with `NewEvents` + `MustNewEvents`       | Release      | 15min  |
| 6 | Update `go.mod` to use tagged version, remove `go.work` replace         | CI fix       | 10min  |
| 7 | Update AGENTS.md with ExternalID parameter changes                      | Docs         | 15min  |
| 8 | Update FEATURES.md branded IDs row (6→7 types, +ExternalID params)      | Docs         | 10min  |

### Medium Priority (Architecture & DX)

| #  | Task                                                                  | Impact        | Effort |
| -- | --------------------------------------------------------------------- | ------------- | ------ |
| 9  | Adopt `sync.LWWResolver[T]` from go-cqrs-lite for conflict resolution | Formalize     | 2hr    |
| 10 | Adopt `sync.VectorClock` for conflict detection                       | Correctness   | 2hr    |
| 11 | Add `middleware.CommandRetry` for transient provider errors           | Resilience    | 1hr    |
| 12 | Add Prometheus metrics middleware for command/query dispatch          | Observability | 1hr    |
| 13 | Add circuit breaker for provider API calls                            | Resilience    | 2hr    |
| 14 | Write BDD tests for sync conflict flows using ginkgo                  | Confidence    | 2hr    |
| 15 | Add `UpcasterRegistry` for schema evolution (prepare for v2 payloads) | Future-proof  | 1hr    |

### Lower Priority (Polish & Scale)

| #  | Task                                                                  | Impact        | Effort |
| -- | --------------------------------------------------------------------- | ------------- | ------ |
| 16 | Generate D2 architecture diagrams via `catalog/` module               | Docs          | 1hr    |
| 17 | Add provider interface versioning (v2 with context, v1 compat)        | Extensibility | 2hr    |
| 18 | Write integration test with real Turso remote (not just local SQLite) | Confidence    | 1hr    |
| 19 | Add benchmark suite for event store operations                        | Performance   | 2hr    |
| 20 | Add snapshot strategy benchmark (EveryNEvents tuning)                 | Performance   | 1hr    |
| 21 | Add `cmd/examples/` for GitLab provider                               | DX            | 3hr    |
| 22 | Add graceful shutdown with drain for `SyncItems` in progress          | Production    | 2hr    |
| 23 | Add structured error codes to sync results (machine-readable)         | API           | 1hr    |
| 24 | Add `ItemSyncResult` to include `AggregateID` for debugging           | Debugging     | 15min  |
| 25 | Write CONTRIBUTING.md with provider development guide                 | OSS           | 1hr    |

---

## G) Top #1 Question I Cannot Figure Out Myself

**Should event payload structs use branded types?**

The remaining 4 strong-id violations are in event payload structs (`ItemSyncedPayload`, `ItemConflictFoundPayload`, `ItemDeletedPayload`). These are serialized to JSON and stored in the event store. The `sourceId` field maps to `ExternalID` in the domain model.

**Option A:** Keep payloads as raw `string` — simpler JSON, no custom marshaling, backward-compatible. The branded type enforcement happens at the decider/readmodel boundary (which we already did).

**Option B:** Use branded types in payloads with custom `MarshalJSON`/`UnmarshalJSON` — full type safety everywhere, but adds complexity and risks breaking stored event compatibility.

**My recommendation:** Option A (keep payloads as raw strings). The type safety boundary is already enforced at the CQRS layer entry points. Event payloads are internal serialization format, not a public API. But this is an architectural decision I shouldn't make alone.

---

## File Change Summary

### go-localsync (15 files, +67 -54 lines)

| File                           | Change                                                                      |
| ------------------------------ | --------------------------------------------------------------------------- |
| `pkg/types/ids.go`             | Added `SourceBrand`/`SourceID`/`NewSourceID` (dead code — should remove)    |
| `pkg/cqrs/aggregate_id.go`     | `itemKey()` + `AggregateID()` now take `types.ExternalID`                   |
| `pkg/cqrs/commands_queries.go` | `DeleteItemCommand.SourceID` + `GetItemQuery.SourceID` → `types.ExternalID` |
| `pkg/cqrs/decider.go`          | `DecideDelete()` takes `types.ExternalID`                                   |
| `pkg/cqrs/readmodel.go`        | `ReadModel.Get()` + `Delete()` take `types.ExternalID`                      |
| `pkg/cqrs/memory_readmodel.go` | Updated impl to match interface                                             |
| `pkg/cqrs/turso_readmodel.go`  | Updated impl to match interface                                             |
| `pkg/cqrs/projection.go`       | Wraps `payload.SourceID` with `types.NewExternalID()`                       |
| `pkg/cqrs/stack.go`            | `DeleteItem()` + `SyncItem()` + `SyncItems()` use `types.ExternalID`        |
| 7 test files                   | Updated string literals to `types.NewExternalID()`                          |

### go-cqrs-lite (2 files, new)

| File                   | Change                                                      |
| ---------------------- | ----------------------------------------------------------- |
| `core/event/batch.go`  | New: `NewEvents()` + `MustNewEvents()` batch event creation |
| `core/event/errors.go` | Added `ErrMismatchedEventCount` sentinel                    |

---

## Strong ID Violations Progress

```
Before:  ████████████████████ 16 violations
After:   ███████░░░░░░░░░░░░░  6 violations (62.5% fixed)
```

Remaining: 4 in event payloads (architectural decision needed) + 1 in scan struct + 1 in result struct.

---

## Resolution (2026-09-05)

Strong-ID migration + NewEvents shipped (v0.1.0); the go-cqrs-lite tagging items superseded by the v2-v4 major model; VectorClock items moot (CRDT machinery deleted in v0.4.0 per ADR-0004); circuit-breaker/ginkgo ideas routed to ROADMAP. No live items remain.
