# Go-LocalSync — Comprehensive Status Report

**Date:** 2026-06-29 15:47 CEST
**Branch:** `master`
**Last Tag:** `v0.3.0` (2026-06-23)
**HEAD:** `7abba09` — Sync vendor documentation and normalize markdown table formatting

---

## Executive Summary

Go-LocalSync is a **single-writer, pull-only, flat-Item sync SDK** built on event-sourced CQRS via go-cqrs-lite v3.3–v3.4. The architecture is clean: no legacy CRUD, no split brains, deterministic idempotent sync, pluggable conflict resolution, and tombstone-based soft-deletes with upstream reconciliation. The codebase is **190 tests, 0 lint issues, build green**.

The biggest recurring pain point is **vendor/ drift**: every time `go.mod` dependency versions are bumped, the committed `vendor/` directory is not re-synced, breaking `go build` / `go test` / `nix build` until someone runs `go mod vendor`. This has happened at least **3 times** (v3.0→v3.1, v3.1→v3.3/v3.4, and during this session). The root cause is that go-cqrs-lite is private, forcing a committed `vendor/` dir + `vendorHash = null`.

---

## a) FULLY DONE ✅

| Area                                | Details                                                                                                                                                                                                                                                                           |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **CQRS Architecture**               | Full event-sourced storage via go-cqrs-lite v3.3–v3.4. Decider, ReadModel, Projection, Stack. No legacy CRUD path. 3 event types: `ItemSynced`, `ItemConflictFound`, `ItemTombstoned`.                                                                                            |
| **Tombstone Soft-Deletes**          | Hard-deletes replaced with tombstones (`Item.Tombstone`). Re-syncing a tombstoned item resurrects it automatically. Per-aggregate version gate prevents replay from resurrecting stale items.                                                                                     |
| **Conflict Detection & Resolution** | `DecideSync` is the single authority. `HasChanged()` compares UpdatedAt/Type/ActorLogin/RepoName/RepoURL. Default: remote-wins LWW. Pluggable: inject any `crdt.ConflictResolver[*model.Item]`. `ConflictWinnerRemote`/`ConflictWinnerLocal` exported with `ParseConflictWinner`. |
| **Reconciliation**                  | Opt-in `SyncOptions.Reconcile` tombstones items absent from a COMPLETE fetch (`ReasonUpstreamGone`). **Data-loss guard**: refuses reconciliation on an incomplete fetch (commit `c294ab3`).                                                                                       |
| **Dual Backends**                   | In-memory (testing/dev) + SQLite (production) via `modernc.org/sqlite` (pure-Go, no CGo). Identical `ReadModel` API. WAL mode, aggregate-ID cache, `CountByType` (fixes N+1).                                                                                                     |
| **Provider Abstraction**            | Generic `Provider` interface — the SDK ships no provider. Reference consumer (GitHub) lives in `github-local-sync`.                                                                                                                                                               |
| **HTTP API**                        | 4 endpoints via Huma v2: `GET /items` (filter+pagination), `GET /stats`, `POST /sync`, `GET /health`. OpenAPI 3 auto-generated.                                                                                                                                                   |
| **Branded IDs**                     | 6 phantom-type IDs via go-branded-id: `ItemID` (ULID), `ExternalID`, `ProviderID`, `ActorLogin`, `RepoID`, `EventTypeID`. Compile-time type safety.                                                                                                                               |
| **Error Taxonomy**                  | `go-error-family` constructors with intrinsic classification (Rejection, Transient, Infrastructure). `IsRetryable` for smart provider-error retry.                                                                                                                                |
| **Resilient Fetch**                 | Exponential backoff + ±25% jitter, `errors.IsRetryable`, honors `Retry-After` header. Configurable via functional options (commit `ca0844c`).                                                                                                                                     |
| **Per-Source Serialization**        | Per-source mutex orders concurrent syncs of same source (TOCTOU guard on latest-timestamp read). Different sources run in parallel.                                                                                                                                               |
| **Projection**                      | Synchronous live delivery via `bus.SubscribeAll` + background `replayJournal`. Idempotent — no checkpoint store needed. Uses `projection/v3` interface (re-introduced in go-cqrs-lite v3.2, ADR-0037).                                                                            |
| **DTO/Domain Boundary**             | `provider.Item` (DTO) → `model.Item` (domain entity) via `item_adapter.go`. Decider, read model, events, and resolver all use `*model.Item`.                                                                                                                                      |
| **Test Suite**                      | **190 test functions** across 9 packages, all passing. Coverage: 5 packages at 100%, lowest is `data/model` at 80.5%.                                                                                                                                                             |
| **Lint**                            | golangci-lint v2 with `enable-all`, **0 issues**. Strict `.golangci.yml`.                                                                                                                                                                                                         |
| **Scope Discipline**                | ADR-0004 explicitly defers multi-aggregate generalisation. The SDK is honestly scoped as a single-aggregate, pull-only, flat-Item sync engine. CRDT machinery (vector clocks, operations, sync messages) deliberately removed.                                                    |

