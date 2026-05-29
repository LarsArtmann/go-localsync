# Status Report: File Size Refactoring Sprint

**Date:** 2026-05-29 04:29
**Session:** 7 (Partial)
**Trigger:** 10 files exceeding 350-line limit (4 critical, 2 warning, 4 info)

---

## Executive Summary

All 10 files that exceeded the 350-line limit have been split into focused, single-concern files. **Zero files now exceed 350 lines.** All 241 tests pass, 0 lint issues, build clean.

---

## A) FULLY DONE

### File Size Compliance — 10/10 files split

| Original File | Before | Split Into | After |
|---|---|---|---|
| `pkg/sync/sync_test.go` | 744 🚨 | `sync_test.go` + `sync_incremental_test.go` + `sync_conflict_test.go` | 295 + 260 + 210 |
| `pkg/providers/github/client_test.go` | 655 🚨 | `client_test.go` + `client_convert_test.go` + `client_ratelimit_test.go` | 333 + 95 + 250 |
| `pkg/cqrs/decider_test.go` | 551 🚨 | `decider_test.go` + `decider_resolver_test.go` | 310 + 248 |
| `pkg/cqrs/stack_test.go` | 528 🚨 | `stack_test.go` + `stack_classify_test.go` | 255 + 283 |
| `pkg/crdt/conflict_test.go` | 404 ⚠️ | `conflict_test.go` + `conflict_sync_test.go` | 280 + 131 |
| `pkg/crdt/vectorclock_test.go` | 404 ⚠️ | `vectorclock_test.go` + `vectorclock_compare_test.go` | 168 + 241 |
| `pkg/cqrs/stack.go` | 402 ℹ️ | `stack.go` + `stack_adapters.go` | 293 + 109 |
| `pkg/cqrs/turso_readmodel_test.go` | 383 ℹ️ | `turso_readmodel_test.go` + `turso_readmodel_filter_test.go` | 217 + 176 |
| `pkg/providers/github/client.go` | 387 ℹ️ | `client.go` + `client_retry.go` | 270 + 123 |
| `cmd/examples/github-sync/main.go` | 355 ℹ️ | `main.go` + `helpers.go` | 209 + 159 |

### Split Strategy Per File

| File | Strategy |
|---|---|
| `sync_test.go` | Mock helpers + core Syncer tests → incremental tests → conflict-aware tests |
| `client_test.go` | Helpers + Fetch/FetchAll → convertEvent tests → rate limit/retry/wait tests |
| `decider_test.go` | Fold/DecideSync core → CRDT resolver tests (localWins, remoteWins, error, LWW) |
| `stack_test.go` | Core CQRS stack tests → classifyAction + conflict integration tests |
| `conflict_test.go` | LWW resolver + Conflict/MergeResult → SyncMessage/SyncRequest/SyncResponse JSON |
| `vectorclock_test.go` | New/Increment/Get/Merge → Cmp/Equal/Clone/String |
| `stack.go` | Constructor + SyncItems → classifyAction + adapter methods (Count, ListItems, Close) |
| `turso_readmodel_test.go` | Helpers + CRUD tests → filter/pagination tests |
| `client.go` | Client struct + Fetch/FetchAll/convertEvent → waitForRateLimit + withRetry + error helpers |
| `main.go` | main() + flag parsing → output types + runStats + runConflictAwareSync + exitCodeForError |

### Pre-existing lint issues fixed during split

- `exhaustruct` on `CQRSConfig` in main.go → added nolint directive
- `noinlineerr` on `ctx.Err()` inline check in client_retry.go → extracted to plain assignment

---

## B) PARTIALLY DONE

Nothing partially done this session. All 10 files fully split.

---

## C) NOT STARTED

See section F below for the full backlog.

---

## D) TOTALLY FUCKED UP

**Nothing.** Clean session — no breakage, no revert needed, no data loss.

---

## E) WHAT WE SHOULD IMPROVE

1. **File naming convention for splits is inconsistent.** Some files use `_test.go` suffix with topic prefix (`sync_conflict_test.go`), production code uses different pattern (`stack_adapters.go`, `client_retry.go`). Should establish and document a convention.

2. **`pkg/sync/sync.go` at 348 lines** is 1 line under the limit. Could be proactively split before it naturally grows over.

3. **Mock types duplicated across test files.** `mockSyncStore` and `mockProvider` are defined in `sync_test.go` but used by `sync_conflict_test.go` and `sync_incremental_test.go` via same-package access. Works fine but could be cleaner with a dedicated `sync_testhelpers_test.go`.

4. **TODO_LIST.md and FEATURES.md are stale.** Last updated 2026-05-25 — 4 sessions behind. Missing session 6 (CRDT integration) and session 7 (this split).

5. **No CI enforcement of the 350-line limit.** The check that flagged these files was manual. Should add a CI step or pre-commit hook.

---

## F) Top #25 Things To Do Next

### 🔴 HIGH — Architecture & Quality

1. **Add Push/Pull tests** — `CQRSStack.Push()`/`Pull()` untested, key differentiator
2. **Enforce 350-line limit in CI** — add `find . -name "*.go" -exec wc -l` check to flake.nix
3. **Split `pkg/sync/sync.go` (348 lines)** — proactively before it grows over limit
4. **Split `pkg/api/server_test.go` (342 lines)** — close to limit, will likely grow
5. **Split `pkg/cqrs/turso_readmodel.go` (324 lines)** — close to limit
6. **Update TODO_LIST.md** — 4 sessions behind, missing CRDT + file splits
7. **Update FEATURES.md** — missing CRDT conflict resolution, file size compliance
8. **OpenTelemetry instrumentation** — `go.opentelemetry.io/otel` already indirect dependency

