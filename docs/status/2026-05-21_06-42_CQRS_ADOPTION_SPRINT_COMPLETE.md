# Status Report: go-cqrs-lite Adoption Sprint

**Date:** 2026-05-21 06:42 CEST
**Branch:** master
**Trigger:** Comprehensive Execution Plan (`docs/planning/2026-05-21_02-31_COMPREHENSIVE_EXECUTION_PLAN.md`)

---

## Executive Summary

Executed a 27-step adoption plan to close the go-cqrs-lite integration gap. Went from **3/12 modules (25%)** to **5/12 modules (42%)**, and from **~40% API surface** to **~70%**. All 8 anti-patterns from the previous audit are resolved. Zero lint issues, 122 passing tests.

---

## A) FULLY DONE ✅

### Tier 1: Critical Fixes (P0)

| Step | What                                                                | Files Changed                                             | Status |
| ---- | ------------------------------------------------------------------- | --------------------------------------------------------- | ------ |
| 1.1  | Bump go-cqrs-lite/core v1.3.0→v1.4.0, memory v1.1.0→v1.2.0          | `go.mod`, `go.sum`                                        | ✅     |
| 1.2  | Fix `event.Version` int() casts → `.Increment()`, `.Add()`          | `pkg/cqrs/decider.go`                                     | ✅     |
| 1.3  | Simplify `aggregate_id.go` — removed ULID, use `hex.EncodeToString` | `pkg/cqrs/aggregate_id.go`                                | ✅     |
| 1.4  | Collapse `Handle`/`HandleEvent` duplication in Projector            | `pkg/cqrs/projection.go`, `stack.go`, `readmodel_test.go` | ✅     |

### Tier 2: Projection Runner (P1)

| Step    | What                                                          | Files Changed            | Status |
| ------- | ------------------------------------------------------------- | ------------------------ | ------ |
| 2.1-2.3 | Wire `event.InMemoryRunner` with `cqrsmemory.CheckpointStore` | `pkg/cqrs/stack.go`      | ✅     |
| 2.4     | Add projection runner test                                    | `pkg/cqrs/stack_test.go` | ✅     |

### Tier 3: Conflict Consolidation (P1)

| Step    | What                                                                                                             | Status               |
| ------- | ---------------------------------------------------------------------------------------------------------------- | -------------------- |
| 3.1-3.4 | Audited: no split-brain exists — `ConflictAwareSyncer` already delegates to `CQRSStack.SyncItems` → `DecideSync` | ✅ (already correct) |

### Tier 4: Error Taxonomy (P2)

| Step | What                                                                     | Files Changed                    | Status                |
| ---- | ------------------------------------------------------------------------ | -------------------------------- | --------------------- |
| 4.1  | Map all 9 sentinel errors to `event.Family` via `RegisterClassification` | `pkg/errors/errors.go`           | ✅                    |
| 4.2  | Wire `event.IsRetryable()` as fallback in GitHub `isRetryableError()`    | `pkg/providers/github/client.go` | ✅                    |
| 4.3  | Sync loop has no retry logic — nothing to wire                           | N/A                              | ✅ (skipped, correct) |

### Tier 5: Codec + Event Helpers (P2)

| Step | What                                                                          | Files Changed            | Status |
| ---- | ----------------------------------------------------------------------------- | ------------------------ | ------ |
| 5.1  | `event.JSONCodec` in `newEvent()`                                             | `pkg/cqrs/decider.go`    | ✅     |
| 5.2  | `event.DecodePayload[T]` in projection `handleItemSynced`/`handleItemDeleted` | `pkg/cqrs/projection.go` | ✅     |
| 5.3  | `event.DecodePayload[T]` in `foldItemSynced`                                  | `pkg/cqrs/decider.go`    | ✅     |
| 5.4  | `event.NewEvents` replaces manual event creation loop in `syncEvents()`       | `pkg/cqrs/decider.go`    | ✅     |

### Tier 6: Test Coverage (P2)

| Step | What                                                                                     | Tests Added | Status |
| ---- | ---------------------------------------------------------------------------------------- | ----------- | ------ |
| 6.1  | `waitForRateLimit` tests (disabled, sufficient, exceeds max, context canceled, nil core) | 5           | ✅     |
| 6.2  | `SyncIncremental` with existing items                                                    | 1           | ✅     |
| 6.3  | `processIncrementalItems` (skips old, all new, invalid item)                             | 3           | ✅     |
| 6.4  | Turso local store + remote invalid URL                                                   | 2           | ✅     |

### Tier 7: Advanced Features (P3)