---

## b) PARTIALLY DONE 🟡

| Area              | What's Done                                                                                                                     | What's Missing                                                                                                                                                                                                                                                                         |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **CI/CD**         | `test` + `lint` jobs pass on every push/PR. Go 1.26, `CGO_ENABLED=0`, race detector, coverage upload.                           | `build` + `release` jobs are **broken** (target deleted `./cmd/examples/github-sync`). No library-appropriate release flow.                                                                                                                                                            |
| **Test Coverage** | 7 of 9 packages ≥ 85%. Overall strong.                                                                                          | `pkg/cqrs` at 82.1% (lowest of the core packages). `pkg/data/model` at 80.5%. Error paths and store-factory branches uncovered.                                                                                                                                                        |
| **Documentation** | Core docs (AGENTS, README, FEATURES, TODO_LIST, ROADMAP, CHANGELOG) exist and are mostly accurate. 5 ADRs, domain language doc. | **Recurring drift**: test counts, dependency versions, and feature-status rows fall behind after refactors. The crdt-package gutting (vector clock removal) left ghost references in FEATURES.md that were only caught in this session. Vendor drift breaks documented build commands. |
| **flake.nix**     | DevShell (Go 1.26, golangci-lint, ginkgo, gotools, gofumpt). `buildGoModule` package. treefmt config.                           | `nix build` / `nix flake check` **fail** because nixpkgs Go lag (1.26.3 < required 1.26.4) and the private-dep vendor workaround (`vendorHash = null`).                                                                                                                                |
| **HTTP API**      | 4 endpoints functional, filterable, OpenAPI spec.                                                                               | No auth, no rate limiting, no pagination headers (`X-Total-Count`), no error-response schemas in OpenAPI.                                                                                                                                                                              |

---

## c) NOT STARTED ⬜

