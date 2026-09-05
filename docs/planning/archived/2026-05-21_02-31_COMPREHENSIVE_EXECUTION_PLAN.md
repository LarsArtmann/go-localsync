# Comprehensive Execution Plan: go-localsync go-cqrs-lite Adoption

**Date:** 2026-05-21 02:31
**Goal:** Close the adoption gap from 40% → 80%+ API surface usage
**Constraint:** Each step ≤ 12 minutes, self-contained, immediately commitable

---

## Philosophy

> **Reuse before rewrite.** Every go-cqrs-lite feature we adopt replaces hand-rolled code.
> **Types before tests.** Fix type safety issues first — they prevent bugs.
> **Tests before features.** Cover untested production paths before adding new ones.

---

## Tier 1: CRITICAL FIXES (P0 — Do First)

These fix actual bugs or type safety violations. Zero risk, immediate value.

### Step 1.1: Bump go-cqrs-lite dependency versions

**What:** Update `go.mod` to core v1.4.0, memory v1.2.0, storage v0.2.0.
**Why:** CI runs against older APIs. New features (Version methods, TypedProjection, LoadToVersion) are unavailable in CI.
**Existing code fits:** Just go.mod edits. No code changes needed — v1.4.0 is backward compatible with v1.3.0 for our usage.
**Effort:** 5 min | **Impact:** HIGH | **Customer value:** CI correctness
**Files:** `go.mod`, `go.sum`

### Step 1.2: Fix `event.Version` int() casts

**What:** Replace 3 `int(version)` casts in `decider.go` with `version.Increment()` and `version.Add()`.
**Why:** Breaks phantom type safety. The library added these methods specifically to eliminate this anti-pattern.
**Existing code fits:** The `syncEvents()` function already does manual version math — we just use the library method.
**Effort:** 8 min | **Impact:** MEDIUM | **Customer value:** Type safety
**Files:** `pkg/cqrs/decider.go`
**Lines:** 119 (`int(currentVersion)+1` → `currentVersion.Increment()`), 143 (`int(version)` → keep as `Version` param), 170 (`ver+len(events)+1` → `version.Add(len(events)+1)`)

### Step 1.3: Simplify `aggregate_id.go` SHA256→ULID round-trip

**What:** Remove ULID dependency from aggregate ID generation. Since `id.AggregateID` is string-backed, use `fmt.Sprintf("%x", sha256[:16])` directly.
**Why:** Unnecessary complexity. 4 lines of crypto + ULID when 1 line of hex encoding suffices.
**Existing code fits:** `id.ParseAggregateID()` accepts any non-empty string.
**Effort:** 10 min | **Impact:** LOW | **Customer value:** Code clarity
**Files:** `pkg/cqrs/aggregate_id.go`
**Note:** Remove `github.com/oklog/ulid/v2` from direct deps if no longer used elsewhere.

### Step 1.4: Collapse Projector `Handle`/`HandleEvent` duplication

**What:** `Handle` delegates to `HandleEvent` — remove one. Subscribe the interface method (`Handle`) to the bus.
**Why:** Indirection with no value. The `event.Projection` interface defines `Handle`.
**Existing code fits:** `Projector` already implements `event.Projection`.
**Effort:** 8 min | **Impact:** LOW | **Customer value:** Code clarity
**Files:** `pkg/cqrs/projection.go`, `pkg/cqrs/stack.go`

---

## Tier 2: PROJECTION RUNNER (P1 — Crash Recovery)

This is the #1 architectural gap. Currently events are silently lost on restart.

### Step 2.1: Add projection module to go.mod

**What:** Add `github.com/larsartmann/go-cqrs-lite/projection` dependency.
**Why:** `projection.Runner` lives in its own module (not `core/`).
**Effort:** 3 min | **Impact:** HIGH
**Files:** `go.mod`

### Step 2.2: Add checkpoint table DDL to Turso schema

