# Status Report — Session 10: Semantic Code De-duplication Sprint

**Date:** 2026-06-04 23:03
**Session:** 10
**Author:** Crush (AI assistant)
**Focus:** `art-dupl` semantic code de-duplication at threshold 30

---

## Executive Summary

Reduced code duplication from **9 clone groups** to **0 clone groups** at threshold 30. Created `pkg/testutil/` as a new shared test helpers package. Net reduction of **100 lines** across 18 test files. All 384 tests pass. Lint clean (2 pre-existing issues unrelated to this session).

---

## a) FULLY DONE

### Session 10 Work (This Session)

| # | What | Files Changed | Impact |
|---|------|---------------|--------|
| 1 | **Created `pkg/testutil/` package** | `testutil.go`, `syncstore.go` | Shared `MustNoError`, `AssertEqual[T]`, `AssertExternalID`, `AssertType`, `SyncStoreListBehavior` — eliminates cross-package test helper duplication |
| 2 | **Extracted `testStateWithTimestamp` helper** | `pkg/cqrs/testing_test.go` | Replaces 4 inline `SyncItemState{Item: &provider.Item{...}}` blocks in decider tests |
| 3 | **Extracted `newUpdatedAtLWWResolver` helper** | `pkg/cqrs/testing_test.go` | Replaces 4 duplicated `crdt.NewLWWResolver[*provider.Item](...)` + error check blocks |
| 4 | **Extracted `newVCTestConflict` + `newTiedVCTestConflict` helpers** | `pkg/crdt/conflict_test.go` | Replaces 4 duplicated `&Conflict[testItem]{Local/Remote/LocalVC/RemoteVC}` blocks |
| 5 | **Refactored BDD field assertions to table-driven** | `pkg/providers/github/client_bdd_test.go` | Eliminated 4 identical assertion clones with different data |
| 6 | **Migrated all test helpers to `testutil` package** | 11 test files across `pkg/cqrs`, `pkg/providers/github` | All calls now use `testutil.MustNoError`, `testutil.AssertEqual`, etc. |
| 7 | **Embedded `SyncStoreListBehavior` in mock stores** | `pkg/api/server_test.go`, `pkg/sync/sync_test.go` | Shared `ListItems()` implementation, eliminated identical error-check pattern |
| 8 | **Removed unused imports** | `decider_resolver_test.go`, `stack_classify_test.go` | Removed `pkg/id`, `pkg/crdt` imports after delegating to helpers |

### Pre-existing (Sessions 1–9)

| Area | Status |
|------|--------|
| CQRS Stack (event sourcing, decider, projection, snapshots, checkpoints) | FULLY_FUNCTIONAL |
| Memory + SQLite backends | FULLY_FUNCTIONAL |
| Provider architecture (generic interface + GitHub) | FULLY_FUNCTIONAL |
| CRDT conflict resolution (LWW, vector clocks, pluggable) | FULLY_FUNCTIONAL |
| Branded IDs (6 phantom types) | FULLY_FUNCTIONAL |
| Structured errors (go-error-family) | FULLY_FUNCTIONAL |
| HTTP API (Huma v2, 4 endpoints) | FULLY_FUNCTIONAL |
| CLI example with server mode | FULLY_FUNCTIONAL |
| go-cqrs-lite v2 migration | FULLY_FUNCTIONAL |
| turso→sqlite rename | FULLY_FUNCTIONAL |
| Command/Query dispatch with typed commands | FULLY_FUNCTIONAL |

---

## b) PARTIALLY DONE

| Item | Status | What's Missing |
|------|--------|----------------|
| `pkg/testutil` test coverage | No tests for testutil itself | Functions are trivial wrappers, but godoc and a basic test would be nice |
| Test framework unification | 6 files still use testify patterns | Ginkgo removed from direct use; testify assertions remain in go-cqrs-lite indirect |
| `cmd/examples/github-sync` coverage | 13.7% | Helpers tested, but main flow (`runSync`, `runStats`, signal handling) is untested |

---

## c) NOT STARTED

From TODO_LIST.md — items with no code:

