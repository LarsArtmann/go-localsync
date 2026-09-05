# Session 6 Comprehensive Status Report

**Date:** 2026-05-29 02:34
**Scope:** go-localsync + github-local-sync
**Commits this session:** 9 (7 go-localsync, 2 github-local-sync)

---

## A) FULLY DONE

### go-localsync (SDK)

| # | What                                                                                                                                       | Commit    | Impact                              |
| - | ------------------------------------------------------------------------------------------------------------------------------------------ | --------- | ----------------------------------- |
| 1 | **Fixed SyncIncremental source filter bug** — was fetching globally latest item instead of per-source                                      | `bab08e6` | HIGH — data correctness             |
| 2 | **Consolidated duplicate Turso store creation** — merged `createTursoRemoteStore` + `createTursoLocalStore` into single `createTursoStore` | `5c5c137` | MED — removed ~50 lines duplication |
| 3 | **Replaced errors.As with errors.AsType** — 4 call sites modernized to Go 1.26                                                             | `7d4838f` | LOW — code modernization            |
| 4 | **Wired ErrDatabase into turso_readmodel** — all DB errors now classified as infrastructure                                                | `33d3ccf` | MED — error taxonomy completeness   |
| 5 | **HTTP error mapping in API triggerSync** — ErrRateLimited→429, ErrInvalidToken→401, etc.                                                  | `da43130` | HIGH — proper REST API behavior     |
| 6 | **Added ~60 doc comments** — all exported symbols in pkg/cqrs, pkg/sync, pkg/provider                                                      | `ae9a32b` | MED — SDK usability                 |

### github-local-sync (CLI)

| # | What                                                                                          | Commit    | Impact                       |
| - | --------------------------------------------------------------------------------------------- | --------- | ---------------------------- |
| 7 | **Removed redundant rate limit check** — saved 1 API call per sync                            | `57e7a43` | LOW — API quota savings      |
| 8 | **Fixed stale README** — v56→v69, Cobra→flag, correct CLI flags, updated relationship section | `d1f61b1` | MED — documentation accuracy |

### Test Results

```
go-localsync:   9 packages, all PASS, 78.3% total coverage
github-local-sync: 4 packages, all PASS
```

| Package                  | Coverage |
| ------------------------ | -------- |
| pkg/errors               | 100.0%   |
| pkg/id                   | 100.0%   |
| pkg/provider             | 100.0%   |
| pkg/crdt                 | 97.6%    |
| pkg/sync                 | 93.2%    |
| pkg/providers/github     | 84.6%    |
| pkg/cqrs                 | 83.0%    |
| pkg/api                  | 76.3%    |
| cmd/examples/github-sync | 12.6%    |

---

## B) PARTIALLY DONE

### Documentation Coverage

- ✅ All exported symbols in `pkg/sync/`, `pkg/cqrs/`, `pkg/provider/` now have doc comments
- ⚠️ `pkg/errors/` — 7 of 9 sentinel vars still lack doc comments (only 2 have them)
- ⚠️ `pkg/api/` — some internal types (`SyncInput`, `SyncOutput`) have comments but endpoint handler comments could be richer
- ⚠️ `pkg/id/` — exported types have no doc comments at all (`ItemID`, `ExternalID`, etc.)
- ⚠️ `pkg/crdt/` — 4 exported types missing doc comments (`MergeResult.String`, `ClockOrder.String`, etc.)

### Type Model Quality

- ✅ Branded IDs (`id.ID[B, V]`) provide compile-time safety
- ⚠️ `ItemFilter` requires exhaustruct-compliant construction (all 8 fields listed even when nil) — this is a DX papercut
- ⚠️ `SyncOptions.Source` is a plain `string` but `ItemFilter.Source` is `*id.ProviderID` — requires conversion at every call site
- ⚠️ No typed constructor for `ItemFilter` — callers must know the field names

---

## C) NOT STARTED

### Architecture & Code Organization

1. **Split `pkg/cqrs/stack.go` (380 lines)** — exceeds 350-line BuildFlow limit. Extract `SyncItems`+`classifyAction` → `sync_items.go`, `Push`/`Pull` → `remote.go`, thin query helpers → `query_helpers.go`
2. **Split `pkg/providers/github/client.go` (387 lines)** — extract constructors → `options.go`, fetch logic → `fetch.go`, conversion → `convert.go`, retry/resilience → `retry.go`
3. **Split `pkg/sync/sync.go` (347 lines)** — extract types → `types.go`, helpers → `helpers.go`
4. **Deduplicate `GetTypes`/`GetItemTypes`** — identical methods on `CQRSStack`, one satisfies `SyncStore`, the other is used directly
5. **Extract shared test helpers** — 3+ nearly-identical `testItem()` / `testSyncItem()` / `tursoTestItem()` helpers across `pkg/sync/`, `pkg/api/`, `pkg/cqrs/`

