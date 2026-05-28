# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-05-25
**Status:** Active Development

## Overview

Actionable tasks for the next 2-4 weeks. Items are organized by priority.

---

## 🔴 HIGH PRIORITY

### Testing & Quality

- [x] **Add CLI tests** (Completed 2026-05-25)
      **Source:** `cmd/examples/github-sync/main.go`
      **Description:** Test exitCodeForError, LoadConfig, flag parsing.
      **Context:** 240-line main.go has zero test coverage. Highest-impact testing gap.

- [ ] **Add Push/Pull tests**
      **Source:** `pkg/cqrs/stack.go`
      **Description:** Test `CQRSStack.Push()` and `Pull()` methods.
      **Context:** Turso remote sync is a key differentiator. Currently untested.

### Architecture

- [x] **Wire error taxonomy** (Completed 2026-05-25)
      **Source:** `pkg/cqrs/`, `cmd/examples/github-sync/main.go`
      **Description:** Use go-cqrs-lite's `event.RegisterClassification` for proper CLI exit codes.
      **Context:** Users get generic exit codes instead of domain-specific ones.

- [x] **Adopt projection.Runner** (Completed 2026-05-25)
      **Source:** `pkg/cqrs/projection.go`
      **Description:** Replace custom Projector with go-cqrs-lite's `projection.Runner` for replay + checkpointing.
      **Context:** Custom projector doesn't support replay.

---

## 🟡 MEDIUM PRIORITY

### Testing & Coverage

- [ ] **Migrate test framework to stdlib**
      **Source:** 6 testify files + 1 Ginkgo file
      **Description:** Replace testify assertions and Ginkgo BDD with stdlib `t.Errorf`/`t.Fatal`.
      **Context:** Inconsistent test frameworks. go-cqrs-lite uses stdlib throughout.

- [ ] **Real GitHub PAT smoke test**
      **Source:** `cmd/examples/github-sync/`
      **Description:** Verify actual API sync works end-to-end with a real token.
      **Context:** All testing is mock-based. Never verified with real GitHub API.

### Features & UX

- [ ] **Add JSON output flag**
      **Source:** `cmd/examples/github-sync/main.go`
      **Description:** Implement `-json` flag for structured output (stats, sync results).
      **Context:** Enables scripting and integration with other tools (jq, etc.).

- [ ] **Add structured logging fields**
      **Source:** `pkg/sync/sync.go`, `pkg/providers/github/client.go`
      **Description:** Add consistent context fields (username, page, event_id) to all log statements.
      **Context:** Improve debuggability when filtering logs for specific users or events.

---

## 📋 COMPLETION CHECKLIST

Before Phase 2 (Production Ready):

- [ ] All HIGH priority items complete
- [x] Test coverage for `pkg/cqrs`, `pkg/providers/github`, `pkg/sync`, `pkg/id`, `pkg/errors`
- [x] CI/CD pipeline configured
- [x] go.mod properly formatted (no replace directives)
- [x] Architecture decoupling (domain types, branded IDs) complete
- [x] CQRS migration complete (legacy CRUD deleted)
- [x] Conflict-aware sync engine functional
- [x] Error handling migrated to stdlib (cockroachdb/errors removed)
- [x] Documentation current (README, FEATURES, TODO_LIST, AGENTS, ROADMAP)
- [ ] Real GitHub API sync verified with PAT
- [x] golangci-lint v2 passing (0 issues)