1. **E2E integration test** — Provider → CQRS → read model → API round-trip
2. **Concurrent read model access tests** — Race condition verification for `MemoryReadModel`
3. **CLI flag for conflict resolver** — `--conflict-strategy=remote-wins|lww|custom`
4. **`HasChanged` table-driven edge case tests** — Subtle field comparison bugs could hide
5. **Performance benchmarks** — No perf data exists for 1k/10k/100k item sync
6. **OpenTelemetry instrumentation** — No observability beyond structured logging
7. **Real GitHub PAT smoke test** — All testing is mock-based
8. **Export to JSON/CSV** — No export functionality
9. **Multiple user sync** — Only single user supported
10. **Daemon/background mode** — Only one-off sync or HTTP server mode
11. **Bubble Tea TUI** — Not started
12. **Conflict resolution per-sync override** — `SyncOptions.ConflictResolver`
13. **Real-time sync protocol** — `SyncRequest`/`SyncResponse` from `pkg/crdt/`

---

## d) TOTALLY FUCKED UP

| Item | Status | Details |
|------|--------|---------|
| Nothing this session | Clean | All 384 tests pass, 0 lint regressions, 0 clone groups at threshold 30 |

### Pre-existing Issues (Not From This Session)

| Item | Location | Details |
|------|----------|---------|
| `nilnil` lint warning | `pkg/cqrs/runner.go:22` | `return nil, nil` — should use sentinel error |
| `noinlineerr` lint warning | `pkg/provider/provider_test.go:151` | Inline error handling pattern |
| go-cqrs-lite upstream WIP | `go.mod` | `Sink→EventSink` rename + `Source` type collision in upstream. Blocks dep upgrades. |

---

## e) WHAT WE SHOULD IMPROVE

### Immediate (This Sprint's Observations)

1. **`pkg/testutil` has no tests** — The test helpers themselves aren't tested. A simple `TestMustNoError_Passes` / `TestMustNoError_Fails` would catch regressions.
2. **gopls LSP diagnostics are stale** — 121 "undefined" errors shown in diagnostics despite clean `go build` / `go test`. LSP cache is stale after bulk renames.
3. **Test item factories differ across packages** — `testItem()` in `pkg/cqrs/testing_test.go`, `testItem()` in `pkg/api/server_test.go`, `testSyncItem()` in `pkg/sync/sync_test.go`. Should consolidate into `testutil.NewTestItem()`.

### Architectural

4. **`CQRSStack.GetTypes` vs `GetItemTypes` duplication** — Two methods that do the same thing. `GetItemTypes` satisfies `SyncStore`; `GetTypes` is used by CLI stats. Should consolidate.
5. **Mock stores still duplicated across packages** — `mockSyncStore` exists in `pkg/api/`, `pkg/sync/`, `pkg/cqrs/` (different fields but same pattern). `testutil.SyncStoreListBehavior` is a start but full mock extraction would eliminate more.
6. **`pkg/sync/sync.go` at 348 lines** — Near the 350-line soft limit. Contains interface, constants, types, and core logic. Should split.

### Testing Gaps

7. **No E2E test** — Everything is mock-based. No test verifies the full pipeline: provider → syncer → CQRS → read model → API response.
8. **No concurrent access tests** — `MemoryReadModel` uses `sync.RWMutex` but no `-race` test exercises it under contention.
9. **No performance benchmarks** — Unknown how system scales past trivial sizes.

---

## f) Top 25 Things to Do Next

### High Priority (Blocking / Quality)