### Missing Tests

6. **API error mapping tests** — `mapSyncError()` has no test coverage. Should verify each error code maps to correct HTTP status
7. **SyncIncremental source filter test** — the bug fix has no specific test proving the source filter is applied
8. **cmd/examples/github-sync coverage** — at 12.6%, barely tested

### Missing Features

9. **OpenTelemetry instrumentation** — `go.opentelemetry.io/otel` is an indirect dependency. No spans, metrics, or traces emitted anywhere
10. **API authentication** — no auth on any endpoint
11. **API pagination headers** — `ListItems` returns items but no `X-Total-Count` or `Link` header
12. **Graceful shutdown for API server** — `runAPIServer` calls `http.ListenAndServe` directly, no `Shutdown(ctx)`
13. **Structured logging in API** — uses `log.Printf` style in some places

---

## D) TOTALLY FUCKED UP

### go-cqrs-lite upstream has broken WIP

**Severity:** Blocking for future development
**Status:** External, not our fault but affects us

`go-cqrs-lite/core/event/store.go` has uncommitted changes:

- Renaming `Sink` → `EventSink`, `Source` → `EventSource`
- New `Source string` type in `types.go` collides with the renamed `EventSource` interface
- `decider.go` references a `Delete` method that doesn't exist on `Repository`

When the stash is applied, `go-localsync` fails to build. We stashed it to verify our changes compile, then restored it. This needs resolution before any go-cqrs-lite upgrade.

### `pkg/crdt/` is architecturally orphaned

**Severity:** Medium — dead code that's well-tested

The CRDT package (97.6% coverage, 52 tests) is explicitly documented as **not wired into the sync path**. The CQRS decider's `UpdatedAt`-based LWW is sufficient for one-way provider sync. The package exists but serves no runtime purpose. It was extracted from go-cqrs-lite/sync "just in case."

Options:

- Remove it entirely (YAGNI)
- Keep it as a separate library (move to its own repo)
- Wire it in for multi-source sync (significant architecture change)

---

## E) WHAT WE SHOULD IMPROVE

### High-Impact, Low-Effort

| # | What                                            | Effort | Impact                          |
| - | ----------------------------------------------- | ------ | ------------------------------- |
| 1 | Add test for `mapSyncError()` in API            | 30min  | Proves HTTP error mapping works |
| 2 | Add test for SyncIncremental with source filter | 30min  | Proves the bug fix works        |
| 3 | Remove duplicate `GetTypes`/`GetItemTypes`      | 15min  | Eliminates dead method          |
| 4 | Add doc comments to `pkg/id/` exports           | 15min  | SDK completeness                |
| 5 | Add doc comments to `pkg/errors/` sentinels     | 10min  | SDK completeness                |

### High-Impact, Medium-Effort

| #  | What                                        | Effort | Impact                            |
| -- | ------------------------------------------- | ------ | --------------------------------- |
| 6  | Split `stack.go` into focused files         | 45min  | File size compliance, readability |
| 7  | Split `github/client.go` into focused files | 45min  | File size compliance, readability |
| 8  | Extract shared `testutil` package           | 60min  | DRY across test packages          |
| 9  | Add `NewItemFilter()` default constructor   | 15min  | Better DX, exhaustruct-compatible |
| 10 | Graceful shutdown for API server            | 30min  | Production readiness              |

### High-Impact, High-Effort

| #  | What                                                           | Effort | Impact                           |
| -- | -------------------------------------------------------------- | ------ | -------------------------------- |
| 11 | Resolve go-cqrs-lite upstream WIP                              | 2-4hr  | Unblocks future development      |
| 12 | OpenTelemetry instrumentation                                  | 4-6hr  | Observability                    |
| 13 | Decide CRDT package fate                                       | 2-3hr  | Architecture clarity             |
| 14 | github-local-sync CQRS migration (Phase 2 from migration plan) | 8-12hr | Eliminates duplicated sync logic |

---

## F) Top #25 Things We Should Get Done Next

Sorted by impact × effort (highest first):

### Tier 1: Quick Wins (under 30 min each)

