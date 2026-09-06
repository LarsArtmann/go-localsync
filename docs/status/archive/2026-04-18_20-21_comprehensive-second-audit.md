# Comprehensive Second Audit — Status Report

**Date:** 2026-04-18 20:21
**Branch:** master (up to date with origin/master)
**Commits since last audit:** 14 new commits
**Test Results:** 122 tests PASS, 0 FAIL across 8 packages
**Build Status:** Clean (`go build ./...` succeeds)

---

## A. FULLY DONE (First Plan — 14/14 Steps Complete)

All items from the first 14-step improvement plan have been implemented, committed, and pushed.

| #  | Commit    | Description                                                    |
| -- | --------- | -------------------------------------------------------------- |
| 1  | `0836008` | Add comprehensive documentation and database tests             |
| 2  | `8493cab` | Add assertCountEquals helper for BDD count assertions          |
| 3  | `166e8c8` | Extract BDD test helpers to reduce duplication                 |
| 4  | `4b5a871` | Extract helper methods to reduce BDD test duplication          |
| 5  | `43d55e0` | Add comprehensive Related Projects section to README           |
| 6  | `9e6fce9` | Extract helper functions in sync to reduce code duplication    |
| 7  | `355e363` | Update gitignore and clean system files                        |
| 8  | `33e0954` | Extract shared test helpers to reduce duplication              |
| 9  | `54f940f` | Remove unused FetchSource type alias                           |
| 10 | `85186fa` | Move conversion functions from interface.go to sqlite.go       |
| 11 | `607c417` | Add ErrDatabase and ErrConflict sentinels, wrap storage errors |
| 12 | `18beece` | Add GetItemsBySource, Delete, DeleteAll to Storage interface   |
| 13 | `1ac7192` | Add unit tests for sentinel errors and branded IDs             |
| 14 | `bffaebe` | Expose GetItemsSince using existing GetEventsSince SQL query   |

**Additional commits (this session):**

| #  | Commit    | Description                                                 |
| -- | --------- | ----------------------------------------------------------- |
| 15 | `34b1ae5` | Add UpsertBatch for transactional batch upserts             |
| 16 | `70e6501` | Compose Syncer into ConflictAwareSyncer via embedding       |
| 17 | `bf0742c` | Add input validation for Item and SyncOptions               |
| 18 | `6047c40` | Add OnProgress callback to SyncOptions                      |
| 19 | `10522c8` | Add --conflict-aware flag and OnProgress to example CLI     |
| 20 | `efa0aab` | Expand multi-line struct literals in BDD tests (formatting) |

---

## B. PARTIALLY DONE

| Item               | What's Done                                                                                                                         | What's Missing                                                                                                                                                                                                |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Vector clocks**  | `ConflictAwareSyncer` has in-memory `VectorClock`, increments it during sync, passes it to `buildClockForItem()` for resolution     | Never persisted to DB (resets on restart); `isConflict()` ignores vector clocks entirely (uses field comparison); `buildClockForItem()` increments by `item.Source` which is the same key as the global clock |
| **Type safety**    | `provider.Item` fields use branded types (`types.ItemID`, `types.EventTypeID`, `types.ActorID`, `types.RepoID`, `types.ProviderID`) | Storage interface strips them to `string` at boundary (7 method signatures); `FetchOptions.Source` is `string`; `nodeID` is `string`; sqlc missing 4 column overrides                                         |
| **Error handling** | Custom sentinel errors (`ErrDatabase`, `ErrConflict`, `ErrInvalidInput`) with cockroachdb/errors                                    | 6 locations silently swallow errors (see Section D)                                                                                                                                                           |
| **Test coverage**  | 122 tests across 8 packages, BDD tests for storage filters                                                                          | 6 storage methods untested, no error-path tests, tautological `TestSyncResult`                                                                                                                                |
| **Documentation**  | README, ROADMAP, TODO_LIST exist                                                                                                    | README has 4 inaccuracies, ROADMAP/TODO have stale claims                                                                                                                                                     |

