# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-07-18
**Tests:** 214 passing, 10 packages | **Lint:** 0 issues (golangci-lint v2)

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

> **Scope note:** go-localsync is deliberately a **single-aggregate Item sync SDK**. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). The tasks below improve the SDK _within its current scope_; do not add tasks that widen it without revisiting that decision.

---

## 🔴 HIGH PRIORITY

- [x] **Rework CI build & release jobs**
      **Source:** `.github/workflows/ci.yml`
      **Description:** The `build` job cross-compiles `./cmd/examples/github-sync`, which was removed when the SDK became a pure contract library (the example now lives in [`github-local-sync`](https://github.com/larsartmann/github-local-sync)). The `release` job depends on `build`, so neither runs successfully. Rework for a library-appropriate release flow (or remove the jobs).
      **Context:** **Done (2026-06-29).** The `build` job now verifies the library compiles across linux/darwin × amd64/arm64; the `release` job creates a binary-free GitHub release with auto-generated notes.

- [ ] **Make `go-cqrs-lite` public**
      **Source:** `go.mod`, `flake.nix`, `vendor/`
      **Description:** `go-cqrs-lite` is the only private dependency. Its privacy forces a committed `vendor/` dir + `vendorHash = null` in `flake.nix` (the nix sandbox can't fetch it). Making it public enables a real `vendorHash` and drops the `vendor/` dir entirely.
      **Context:** Cleanest long-term fix for the vendored-private-dep workaround documented in AGENTS.md.

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

- [ ] **Improve `pkg/cqrs` coverage (80.9%)**
      **Source:** `pkg/cqrs/`
      **Description:** Add tests for remaining error paths and store-factory branches.

- [ ] **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution (the `schema.Version` foundation in `pkg/data/schema/` is ready for it).

---

## 🟢 LOWER PRIORITY

- [ ] **Add `govalid` struct tags** to `SyncOptions`, `CQRSConfig`.
- [ ] **Improve `CONTRIBUTING.md`** — add architecture guide, file-split conventions, testing requirements.
- [ ] **Conflict resolution per-sync override** — `SyncOptions.ConflictResolver` for per-sync strategy (currently only `CQRSConfig.ConflictResolver`).