1. **Test `mapSyncError()`** — table-driven test covering all 6 error→HTTP mappings
2. **Test SyncIncremental source filter** — prove the fix with a test that checks the filter passed to the mock store
3. **Remove `CQRSStack.GetTypes`** — replace callers with `GetItemTypes`, remove duplicate
4. **Doc comments for `pkg/id/`** — `ItemID`, `ExternalID`, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID` + constructors
5. **Doc comments for `pkg/errors/` sentinels** — 7 missing
6. **Add `SyncStore.SyncItem` to interface** — currently missing from the interface but `CQRSStack` implements it; the interface is the architectural seam
7. **Add `//nolint:exhaustruct` to remaining ItemFilter literals** — or add a `NewItemFilter()` constructor
8. **Fix `cmd/examples/github-sync` server graceful shutdown** — use `http.Server` with `Shutdown(ctx)` instead of `ListenAndServe`

### Tier 2: Medium Effort (30-60 min each)

9. **Split `pkg/cqrs/stack.go`** into `stack.go` (NewCQRSStack + types), `sync_items.go`, `remote.go`, `query_helpers.go`
10. **Split `pkg/providers/github/client.go`** into `client.go`, `options.go`, `fetch.go`, `convert.go`, `retry.go`
11. **Extract shared `testutil` package** with a single `TestItem(sourceID, eventType string) *provider.Item`
12. **Add `NewItemFilter()` constructor** returning zero-value filter with `With*` builder methods
13. **Add API authentication middleware** — basic token-based auth
14. **Clean up `SyncStore` interface** — add `SyncItem(ctx, item)` and `DeleteItem(ctx, source, sourceID)` for full CQRS parity

### Tier 3: Strategic (2+ hours)

15. **Resolve go-cqrs-lite upstream WIP** — coordinate the `Sink→EventSink` rename + `Source` collision
16. **Decide CRDT package fate** — remove, extract to own repo, or wire in
17. **OpenTelemetry: basic spans** — instrument `Syncer.Sync`, `CQRSStack.SyncItems`, `Server.triggerSync`
18. **OpenTelemetry: metrics** — items synced, conflicts detected, errors, latency histograms
19. **API: pagination headers** — `X-Total-Count`, cursor-based pagination
20. **API: OpenAPI spec enhancement** — add error response schemas per endpoint
21. **github-local-sync Phase 2 migration** — CQRS stack for event storage, keeping branch domain
22. **Integration test: full sync cycle** — GitHub mock → CQRS → read model → API → HTTP response
23. **Performance benchmarks** — `SyncItems` with 1k/10k/100k items
24. **API rate limiting middleware** — prevent abuse of POST /sync
25. **Structured logging everywhere** — replace `fmt.Printf`/`log.Printf` with charm structured logger

---

## G) Top #1 Question I Cannot Figure Out Myself

**What is the intended relationship between `pkg/crdt/` and the sync path?**

The CRDT package has excellent test coverage (97.6%) and was extracted from `go-cqrs-lite/sync`. AGENTS.md says "SKIPPED — timestamp-based LWW in CQRS decider is sufficient for one-way provider sync." But:

- If multi-source sync (e.g., GitHub + GitLab simultaneously) is ever needed, vector clocks become relevant
- If conflict resolution needs to be configurable (remote-wins, local-wins, merge), the `ConflictResolver[T]` interface would be useful
- The current `pkg/sync/ConflictAwareSyncer` detects conflicts but always resolves them as remote-wins — there's no strategy abstraction

**Question:** Should we (a) delete `pkg/crdt/`, (b) extract it to its own module, or (c) plan to wire it in for v2? This decision affects whether we invest in improving it or remove it as dead weight.

---

## Session Summary

| Metric             | Value                                                     |
| ------------------ | --------------------------------------------------------- |
| Commits            | 9 (7 + 2)                                                 |
| Bugs fixed         | 2 (SyncIncremental source filter, rate limit double call) |
| Lines removed      | ~65 (deduplication)                                       |
| Lines added        | ~123 (docs, features, fixes)                              |
| Net change         | +58 lines                                                 |
| New tests needed   | 2 (mapSyncError, incremental source filter)               |
| Files split needed | 3 (stack.go, client.go, sync.go)                          |
| External blockers  | 1 (go-cqrs-lite WIP)                                      |
| Coverage           | 78.3% total (up from ~78.0%)                              |

---

## Resolution (2026-09-05)

The sprint's cleanups shipped; the go-cqrs-lite upstream-WIP blocker died with the v2 migration; CLI items moot (v0.2.0); stack split executed 2026-05-29 04:29; API auth/pagination/rate-limit tracked in TODO_LIST; OTel tracked in TODO_LIST. No live items remain.