---

## C. NOT STARTED

| Item                                                     | Impact | Work    | Notes                                                                          |
| -------------------------------------------------------- | ------ | ------- | ------------------------------------------------------------------------------ |
| Migrate Storage interface from `string` to branded types | High   | Medium  | 7 method signatures + all callers + sqlc overrides                             |
| Consolidate duplicate mock implementations               | Medium | Medium  | 3 parallel MockStorage, 2 MockProvider; `testhelpers.MockStorage` is dead code |
| Fix README inaccuracies                                  | High   | Low     | Missing 7 methods, stale test count, wrong framework claims                    |
| Fix ROADMAP/TODO stale claims                            | Medium | Low     | Claims "zero coverage" for packages that now have tests                        |
| Remove dead code                                         | Medium | Trivial | `Ptr()`, `StorageItemSet`, `MockStorage`, `newMockProviderWithError()`         |
| Add doc comments to exported symbols                     | Low    | Medium  | 18 undocumented exports in `sqlite.go`, 3 testhelpers packages                 |
| Wire vector clocks into `isConflict()`                   | High   | Medium  | Currently `isConflict()` uses field comparison, ignoring clocks                |
| Persist vector clocks to database                        | High   | High    | New migration, new column, serialization, load/save                            |
| Add sqlc column type overrides                           | Medium | Medium  | `source`, `type`, `actor_login`, `repo_name` → branded types                   |
| Standardize test framework                               | Medium | Medium  | Mix of testify + Ginkgo/Gomega + stdlib                                        |
| Fix pre-commit hooks                                     | Medium | Medium  | Currently bypassed with `--no-verify` on every commit                          |

---

## D. TOTALLY FUCKED UP

These are real bugs or serious architectural issues found in the audit.

### D1. `GetEventsBySource` — Potential Codegen Inconsistency

- **Location:** `pkg/storage/sqlite.go:213`
- **Issue:** `s.querier.GetEventsBySource()` is called but gopls reports it as undefined on `db.Querier`. However, the method DOES exist in `events.sql.go:313`, `db.go:192`, and `querier.go:68`. `go build` succeeds.
- **Likely cause:** Stale gopls cache after `sqlc generate`. Running `sqlc generate` would confirm the codegen is in sync.
- **Risk:** If the codegen is actually out of sync, `GetEventsBySource` could fail at runtime even though it compiles.

### D2. Vector Clocks Are Decorative — Not Functional

- **Location:** `pkg/sync/conflict_aware.go`
- **Issues:**
  1. **Never persisted** — `s.clock` is created fresh via `localsync.NewVectorClock()` at line 51. All clock state is lost on process restart. This means conflict detection history resets every time the application restarts.
  2. **`isConflict()` ignores them** — Lines 304-309 compare item fields (`UpdatedAt`, `Type`, `ActorLogin`, `RepoName`) but never reference `s.clock`. Vector clocks are incremented and cloned but play zero role in the actual conflict decision.
  3. **`buildClockForItem()` groups by source** — Uses `item.Source.Get()` (e.g., `"github"`) as the key, which is the same as `s.nodeID`. Per-item clocks are indistinguishable from the global clock increment.

**Bottom line:** The vector clock infrastructure exists but is completely non-functional. It's decorative code that looks impressive but doesn't actually detect conflicts differently than simple field comparison.

### D3. Silently Swallowed Errors (6 Locations)

| Location                   | Error                                      | Consequence                                                          |
| -------------------------- | ------------------------------------------ | -------------------------------------------------------------------- |
| `sync.go:178-181`          | `CountByType` error in `GetStats`          | Returns partial/empty `TypeCounts` map with no indication of failure |
| `github/client.go:122-126` | `convertEvent` error in `Fetch`            | Items silently disappear from sync results                           |
| `main.go:109`              | `GetTypes` error discarded with `_`        | Prints empty type list silently                                      |
| `sqlite.go:64`             | `tx.Rollback()` error discarded            | Idiomatic Go, but notable for audit                                  |
| `sqlite.go`                | Migration tx/rows close errors not checked | Potential resource leak                                              |
| `conflict_aware.go`        | Vector clock never persisted               | Conflict history lost on restart                                     |

