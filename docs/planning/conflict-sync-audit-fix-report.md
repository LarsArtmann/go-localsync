# Conflict-Aware Sync Bug Fix Report

**Date:** 2025-04-07
**Scope:** Fix all 5 critical bugs in go-localsync's `ConflictAwareSyncer`

## Summary

The audit found 5 critical bugs that made conflict resolution completely non-functional. All have been fixed across 7 commits.

## Bugs Fixed

### 1. `findExistingItem` fetched wrong item (CRITICAL)

- **Before:** Used `s.storage.GetItems(ctx, 1, 0)` — fetched the first item by date, ignoring the target ID entirely
- **After:** Uses `s.storage.GetByID(ctx, item.ID.Get())` — correct ID-based lookup
- **Required:** Added `GetByID` method to `Storage` interface, implemented in `SQLiteStorage`, added to all mock implementations

### 2. Upsert SQL was `DO NOTHING` (CRITICAL)

- **Before:** `ON CONFLICT(github_id) DO NOTHING` — existing items were never updated
- **After:** `ON CONFLICT(github_id) DO UPDATE SET ...` with all updatable fields + `updated_at = CURRENT_TIMESTAMP`

### 3. LWW resolver compared `CreatedAt` instead of `UpdatedAt` (CRITICAL)

- **Before:** `TimestampFunc` used `item.CreatedAt` — never changes between versions, so LWW always hit tiebreaker
- **After:** Uses `item.UpdatedAt` — correctly reflects the most recent modification

### 4. Missing `UpdatedAt` field on `provider.Item` (CRITICAL)

- **Added:** `UpdatedAt time.Time` field to `provider.Item` struct
- **Added:** `updated_at DATETIME DEFAULT CURRENT_TIMESTAMP` column to SQL schema
- **Updated:** All construction sites now set `UpdatedAt = CreatedAt` (GitHub events are immutable)

### 5. Missing `source` column tracking (HIGH)

- **Added:** `source TEXT NOT NULL DEFAULT 'github'` column to SQL schema
- **Added:** `Source` field to `EventCoreMixin` for sqlc code generation
- **Updated:** `toItem` reads source from DB, `toDBParams` passes source to upsert

## Commits

```
1fd6f4c fix: resolve all golangci-lint warnings in modified packages
6719986 fix: set UpdatedAt on all provider.Item construction sites
1c42845 fix: rewrite conflict_aware.go with critical findExistingItem fix and lint cleanup
e94b765 fix: add GetByID to Storage interface and implement in SQLite
c853f5f feat: add UpdatedAt field to provider.Item
0f811db chore: regenerate sqlc code with source/updated_at columns
e12d64a fix: add source/updated_at columns and change upsert to DO UPDATE SET
```

## Files Modified

| File                              | Change                                                     |
| --------------------------------- | ---------------------------------------------------------- |
| `sql/schema/001_events.sql`       | Added `source` + `updated_at` columns                      |
| `sql/queries/events.sql`          | Changed upsert to `DO UPDATE SET`                          |
| `internal/database/connection.go` | Added same columns to inline schema                        |
| `internal/db/models.go`           | Regenerated — `Source`, `UpdatedAt` fields                 |
| `internal/db/events.sql.go`       | Regenerated — updated Scan calls, upsert params            |
| `internal/db/querier.go`          | Regenerated — updated interface                            |
| `internal/db/mixins.go`           | Added `Source` to `EventCoreMixin`                         |
| `pkg/provider/provider.go`        | Added `UpdatedAt time.Time` to `Item`                      |
| `pkg/storage/interface.go`        | Added `GetByID` to `Storage` interface                     |
| `pkg/storage/sqlite.go`           | Implemented `GetByID` using sqlc query                     |
| `pkg/providers/github/client.go`  | Set `UpdatedAt = createdAt`                                |
| `pkg/testhelpers/sync.go`         | Set `UpdatedAt` in `NewTestItem`, added `GetByID` to mocks |
| `pkg/testhelpers/storage.go`      | Set `UpdatedAt` in `NewStorageItem`                        |
| `pkg/storage/sqlite_test.go`      | Set `UpdatedAt` in `testItem`                              |
| `pkg/sync/sync_test.go`           | Added `GetByID` to `mockStorage`                           |
| `pkg/sync/conflict_aware.go`      | Complete rewrite with all fixes + lint compliance          |

## Verification

- **Build:** `go build ./...` — clean
- **Tests:** `go test ./...` — all pass (pkg/providers/github, pkg/storage, pkg/sync)
- **Lint:** `golangci-lint run ./pkg/sync/ ./pkg/storage/ ./pkg/providers/github/ ./pkg/testhelpers/` — 0 issues
- **go-localfirst:** `go test ./pkg/sync/ -v` — all pass

## Remaining Work (Future)

1. **Rename `github_id` column to `source_id`** — Currently using `github_id` for generic IDs, which is confusing
2. **Add integration test** — End-to-end test that fetches from GitHub, detects conflict, and resolves via LWW
3. **Conflict resolution for multi-source** — When multiple providers sync the same item type
4. **Pre-commit hook cleanup** — Several pre-existing failures in BuildFlow hook (library-policy, file-size warnings)
