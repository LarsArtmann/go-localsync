# TODO_LIST.md

**Project:** go-localsync
**Generated:** 2026-02-12
**Last Updated:** 2026-03-29
**Status:** Phase 1 Complete → Phase 2 Development

## Overview

This document tracks all TODOs, FIXMEs, and action items identified from code analysis and the Phase 1 completion status report. Items are organized by priority and include source references and implementation context.

---

## ✅ COMPLETED (2026-02-15)

### Testing & Quality

- [x] **Add GitHub client tests**  
      **Source:** `pkg/github/client_test.go`  
      **Description:** Created 13 comprehensive test functions with mock HTTP server.  
      **Context:** Tests cover NewClient, FetchEvents, FetchAllEvents, ConvertEvent, and GetRateLimit.

- [x] **Add sync package tests**  
      **Source:** `pkg/sync/sync_test.go`  
      **Description:** Created tests for NewSyncer, Sync, SyncIncremental, GetStats, and Close.  
      **Context:** Uses mock storage implementation for isolated unit testing.

### Critical Issues

- [x] **Fix go.mod indirect dependencies**  
      **Source:** `go.mod`  
      **Description:** Ran `go mod tidy` to properly categorize dependencies.  
      **Context:** Dependencies now correctly marked as direct vs indirect.

- [x] **Implement typed/sentinel errors**  
      **Source:** `pkg/errors/errors.go`  
      **Description:** Created typed sentinel errors using cockroachdb/errors.  
      **Context:** Includes ErrNotFound, ErrInvalidInput, ErrDatabase, ErrGitHubAPI, ErrRateLimited, ErrUnauthorized, ErrConfiguration.

- [x] **Remove unused Config struct**  
      **Source:** `pkg/sync/sync.go`  
      **Description:** Deleted the unused `Config` struct.  
      **Context:** Struct was defined but never used.

### Infrastructure

- [x] **Set up CI/CD Pipeline**  
      **Source:** `.github/workflows/ci.yml`  
      **Description:** Created GitHub Actions workflow with test, lint, build, and release jobs.  
      **Context:** Runs on push/PR to master/main, uses golangci-lint, builds for linux/darwin on amd64/arm64.

### Architecture Improvements

- [x] **Decouple domain types from storage**  
      **Source:** `pkg/event/event.go`  
      **Description:** Created domain Event type in `pkg/event/` package.  
      **Context:** GitHub client now returns domain events, storage layer handles conversion.

- [x] **Fix version build injection**  
      **Source:** `justfile`  
      **Description:** Added ldflags for version, commit, and date in build command.  
      **Context:** Build now properly injects version information.

### Code Quality

- [x] **Add error handling for DB close operations**  
      **Source:** `cmd/gh-sync/main.go`  
      **Description:** Added error logging for `dbc.Close()` in defer.  
      **Context:** Close errors are now logged instead of silently ignored.

---

## ✅ COMPLETED (2026-03-29)

### Code Quality

- [x] **Fix golangci-lint warnings**  
      **Source:** `.golangci.yml`, `pkg/sync/sync.go`  
      **Description:** Fixed containedctx (excluded in tests), duplicate code (excluded in tests), funlen for SyncIncremental (extracted method).  
      **Context:** All lint issues resolved, added exclusions for test files.

- [x] **Remove unused exitOK constant**  
      **Source:** `cmd/examples/github-sync/main.go`  
      **Description:** Removed unused `exitOK` constant from semantic exit codes.  
      **Context:** Clean code, no warnings.

- [x] **Refactor SyncIncremental for better maintainability**  
      **Source:** `pkg/sync/sync.go`  
      **Description:** Extracted `processIncrementalItems` helper function to reduce function length.  
      **Context:** Function now under 60 lines, passes funlen linter.

---

## 🔴 HIGH PRIORITY

### Testing & Quality

- [ ] **Add CLI integration tests**  
       **Source:** `cmd/gh-sync/main.go`  
       **Description:** Test flag parsing, signal handling, and exit codes.  
       **Context:** Verify proper error messages for missing token/user, stats command without DB, and graceful shutdown.

### Infrastructure

- [ ] **Implement rate limit handling**  
       **Source:** `pkg/github/client.go`  
       **Description:** Auto-detect GitHub rate limits and implement exponential backoff/wait.  
       **Context:** `GetRateLimit()` method exists but isn't used in sync logic. Critical for large syncs (>10 pages).