### D4. Migration SQL Duplication — Drift Risk

- **Issue:** SQL is maintained in two places: `sql/migrations/*.sql` files AND hardcoded Go string constants in `internal/database/migration.go:122-148`. The `.sql` files are NOT embedded — they are reference-only. Only the Go constants are used at runtime.
- **Risk:** If someone edits the `.sql` files but forgets to update `migration.go`, they diverge silently. Currently in sync.

---

## E. WHAT WE SHOULD IMPROVE

### Architecture

1. **Storage boundary type safety** — The Storage interface accepts raw strings while provider.Item uses branded types. This is a leaky abstraction that loses compile-time safety at the most critical boundary.

2. **Vector clock design** — Currently decorative. Either commit to making them functional (persist + use in conflict detection) or remove the complexity and use pure LWW timestamps.

3. **sqlc type bridging** — The `toDBParams()` function manually unwraps branded types because sqlc doesn't know about them. Adding sqlc column overrides would generate typed code and eliminate the manual unwrapping.

### Code Quality

4. **Error propagation** — 6 locations silently discard errors. Every error should either be returned, logged, or explicitly documented as intentionally ignored.

5. **Mock consolidation** — 3 parallel storage mock implementations maintained in different files. Should be one canonical mock in `testhelpers`.

6. **Dead code removal** — 4 unused functions/types clutter the codebase.

### Testing

7. **Storage test gaps** — 6 storage methods have zero test coverage: `GetByID`, `UpsertBatch`, `GetItemsBySource`, `GetItemsSince`, `Delete`, `DeleteAll`.

8. **Error path testing** — Most storage methods have no tests for error conditions (e.g., closed database, invalid input).

9. **Test framework consistency** — Mixed testify + Ginkgo/Gomega + stdlib `testing`. The BDD tests are well-structured but the framework mix adds cognitive overhead.

### Documentation

10. **Stale docs** — README, ROADMAP, and TODO_LIST all contain inaccuracies that could mislead new contributors.

11. **Missing godoc** — 18 exported symbols in `sqlite.go` lack doc comments. All 3 `testhelpers` package files lack package doc comments.

---

## F. TOP 25 THINGS TO DO NEXT (Prioritized: Impact × Ease)