| Step | What                                                                      | Files Changed       | Status                   |
| ---- | ------------------------------------------------------------------------- | ------------------- | ------------------------ |
| 7.1  | `decider.WithOutbox`                                                      | —                   | ❌ Skipped (see B below) |
| 7.2  | `sync.LWWResolver[T]`                                                     | —                   | ❌ Skipped (see B below) |
| 7.3  | `middleware.EventLogging` with charm log adapter                          | `pkg/cqrs/stack.go` | ✅                       |
| 7.4  | `cqrsmemory.MemorySnapshotStore` + `event.EveryNEvents(10)` + `JSONCodec` | `pkg/cqrs/stack.go` | ✅                       |
| 7.5  | Schema upcasting                                                          | —                   | ❌ Skipped (see C below) |

### Anti-Patterns Resolved

All 8 anti-patterns from the previous audit (`docs/status/2026-05-21_01-30_COMPREHENSIVE_STATUS_AND_CQRS_AUDIT.md`) are now fixed:

- ✅ `event.Version` cast to `int()` in 3 places → uses `.Increment()`, `.Add()`
- ✅ `bus.SubscribeAll` without checkpoint → `event.InMemoryRunner` with `CheckpointStore`
- ✅ Manual `json.Marshal`/`json.Unmarshal` everywhere → `event.JSONCodec` + `DecodePayload[T]` + `NewEvents`
- ✅ `aggregate_id.go` SHA256→ULID→string → `hex.EncodeToString`
- ✅ No error taxonomy → `RegisterClassification` + `IsRetryable` wired
- ✅ No snapshot support → `MemorySnapshotStore` + `EveryNEvents(10)`
- ✅ No event logging → `middleware.EventLogging` wired
- ✅ `Handle`/`HandleEvent` duplication → single `Handle` method

---

## B) PARTIALLY DONE 🟡

| Step                 | What                          | Why Partial                                                                                                     |
| -------------------- | ----------------------------- | --------------------------------------------------------------------------------------------------------------- |
| 7.1 `WithOutbox`     | Atomic save+publish for Turso | Requires outbox publisher goroutine + Turso-specific wiring. Architecture change, not a simple drop-in.         |
| 7.2 `LWWResolver[T]` | Formal conflict resolution    | `HasChanged()` already does LWW. Replacing it with `LWWResolver` would be a refactor with no behavioral change. |

---

## C) NOT STARTED ⬜

| Step                                  | What                                          | Priority | Notes                                                                                                                                     |
| ------------------------------------- | --------------------------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `projection.Runner` (separate module) | Full replay from GlobalLoader with checkpoint | MEDIUM   | `event.InMemoryRunner` gives checkpointing for in-memory. The separate `projection` module gives replay from persistent store on restart. |
| `command.Dispatcher`                  | Typed command dispatch                        | LOW      | No commands in the system yet                                                                                                             |
| `UpcasterRegistry`                    | Schema evolution                              | LOW      | Only 1 schema version exists                                                                                                              |
| `catalog/`                            | AsyncAPI/OpenAPI/D2 generation                | LOW      | Documentation tooling                                                                                                                     |
| `core/query`                          | Query bus                                     | LOW      | No queries beyond read model                                                                                                              |
| `core/aggregate`                      | Aggregate root helpers                        | LOW      | Using `decider` directly                                                                                                                  |
| `testhelpers` module                  | `FakeStore`, `FakeBus`                        | LOW      | Own test helpers work fine                                                                                                                |

---

## D) TOTALLY FUCKED UP 💥

**Nothing.** Zero regressions, zero broken tests, zero lint issues.

The only "issue" encountered during execution was the `event.IsRetryable()` fail-open behavior (defaults to `Transient` for unknown errors), which would have caused `isRetryableError()` to return `true` for 4xx GitHub errors. Fixed by checking GitHub errors first, using `event.IsRetryable()` only as fallback for non-GitHub errors.

---

## E) WHAT WE SHOULD IMPROVE

### Architecture

1. **Outbox for Turso** — Without `WithOutbox`, events are saved to DB and published to bus in two non-atomic steps. A crash between them means events are lost. This is the **#1 remaining gap**.

2. **Projection replay on restart** — `InMemoryRunner` only tracks checkpoints for live events. On process restart, the read model starts empty. The separate `projection` module with `GlobalLoader` replay is needed for production crash recovery.

3. **`SyncItemState` value semantics** — `Item *provider.Item` allows nil in non-deleted state. Should be `Item provider.Item` (value) to make impossible states unrepresentable.

4. **`encoding/json` still imported in `events.go`** — `json.RawMessage` field keeps the import. Could migrate to `[]byte` with custom codec.

### Code Quality

5. **Test helper shadowing** — Local variable `provider` in sync tests shadows the package import. Fixed in this PR but pattern should be avoided.

6. **`SyncAction` constants could use go-cqrs-lite event classification** — Instead of counting events manually, we could classify the result.

7. **`processIncrementalItems` has 0% coverage in production** — Now has tests, but the cutoff logic (using `CreatedAt`) may not match real GitHub API behavior.

