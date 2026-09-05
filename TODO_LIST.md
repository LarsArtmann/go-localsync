# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-09-05
**Tests:** 10 packages passing (full suite) | **Lint:** 0 issues (golangci-lint v2, `nix run .#lint`)

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

> **Scope note:** go-localsync is deliberately a **single-aggregate Item sync SDK**. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). The tasks below improve the SDK _within its current scope_; do not add tasks that widen it without revisiting that decision.

---

## 🔴 HIGH PRIORITY

### Provider release (unblocks `provider/github` consumers)

- [ ] **Release the core, then the provider module**
      **Source:** `CHANGELOG.md`, `provider/github/go.mod`
      **Description:** Master is far ahead of `v0.4.2` (new `pkg/id`, `pkg/provider` shape, `RateLimit` in `FetchResult`) and the changelog's `[Unreleased]` does not reflect it. Cut the next core release (reconcile the changelog against `v0.4.2..HEAD` first), then bump `provider/github`'s parent pin from the master pseudo-version to that tag and tag the provider module itself. Until then the module resolves standalone only against the pinned master commit.
      **Context:** Also wire CI for the nested module (the root workflow only tests the root module) and consider migrating github-local-sync's `internal/github` onto the shared provider afterwards.

---

## 🟡 MEDIUM PRIORITY

### Observability

- [ ] **OpenTelemetry instrumentation**
      **Source:** `pkg/sync/sync.go`, `pkg/cqrs/stack.go`, `pkg/api/server.go`
      **Description:** Add spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, and HTTP middleware. go-cqrs-lite v4 already ships an `otel/v4` module.
      **Context:** No observability today; production debugging requires log spelunking.

- [ ] **Structured logging fields**
      **Source:** `pkg/sync/sync.go`
      **Description:** Add consistent context fields (source, page, event_id) to all log statements.

### API Hardening

- [ ] **API authentication middleware** (API key or JWT) — the HTTP API is currently unauthenticated.
- [ ] **API pagination headers** (`X-Total-Count`, cursor-based).
- [ ] **API rate limiting middleware** (prevent `POST /sync` abuse).
- [ ] **API OpenAPI spec enhancement** (error response schemas per endpoint).

### Code Quality

- [ ] **Improve `pkg/cqrs` coverage (82.5%)**
      **Source:** `pkg/cqrs/`
      **Description:** Add tests for remaining error paths and store-factory branches.

- [ ] **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution (the `schema.Version` foundation in `pkg/data/schema/` is ready for it).

---

## 🟢 LOWER PRIORITY

- [ ] **Add `govalid` struct tags** to `SyncOptions`, `CQRSConfig`.
- [ ] **Improve `CONTRIBUTING.md`** — add architecture guide, file-split conventions, testing requirements.
- [ ] **Conflict resolution per-sync override** — `SyncOptions.ConflictResolver` for per-sync strategy (currently only `CQRSConfig.ConflictResolver`).
