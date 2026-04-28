# ROADMAP.md

**Project:** go-localsync  
**Last Updated:** 2026-04-08

## Overview

Aspirational features and improvements with no fixed timeline. These are planned for future phases.

---

## ✅ COMPLETED (Recent Sessions)

- [x] **Conflict-aware sync with CRDT**  
       **Source:** `pkg/sync/conflict_aware.go`  
       **Description:** `ConflictAwareSyncer` using go-localfirst `VectorClock` and `LWWResolver[T]` for proper last-writer-wins conflict resolution.  
       **Completed:** Session 3 — fixed 5 critical bugs making it functional.

- [x] **Database migration system**  
       **Source:** `internal/database/migration.go`  
       **Description:** Version-tracked migrations with `schema_migrations` table, transactional applies, 6 tests.

- [x] **Provider architecture refactor**  
       **Source:** `pkg/provider/`, `pkg/providers/github/`  
       **Description:** Generic provider interface with GitHub as first implementation. Branded IDs from go-composable-business-types.

- [x] **Branded type IDs**  
       **Source:** `pkg/types/ids.go`  
       **Description:** `ItemID`, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`, `GithubEventID` — phantom types for compile-time safety.

- [x] **Source provider indexes**  
       **Source:** `internal/database/migration.go` (migration 002)  
       **Description:** `idx_events_source` and `idx_events_source_github_id` for multi-provider queries.

- [x] **Unused code cleanup**  
       **Source:** `internal/db/mixins.go` (deleted)  
       **Description:** Removed `PaginationMixin` and `EventCoreMixin` — never embedded in any struct.

---

## 🟢 FUTURE PHASES

### Enhanced Features

- [ ] **Build TUI with Bubble Tea**  
       **Source:** New package `pkg/tui/`  
       **Description:** Interactive terminal UI for browsing events, filtering, and real-time sync.  
       **Context:** Effort: ~2h. Low priority compared to core stability.

- [ ] **Support multiple user sync**  
       **Source:** `cmd/examples/github-sync/main.go`, `pkg/sync/sync.go`  
       **Description:** Accept multiple `-user` flags or user list from file.  
       **Context:** Requires DB schema update to track which user each event belongs to (if not already in raw JSON).

- [ ] **Implement daemon/background mode**  
       **Source:** New package `cmd/examples/github-sync/daemon.go`  
       **Description:** Run as cron job or systemd service for periodic sync.  
       **Context:** Single-shot mode is current focus. Daemon mode needs lockfile handling.

### Data & Export

- [x] **Add Turso backend support**  
       **Source:** `pkg/storage/turso.go`  
       **Description:** Support Turso remote SQLite databases via turso.tech/database/tursogo.  
       **Context:** Completed. Migrated from deprecated libsql-client-go to tursogo.

- [ ] **Create HTTP API endpoint**  
       **Source:** New package `pkg/api/`  
       **Description:** REST API for querying events (GET /events, /stats, /types).  
       **Context:** Would turn CLI tool into server. Effort: ~2h.

- [ ] **Add export to JSON/CSV**  
       **Source:** `cmd/examples/github-sync/main.go`  
       **Description:** Export stored events to file formats (`-export json` or `-export csv`).  
       **Context:** Useful for data analysis in external tools.

---

## 🔧 TECHNICAL DEBT

### Code Quality

- [ ] **Migrate testify→Ginkgo/GOmega**  
       **Source:** 8 test files across `pkg/storage`, `pkg/sync`, `pkg/providers/github`  
       **Description:** Pre-commit hooks ban testify. All 48 tests use it.  
       **Context:** ~3h effort. Unblocks pre-commit hooks.

- [ ] **Standardize null string conversion**  
       **Source:** `pkg/storage/interface.go`  
       **Description:** Consider using generics or code generation for NullString conversions.  
       **Context:** Current helper functions `toNullString`/`fromNullString` are repetitive.

- [ ] **Generalize github_id column name**  
       **Source:** `internal/db/`, `sql/queries/events.sql`  
       **Description:** Rename `github_id` column to `source_id` for multi-provider support.  
       **Context:** Breaking schema change — needs migration 003.

### Infrastructure

- [ ] **Align Go toolchain (1.26.0 vs 1.26.1)**  
       **Source:** `go.mod`  
       **Description:** `go.mod` says 1.26.1 but installed toolchain is 1.26.0. Blocks coverage reports.  
       **Context:** Regular build/test works. Only `-cover` flag fails.

- [ ] **Install golangci-lint v2 binary**  
       **Source:** `.golangci.yml`  
       **Description:** Config is v2 format. Binary is v1.64.8.  
       **Context:** Blocks lint gate. `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`.

### Missing Test Coverage

- [ ] **Storage error path tests**  
       **Source:** `pkg/storage/sqlite.go`  
       **Description:** Coverage at 56%. Error paths for GetItemsByActor, GetItemsByRepo, CountByType untested.  
       **Context:** Target 80%+.

- [ ] **CLI integration tests**  
       **Source:** `cmd/examples/github-sync/main.go`  
       **Description:** Zero test coverage for flag parsing, signal handling, exit codes.

- [x] **pkg/errors and pkg/types tests**  
       **Source:** `pkg/errors/errors_test.go`, `pkg/types/ids_test.go`  
       **Description:** 4 error tests + 5 type tests.  
       **Completed:** Round 1 audit sessions.

---

## ❓ OPEN QUESTIONS

These require product/architecture decisions before becoming actionable tasks:

1. **End-to-end testing strategy**
   - Do we need a real GitHub PAT for integration tests in CI, or are mocks sufficient?

2. **Turso priority**
   - Is LibSQL/Turso support needed for Phase 2, or should we stick to local SQLite?

3. **Multi-user sync architecture**
   - Should the DB schema track which user each event belongs to, or rely on GitHub API data only?

4. **Event retention/TTL**
   - Should we add automatic cleanup of events older than N days? If so, configurable per user?

5. **Update strategy for existing events**
   - Current `ON CONFLICT(github_id) DO UPDATE SET` correctly updates all fields using `excluded.updated_at` for LWW. The `updated_at` is now properly passed from provider data instead of using `CURRENT_TIMESTAMP`.

6. **testify vs Ginkgo decision**
   - Migrate to Ginkgo/GOmega to satisfy pre-commit hooks, or disable the hook rule? ~3h migration effort.