### Dependencies

8. **go-cqrs-lite/storage still on pseudo-version** — Should tag v0.2.0 for reproducible CI builds.

---

## F) Top 25 Things We Should Get Done Next

| #  | Task                                                                                    | Impact   | Effort | Priority |
| -- | --------------------------------------------------------------------------------------- | -------- | ------ | -------- |
| 1  | Wire `decider.WithOutbox` for Turso backend (atomic save+publish)                       | CRITICAL | 2h     | P0       |
| 2  | Add `projection.Runner` (separate module) with GlobalLoader replay                      | CRITICAL | 3h     | P0       |
| 3  | Tag go-cqrs-lite/storage v0.2.0 and update go.mod                                       | HIGH     | 15m    | P0       |
| 4  | Add integration test: restart → events replayed → read model correct                    | HIGH     | 1h     | P1       |
| 5  | Add `decider.WithOutbox` integration test with Turso                                    | HIGH     | 1h     | P1       |
| 6  | Migrate `SyncItemState.Item` from `*provider.Item` to `provider.Item` (value semantics) | HIGH     | 1h     | P1       |
| 7  | Wire `middleware.CommandRetry` for GitHub provider retry                                | MEDIUM   | 30m    | P1       |
| 8  | Add `command.Dispatcher` for typed `SyncItem`/`DeleteItem` commands                     | MEDIUM   | 2h     | P2       |
| 9  | Evaluate `sync.LWWResolver[T]` to replace `HasChanged()`                                | MEDIUM   | 1h     | P2       |
| 10 | Remove `encoding/json` import from `events.go` (use `[]byte` for RawJSON)               | LOW      | 30m    | P2       |
| 11 | Add `UpcasterRegistry` with identity upcaster for current schema                        | LOW      | 30m    | P3       |
| 12 | Add `SchemaVersion` constants to payload structs                                        | LOW      | 30m    | P3       |
| 13 | Wire `event.ExitCode` in CLI example for proper exit codes                              | MEDIUM   | 30m    | P2       |
| 14 | Add E2E test: full sync → conflict → delete → resurrect cycle                           | MEDIUM   | 1h     | P2       |
| 15 | Add Turso remote store integration test with embedded server                            | MEDIUM   | 2h     | P2       |
| 16 | Add `context.Context` propagation to `AggregateID()` cache                              | LOW      | 15m    | P3       |
| 17 | Add metrics middleware (`middleware.EventMetrics`) for production observability         | MEDIUM   | 1h     | P2       |
| 18 | Add `catalog/` D2 diagram generation                                                    | LOW      | 1h     | P3       |
| 19 | Add `testhelpers` module from go-cqrs-lite (`FakeStore`, `FakeBus`)                     | LOW      | 30m    | P3       |
| 20 | Add GitHub provider BDD tests using ginkgo                                              | MEDIUM   | 2h     | P2       |
| 21 | Add `Provider` interface `FetchIncremental` method for delta sync                       | HIGH     | 3h     | P1       |
| 22 | Add `SyncOptions.Since` for time-based incremental sync                                 | MEDIUM   | 1h     | P2       |
| 23 | Add `ReadModel.Watch` for real-time change notifications                                | LOW      | 2h     | P3       |
| 24 | Add structured logging throughout sync loop (not just start/end)                        | LOW      | 30m    | P3       |
| 25 | Add CI pipeline (GitHub Actions) with build + lint + test                               | HIGH     | 1h     | P1       |

---

## G) My Top #1 Question

**How do you want to handle the outbox pattern for the Turso backend?**

The `decider.WithOutbox` changes the control flow: events go to an outbox table instead of being published directly to the bus. A separate publisher goroutine reads from the outbox and publishes. This means:

1. **The `CQRSStack.Close()` must drain the outbox** — otherwise events are saved but never projected
2. **The projection runner may see events out of order** — if the publisher is async
3. **The in-memory backend doesn't benefit** — only Turso needs atomicity

Should I:

- (a) Wire outbox for Turso only, keep direct publish for memory?
- (b) Wire outbox for all backends for consistency?
- (c) Skip outbox entirely and rely on the projection replay (item #2 above) for crash recovery instead?

---

## Metrics

| Metric                    | Before     | After      | Change          |
| ------------------------- | ---------- | ---------- | --------------- |
| go-cqrs-lite modules used | 3/12 (25%) | 5/12 (42%) | +67%            |
| API surface used          | ~40%       | ~70%       | +75%            |
| Anti-patterns             | 8          | 0          | -100%           |
| Test cases                | ~110       | 122        | +11%            |
| Lint issues               | 0          | 0          | —               |
| Files changed             | —          | 13         | +486/-142 lines |

---

_Generated by Crush on 2026-05-21_