**What:** Add `projection_checkpoints` table to `initTursoDB` / `SQLiteSchema()` call.
**Why:** `projection.Runner` needs a `CheckpointStore`.
**Existing code fits:** `turso_readmodel.go` already manages DDL. We add one more table.
**Effort:** 10 min | **Impact:** HIGH
**Files:** `pkg/cqrs/stack.go`

### Step 2.3: Wire `projection.Runner` in CQRSStack

**What:** Replace `bus.SubscribeAll(proj.HandleEvent)` with `projection.NewRunner(...)` + `runner.Register(proj)` + `bus.SubscribeAll(runner.Handle)`.
**Why:** Gives replay + checkpoint + event type filtering + crash recovery.
**Existing code fits:** `Projector` already satisfies `event.Projection`. `CQRSStack` already has Store+Bus.
**Effort:** 12 min | **Impact:** CRITICAL
**Files:** `pkg/cqrs/stack.go`

### Step 2.4: Update projection tests for Runner

**What:** Add tests that verify runner replay behavior (simulate crash → restart → events replayed).
**Why:** The runner is new behavior — it needs test coverage.
**Effort:** 12 min | **Impact:** HIGH
**Files:** `pkg/cqrs/stack_test.go` or new `pkg/cqrs/projection_runner_test.go`

---

## Tier 3: CONFLICT CONSOLIDATION (P1 — Split-Brain Fix)

The #1 architectural smell. Two independent conflict detectors.

### Step 3.1: Audit current conflict flow

**What:** Trace exactly how `ConflictAwareSyncer` and `DecideSync` interact. Document the data flow.
**Why:** Before changing, understand. The split-brain means they can disagree.
**Existing code fits:** `decider.go` already emits `ItemConflictFound` events. `ConflictAwareSyncer` already wraps `Syncer`.
**Effort:** 10 min | **Impact:** HIGH (research)
**Files:** `pkg/sync/conflict_aware.go`, `pkg/cqrs/decider.go`, `pkg/cqrs/stack.go`

### Step 3.2: Make `DecideSync` the single authority

**What:** Ensure `DecideSync` always emits `ItemConflictFound` when `HasChanged()` is true. Remove any secondary conflict detection.
**Why:** The decider owns the event-sourced state. It must be the single source of truth.
**Existing code fits:** `DecideSync` already does this. We just verify it's the ONLY place.
**Effort:** 8 min | **Impact:** HIGH
**Files:** `pkg/cqrs/decider.go`

### Step 3.3: Refactor `ConflictAwareSyncer` to read events

**What:** Change `ConflictAwareSyncer` from "independently detect conflicts" to "read conflict events from the stream". Use `repo.Load()` to get aggregate state + events, count `ItemConflictFound` events.
**Why:** Eliminates the split-brain. The syncer becomes a reporting layer, not a decision layer.
**Existing code fits:** `decider.Repository.Load()` returns state + version. Events are in the store.
**Effort:** 12 min | **Impact:** CRITICAL
**Files:** `pkg/sync/conflict_aware.go`

### Step 3.4: Update conflict tests

**What:** Rewrite `sync_test.go` to test the new flow: sync → check events → count conflicts.
**Why:** Tests must reflect the consolidated architecture.
**Effort:** 12 min | **Impact:** HIGH
**Files:** `pkg/sync/sync_test.go`

---

## Tier 4: ERROR TAXONOMY (P2 — Smart Retry)

Enables programmatic error handling and proper retry logic.

### Step 4.1: Map sentinel errors to event.Family

**What:** In `pkg/errors/errors.go`, add `Classify()` method that maps each sentinel to an `event.Family`.
**Why:** The library provides 5 families. Our errors are flat.
**Existing code fits:** `pkg/errors/errors.go` already has all sentinels. We add a classifier.
**Effort:** 10 min | **Impact:** MEDIUM
**Files:** `pkg/errors/errors.go`

### Step 4.2: Wire `IsRetryable` in GitHub client

**What:** Replace `isRetryableError()` logic with `event.IsRetryable(err)`.
**Why:** Centralizes retry classification. Business errors (Rejection) don't retry. Transient errors do.
**Existing code fits:** `github/client.go:361` already has `isRetryableError`.
**Effort:** 8 min | **Impact:** MEDIUM
**Files:** `pkg/providers/github/client.go`

