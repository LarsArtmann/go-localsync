# TODO_LIST.md

**Project:** go-localsync  
**Generated:** 2026-02-12  
**Last Updated:** 2026-04-05  
**Status:** Active Development

## Overview

Actionable tasks for the next 2-4 weeks. Items are organized by priority.

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

## 📋 COMPLETION CHECKLIST

Before Phase 2 (Production Ready):

- [ ] All HIGH priority items complete (except CLI integration tests and rate limit handling)
- [x] Test coverage for `pkg/github`, `pkg/sync`
- [x] CI/CD pipeline configured
- [x] go.mod properly formatted
- [ ] Real GitHub API sync verified with PAT
- [x] Architecture decoupling (domain types) complete

---

**Last Updated:** 2026-04-05