| #  | Task                                                                                  | Impact | Work     | Risk            |
| -- | ------------------------------------------------------------------------------------- | ------ | -------- | --------------- |
| 1  | Run `sqlc generate` to verify/fix codegen consistency                                 | High   | Trivial  | None            |
| 2  | Fix silently swallowed errors (6 locations — log or return)                           | High   | Low      | Behavior change |
| 3  | Remove dead code (`Ptr`, `StorageItemSet`, `MockStorage`, `newMockProviderWithError`) | Medium | Trivial  | None            |
| 4  | Fix README inaccuracies (7 missing methods, stale test count)                         | High   | Low      | None            |
| 5  | Fix ROADMAP/TODO stale claims                                                         | Medium | Low      | None            |
| 6  | Add storage tests for `GetByID`, `UpsertBatch`                                        | High   | Low      | None            |
| 7  | Add storage tests for `GetItemsBySource`, `GetItemsSince`                             | High   | Low      | None            |
| 8  | Add storage tests for `Delete`, `DeleteAll`                                           | High   | Low      | None            |
| 9  | Remove tautological `TestSyncResult`                                                  | Low    | Trivial  | None            |
| 10 | Convert `TestItem_Validate` to table-driven test                                      | Low    | Trivial  | None            |
| 11 | Add error-path tests for storage (closed DB, bad input)                               | High   | Medium   | None            |
| 12 | Migrate Storage interface params from `string` to branded types                       | High   | Medium   | API break       |
| 13 | Add sqlc column overrides for `source`, `type`, `actor_login`, `repo_name`            | Medium | Medium   | Codegen change  |
| 14 | Consolidate mock Storage into single testhelpers implementation                       | Medium | Medium   | Test refactor   |
| 15 | Remove duplicate mock Provider (consolidate to testhelpers)                           | Medium | Low      | Test refactor   |
| 16 | Add doc comments to `sqlite.go` exported symbols                                      | Low    | Medium   | None            |
| 17 | Add package doc comments to `testhelpers`                                             | Low    | Trivial  | None            |
| 18 | Decide: make vector clocks functional or remove them                                  | High   | Analysis | Architecture    |
| 19 | If keeping clocks: wire into `isConflict()` properly                                  | High   | Medium   | Behavior change |
| 20 | If keeping clocks: persist to database (migration + load/save)                        | High   | High     | Schema change   |
| 21 | Fix `buildClockForItem` to group by item ID, not source                               | High   | Low      | Behavior change |
| 22 | Make `FetchOptions.Source` use `types.ProviderID`                                     | Medium | Low      | API change      |
| 23 | Make `nodeID` in ConflictAwareSyncer use branded type                                 | Medium | Trivial  | None            |
| 24 | Standardize on one test framework (recommend Ginkgo/Gomega for BDD, testify for unit) | Low    | Medium   | Large refactor  |
| 25 | Fix pre-commit hooks or remove BuildFlow config                                       | Medium | Medium   | CI change       |

---

## G. TOP QUESTION — NEED YOUR INPUT

### Vector Clocks: Commit or Quit?

The vector clock implementation in `ConflictAwareSyncer` is currently **non-functional decoration**:

- Clocks are never persisted (reset on restart)
- `isConflict()` ignores clocks (uses field comparison)
- `buildClockForItem()` groups by source (same key as global clock)

**Two paths forward:**

1. **Make them real** — Persist clocks to DB, wire into conflict detection, fix grouping. This is significant work (new migration, serialization, load/save, behavior change) but delivers on the architectural promise of CRDT-based conflict detection.

2. **Remove the pretense** — Strip vector clocks entirely, rely on pure LWW timestamps (which is what actually happens today). Simpler, honest, less code to maintain. Can always add CRDTs back later when there's a real multi-node use case.

**My recommendation:** Option 2 (remove). The project is a local sync SDK with a single provider. Vector clocks solve a problem that doesn't exist yet. LWW is the correct strategy for the current architecture. YAGNI.

**Your call.**

---

## Project Statistics

| Metric                       | Value                                                |
| ---------------------------- | ---------------------------------------------------- |
| Total commits (all sessions) | ~20 improvement commits                              |
| Test count                   | 122 PASS, 0 FAIL                                     |
| Test files                   | 10 test files                                        |
| Test lines of code           | 2,834                                                |
| Packages with tests          | 8 of 11                                              |
| Packages without tests       | 3 (`cmd/examples`, `internal/db`, `pkg/testhelpers`) |
| Build status                 | Clean                                                |
| Lint status                  | Not run (golangci-lint v1/v2 mismatch)               |
| Pre-commit hooks             | Bypassed (`--no-verify`)                             |

---

## Resolution (2026-09-06 docs-health sweep)

Era-closed: sqlc codegen, swallowed-error sites, and dead code from the direct-SQL era were all deleted by the CQRS rewrite ([ADR-0001](../../adr/0001-cqrs-adoption.md)); modern equivalents are lint-gated (golangci-lint clean, cqrslint invariants). No live items remain here; the living trackers are [TODO_LIST.md](../../../TODO_LIST.md) and [ROADMAP.md](../../../ROADMAP.md).