### Step 4.3: Wire `IsRetryable` in sync loop

**What:** In `Syncer.Sync()`, use `event.IsRetryable(err)` to decide whether to retry or abort.
**Why:** The sync loop currently treats all errors the same.
**Existing code fits:** `Syncer` already has error handling in `fetchItems()`.
**Effort:** 8 min | **Impact:** MEDIUM
**Files:** `pkg/sync/sync.go`

---

## Tier 5: CODEC + EVENT HELPERS (P2 — DRY)

Eliminates scattered json.Marshal/Unmarshal and manual version math.

### Step 5.1: Adopt `event.JSONCodec` in `newEvent()`

**What:** Replace `json.Marshal(payload)` with `event.JSONCodec{}.Encode(payload)`.
**Why:** Centralizes serialization. Future-proof for protobuf/etc.
**Existing code fits:** `newEvent()` is the single factory for events.
**Effort:** 8 min | **Impact:** LOW
**Files:** `pkg/cqrs/decider.go`

### Step 5.2: Adopt `DecodePayload[T]` in projection

**What:** Replace `json.Unmarshal(evt.Payload(), &payload)` with `event.DecodePayload[ItemSyncedPayload](evt, event.JSONCodec{})`.
**Why:** Type-safe deserialization. Eliminates 2 `json.Unmarshal` calls.
**Existing code fits:** `handleItemSynced` and `handleItemDeleted` both unmarshal payloads.
**Effort:** 8 min | **Impact:** LOW
**Files:** `pkg/cqrs/projection.go`

### Step 5.3: Adopt `DecodePayload[T]` in Fold

**What:** Replace `json.Unmarshal(evt.Payload(), &payload)` in `foldItemSynced` with `DecodePayload`.
**Why:** DRY. Same deserialization pattern in 3 places.
**Existing code fits:** `foldItemSynced` in `decider.go`.
**Effort:** 8 min | **Impact:** LOW
**Files:** `pkg/cqrs/decider.go`

### Step 5.4: Adopt `event.NewEvents` in `syncEvents()`

**What:** Replace manual event creation loop with `event.NewEvents(...)`.
**Why:** Auto-versioning, auto-marshaling, batch creation. Replaces ~15 lines.
**Existing code fits:** `syncEvents()` creates 1-2 events with sequential versions.
**Effort:** 10 min | **Impact:** LOW
**Files:** `pkg/cqrs/decider.go`

---

## Tier 6: TEST COVERAGE (P2 — Production Safety)

Cover the undertested production paths.

### Step 6.1: Add `waitForRateLimit` tests

**What:** Test rate limit wait logic with mocked clock.
**Why:** 10.5% coverage on the function that prevents API abuse.
**Existing code fits:** `client_test.go` already has mock HTTP servers.
**Effort:** 12 min | **Impact:** HIGH
**Files:** `pkg/providers/github/client_test.go`

### Step 6.2: Add `SyncIncremental` tests

**What:** Test incremental sync with mocked provider returning partial results.
**Why:** 37.5% coverage on the production sync path.
**Existing code fits:** `sync_test.go` already has mock providers.
**Effort:** 12 min | **Impact:** HIGH
**Files:** `pkg/sync/sync_test.go`

### Step 6.3: Add `processIncrementalItems` tests

**What:** Test the item filtering + batching logic in incremental sync.
**Why:** 0% coverage. This is where items are filtered and batched.
**Existing code fits:** `sync_test.go` has the test infrastructure.
**Effort:** 12 min | **Impact:** HIGH
**Files:** `pkg/sync/sync_test.go`

### Step 6.4: Add Turso remote store tests

**What:** Test `createTursoRemoteStore` with a temporary SQLite file.
**Why:** 0% coverage on the remote sync path.
**Existing code fits:** `turso_readmodel_test.go` already tests Turso.
**Effort:** 12 min | **Impact:** HIGH
**Files:** `pkg/cqrs/stack_test.go` or new test file

---

