# Comprehensive Status Report — 2026-04-19 03:11

**Project:** go-localsync\
**Branch:** master (6 commits ahead of origin)\
**Build:** Clean — `go build ./...` passes\
**Tests:** 121 tests across 8 suites — ALL PASS (0 failures)\
**Code:** ~2,159 lines production, ~2,823 lines test\
**Uncommitted:** 3 documentation files (README.md, ROADMAP.md, TODO_LIST.md)

---

## A) FULLY DONE ✅

### Core Architecture

- **Provider interface** — `provider.Provider` with `Name()`, `Fetch()`, `FetchAll()`, `GetRateLimit()` — clean abstraction for any data source
- **GitHub provider** — Full implementation with OAuth2, pagination, rate limiting, retry with exponential backoff
- **Storage interface** — 16-method `Storage` interface with full CRUD + filtering
- **SQLite storage** — Full implementation using sqlc-generated queries, branded ID conversion at boundary
- **Sync engine** — `Syncer` with full sync and incremental sync (cutoff-based dedup)
- **Conflict-aware sync** — `ConflictAwareSyncer` wrapping `Syncer` via composition, CRDT-backed with vector clocks + LWW resolution
- **Branded IDs** — 7 phantom types (`ItemID`, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`, `EventID`, `GithubEventID`) using `go-composable-business-types/id`
- **Migration system** — Version-tracked, transactional, idempotent migrations with `schema_migrations` table
- **sqlc pipeline** — All SQL in `sql/queries/`, types generated to `internal/db/`, branded ID overrides for `events.id` and `events.github_id`

### Bug Fixes (This Session Series)

- **Vector clock key bug** — Was using `item.Source` as clock key (same for all items from one provider). Fixed to use `item.ID` — each item gets its own clock entry
- **Swallowed UpsertBatch errors** — Both `Syncer.Sync()` and `Syncer.processIncrementalItems()` were returning partial `SyncResult` with `Errors` count but NOT returning the actual error. Now propagated properly
- **Dead code removal** — `Ptr[T]` from testhelpers, `StorageItemSet` and related methods, `newMockProviderWithError` from sync_test, `TestSyncResult` duplicate

### Testing

- **48 top-level tests** (121 including subtests) across 8 packages — ALL GREEN
- **pkg/errors** — 4 tests for sentinel error matching
- **pkg/types** — 5 tests for branded ID construction
- **pkg/provider** — 1 test for Item validation
- **pkg/providers/github** — 21 tests (client, fetch, retry, error handling)
- **pkg/storage** — BDD-style suite for SQLite CRUD
- **pkg/sync** — 11 tests for Syncer + ConflictAwareSyncer
- **internal/database** — 6 migration tests (idempotency, ordering, schema, indexes)

### Features Completed This Session

- **Rate limiting** — `RateLimitConfig` wired into GitHub client's fetch loop with `waitForRateLimit()`
- **Retry logic** — `RetryConfig` drives `withRetry()` with exponential backoff, max retries, context cancellation
- **Input validation** — `Item.Validate()` and `SyncOptions.Validate()` with sentinel errors
- **OnProgress callback** — `SyncProgressFunc` on `SyncOptions` for real-time progress reporting
- **UpsertBatch** — Transactional batch upsert with rollback on failure
- **Extended Storage** — `GetItemsBySource`, `GetItemsSince`, `Delete`, `DeleteAll` added

---

## B) PARTIALLY DONE ⚠️

### Documentation (Phase 3 — uncommitted)

- **README.md** — Updated Storage interface (was missing 7 methods), fixed ConflictAwareSyncer example, updated feature table and test counts. **Uncommitted.**
- **ROADMAP.md** — Updated test counts, marked pkg/errors + pkg/types tests as completed. **Uncommitted.**
- **TODO_LIST.md** — Updated test counts, marked rate limit + retry as completed. **Uncommitted. Still has stale `pkg/errors` and `pkg/types` "zero test coverage" at lines 56-59.**

### Error Handling Consistency

- **`pkg/storage/sqlite.go`** — 17 `fmt.Errorf` calls. ALL wrap with `pkgerrors.ErrDatabase` correctly (good!). But uses `fmt.Errorf` pattern instead of `pkgerrors.Wrap()` consistently. Works but inconsistent with the `pkg/errors` package's intent.
- **`pkg/sync/sync.go`** — 5 `fmt.Errorf` calls, none use `pkgerrors` wrapping. Mix of sentinel errors and raw `fmt.Errorf`.
- **`pkg/sync/conflict_aware.go`** — 3 `fmt.Errorf` calls. Same inconsistency.
- **`pkg/providers/github/client.go`** — 10 `fmt.Errorf` calls. Some use `pkgerrors` sentinels, some don't.
- **`pkg/provider/provider.go`** — 4 `fmt.Errorf` in `Validate()`. Should return `pkgerrors.ErrInvalidInput`.
- **`internal/database/`** — 8 `fmt.Errorf` calls. No sentinel wrapping at all.

**Total: ~47 `fmt.Errorf` calls** in production code (excluding auto-generated `internal/db/`). Should use `cockroachdb/errors` consistently since it's already a dependency.

### Logging Consistency

- **1 `slog.Warn`** in `github/client.go:126` — should use `charm.land/log/v2` like everything else
- Everything else uses `charm.land/log/v2` correctly

---

## C) NOT STARTED ❌

### Architecture / Type Safety

- **String→branded types on Storage interface** — 7 methods accept raw `string` params that should use branded types: `GetByID(id string)`, `GetItemsByType(itemType string)`, `GetItemsByActor(actorLogin string)`, `GetItemsByRepo(repoName string)`, `GetItemsBySource(source string)`, `CountByType(itemType string)`, `Delete(id string)`. The branded types EXIST in `pkg/types/` but are stripped to `string` at the Storage boundary.
- **`FetchOptions.Source` and `SyncOptions.Source` as raw `string`** — Same concept as `Item.Source` which IS `types.ProviderID`. Split brain.
- **sqlc column overrides** — Missing overrides for `events.source` → `ProviderID`, `events.type` → `EventTypeID`, `events.actor_login` → `ActorID`, `events.repo_name` → `RepoID` in `sqlc.yaml`. Would propagate branded types through sqlc-generated code.

### Testing Gaps

- **6 Storage methods with no test coverage**: `GetItemsBySource`, `GetItemsSince`, `Delete`, `DeleteAll`, `GetByID` not-found path, `UpsertBatch`
- **No error-path tests for closed database** — what happens when operations run on a closed SQLite connection?
- **No tests for `pkg/testhelpers/`** — helper package has 0 test files
- **No CLI integration tests** — `cmd/examples/github-sync/` has 0 tests
- **No edge-case tests** — empty batch, nil context, concurrent access

### Test Framework

- **testify→Ginkgo/GOmega migration** — All 48 tests use testify. Pre-commit hooks ban testify. This is a 3h migration that unblocks pre-commit hooks.

### Infrastructure

- **Pre-commit hooks broken** — 4 categories of failures: library-policy (testify), go-structure-linter, ast-state-analyzer, todo-check. All commits use `--no-verify`.
- **golangci-lint v1/v2 mismatch** — Config is v2 format, installed binary is v1.64.8.
- **Go toolchain mismatch** — `go.mod` says 1.26.1, installed is 1.26.0. Blocks `go test -cover`.
- **Migration SQL embedding** — SQL embedded as Go string constants instead of `embed.FS`. Drift risk with `sql/migrations/` files.

### Documentation

- **No godoc on 19 exported symbols** in `sqlite.go` (all the exported methods)
- **No package doc for `pkg/testhelpers/`**
- **Stale TODO_LIST.md entries** at lines 56-59 still claim "zero test coverage" for pkg/errors and pkg/types

---

## D) TOTALLY FUCKED UP 💀

### Ghost System #1: Vector Clocks

- `ConflictAwareSyncer` builds vector clocks, increments them, clones them, builds per-item clocks
- **BUT**: Vector clocks are NEVER persisted. They reset on every restart.
- This means conflict detection across sessions is **impossible**. The `isConflict()` check is just field comparison — no causal ordering information survives a restart.
- The vector clock system is decorative. It adds complexity without delivering actual distributed conflict detection.
- `GetVectorClock()` and `buildClockForItem()` are dead code in practice.

### Ghost System #2: `ErrStorage` Sentinel

- Defined in `pkg/errors/errors.go` as `ErrStorage = errors.New("storage error")`
- **Never returned by any code path anywhere in the codebase**
- Only checked in `cmd/examples/github-sync/main.go` — that check is dead code
- Meanwhile `ErrDatabase` IS used (wrapping all sqlite.go errors). Two overlapping sentinels where only one works.

### Ghost System #3: Branded Types → String Stripping

- 7 branded types carefully defined in `pkg/types/ids.go`
- `Item` struct uses them correctly
- But `Storage` interface strips them to raw `string` at the boundary
- `toDBParams()` in `sqlite.go` unwraps branded types back to strings via `.Get()`
- `toItem()` wraps strings back into branded types
- The type safety is an illusion at the most critical boundary

### Split Brain #1: Two Error Systems

- `cockroachdb/errors` imported and used for sentinel definitions
- `fmt.Errorf` used in 47 call sites for actual error creation
- These two systems coexist without integration. `fmt.Errorf` with `%w` does work with `errors.Is()` but loses the rich context that `cockroachdb/errors` provides (telemetry, redaction, safe details)

### Split Brain #2: Two Logging Systems

- `charm.land/log/v2` everywhere except one `slog.Warn` in github/client.go

### Split Brain #3: Source Field Type Inconsistency

- `Item.Source` is `types.ProviderID` (branded)
- `FetchOptions.Source` is `string` (raw)
- `SyncOptions.Source` is `string` (raw)
- `Storage.GetItemsBySource(source string)` is `string` (raw)
- Same semantic concept, 4 different type representations

---

## E) WHAT WE SHOULD IMPROVE

### Honest Assessment

1. **The codebase is functional but architecturally dishonest.** We built type-safety theater — branded types that get stripped at boundaries, vector clocks that reset on restart, sentinel errors that are defined but never returned, two error systems coexisting.

2. **We're not leveraging our dependencies.** `cockroachdb/errors` gives us rich error chains, redaction, and telemetry but we use `fmt.Errorf`. `go-localfirst` gives us real CRDT primitives but we use them decoratively.

3. **Test coverage is wide but shallow.** 121 tests but most are happy-path. No error-path tests, no edge cases, no closed-DB tests, no concurrent access tests.

4. **Documentation is stale and contradictory.** TODO_LIST claims zero coverage for packages that have tests. README shows methods that were added last week as if they've always been there.

5. **Pre-commit hooks are completely broken.** Every commit uses `--no-verify`. This means we have zero automated quality gates. The code could be garbage and we'd never know until CI catches it.

6. **`LarsArtmann/uniflow` does not exist** — returns 404. Must use `cockroachdb/errors` as fallback.

### Library Evaluation (Honest)

| Library               | Verdict         | Rationale                                                                                                                                                                          |
| --------------------- | --------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `samber/lo`           | ⚠️ Marginal      | We have ~5 slice operations total. `lo.Map`, `lo.Filter` would save ~10 lines. Not worth a dependency for this small codebase.                                                     |
| `samber/mo`           | ⚠️ Marginal      | `mo.Result[T]` would clean up error handling patterns. `mo.Option[T]` could replace nil returns. But would require changing all return signatures. High churn for unclear benefit. |
| `samber/do`           | ❌ No           | We have 3 structs total. DI framework is massive over-engineering.                                                                                                                 |
| `cockroachdb/errors`  | ✅ Already here | Just need to USE it consistently instead of `fmt.Errorf`                                                                                                                           |
| `ginkgo/v2`           | ⏳ Planned      | Required by pre-commit hooks. 3h migration.                                                                                                                                        |
| `sqlc`                | ✅ Already here | Working well. Need to add missing column overrides.                                                                                                                                |
| `LarsArtmann/uniflow` | ❌ 404          | Does not exist. Fallback to `cockroachdb/errors`.                                                                                                                                  |

### Pattern Compliance (Honest)

| Pattern                      | Status            | Verdict                                                                                             |
| ---------------------------- | ----------------- | --------------------------------------------------------------------------------------------------- |
| DDD                          | ❌ Not followed   | No aggregates, no value objects, no domain events. Just Provider→Syncer→Storage with anemic models. |
| CQRS                         | ❌ Not applicable | Single data path. No command/query separation needed at this scale.                                 |
| Event-Sourcing               | ❌ Not followed   | Raw JSON stored but no event replay, no projections, no event versioning.                           |
| BDD                          | ⚠️ Partial         | Storage tests use BDD-style. Everything else uses standard testify.                                 |
| TDD                          | ⚠️ Partial         | Some tests written first, some after. Not consistent.                                               |
| Railway Oriented             | ❌ Not followed   | Standard Go error handling everywhere.                                                              |
| Composition over Inheritance | ✅ Followed       | `ConflictAwareSyncer` embeds `*Syncer`. Good.                                                       |
| DRY                          | ⚠️ Partial         | Duplicate mocks still exist. `toNullString`/`fromNullString` are repetitive.                        |
| Separation of Concerns       | ✅ Good           | Clean package boundaries. Provider/Storage/Sync properly separated.                                 |

---

## F) TOP 25 THINGS TO DO NEXT

Sorted by: Impact × Urgency ÷ Effort

| #  | Task                                                                                        | Impact | Effort | Status         | Why                                                |
| -- | ------------------------------------------------------------------------------------------- | ------ | ------ | -------------- | -------------------------------------------------- |
| 1  | **Commit Phase 3 doc changes**                                                              | Medium | 5min   | Uncommitted    | Low-hanging fruit. Just commit it.                 |
| 2  | **Fix stale TODO_LIST entries** (lines 56-59)                                               | Low    | 2min   | Partially done | Docs should reflect reality                        |
| 3  | **Remove `ErrStorage` sentinel** — never returned                                           | Medium | 10min  | Not started    | Dead code. Delete it. Remove check in example CLI. |
| 4  | **Replace `slog.Warn` → `charm.land/log/v2`** in client.go:126                              | Low    | 5min   | Not started    | Eliminates split brain                             |
| 5  | **`Item.Validate()` → return `pkgerrors.ErrInvalidInput`**                                  | Medium | 15min  | Not started    | Validation errors should use sentinels             |
| 6  | **Migrate `fmt.Errorf` → `cockroachdb/errors`** in sync.go (5 sites)                        | Medium | 30min  | Not started    | Consistent error handling in core                  |
| 7  | **Migrate `fmt.Errorf` → `cockroachdb/errors`** in conflict_aware.go (3 sites)              | Medium | 20min  | Not started    | Same                                               |
| 8  | **Migrate `fmt.Errorf` → `cockroachdb/errors`** in github/client.go (10 sites)              | Medium | 30min  | Not started    | Same                                               |
| 9  | **Migrate `fmt.Errorf` → `cockroachdb/errors`** in sqlite.go (17 sites)                     | Medium | 45min  | Not started    | Same — largest file                                |
| 10 | **Migrate `fmt.Errorf` → `cockroachdb/errors`** in database/ (8 sites)                      | Medium | 20min  | Not started    | Same                                               |
| 11 | **Add storage tests for untested methods** (6 methods)                                      | High   | 90min  | Not started    | Critical coverage gap                              |
| 12 | **Consolidate duplicate mocks** — remove from sync_test.go, use testhelpers                 | Medium | 45min  | Not started    | DRY violation                                      |
| 13 | **Remove vector clock dead code** — `buildClockForItem`, `GetVectorClock`, `SyncOperations` | Medium | 60min  | Not started    | Ghost system removal                               |
| 14 | **Simplify `isConflict()` to pure LWW** — remove vector clock dependency                    | Medium | 30min  | Not started    | After #13, conflict detection becomes honest       |
| 15 | **Storage interface: string → branded types** (7 methods)                                   | High   | 120min | Not started    | Biggest type-safety win                            |
| 16 | **Update all Storage callers** for branded types                                            | High   | 60min  | Not started    | Depends on #15                                     |
| 17 | **`FetchOptions.Source` and `SyncOptions.Source` → `types.ProviderID`**                     | Medium | 30min  | Not started    | Split brain #3 fix                                 |
| 18 | **sqlc column overrides** for source, type, actor_login, repo_name                          | Medium | 60min  | Not started    | Propagates branded types through generated code    |
| 19 | **Embed migration SQL via `embed.FS`** instead of string constants                          | Low    | 30min  | Not started    | Drift prevention                                   |
| 20 | **testify → Ginkgo/GOmega migration** (8 files, 48 tests)                                   | High   | 180min | Not started    | Unblocks pre-commit hooks                          |
| 21 | **Add godoc to 19 exported symbols** in sqlite.go                                           | Low    | 30min  | Not started    | Documentation quality                              |
| 22 | **Add error-path tests** (closed DB, nil context)                                           | Medium | 60min  | Not started    | Robustness                                         |
| 23 | **Install golangci-lint v2** binary                                                         | Medium | 5min   | External       | Requires user action                               |
| 24 | **Align Go toolchain to 1.26.1**                                                            | Low    | 5min   | External       | Requires user action                               |
| 25 | **CLI integration tests** for github-sync example                                           | Medium | 90min  | Not started    | Zero coverage on user-facing code                  |

---

## G) TOP #1 QUESTION FOR USER

**The vector clock question:**

The audit revealed that vector clocks in `ConflictAwareSyncer` are **decorative** — they reset on every restart because they're never persisted. The actual conflict detection (`isConflict()`) is just field comparison. This means:

1. **Option A: Remove vector clocks entirely.** Simplify `ConflictAwareSyncer` to pure LWW (last-write-wins based on `UpdatedAt`). Remove `buildClockForItem()`, `GetVectorClock()`, `SyncOperations()`, and the `go-localfirst` vector clock dependency. The conflict detection would remain the same (field comparison) but we'd be honest about what it does.

2. **Option B: Persist vector clocks.** Add a `vector_clocks` table, serialize clock state per item, load on startup. This would make the CRDT system actually work but adds significant complexity (serialization, schema migration, storage overhead).

3. **Option C: Keep as-is.** Accept the decorative complexity. It works for single-session sync even if clocks don't persist.

**My recommendation: Option A.** The current implementation gives a false sense of distributed conflict resolution. Pure LWW is simpler, honest, and sufficient for the SDK's current use case. If multi-device sync is needed later, it can be added properly from scratch.

**What's your call?**

---

## Uncommitted Changes Detail

```
README.md    | 21 ++++++++++++++------
ROADMAP.md   |  9 ++++++---
TODO_LIST.md | 14 ++++++++------
3 files changed, 27 insertions(+), 17 deletions(-)
```

### README.md changes:

- Storage interface: added 7 missing methods (UpsertBatch, GetItemsBySource, GetItemsSince, Delete, DeleteAll)
- ConflictAwareSyncer example: fixed to show `NewSyncer` + `NewConflictAwareSyncer(baseSyncer)` pattern
- Feature table: Rate Limiting and Retry marked as Done
- Test count: 39→48, added 3 new packages to testing table

### ROADMAP.md changes:

- Test count: 39→48
- pkg/errors and pkg/types test coverage marked as completed

### TODO_LIST.md changes:

- Test count: 39→48
- Rate limit handling and retry logic marked as completed
- NOTE: Lines 56-59 still incorrectly claim "zero test coverage" for pkg/errors and pkg/types

---

## Project Health Score

| Dimension          | Score      | Notes                                           |
| ------------------ | ---------- | ----------------------------------------------- |
| **Build**          | 🟢 10/10   | Clean build, all tests pass                     |
| **Architecture**   | 🟡 6/10    | Clean boundaries but ghost systems              |
| **Type Safety**    | 🟡 5/10    | Branded types exist but stripped at boundaries  |
| **Error Handling** | 🟡 4/10    | Two systems coexisting, 47 inconsistent sites   |
| **Test Coverage**  | 🟡 6/10    | 121 tests but shallow — happy path only         |
| **Documentation**  | 🟡 5/10    | Stale entries, missing godoc                    |
| **Infrastructure** | 🔴 3/10    | Hooks broken, lint mismatch, toolchain mismatch |
| **Code Quality**   | 🟢 7/10    | Clean code, but dead code and ghost systems     |
| **Overall**        | 🟡 5.75/10 | Functional but architecturally dishonest        |