| # | Task | Package | Effort | Impact |
|---|------|---------|--------|--------|
| 1 | Add E2E integration test (Provider → CQRS → RM → API) | cross-cutting | ~2h | High — first test that verifies everything works together |
| 2 | Add concurrent read model access test with `-race` | `pkg/cqrs` | ~30min | High — catch data races before production |
| 3 | Fix `nilnil` lint: `runner.go:22` sentinel error | `pkg/cqrs` | ~5min | Low — but keeps lint at 0 |
| 4 | Fix `noinlineerr` lint: `provider_test.go:151` | `pkg/provider` | ~5min | Low — but keeps lint at 0 |
| 5 | Resolve go-cqrs-lite upstream WIP (Sink→EventSink rename) | `go.mod` | ~2h | High — unblocks future dep upgrades |
| 6 | Add `HasChanged` table-driven edge case tests | `pkg/cqrs` | ~30min | Medium — subtle bugs hide in field comparison |
| 7 | Verify `ConflictAwareSyncer` local-wins path in integration | `pkg/sync` + `pkg/cqrs` | ~30min | Medium — new code path never tested end-to-end |

### Medium Priority (Architecture / DRY)

| # | Task | Package | Effort | Impact |
|---|------|---------|--------|--------|
| 8 | Consolidate test item factories into `testutil.NewTestItem()` | cross-cutting | ~1h | Medium — 3 different test item constructors exist |
| 9 | Add `pkg/testutil` tests (MustNoError, AssertEqual) | `pkg/testutil` | ~20min | Low — trivial but good hygiene |
| 10 | Split `pkg/sync/sync.go` (348 lines) into focused files | `pkg/sync` | ~30min | Medium — approaching soft limit |
| 11 | Remove `CQRSStack.GetTypes` duplicate, keep `GetItemTypes` | `pkg/cqrs` | ~15min | Low — two names for same method |
| 12 | Extract full `MockSyncStore` to `pkg/testutil` | cross-cutting | ~1h | Medium — 3 packages have their own mock |
| 13 | Add CLI flag `--conflict-strategy` for resolver selection | `cmd/examples` | ~1h | Medium — CRDT resolver wired but no CLI flag |
| 14 | Add `mapSyncError()` table-driven tests | `pkg/api` | ~20min | Medium — error→HTTP mapping is critical for API |

### Lower Priority (Features / Nice-to-Have)

| # | Task | Package | Effort | Impact |
|---|------|---------|--------|--------|
| 15 | Improve `cmd/examples/github-sync` coverage (13.7%) | `cmd/examples` | ~2h | Medium — lowest coverage in project |
| 16 | Improve `pkg/api` coverage to 85%+ | `pkg/api` | ~1h | Medium — error path gaps |
| 17 | Performance benchmarks for SyncItems (1k/10k/100k) | `pkg/cqrs` + `pkg/sync` | ~1h | Medium — unknown scaling characteristics |
| 18 | OpenTelemetry instrumentation (spans for sync, API) | cross-cutting | ~3h | High — no observability today |
| 19 | Real GitHub PAT smoke test | `cmd/examples` | ~30min | Medium — never tested with real API |
| 20 | Doc comments for ~18 exported types | `pkg/id`, `pkg/errors`, `pkg/crdt` | ~30min | Low — `go doc` returns empty |
| 21 | File-based SQLite persistence test across restarts | `pkg/cqrs` | ~1h | Medium — only `:memory:` tested |
| 22 | Export to JSON/CSV (`-export json`) | `cmd/examples` | ~1h | Low — user request |
| 23 | Unify test framework (remove remaining testify patterns) | cross-cutting | ~2h | Low — consistency |
| 24 | Conflict resolution per-sync override (`SyncOptions.ConflictResolver`) | `pkg/sync` | ~1h | Medium — flexibility |
| 25 | Real-time sync protocol (`SyncRequest`/`SyncResponse` from CRDT) | `pkg/crdt` | ~4h | High — multi-node sync foundation |

---

## g) Top #1 Question I Cannot Figure Out Myself

**What is the go-cqrs-lite upstream status on the `Sink→EventSink` rename and `Source` type collision?**

Our `go.mod` uses pseudo-versions (`v2.0.0` / `v2.1.0`) from specific commits. The TODO_LIST mentions a `Sink→EventSink` rename and `Source` type collision in upstream. This blocks future dependency upgrades. I cannot determine:
- Is this resolved in newer upstream commits?
- Should we adopt the rename proactively?
- Is there a timeline for a stable v2 release with these fixes?

This requires upstream repository access / maintainer communication.

---

## Metrics