## Tier 7: ADVANCED FEATURES (P3 — Future-Proofing)

### Step 7.1: Wire `decider.WithOutbox`

**What:** Pass `decider.WithOutbox(outbox)` when creating Repository for Turso backend.
**Why:** Atomic save+publish. Prevents event loss on crash.
**Existing code fits:** `stack.go` already creates the Repository.
**Effort:** 12 min | **Impact:** MEDIUM
**Files:** `pkg/cqrs/stack.go`

### Step 7.2: Evaluate `sync.LWWResolver[T]`

**What:** Spike replacing `HasChanged()` with `sync.LWWResolver[provider.Item]`.
**Why:** Formal conflict resolution with vector clocks.
**Existing code fits:** `HasChanged()` already does timestamp comparison (LWW).
**Effort:** 12 min | **Impact:** MEDIUM
**Files:** `pkg/cqrs/decider.go`, `pkg/sync/conflict_aware.go`

### Step 7.3: Wire `middleware.EventLogging`

**What:** Add `EventLogging` middleware to the event bus.
**Why:** Structured logging of all domain events.
**Existing code fits:** `stack.go` creates the bus. Just add `bus.Use(...)`.
**Effort:** 8 min | **Impact:** LOW
**Files:** `pkg/cqrs/stack.go`

### Step 7.4: Add snapshot support

**What:** Wire `decider.WithSnapshotStore` + `event.EveryNEvents(10)`.
**Why:** Cap replay cost for frequently-synced items.
**Existing code fits:** `stack.go` creates the Repository. Add snapshot store creation.
**Effort:** 12 min | **Impact:** LOW
**Files:** `pkg/cqrs/stack.go`

### Step 7.5: Add schema upcasting registration

**What:** Create `UpcasterRegistry` and register identity upcaster for current schema.
**Why:** Future-proof event schema changes.
**Existing code fits:** No current upcasters needed — just wire the infrastructure.
**Effort:** 10 min | **Impact:** LOW
**Files:** `pkg/cqrs/stack.go`

---

## Summary Table (Sorted by Priority × Impact / Effort)

| Step | Task                              | Tier | Effort | Impact   | Customer Value      | Files             |
| ---- | --------------------------------- | ---- | ------ | -------- | ------------------- | ----------------- |
| 1.1  | Bump go-cqrs-lite versions        | P0   | 5m     | HIGH     | CI correctness      | go.mod            |
| 1.2  | Fix Version int() casts           | P0   | 8m     | MEDIUM   | Type safety         | decider.go        |
| 2.1  | Add projection module dep         | P1   | 3m     | HIGH     | Crash recovery      | go.mod            |
| 2.2  | Add checkpoint DDL                | P1   | 10m    | HIGH     | Crash recovery      | stack.go          |
| 2.3  | Wire projection.Runner            | P1   | 12m    | CRITICAL | Crash recovery      | stack.go          |
| 3.2  | Make DecideSync single authority  | P1   | 8m     | HIGH     | Data integrity      | decider.go        |
| 3.3  | Refactor ConflictAwareSyncer      | P1   | 12m    | CRITICAL | Data integrity      | conflict_aware.go |
| 6.1  | Test waitForRateLimit             | P2   | 12m    | HIGH     | API safety          | client_test.go    |
| 6.2  | Test SyncIncremental              | P2   | 12m    | HIGH     | Production path     | sync_test.go      |
| 6.3  | Test processIncrementalItems      | P2   | 12m    | HIGH     | Production path     | sync_test.go      |
| 6.4  | Test Turso remote store           | P2   | 12m    | HIGH     | Remote sync         | stack_test.go     |
| 4.1  | Map errors to event.Family        | P2   | 10m    | MEDIUM   | Smart retry         | errors.go         |
| 4.2  | Wire IsRetryable in client        | P2   | 8m     | MEDIUM   | Smart retry         | client.go         |
| 4.3  | Wire IsRetryable in sync          | P2   | 8m     | MEDIUM   | Smart retry         | sync.go           |
| 3.1  | Audit conflict flow               | P1   | 10m    | HIGH     | Research            | —                 |
| 3.4  | Update conflict tests             | P1   | 12m    | HIGH     | Data integrity      | sync_test.go      |
| 2.4  | Test projection replay            | P1   | 12m    | HIGH     | Crash recovery      | stack_test.go     |
| 1.3  | Simplify aggregate_id.go          | P0   | 10m    | LOW      | Clarity             | aggregate_id.go   |
| 1.4  | Collapse Handle/HandleEvent       | P0   | 8m     | LOW      | Clarity             | projection.go     |
| 5.1  | Adopt JSONCodec in newEvent       | P2   | 8m     | LOW      | DRY                 | decider.go        |
| 5.2  | Adopt DecodePayload in projection | P2   | 8m     | LOW      | DRY                 | projection.go     |
| 5.3  | Adopt DecodePayload in Fold       | P2   | 8m     | LOW      | DRY                 | decider.go        |
| 5.4  | Adopt NewEvents in syncEvents     | P2   | 10m    | LOW      | DRY                 | decider.go        |
| 7.1  | Wire WithOutbox                   | P3   | 12m    | MEDIUM   | Atomicity           | stack.go          |
| 7.2  | Evaluate LWWResolver              | P3   | 12m    | MEDIUM   | Conflict resolution | decider.go        |
| 7.3  | Wire EventLogging middleware      | P3   | 8m     | LOW      | Observability       | stack.go          |
| 7.4  | Add snapshot support              | P3   | 12m    | LOW      | Performance         | stack.go          |
| 7.5  | Add upcasting infra               | P3   | 10m    | LOW      | Future-proofing     | stack.go          |

