 ```markdown
# TODO_LIST.md

**Project:** go-localsync  
**Generated:** 2026-02-12  
**Status:** Phase 1 Complete → Phase 2 Development  

## Overview

This document tracks all TODOs, FIXMEs, and action items identified from code analysis and the Phase 1 completion status report. Items are organized by priority and include source references and implementation context.

---

## 🔴 HIGH PRIORITY

### Testing & Quality

- [ ] **Add GitHub client tests**  
  **Source:** `pkg/github/client.go`  
  **Description:** Create mock HTTP server to test API client, pagination logic, and error handling.  
  **Context:** Currently 0% test coverage. Needs mock for `go-github` client and rate limit testing.

- [ ] **Add sync package tests**  
  **Source:** `pkg/sync/sync.go`  
  **Description:** Test full sync and incremental sync with mocked storage interface.  
  **Context:** Test edge cases including nil options, error handling in batch operations, and cutoff time logic.

- [ ] **Add CLI integration tests**  
  **Source:** `cmd/gh-sync/main.go`  
  **Description:** Test flag parsing, signal handling, and exit codes.  
  **Context:** Verify proper error messages for missing token/user, stats command without DB, and graceful shutdown.

### Critical Issues

- [ ] **Fix go.mod indirect dependencies**  
  **Source:** `go.mod`  
  **Description:** Run `go mod tidy` to properly categorize direct vs indirect dependencies.  
  **Context:** All dependencies currently marked as `// indirect` which is incorrect for required packages like `charmbracelet/log`, `go-github`, etc.

- [ ] **Implement typed/sentinel errors**  
  **Source:** New file: `pkg/errors/errors.go`  
  **Description:** Define domain-specific error types (e.g., `ErrRateLimited`, `ErrInvalidToken`, `ErrUserNotFound`).  
  **Context:** Currently using generic `fmt.Errorf` with `%w` wrapping. Need proper error types for programmatic error handling.

- [ ] **Remove unused Config struct**  
  **Source:** `pkg/sync/sync.go:18`  
  **Description:** Delete the `Config` struct or implement usage in `NewSyncer`.  
  **Context:** Struct defined but never used; creates confusion about configuration pattern.

### Infrastructure

- [ ] **Set up CI/CD Pipeline**  
  **Source:** `.github/workflows/` (new)  
  **Description:** GitHub Actions for test, lint, build, and release.  
  **Context:** Needs Go 1.24, golangci-lint, sqlc validation, and artifact building with proper ldflags for version.

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

### Architecture Improvements

- [ ] **Decouple domain types from storage**  
  **Source:** `pkg/github/client.go:79-90`, `pkg/storage/interface.go`  
  **Description:** Create `pkg/event/types.go` with domain Event type. Remove `storage.Event` import from `github` package.  
  **Context:** Current `convertEvent()` couples GitHub API to storage layer, violating separation of concerns. Sync layer should handle conversion.

- [ ] **Fix version build injection**  
  **Source:** `cmd/gh-sync/main.go:14-16`, `justfile`  
  **Description:** Add ldflags to build process to set version, commit, and date at compile time.  
  **Context:** Currently always shows "dev" version. Needs `-ldflags "-X main.version={{.Version}}"` in build command.

### Code Quality

- [ ] **Add error handling for DB close operations**  
  **Source:** `cmd/gh-sync/main.go:61`  
  **Description:** Check and log error from `dbc.Close()` in defer.  
  **Context:** Currently silently ignoring close errors which could mask data corruption issues.

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

- [ ] All HIGH priority items complete
- [ ] Test coverage >80% for `pkg/github`, `pkg/sync`
- [ ] CI/CD pipeline passing
- [ ] go.mod properly formatted
- [ ] Real GitHub API sync verified with PAT
- [ ] Architecture decoupling (domain types) complete

---

**Last Updated:** 2026-02-12  
**Next Review:** After CI/CD implementation
```