| Area                                    | Description                                                                                                                                                                     |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **OpenTelemetry instrumentation**       | go-cqrs-lite v3 ships `otel/v3` (it's an indirect dep). No spans in `Syncer.Sync()`, `CQRSStack.SyncItems()`, or HTTP middleware. Production debugging requires log spelunking. |
| **API authentication**                  | No API key, JWT, or any auth middleware. The HTTP API is not safe to expose on a network.                                                                                       |
| **API rate limiting**                   | No middleware to prevent `POST /sync` abuse.                                                                                                                                    |
| **API pagination headers**              | No `X-Total-Count` or cursor-based pagination metadata.                                                                                                                         |
| **Data export**                         | No JSON/CSV export of stored events or read-model data.                                                                                                                         |
| **Upcaster registry**                   | `schema.Version` (V1/V2) foundation is in place but go-cqrs-lite's `UpcasterRegistry` is not adopted for automatic event migration.                                             |
| **Per-sync conflict resolver override** | `SyncOptions.ConflictResolver` for per-sync strategy (currently only `CQRSConfig.ConflictResolver`). Listed in both TODO_LIST and ROADMAP.                                      |
| **Structured logging fields**           | No consistent context fields (source, page, event_id) across all log statements.                                                                                                |
| **go-cqrs-lite public visibility**      | Still a private GitHub repo. Forces committed `vendor/` + `vendorHash = null`. Making it public enables real `vendorHash` and drops `vendor/` entirely.                         |
| **CONTRIBUTING.md depth**               | Currently a minimal 2-command guide. No architecture overview, testing requirements, or PR checklist.                                                                           |

---

## d) TOTALLY FUCKED UP! 🔴

| Issue                                     | Severity        | Details                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| ----------------------------------------- | --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Recurring vendor/ drift**               | 🔴 **Critical** | Every `go.mod` dependency bump leaves `vendor/` stale. `go build`, `go test`, and `nix build` all fail with "inconsistent vendoring" until someone manually runs `go mod vendor`. This happened at v3.0→v3.1 (commit `12b15c9`), at v3.1→v3.3/v3.4 (commit `7abba09` or earlier), and again in this session. **Every documented build/test command was broken at HEAD until this session re-vendored.** The root cause is structural: go-cqrs-lite is private, so `vendor/` must be committed, but nothing automates re-vendoring on `go.mod` changes. There is no CI check, pre-commit hook, or Makefile target that catches this. |
| **CI build/release jobs**                 | 🔴 **High**     | The `build` job (`.github/workflows/ci.yml:96`) cross-compiles `./cmd/examples/github-sync`, which was deleted when the SDK became a pure contract library. The `release` job depends on `build`, so neither runs. Tag-triggered releases via `softprops/action-gh-release` never fire. This has been broken since v0.2.0 (over a month).                                                                                                                                                                                                                                                                                           |
| **nix flake check is permanently broken** | 🟠 **Medium**   | Two compounding issues: (1) nixpkgs `go_1_26` packages 1.26.3 but `go.mod` requires 1.26.4; (2) the private-dep vendor workaround (`vendorHash = null`) is fragile. The `GOTOOLCHAIN=local` sandbox flag makes nix build impossible. This self-resolves only when nixpkgs bumps Go.                                                                                                                                                                                                                                                                                                                                                 |

---

## e) WHAT WE SHOULD IMPROVE! 💡

### Architecture & Code Quality

1. **Automate vendor sync** — Add a pre-commit hook, CI check, or Makefile target that runs `go mod vendor` and fails if `vendor/` is stale after a `go.mod` change. This single fix eliminates the most frequent breakage.
2. **Make go-cqrs-lite public** — Eliminates the `vendor/` workaround entirely, enables real `vendorHash`, and unblocks `nix flake check`. This is the cleanest long-term fix for the vendor drift.
3. **Add an integration test that actually builds** — A CI step that runs `go build ./...` in vendor mode would catch vendor drift before merge.
4. **Reduce vendor/ size** — At 247MB, the vendor directory is enormous. Pruning test files from vendored deps (`go mod vendor` already does this, but the committed state may include stale extras) would reduce clone time.

### Testing & Observability

5. **Instrument with OpenTelemetry** — `otel/v3` is already a transitive dependency. Adding spans to `Syncer.Sync()`, `CQRSStack.SyncItems()`, and HTTP middleware is low-hanging fruit with high production-debugging value.
6. **Push cqrs coverage past 85%** — The lowest-coverage core package. Focus on error paths and store-factory branches.
7. **Add contract tests for the Provider interface** — A standard test suite that any provider implementation can embed, ensuring contract compliance.

### Documentation Discipline

8. **Stop hardcoding counts** — Test counts, package counts, and version numbers rot within one commit. Either compute them dynamically in docs or avoid stating them. The test count has been wrong (188, 224, 225, 190) in different docs simultaneously.
9. **Single source of truth for metrics** — Consider a `scripts/metrics.sh` that emits current test count, coverage, lint status. Docs reference it rather than hardcoding.

### Security & API

10. **API auth middleware** — Even a simple API-key middleware would make the HTTP API safe for local-network exposure.
11. **Rate-limit POST /sync** — Prevent accidental or intentional sync-spam.

---

## f) Top #25 Things We Should Get Done Next

Ranked by impact × urgency. Pareto-ordered.

| #  | Task                                                                                                        | Impact      | Effort | Package/Area                      |
| -- | ----------------------------------------------------------------------------------------------------------- | ----------- | ------ | --------------------------------- |
| 1  | **Add CI check for vendor consistency** (`go mod verify` or build-in-vendor-mode step)                      | 🔴 Critical | S      | `.github/workflows/ci.yml`        |
| 2  | **Rework CI build/release jobs** for a pure library (remove `./cmd/examples/github-sync` target)            | 🔴 Critical | S      | `.github/workflows/ci.yml`        |
| 3  | **Make go-cqrs-lite public** — eliminates vendor workaround, unblocks nix                                   | 🔴 Critical | S      | `go-cqrs-lite` repo               |
| 4  | **Add pre-commit hook for `go mod vendor`** — catches vendor drift before commit                            | 🔴 Critical | S      | git hooks / justfile / flake      |
| 5  | **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution                                         | 🟡 High     | M      | `pkg/data/schema`, `pkg/cqrs`     |
| 6  | **OpenTelemetry instrumentation** for Sync, CQRS, HTTP                                                      | 🟡 High     | M      | `pkg/sync`, `pkg/cqrs`, `pkg/api` |
| 7  | **API authentication middleware** (API key or JWT)                                                          | 🟡 High     | S      | `pkg/api`                         |
| 8  | **Push `pkg/cqrs` coverage past 85%** (currently 82.1%)                                                     | 🟡 High     | M      | `pkg/cqrs`                        |
| 9  | **Per-sync conflict resolver override** (`SyncOptions.ConflictResolver`)                                    | 🟡 Medium   | S      | `pkg/sync`                        |
| 10 | **API rate limiting middleware** for `POST /sync`                                                           | 🟡 Medium   | S      | `pkg/api`                         |
| 11 | **API pagination headers** (`X-Total-Count`, cursor links)                                                  | 🟡 Medium   | S      | `pkg/api`                         |
| 12 | **Data export** (JSON/CSV of events and read-model)                                                         | 🟡 Medium   | M      | new `pkg/export`                  |
| 13 | **Structured logging fields** (source, page, event_id consistently)                                         | 🟢 Low      | S      | `pkg/sync`, `pkg/cqrs`            |
| 14 | **Drop `vendor/` once go-cqrs-lite is public** — switch to real `vendorHash`                                | 🟡 High     | S      | `flake.nix`, `go.mod`             |
| 15 | **Provider contract test suite** — embeddable test harness for any `Provider` impl                          | 🟢 Low      | M      | `pkg/testutil`                    |
| 16 | **`govalid` struct tags** on `SyncOptions`, `CQRSConfig`                                                    | 🟢 Low      | S      | `pkg/sync`, `pkg/cqrs`            |
| 17 | **Improve `CONTRIBUTING.md`** — architecture guide, testing requirements, PR checklist                      | 🟢 Low      | S      | `CONTRIBUTING.md`                 |
| 18 | **OpenAPI error-response schemas** per endpoint                                                             | 🟢 Low      | S      | `pkg/api`                         |
| 19 | **Lower `go.mod` directive when nixpkgs catches up** OR document the nix-Go-lag workaround more prominently | 🟢 Low      | S      | `go.mod`, `AGENTS.md`             |
| 20 | **Add `nix flake check` to CI** once nixpkgs Go ≥ 1.26.4                                                    | 🟢 Low      | S      | `.github/workflows/ci.yml`        |
| 21 | **Conflict result error detail** — ensure `ConflictResult.Errors` is surfaced in API `SyncSummary`          | 🟢 Low      | S      | `pkg/api`, `pkg/sync`             |
| 22 | **Snapshot store integration test** — verify aggregate state survives restart across SQLite                 | 🟢 Low      | M      | `pkg/cqrs`                        |
| 23 | **Benchmarks for projection replay** — measure `replayJournal` cost at scale                                | 🟢 Low      | S      | `pkg/cqrs`                        |
| 24 | **Review `pkg/crdt` package name** — it carries no CRDT machinery; consider renaming to `pkg/conflict`      | 🟢 Low      | S      | `pkg/crdt` → `pkg/conflict`       |
| 25 | **Consolidate status report archive** — 48 status reports in `docs/status/`; consider archiving old ones    | 🟢 Low      | S      | `docs/status/`                    |

---

## g) Top #1 Question I Cannot Figure Out Myself

> **Why does `vendor/` keep drifting from `go.mod`?**
>
> This has happened at least 3 times: `go.mod` is bumped to new go-cqrs-lite versions, but `vendor/modules.txt` and the vendored source still reference the old versions. Every documented `go build` / `go test` command breaks until someone manually runs `go mod vendor`.
>
> I cannot determine the **root cause** from the codebase alone:
>
> - Is `go.mod` being edited by hand without a subsequent `go mod vendor`?
> - Is a tool (buildflow, a rename script, an AI agent) bumping `go.mod` without re-vendoring?
> - Is there a workflow where `go mod tidy` or `go get` runs but `go mod vendor` is skipped?
> - Is the `vendor/` directory being partially reverted by a git operation?
>
> **What I need from you:** Confirmation of how `go.mod` gets modified in your workflow, so we can add the right guard (pre-commit hook, CI check, or Makefile target) at the correct point in the process. The structural fix (making go-cqrs-lite public) eliminates the problem entirely, but until then we need a guard.

---

## Verification Commands

```bash
go build ./...                          # ✅ Build green
go test ./... -count=1                  # ✅ 190 tests, 9 packages, all pass
golangci-lint run ./... --timeout=5m    # ✅ 0 issues
go mod verify                           # ✅ All modules verified
```

## Metrics Snapshot

| Metric              | Value                        |
| ------------------- | ---------------------------- |
| Packages            | 10 (9 with tests)            |
| Test functions      | 190 (+ 10 Example/Benchmark) |
| Non-test LOC        | 4,408                        |
| Test LOC            | 6,043                        |
| Direct dependencies | 18                           |
| Total dependencies  | 71                           |
| Vendor size         | 247 MB                       |
| Coverage (avg)      | ~93% (5 packages at 100%)    |
| Lint issues         | 0                            |
| ADRs                | 5                            |
| Status reports      | 48 (this is #49)             |
| Go version          | 1.26.4                       |
| go-cqrs-lite        | v3.3.0–v3.4.0                |

---

_Generated 2026-06-29 15:47 CEST. Point-in-time snapshot — verify against code before acting._

---

## Resolution (2026-07-22)

This snapshot was taken at go-cqrs-lite v3.3–v3.4. Since then, **v0.4.0** shipped (2026-07-18) with major changes:

| Claim in this report                 | Current state                                                                                                          |
| ------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| 190 tests, go-cqrs-lite v3.3–v3.4    | **216 tests**, go-cqrs-lite **v4** (all modules at v4.x paths)                                                         |
| CI `build`/`release` jobs broken     | **Fixed** — cross-platform compile verify + binary-free GitHub releases                                                |
| `nix flake check` permanently broken | **Fixed** — nixpkgs `go_1_26` now at 1.26.4                                                                            |
| vendor/ drift recurring              | Still requires manual `GOWORK=off go mod vendor` + force-add, but documented in AGENTS.md                              |
| `pkg/cqrs` coverage 82.1%            | **82.5%**                                                                                                              |
| Tombstones / projectionhost          | **Shipped** — ADR-0005 (tombstones) + ADR-0006 (`projectionhost.Host` with checkpoint, DLQ, crash-restart)             |
| Error-handling overhaul              | **Shipped** — `go-error-family` constructors, `pkgerrors.HTTPStatus`, `WithCtx`/`InvalidField`, partial-sync surfacing |
| De-githubify                         | **Shipped** — ADR-0007, provider-agnostic `Attributes` map                                                             |
| cqrs-lint                            | **Shipped** — 10 AST invariants (C0001–C0010)                                                                          |

**Still open:** OpenTelemetry wiring, API auth/rate-limiting, making `go-cqrs-lite` public. See TODO_LIST.md.
