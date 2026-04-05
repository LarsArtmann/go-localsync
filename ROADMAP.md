# ROADMAP.md

**Project:** go-localsync  
**Last Updated:** 2026-04-05

## Overview

Aspirational features and improvements with no fixed timeline. These are planned for future phases.

---

## 🟢 FUTURE PHASES

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
