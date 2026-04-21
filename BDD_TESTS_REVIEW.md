# BDD Tests — go-localsync

**Last updated:** 2026-04-22
**Framework:** Ginkgo v2 + Gomega
**Status:** Active

---

## Test Files

| File | Specs | Focus |
|---|---|---|
| `pkg/storage/sqlite_bdd_test.go` | 14 | SQLite storage CRUD, filtering, pagination |
| `pkg/storage/memory_storage_bdd_test.go` | 9 | Memory storage edge cases (concurrency, boundaries, batching) |
| `pkg/sync/sync_bdd_test.go` | 22 | Full sync, incremental sync, error handling, progress callbacks |

**Total: 45 specs**

### Supporting files

| File | Purpose |
|---|---|
| `pkg/storage/storage_bdd_suite_test.go` | `TestStorageBDD` — Ginkgo suite entry point |
| `pkg/sync/sync_bdd_suite_test.go` | `TestSyncBDD` — Ginkgo suite entry point |

---

## Running

```bash
# BDD suites only
go test ./pkg/storage/ -v -run TestStorageBDD
go test ./pkg/sync/ -v -run TestSyncBDD

# All tests
go test ./...
```

---

## Scenarios

### SQLite Storage (`sqlite_bdd_test.go`)

**Persona:** "As a developer building an offline-first dashboard"

Uses real SQLite (in-memory) with full schema migrations:

- Store GitHub events with complete JSON payload preservation
- Idempotent upserts (same ID twice → one copy)
- Query latest event by timestamp
- Empty database → `ErrNotFound` for latest query
- Filter by event type, actor login, repository name
- Statistics: total count, per-type counts, distinct types
- Offset-based pagination (25 items across 3 pages)

### Memory Storage Edge Cases (`memory_storage_bdd_test.go`)

**Persona:** "As a developer building a production sync pipeline"

Unique scenarios that complement (not duplicate) the compliance suite:

- **Concurrent writes** — 100 goroutines insert unique items, no data loss, correct final count
- **`GetItemsSince` boundary** — item at exactly the cutoff timestamp is excluded (uses `After`, not `AfterOrEqual`)
- **`UpsertBatch` empty slice** — no-op, no error, count stays 0
- **`UpsertBatch` nil slice** — no-op, no error
- **`BatchGetByIDs` mixed** — mix of existing and missing IDs → only found items returned
- **`BatchGetByIDs` all missing** — empty slice without error
- **`GetItemsBySource`** — filter by provider source (github vs gitlab)
- **Large dataset pagination** — 50 items traversed via offset/limit paging
- **Upsert same ID different data** — last write wins, verified type/source/actor update

### Sync Engine (`sync_bdd_test.go`)

**Persona:** "As a developer using go-localsync"

Uses real SQLite storage + mock provider:

**Full sync (Sync):**
- First sync: fetch 3 events, store all, preserve event types
- Double sync: same data twice → no duplicates
- Provider failure: returns error, no items stored
- Storage failure during sync: batch upsert returns error
- Nil options: returns error
- Statistics: total count and per-type breakdown
- OnProgress callback invoked after sync

**Incremental sync (SyncIncremental):**
- Incremental with new items: skip items at or older than latest, store new ones
- **Empty store fallback** — `GetLatest` returns `ErrNotFound` → falls back to full `Sync`
- **Nil options** → error
- **Empty source** → validation error
- **GetLatest non-ErrNotFound error** — wrapped error returned (uses `getLatestErrStorage` test helper)
- **Provider failure during incremental** — returns fetch error
- **Storage failure during incremental batch upsert** — returns error
- **All items fail validation** — reports errors, stores nothing new

---

## Complementary Test Suites

The BDD tests sit alongside these existing test suites:

| Suite | File | Type |
|---|---|---|
| Storage compliance | `pkg/storage/compliance_test.go` | 22 tests × 2 backends (testify) |
| Memory storage unit | `pkg/storage/memory_storage_test.go` | Unit tests (testify) |
| SQLite unit | `pkg/storage/sqlite_test.go` | Unit tests (testify) |
| Sync unit | `pkg/sync/sync_test.go` | Unit tests (testify) |
| Conflict-aware sync | `pkg/sync/conflict_aware_test.go` | Unit tests (testify) |
