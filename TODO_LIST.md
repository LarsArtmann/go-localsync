# TODO_LIST.md

**Project:** go-localsync  
**Generated:** 2026-02-12  
**Last Updated:** 2026-04-08  
**Status:** Active Development

## Overview

Actionable tasks for the next 2-4 weeks. Items are organized by priority.

---

## 🔴 HIGH PRIORITY

### Testing & Quality

- [ ] **Add CLI integration tests**  
       **Source:** `cmd/examples/github-sync/main.go`  
       **Description:** Test flag parsing, signal handling, and exit codes.  
       **Context:** Verify proper error messages for missing token/user, stats command without DB, and graceful shutdown.

- [ ] **Add migration tests**  
       **Source:** `internal/database/migration.go`  
       **Description:** Test migration idempotency, ordering, and fresh vs existing DB scenarios.  
       **Context:** Migration system was added but has no test coverage.

### Infrastructure

- [ ] **Install golangci-lint v2 binary**  
       **Source:** `.golangci.yml`  
       **Description:** Config uses v2 format but installed binary is v1.64.8.  
       **Context:** `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` — requires user action.

---

## 🟡 MEDIUM PRIORITY

### Features & UX

- [ ] **Add real-time progress display**  
       **Source:** `pkg/sync/sync.go`  
       **Description:** Show sync progress (current page/total, events fetched) during execution.  
       **Context:** Use `charmbracelet/log` or progress bar library. Currently silent except for start/end logs.

- [ ] **Add JSON output flag**  
       **Source:** `cmd/examples/github-sync/main.go`  
       **Description:** Implement `-json` flag for structured output (stats, sync results).  
       **Context:** Enables scripting and integration with other tools (jq, etc.).

- [ ] **Support configuration file**  
       **Source:** `cmd/examples/github-sync/main.go`  
       **Description:** Load defaults from YAML/TOML config file (`~/.config/gh-sync/config.yaml`).  
       **Context:** Store default user, token path, db path, and page limits.

### Reliability

- [ ] **Add structured logging fields**  
       **Source:** `pkg/sync/sync.go`, `pkg/providers/github/client.go`  
       **Description:** Add consistent context fields (username, page, event_id) to all log statements.  
       **Context:** Improve debuggability when filtering logs for specific users or events.

- [ ] **Handle edge cases in incremental sync**  
       **Source:** `pkg/sync/sync.go`  
       **Description:** Handle clock skew and duplicate timestamps in cutoff logic.  
       **Context:** Current logic uses `event.CreatedAt.Before(cutoff)` — need inclusive comparison and handle identical timestamps.

---

## 📋 COMPLETION CHECKLIST

Before Phase 2 (Production Ready):

- [ ] All HIGH priority items complete
- [x] Test coverage for `pkg/providers/github`, `pkg/sync`, `pkg/storage`
- [x] CI/CD pipeline configured
- [x] go.mod properly formatted (no replace directives)
- [x] Architecture decoupling (domain types, branded IDs) complete
- [x] Migration system for schema evolution
- [x] Conflict-aware sync engine functional
- [ ] Real GitHub API sync verified with PAT
- [ ] golangci-lint v2 binary installed and passing

---

**Last Updated:** 2026-04-08
