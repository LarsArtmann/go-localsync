# Session Status Report — 2026-04-22

**Scope**: go-localsync (storage layer hardening + schema generalization)
**Based on**: `2026-04-21_19-39_COMPREHENSIVE_STATUS_REPORT.md` action items

---

## Summary

Executed 4 high-priority items from the comprehensive status report (#4, #12, #17, #21): resolved a behavioral inconsistency in `Storage.Delete`, added concurrent safety tests, generalized the `github_id` column to `source_id` across the entire stack, and formalized the interface contract documentation.

---

## Changes

### 1. Storage.Delete Idempotency Contract (report items #4, #13)

**Problem**: `MemoryStorage.Delete` returned `ErrNotFound` on missing items, while `SQLiteStorage.Delete` silently returned `nil`. The compliance test tolerated either behavior with a permissive assertion.

**Resolution**: Standardized on **idempotent delete** — both backends return `nil` for non-existent items. This matches SQL DELETE semantics and is the more common pattern.

**Files changed**:

- `pkg/storage/interface.go` — Added idempotency documentation to `Delete` doc comment
- `pkg/storage/memory_storage.go` — Removed `ErrNotFound` check, removed unused `fmt` import
- `pkg/storage/memory_storage_test.go` — Changed `TestMemoryStorage_DeleteNotFound` to assert `NoError`
- `pkg/storage/compliance_test.go` — `Delete_NotFound` now asserts `NoError`

### 2. Concurrent Write Safety Tests (report item #12)

Added 3 compliance tests that run against both backends with Go's race detector:

| Test                       | What it proves                                                           |
| -------------------------- | ------------------------------------------------------------------------ |
| `ConcurrentUpsert`         | 20 goroutines upserting simultaneously, all succeed, correct final count |
| `ConcurrentUpsertBatch`    | 10 batches of 5 items each, all succeed, correct total                   |
| `ConcurrentReadsAndWrites` | 30 goroutines reading (GetByID, GetItems, Count) while data exists       |

**Notable fix**: SQLite `:memory:` databases give each connection in the pool its own schema. Fixed by setting `db.SetMaxOpenConns(1)` in the test factory so all goroutines share the same underlying connection.

### 3. `github_id` → `source_id` Generalization (report item #21)

**Problem**: The database column `github_id` was provider-specific, preventing clean multi-provider support despite the `source` column already existing.

**Resolution**: Full-stack rename from `github_id` to `source_id`.

**Migration**: `internal/database/migrations/003_rename_github_id.sql`

```sql
ALTER TABLE events RENAME COLUMN github_id TO source_id;
DROP INDEX IF EXISTS idx_events_github_id;
CREATE INDEX IF NOT EXISTS idx_events_source_id ON events(source_id);
DROP INDEX IF EXISTS idx_events_source_github_id;
CREATE INDEX IF NOT EXISTS idx_events_source_source_id ON events(source, source_id);
```

**Rename scope** (15 files, 239 insertions, 110 deletions):

| Layer          | Old                                                          | New                                                          |
| -------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| SQL schema     | `github_id` column                                           | `source_id` column                                           |
| SQL queries    | `GetEventByGithubID`, `DeleteEventByGithubID`, `UpsertEvent` | `GetEventBySourceID`, `DeleteEventBySourceID`, `UpsertEvent` |
| sqlc config    | `events.github_id` → `GithubEventID`                         | `events.source_id` → `SourceItemID`                          |
| Go types       | `GithubEventBrand`, `GithubEventID`, `NewGithubEventID`      | `SourceItemBrand`, `SourceItemID`, `NewSourceItemID`         |
| Generated code | `Events.GithubID`, `UpsertEventParams.GithubID`              | `Events.SourceID`, `UpsertEventParams.SourceID`              |
| SQLite adapter | All `NewGithubEventID` calls                                 | `NewSourceItemID`                                            |
| Index names    | `idx_events_github_id`, `idx_events_source_github_id`        | `idx_events_source_id`, `idx_events_source_source_id`        |

### 4. Interface Contract Documentation (report item #17)

Added a formal contract block to the `Storage` interface doc comment covering:

- Upsert idempotency
- UpsertBatch atomicity and empty-slice behavior
- Delete idempotency
- GetByID/GetLatest NotFound behavior
- BatchGetByIDs silent omission of missing IDs
- Pagination ordering (CreatedAt descending)
- Concurrent safety requirement

---

## Test Results

| Package                | Status                              |
| ---------------------- | ----------------------------------- |
| `internal/database`    | 7 ok                                |
| `pkg/errors`           | 5 ok                                |
| `pkg/provider`         | 8 ok                                |
| `pkg/providers/github` | 12 ok                               |
| `pkg/storage`          | 25 ok (was 22, +3 concurrent tests) |
| `pkg/sync`             | 15 ok                               |
| `pkg/types`            | 7 ok                                |
| **Total**              | **79 PASS** — zero failures         |

All tests pass with `-race` enabled.

## Code Stats

| Metric                       | Value                    |
| ---------------------------- | ------------------------ |
| Files changed                | 15                       |
| Lines added                  | +239                     |
| Lines removed                | -110                     |
| Non-test Go code             | ~3,765 lines             |
| Test Go code                 | ~4,065 lines             |
| Migrations                   | 3 (was 2)                |
| Compliance tests per backend | 25 (was 22) × 2 = **50** |

---

## Remaining Items from Report

Still not addressed (cross-project or lower priority):

| #   | Item                                      | Reason                |
| --- | ----------------------------------------- | --------------------- |
| #1  | SSE end-to-end integration test           | go-localfirst project |
| #2  | Fix `convertTodoStatusChanged` event type | go-localfirst project |
| #3  | Remove `const _ = "*"` dead code          | go-localfirst project |
| #5  | go-cqrs-lite compliance test suite        | go-cqrs-lite project  |
| #6  | Migrate TodoService tests → CQRS handlers | go-localfirst project |
| #7  | Delete SyncService stub                   | go-localfirst project |
| #8  | Standardize ID types across projects      | cross-project         |
| #9  | Standardize error handling                | cross-project         |
| #10 | NATS JetStream backend                    | new feature           |
| #14 | Prometheus → OpenTelemetry migration      | go-localfirst project |
| #19 | Fix go-localsync CI blockers              | CI config             |