### 🟡 MEDIUM — Testing & Coverage

9. **Improve `cmd/examples/github-sync` coverage (12.6%)** — lowest in project
10. **Improve `pkg/api` coverage (76.3%)** — add error path tests
11. **Add integration test for full sync pipeline** — provider → CQRS → read model round-trip
12. **Test concurrent read model access** — memory read model uses sync.Map but no concurrent tests
13. **Add table-driven tests for `HasChanged`** — edge cases in decider field comparison
14. **Test Turso read model with real database file** — currently only `:memory:`

### 🟢 LOWER — Polish & DX

15. **Document file split naming convention** — establish project rule for split file names
16. **Extract shared test helpers** — `mustNoError`, `assertEqual`, etc. duplicated across packages
17. **Add example CLI for another provider** — GitLab or generic webhook provider
18. **Add `CONTRIBUTING.md`** — with file size limits, testing requirements, code style
19. **Profile memory usage** — large sync batches in production
20. **Add structured logging to sync progress** — JSON-friendly output mode

### 🔵 NICE-TO-HAVE — Future

21. **Adopt `middleware.CommandRetry`** — go-cqrs-lite provides this but API mismatch
22. **Adopt `UpcasterRegistry`** — schema evolution when events change
23. **Adopt `catalog/`** — AsyncAPI/OpenAPI/D2 generation from go-cqrs-lite
24. **Multi-provider sync** — sync from multiple sources simultaneously
25. **Webhook provider** — receive events via webhook instead of polling

---

## G) Top #1 Question I Cannot Figure Out Myself

**Should `pkg/sync/sync.go` be split now (348 lines) or only when it naturally crosses 350?**

It's 2 lines under the limit. Adding any new feature (e.g., new SyncOption field, progress callback enhancement, batch optimization) will push it over. Splitting proactively would be cleaner, but the current structure is cohesive — it's one `Syncer` type with `Sync`, `SyncIncremental`, `GetStats`, `Close` and helpers. There's no clean seam like the other files had. The user's call.

---

## Metrics Snapshot

| Metric | Value |
|---|---|
| Total Go files | 66 |
| Files over 350 lines | **0** ✅ |
| Largest file | `pkg/sync/sync.go` (348) |
| Total lines of Go code | 11,152 |
| Total test functions | 241 |
| Packages | 9 |
| Test pass rate | 100% (9/9) |
| Lint issues | 0 |
| Coverage (avg) | ~84% |
| Coverage range | 12.6%–100% |

### Coverage Per Package

| Package | Coverage |
|---|---|
| `pkg/errors` | 100.0% |
| `pkg/id` | 100.0% |
| `pkg/provider` | 100.0% |
| `pkg/crdt` | 97.6% |
| `pkg/sync` | 91.7% |
| `pkg/providers/github` | 84.7% |
| `pkg/cqrs` | 83.6% |
| `pkg/api` | 76.3% |
| `cmd/examples/github-sync` | 12.6% |

---

## Files Changed This Session

### Modified (10)
- `cmd/examples/github-sync/main.go` — extracted helpers to `helpers.go`
- `pkg/cqrs/decider_test.go` — extracted CRDT resolver tests
- `pkg/cqrs/stack.go` — extracted adapter methods + classifyAction
- `pkg/cqrs/stack_test.go` — extracted classify + conflict tests
- `pkg/cqrs/turso_readmodel_test.go` — extracted filter tests
- `pkg/crdt/conflict_test.go` — extracted sync message tests
- `pkg/crdt/vectorclock_test.go` — extracted compare/clone/equal tests
- `pkg/providers/github/client.go` — extracted retry/ratelimit helpers
- `pkg/providers/github/client_test.go` — extracted convert + ratelimit tests
- `pkg/sync/sync_test.go` — extracted incremental + conflict tests

### Created (12)
- `cmd/examples/github-sync/helpers.go` — output types, runStats, runConflictAwareSync, exitCodeForError, runAPIServer
- `pkg/cqrs/decider_resolver_test.go` — localWins, remoteWins, error, LWW resolver tests
- `pkg/cqrs/stack_adapters.go` — classifyAction, Count, GetTypes, ListItems, CountItems, GetItemTypes, Close
- `pkg/cqrs/stack_classify_test.go` — classifyAction table tests, SyncItems conflict tests
- `pkg/cqrs/turso_readmodel_filter_test.go` — filter by actor, repo, source, since, pagination
- `pkg/crdt/conflict_sync_test.go` — SyncMessage/SyncRequest/SyncResponse JSON tests
- `pkg/crdt/vectorclock_compare_test.go` — Cmp, Equal, Clone, String, ClockOrder tests
- `pkg/providers/github/client_convert_test.go` — convertEvent tests (full, minimal, nil)
- `pkg/providers/github/client_ratelimit_test.go` — rate limit, retry, error wrapping tests
- `pkg/providers/github/client_retry.go` — waitForRateLimit, withRetry, isRetryableError, wrapGitHubError
- `pkg/sync/sync_conflict_test.go` — ConflictAwareSyncer + actionMockSyncStore tests
- `pkg/sync/sync_incremental_test.go` — processIncrementalItems, SyncIncremental, reportProgress tests

---

_Arte in Aeternum_