| Metric | Value |
|--------|-------|
| Total tests | 384 (9 packages) |
| Overall coverage | 79.1% |
| Clone groups (threshold 30) | **0** (was 9) |
| Clone groups (threshold 22) | ~10 (structural patterns, acceptable) |
| Lint issues | 2 (pre-existing, not from this session) |
| Lines changed this session | -100 net (264 added, 364 removed) |
| Files changed this session | 18 modified, 2 new |
| New package | `pkg/testutil` |

### Per-Package Coverage

| Package | Coverage |
|---------|----------|
| `pkg/errors` | 100.0% |
| `pkg/id` | 100.0% |
| `pkg/crdt` | 97.6% |
| `pkg/provider` | 95.8% |
| `pkg/sync` | 91.7% |
| `pkg/cqrs` | 85.7% |
| `pkg/providers/github` | 84.7% |
| `pkg/api` | 76.3% |
| `cmd/examples/github-sync` | 13.7% |
| `pkg/testutil` | N/A (test helpers) |

---

## Files Changed This Session

### New Files
- `pkg/testutil/testutil.go` — Shared test helpers (`MustNoError`, `AssertEqual[T]`, `AssertExternalID`, `AssertType`)
- `pkg/testutil/syncstore.go` — `SyncStoreListBehavior` embeddable mock ListItems behavior

### Modified Files (18)
- `pkg/api/server_test.go` — Embedded `SyncStoreListBehavior`, field renames
- `pkg/cqrs/testing_test.go` — Added `testStateWithTimestamp`, `newUpdatedAtLWWResolver`, delegates to `testutil`
- `pkg/cqrs/decider_test.go` — Uses `testStateWithTimestamp`, `testutil.*` calls
- `pkg/cqrs/decider_resolver_test.go` — Uses `testStateWithTimestamp`, `newUpdatedAtLWWResolver`
- `pkg/cqrs/stack_classify_test.go` — Uses `newUpdatedAtLWWResolver`
- `pkg/cqrs/dispatch_test.go` — `testutil.*` calls
- `pkg/cqrs/readmodel_test.go` — `testutil.*` calls
- `pkg/cqrs/sqlite_test.go` — `testutil.*` calls
- `pkg/cqrs/sqlite_readmodel_test.go` — `testutil.*` calls
- `pkg/cqrs/sqlite_readmodel_filter_test.go` — `testutil.*` calls
- `pkg/cqrs/stack_test.go` — `testutil.*` calls
- `pkg/crdt/conflict_test.go` — Extracted `newVCTestConflict`, `newTiedVCTestConflict` helpers
- `pkg/providers/github/client_bdd_test.go` — Table-driven field assertions
- `pkg/providers/github/client_test.go` — Delegates to `testutil.*`
- `pkg/providers/github/client_convert_test.go` — `testutil.*` calls
- `pkg/providers/github/client_ratelimit_test.go` — `testutil.*` calls
- `pkg/sync/sync_test.go` — Embedded `SyncStoreListBehavior`
- `pkg/sync/sync_incremental_test.go` — Field renames for embedded struct

---

## Session Timeline

1. Ran `art-dupl -t 30 . --semantic --sort total-tokens` → **9 clone groups**
2. Analyzed each group: state init patterns, LWW resolver creation, BDD assertions, cross-package helpers, conflict structs, mock ListItems
3. Added `testStateWithTimestamp` helper → eliminated 4 state init clones
4. Added `newUpdatedAtLWWResolver` helper → eliminated 4 LWW resolver creation clones
5. Extracted `newVCTestConflict` + `newTiedVCTestConflict` → eliminated 4 conflict struct clones
6. Table-driven BDD assertions → eliminated 4 field assertion clones
7. Created `pkg/testutil/` with shared helpers → eliminated 4 cross-package helper clones
8. Created `SyncStoreListBehavior` → eliminated ListItems duplication across 2 packages
9. Bulk-migrated all test files to `testutil.*` calls
10. Fixed all build/lint issues
11. Verified: **0 clone groups at threshold 30**, all 384 tests pass, 0 lint regressions
