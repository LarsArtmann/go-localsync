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

- [ ] **Migrate testify→Ginkgo/GOmega**  
       **Source:** All 8 `*_test.go` files across `pkg/storage`, `pkg/sync`, `pkg/providers/github`  
       **Description:** Pre-commit hooks ban testify; entire test suite uses it.  
       **Context:** 8 test files, 48 test cases. Required to unblock pre-commit hooks. ~3h effort.

### Infrastructure

- [ ] **Install golangci-lint v2 binary**  
       **Source:** `.golangci.yml`  
       **Description:** Config uses v2 format but installed binary is v1.64.8.  
       **Context:** `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` — requires user action.

- [ ] **Align Go toolchain to 1.26.1**  
       **Source:** `go.mod`  
       **Description:** `go.mod` says 1.26.1 but installed toolchain is 1.26.0. Blocks `go test -cover`.  
       **Context:** Coverage reports fail with compile errors. Regular build/test works fine.

- [ ] **Fix pre-commit hooks**  
       **Source:** BuildFlow hooks  
       **Description:** 4 categories of failures: library-policy (testify), go-structure-linter, ast-state-analyzer, todo-check.  
       **Context:** Currently bypassed with `--no-verify`. Blocked on testify→Ginkgo migration.

---

## 🟡 MEDIUM PRIORITY

### Testing & Coverage

- [ ] **Increase storage test coverage (56%→80%)**  
       **Source:** `pkg/storage/sqlite.go`, `pkg/storage/sqlite_test.go`  
       **Description:** Add tests for error paths, GetItemsByActor, GetItemsByRepo, CountByType edge cases.  
       **Context:** Current 56% coverage. Many methods untested for error conditions.

- [ ] **Add test coverage for pkg/errors and pkg/types**  
       **Source:** `pkg/errors/errors.go`, `pkg/types/ids.go`  
       **Description:** Sentinel errors and branded IDs have zero test coverage.  
       **Context:** Quick wins — small files, simple logic.

- [ ] **Real GitHub PAT smoke test**  
       **Source:** `cmd/examples/github-sync/`  
       **Description:** Verify actual API sync works end-to-end with a real token.  
       **Context:** All testing is mock-based. Never verified with real GitHub API.

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

- [x] **Rate limit handling in sync flow**  
       **Source:** `pkg/providers/github/client.go`, `pkg/sync/sync.go`  
       **Description:** `GetRateLimit()` and `RateLimitConfig` are wired into the GitHub client's fetch loop.  
       **Completed:** Round 1 audit sessions.

- [x] **Retry logic with exponential backoff**  
       **Source:** `pkg/providers/github/client.go`  
       **Description:** `RetryConfig` drives configurable exponential backoff with jitter.  
       **Completed:** Round 1 audit sessions.

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
- [x] Test coverage for `pkg/providers/github`, `pkg/sync`, `pkg/storage`, `internal/database`
- [x] CI/CD pipeline configured
- [x] go.mod properly formatted (no replace directives)
- [x] Architecture decoupling (domain types, branded IDs) complete
- [x] Migration system for schema evolution
- [x] Conflict-aware sync engine functional
- [x] Source provider indexes for multi-provider queries
- [x] Unused code cleaned up (PaginationMixin, EventCoreMixin removed)
- [x] Documentation current (CHANGELOG, ROADMAP, TODO_LIST, AGENTS, README)
- [ ] Real GitHub API sync verified with PAT
- [ ] golangci-lint v2 binary installed and passing
- [ ] Pre-commit hooks passing
- [ ] testify→Ginkgo migration complete

---

**Last Updated:** 2026-04-08