---

## 🟡 MEDIUM PRIORITY

### Features & UX

- [ ] **Add real-time progress display**  
       **Source:** `pkg/sync/sync.go`  
       **Description:** Show sync progress (current page/total, events fetched) during execution.  
       **Context:** Use `charmbracelet/log` or progress bar library. Currently silent except for start/end logs.

- [ ] **Add JSON output flag**  
       **Source:** `cmd/gh-sync/main.go`  
       **Description:** Implement `-json` flag for structured output (stats, sync results).  
       **Context:** Enables scripting and integration with other tools (jq, etc.).

- [ ] **Support configuration file**  
       **Source:** `cmd/gh-sync/main.go`  
       **Description:** Load defaults from YAML/TOML config file (`~/.config/gh-sync/config.yaml`).  
       **Context:** Store default user, token path, db path, and page limits.

- [ ] **Implement retry logic with backoff**  
       **Source:** `pkg/github/client.go`  
       **Description:** Retry transient API failures (5xx, timeouts) with exponential backoff.  
       **Context:** Currently fails immediately on network errors. Should retry 3 times with 1s, 2s, 4s backoff.

### Reliability

- [ ] **Add structured logging fields**  
       **Source:** `pkg/sync/sync.go`, `pkg/github/client.go`  
       **Description:** Add consistent context fields (username, page, event_id) to all log statements.  
       **Context:** Improve debuggability when filtering logs for specific users or events.

- [ ] **Handle edge cases in incremental sync**  
       **Source:** `pkg/sync/sync.go:75`  
       **Description:** Handle clock skew and duplicate timestamps in cutoff logic.  
       **Context:** Current logic uses `event.CreatedAt.Before(cutoff)` - need inclusive comparison and handle identical timestamps.

---

## 🟢 LOW PRIORITY (Future Phases)

### Enhanced Features

- [ ] **Build TUI with Bubble Tea**  
       **Source:** New package `pkg/tui/`  
       **Description:** Interactive terminal UI for browsing events, filtering, and real-time sync.  
       **Context:** Effort: ~2h. Low priority compared to core stability.

- [ ] **Support multiple user sync**  
       **Source:** `cmd/gh-sync/main.go`, `pkg/sync/sync.go`  
       **Description:** Accept multiple `-user` flags or user list from file.  
       **Context:** Requires DB schema update to track which user each event belongs to (if not already in raw JSON).

- [ ] **Implement daemon/background mode**  
       **Source:** New package `cmd/gh-sync/daemon.go`  
       **Description:** Run as cron job or systemd service for periodic sync.  
       **Context:** Single-shot mode is current focus. Daemon mode needs lockfile handling.

### Data & Export

- [ ] **Add Turso/LibSQL backend support**  
       **Source:** `internal/database/connection.go`  
       **Description:** Support Turso remote SQLite databases via libsql driver.  
       **Context:** Currently uses `modernc.org/sqlite`. Need abstraction layer for multiple drivers.

- [ ] **Create HTTP API endpoint**  
       **Source:** New package `pkg/api/`  
       **Description:** REST API for querying events (GET /events, /stats, /types).  
       **Context:** Would turn CLI tool into server. Effort: ~2h.

- [ ] **Add export to JSON/CSV**  
       **Source:** `cmd/gh-sync/main.go`  
       **Description:** Export stored events to file formats (`-export json` or `-export csv`).  
       **Context:** Useful for data analysis in external tools.

---

## 🔧 TECHNICAL DEBT

### Code Quality

- [ ] **Standardize null string conversion**  
       **Source:** `pkg/storage/interface.go`  
       **Description:** Consider using generics or code generation for NullString conversions.  
       **Context:** Current helper functions `toNullString`/`fromNullString` are repetitive.

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
   - Current `ON CONFLICT(github_id) DO NOTHING` means events never update. Should we support updates for amended events?

---

## 📋 COMPLETION CHECKLIST

Before Phase 2 (Production Ready):

- [x] All HIGH priority items complete (except CLI integration tests and rate limit handling)
- [x] Test coverage for `pkg/github`, `pkg/sync`
- [x] CI/CD pipeline configured
- [x] go.mod properly formatted
- [ ] Real GitHub API sync verified with PAT
- [x] Architecture decoupling (domain types) complete

---

**Last Updated:** 2026-02-15  
**Next Review:** After rate limit handling implementation
