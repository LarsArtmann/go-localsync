# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-06-22
**Tests:** 225 passing, 9 packages | **Lint:** 0 issues (golangci-lint v2)

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

---

## 🔴 HIGH PRIORITY

- [ ] **Rework CI build & release jobs**
      **Source:** `.github/workflows/ci.yml`
      **Description:** The `build` job cross-compiles `./cmd/examples/github-sync`, which was removed when the SDK became a pure contract library (the example now lives in [`github-local-sync`](https://github.com/larsartmann/github-local-sync)). The `release` job depends on `build`, so neither runs successfully. Rework for a library-appropriate release flow (or remove the jobs).
      **Context:** Currently the only failing piece of CI. `test` + `lint` jobs pass.

- [ ] **Make `go-cqrs-lite` public**
      **Source:** `go.mod`, `flake.nix`, `vendor/`
      **Description:** `go-cqrs-lite` is the only private dependency. Its privacy forces a committed `vendor/` dir + `vendorHash = null` in `flake.nix` (the nix sandbox can't fetch it). Making it public enables a real `vendorHash` and drops the `vendor/` dir entirely.
      **Context:** Cleanest long-term fix for the vendored-private-dep workaround documented in AGENTS.md.

---

## 🟡 MEDIUM PRIORITY

### Observability

- [ ] **OpenTelemetry instrumentation**
      **Source:** `pkg/sync/sync.go`, `pkg/cqrs/stack.go`, `pkg/api/server.go`
      **Description:** Add spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, and HTTP middleware. go-cqrs-lite v3 already ships an `otel/v3` module.
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

- [ ] **Improve `pkg/cqrs` coverage (81.4%)**
      **Source:** `pkg/cqrs/`
      **Description:** Lowest-coverage package. Add tests for remaining error paths and store-factory branches.

- [ ] **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution (the `schema.Version` foundation in `pkg/data/schema/` is ready for it).

---

## 🟢 LOWER PRIORITY

- [ ] **Add `govalid` struct tags** to `SyncOptions`, `CQRSConfig`.
- [ ] **Improve `CONTRIBUTING.md`** — add architecture guide, file-split conventions, testing requirements.
- [ ] **Conflict resolution per-sync override** — `SyncOptions.ConflictResolver` for per-sync strategy (currently only `CQRSConfig.ConflictResolver`).