**Total: 27 steps, ~4.5 hours estimated, all ≤ 12 min each.**

---

## Type Model Improvements (Cross-Cutting)

Throughout execution, apply these type improvements:

1. **`event.Version` everywhere** — No `int` for versions. Use `.Increment()`, `.Add()`, `.Cmp()`.
2. **`event.SchemaVersion` for payloads** — Tag payload structs with schema version constants.
3. **Value semantics for `SyncItemState`** — Consider `Item provider.Item` (value) instead of `*provider.Item` (pointer) to make impossible states unrepresentable (nil item in non-deleted state).
4. **Branded types for more fields** — `Source`, `IPAddress`, `UserAgent` from go-cqrs-lite already exist. Consider using them in metadata.

## Libraries to Leverage

| Library                     | Feature                                                                                      | Where We Use It                      |
| --------------------------- | -------------------------------------------------------------------------------------------- | ------------------------------------ |
| `go-cqrs-lite/core/event`   | `Version`, `SchemaVersion`, `Codec`, `DecodePayload`, `NewEvents`, `Classify`, `IsRetryable` | decider.go, projection.go, errors.go |
| `go-cqrs-lite/core/decider` | `WithOutbox`, `WithSnapshotStore`, `WithCodec`, `LoadAtVersion`                              | stack.go                             |
| `go-cqrs-lite/projection`   | `Runner`, `CheckpointStore`                                                                  | stack.go                             |
| `go-cqrs-lite/middleware`   | `EventLogging`, `CommandRetry`, `EventRecovery`                                              | stack.go, client.go                  |
| `go-cqrs-lite/sync`         | `LWWResolver`, `VectorClock`                                                                 | decider.go, conflict_aware.go        |
| `go-cqrs-lite/testhelpers`  | `FakeStore`, `FakeBus`                                                                       | Tests                                |

**No new external dependencies needed.** Everything is in go-cqrs-lite.

---

_Generated by Crush on 2026-05-21_

---

## Resolution (2026-09-05)

Superseded the same evening by the 2026-05-21 20-11 sprint, which re-planned the Tier 2/3 items. Outcome of the shared items: conflict consolidation + projection runner + outbox shipped 2026-05-21/22 (Turso machinery later deleted in v0.2.0); `LWWResolver` wired 2026-05-29; `query.Pagination` rejected by design (no query dispatcher — see the ADR note in `stack_adapters.go`). No live items remain here; anything still open lives in TODO_LIST.md.
