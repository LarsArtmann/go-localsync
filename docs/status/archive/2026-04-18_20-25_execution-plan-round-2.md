# Second Round Execution Plan — Comprehensive Improvement

**Created:** 2026-04-18
**Status:** Awaiting approval
**Constraint:** Each step ≤ 12 minutes. Sorted by importance → impact → effort → customer-value.

---

## Sorting Rationale

| Priority | Meaning                            | Examples                                |
| -------- | ---------------------------------- | --------------------------------------- |
| **P0**   | Correctness bugs / data loss risk  | Swallowed errors, broken codegen        |
| **P1**   | High value, type safety, test gaps | Missing tests, string→branded migration |
| **P2**   | Code quality, DRY, consistency     | Dead code, mock consolidation           |
| **P3**   | Polish, documentation              | Doc comments, test style                |

Within each priority: highest impact + lowest effort first.

---

## Phase 1: Critical Correctness (P0)

| Step | Task                                                                                      | Files                                                      | Est. |
| ---- | ----------------------------------------------------------------------------------------- | ---------------------------------------------------------- | ---- |
| 1    | Run `sqlc generate` to regenerate `internal/db/`; verify build + tests still pass         | `sqlc.yaml`, `internal/db/*`                               | 5m   |
| 2    | Fix `GetStats` — log CountByType errors instead of silent `continue`                      | `pkg/sync/sync.go:179`                                     | 5m   |
| 3    | Fix `Sync` — return UpsertBatch error (don't swallow as logged warning)                   | `pkg/sync/sync.go:95-98`                                   | 5m   |
| 4    | Fix `processIncrementalItems` — return UpsertBatch error                                  | `pkg/sync/sync.go:225-228`                                 | 5m   |
| 5    | Fix `Fetch` — log convertEvent errors instead of silent `continue`                        | `pkg/providers/github/client.go:124`                       | 5m   |
| 6    | Fix `main.go` — handle GetTypes error instead of `_` discard                              | `cmd/examples/github-sync/main.go:109`                     | 3m   |
| 7    | Fix `isConflict` — verify branded type `!=` comparison works correctly (add test)         | `pkg/sync/conflict_aware.go:304`, `conflict_aware_test.go` | 10m  |
| 8    | Fix `buildClockForItem` — use `item.ID.Get()` instead of `item.Source.Get()` as clock key | `pkg/sync/conflict_aware.go:316`                           | 5m   |

---

## Phase 2: Dead Code Removal (P2, trivial)

| Step | Task                                                                                             | Files                              | Est. |
| ---- | ------------------------------------------------------------------------------------------------ | ---------------------------------- | ---- |
| 9    | Remove `Ptr()` from testhelpers/helpers.go                                                       | `pkg/testhelpers/helpers.go:17`    | 2m   |
| 10   | Remove `StorageItemSet`, `AddItem`, `UpsertAll`, `NewStorageItemSet` from testhelpers/storage.go | `pkg/testhelpers/storage.go:28-49` | 3m   |
| 11   | Remove `newMockProviderWithError()` from sync_test.go                                            | `pkg/sync/sync_test.go:200-203`    | 2m   |
| 12   | Remove tautological `TestSyncResult` from sync_test.go                                           | `pkg/sync/sync_test.go:417-427`    | 2m   |

---

## Phase 3: Documentation Fixes (P1, low effort)

| Step | Task                                                                                                                                          | Files                              | Est. |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------- | ---- |
| 13   | Fix README Storage interface — add 7 missing methods (UpsertBatch, GetItemsBySource, GetItemsSince, Delete, DeleteAll, CountByType, GetTypes) | `README.md:125-137`                | 8m   |
| 14   | Fix README ConflictAwareSyncer signature — correct usage example to `NewConflictAwareSyncer(baseSyncer)`                                      | `README.md:145`                    | 3m   |
| 15   | Fix README feature table — rate limiting and retry are fully wired, not just "config"                                                         | `README.md:202-203`                | 3m   |
| 16   | Fix README test count (122, not 39)                                                                                                           | `README.md`                        | 2m   |
| 17   | Fix ROADMAP — mark rate limiting + retry as done, correct file references, update test count                                                  | `ROADMAP.md:85-88,100-103,202-203` | 8m   |
| 18   | Fix TODO_LIST — remove stale "zero coverage" and "not wired" claims                                                                           | `TODO_LIST.md:85-93,100-103`       | 5m   |

---

## Phase 4: Storage Test Coverage (P1)

| Step | Task                                                                         | Files                        | Est. |
| ---- | ---------------------------------------------------------------------------- | ---------------------------- | ---- |
| 19   | Add test for `GetByID` — existing item, non-existent item                    | `pkg/storage/sqlite_test.go` | 8m   |
| 20   | Add test for `UpsertBatch` — multiple items, empty batch, duplicate handling | `pkg/storage/sqlite_test.go` | 10m  |
| 21   | Add test for `GetItemsBySource` — filter by source, no results               | `pkg/storage/sqlite_test.go` | 8m   |
| 22   | Add test for `GetItemsSince` — filter by time, no results                    | `pkg/storage/sqlite_test.go` | 8m   |
| 23   | Add test for `Delete` — existing item, non-existent item                     | `pkg/storage/sqlite_test.go` | 8m   |
| 24   | Add test for `DeleteAll` — items present, empty table                        | `pkg/storage/sqlite_test.go` | 8m   |
| 25   | Add error-path test — operations on closed database                          | `pkg/storage/sqlite_test.go` | 10m  |

---

## Phase 5: sqlc Type Overrides (P1)

| Step | Task                                                                              | Files                        | Est. |
| ---- | --------------------------------------------------------------------------------- | ---------------------------- | ---- |
| 26   | Add sqlc column override for `events.source` → `types.ProviderID`                 | `sqlc.yaml`                  | 5m   |
| 27   | Add sqlc column override for `events.type` → `types.EventTypeID`                  | `sqlc.yaml`                  | 3m   |
| 28   | Add sqlc column override for `events.actor_login` → `types.ActorID`               | `sqlc.yaml`                  | 3m   |
| 29   | Add sqlc column override for `events.repo_name` → `types.RepoID`                  | `sqlc.yaml`                  | 3m   |
| 30   | Run `sqlc generate` and fix compilation errors from new types in generated code   | `sqlc.yaml`, `internal/db/*` | 10m  |
| 31   | Update `toDBParams()` in sqlite.go — remove manual unwrapping now handled by sqlc | `pkg/storage/sqlite.go`      | 8m   |
| 32   | Update `fromDBEvent()` in sqlite.go — adjust for new generated types              | `pkg/storage/sqlite.go`      | 8m   |

---

## Phase 6: Storage Interface — String → Branded Types (P1)

| Step | Task                                                                                            | Files                                                     | Est. |
| ---- | ----------------------------------------------------------------------------------------------- | --------------------------------------------------------- | ---- |
| 33   | Change `GetByID(id string)` → `GetByID(id types.ItemID)` in interface + sqlite impl + callers   | `interface.go`, `sqlite.go`, `sync.go`                    | 10m  |
| 34   | Change `Delete(id string)` → `Delete(id types.ItemID)` in interface + sqlite impl + callers     | `interface.go`, `sqlite.go`, tests                        | 8m   |
| 35   | Change `GetItemsByType(itemType string)` + `CountByType(itemType string)` → `types.EventTypeID` | `interface.go`, `sqlite.go`, `sync.go`, tests             | 10m  |
| 36   | Change `GetItemsByActor(actorLogin string)` → `types.ActorID`                                   | `interface.go`, `sqlite.go`, tests                        | 8m   |
| 37   | Change `GetItemsByRepo(repoName string)` → `types.RepoID`                                       | `interface.go`, `sqlite.go`, tests                        | 8m   |
| 38   | Change `GetItemsBySource(source string)` → `types.ProviderID`                                   | `interface.go`, `sqlite.go`, tests                        | 8m   |
| 39   | Change `GetTypes()` return from `[]string` → `[]types.EventTypeID`                              | `interface.go`, `sqlite.go`, `main.go`, tests             | 10m  |
| 40   | Update all mock storage implementations to match new interface                                  | `testhelpers/sync.go`, `sync_test.go`, `sync_bdd_test.go` | 10m  |
| 41   | Update `FetchOptions.Source` from `string` → `types.ProviderID`                                 | `provider.go`, `github/client.go`, tests                  | 8m   |
| 42   | Update `nodeID` from `string` → branded type in ConflictAwareSyncer                             | `conflict_aware.go`, `conflict_aware_test.go`             | 5m   |

---

## Phase 7: Mock Consolidation (P2)

| Step | Task                                                                                   | Files                                             | Est. |
| ---- | -------------------------------------------------------------------------------------- | ------------------------------------------------- | ---- |
| 43   | Remove local `mockStorage` from sync_test.go — use `testhelpers.MockStorage` instead   | `pkg/sync/sync_test.go`                           | 10m  |
| 44   | Remove local `mockProvider` from sync_test.go — use `testhelpers.MockProvider` instead | `pkg/sync/sync_test.go`                           | 10m  |
| 45   | Update all test references to use testhelpers mocks                                    | `pkg/sync/sync_test.go`, `conflict_aware_test.go` | 10m  |

---

## Phase 8: Test Quality Improvements (P2)

| Step | Task                                                                        | Files                                | Est. |
| ---- | --------------------------------------------------------------------------- | ------------------------------------ | ---- |
| 46   | Convert `TestItem_Validate` to table-driven test                            | `pkg/provider/provider_test.go`      | 8m   |
| 47   | Improve `TestSyncer_GetStats` — verify TypeCounts contents, test error path | `pkg/sync/sync_test.go`              | 10m  |
| 48   | Add `FailingStorage` test — verify errors propagate correctly               | `pkg/testhelpers/sync_test.go` (new) | 10m  |
| 49   | Add `SyncIncremental` edge-case tests — empty opts, since=cutoff boundary   | `pkg/sync/sync_test.go`              | 10m  |

---

## Phase 9: Doc Comments (P3)

| Step | Task                                                                                                                                    | Files                   | Est. |
| ---- | --------------------------------------------------------------------------------------------------------------------------------------- | ----------------------- | ---- |
| 50   | Add doc comments to `SQLiteStorage` struct + `NewSQLiteStorage`, `Open`, `Close`                                                        | `pkg/storage/sqlite.go` | 5m   |
| 51   | Add doc comments to CRUD methods: `Upsert`, `UpsertBatch`, `GetByID`, `GetLatest`, `GetItems`                                           | `pkg/storage/sqlite.go` | 8m   |
| 52   | Add doc comments to filter methods: `GetItemsByType/Actor/Repo/Source/Since`, `Count`, `CountByType`, `GetTypes`, `Delete`, `DeleteAll` | `pkg/storage/sqlite.go` | 8m   |
| 53   | Add package doc comments to all 3 testhelpers files                                                                                     | `pkg/testhelpers/*.go`  | 5m   |
| 54   | Add doc comments to helper functions: `toDBParams`, `fromDBEvent`, `toNullString`, `fromNullString`, `convertItems`                     | `pkg/storage/sqlite.go` | 8m   |

---

## Phase 10: Vector Clock Decision (BLOCKED — needs user input)

**Decision needed:** Make vector clocks functional OR remove them entirely.

### Track A: Remove vector clocks (recommended)

| Step | Task                                                               | Files                    | Est. |
| ---- | ------------------------------------------------------------------ | ------------------------ | ---- |
| 55A  | Remove `VectorClock` from `ConflictAwareSyncer` struct             | `conflict_aware.go`      | 5m   |
| 56A  | Remove `buildClockForItem()` and `GetVectorClock()` methods        | `conflict_aware.go`      | 3m   |
| 57A  | Remove vector clock increment from sync loop                       | `conflict_aware.go`      | 3m   |
| 58A  | Simplify `isConflict()` to pure LWW comparison by `UpdatedAt` only | `conflict_aware.go`      | 5m   |
| 59A  | Update tests — remove vector clock assertions                      | `conflict_aware_test.go` | 8m   |

### Track B: Make vector clocks functional

| Step | Task                                                                     | Files                             | Est. |
| ---- | ------------------------------------------------------------------------ | --------------------------------- | ---- |
| 55B  | Add `vector_clocks` migration — JSON column on events table              | `sql/migrations/`, `migration.go` | 10m  |
| 56B  | Persist vector clock state after each sync operation                     | `conflict_aware.go`, `storage.go` | 10m  |
| 57B  | Load vector clock state on ConflictAwareSyncer init                      | `conflict_aware.go`               | 8m   |
| 58B  | Wire vector clocks into `isConflict()` — compare clocks, not just fields | `conflict_aware.go`               | 10m  |
| 59B  | Fix `buildClockForItem` to use item ID as key                            | `conflict_aware.go`               | 5m   |
| 60B  | Add integration test — verify clocks survive restart                     | `conflict_aware_test.go`          | 10m  |

---

## Phase 11: Infrastructure & Polish (P3)

| Step | Task                                                                           | Files                                    | Est. |
| ---- | ------------------------------------------------------------------------------ | ---------------------------------------- | ---- |
| 55   | Embed migration SQL files via `embed.FS` instead of hardcoded Go constants     | `internal/database/migration.go`         | 10m  |
| 56   | Remove `sql/schema/` duplicate if it exists; canonicalize on `sql/migrations/` | `sql/`                                   | 5m   |
| 57   | Investigate and fix pre-commit hooks (or document bypass rationale)            | `.git/hooks/`, `.pre-commit-config.yaml` | 10m  |
| 58   | Final lint pass — `golangci-lint run ./... --timeout=5m`                       | All                                      | 10m  |

---

## Summary Statistics

| Category                      | Steps     | Est. Total       |
| ----------------------------- | --------- | ---------------- |
| Phase 1: Critical correctness | 8         | ~43 min          |
| Phase 2: Dead code            | 4         | ~9 min           |
| Phase 3: Documentation        | 6         | ~29 min          |
| Phase 4: Test coverage        | 7         | ~60 min          |
| Phase 5: sqlc overrides       | 7         | ~40 min          |
| Phase 6: Branded types        | 10        | ~85 min          |
| Phase 7: Mock consolidation   | 3         | ~30 min          |
| Phase 8: Test quality         | 4         | ~38 min          |
| Phase 9: Doc comments         | 5         | ~34 min          |
| Phase 10: Vector clocks (A/B) | 5 or 6    | ~24 or ~53 min   |
| Phase 11: Infrastructure      | 4         | ~35 min          |
| **TOTAL**                     | **63-64** | **~427-456 min** |

**Recommended execution order:** Phases 1→2→3→4→5→6→7→8→9→10→11

Each step produces a self-contained commit with build + tests passing.

---

## Resolution (2026-09-06 docs-health sweep)

Era-closed: this 63-step plan operated on the sqlc/SQL layer deleted by the 2026-05-03 CQRS rewrite ([ADR-0001](../../adr/0001-cqrs-adoption.md)); its conflict/stats concerns live on in the decider + read model, implemented fresh. 'Status: Awaiting approval' above is historical. No live items remain here; the living trackers are [TODO_LIST.md](../../../TODO_LIST.md) and [ROADMAP.md](../../../ROADMAP.md).
