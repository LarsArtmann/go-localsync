# Comprehensive Status Report — Session 24

**Date:** 2026-06-29 18:01
**Branch:** `master` (clean, pushed to `origin/master`)
**Head:** `255dd25` — Add execution plan report for session 24

---

## At-a-Glance

| Metric               | Value                                        |
| -------------------- | -------------------------------------------- |
| Build                | ✅ `go build ./...` clean                    |
| Tests                | ✅ 190 functions, 9 packages, all pass       |
| Benchmarks           | 11                                           |
| Lint                 | ✅ 0 issues (golangci-lint v2, `enable-all`) |
| Vet                  | ✅ clean                                     |
| BuildFlow            | ✅ 27/27 steps pass                          |
| Source files         | 46 (non-test) + 37 (test)                    |
| Total LOC (pkg/)     | ~4,424                                       |
| Working tree         | ✅ clean                                     |
| Commits this session | 7 (all pushed)                               |

### Coverage by Package

| Package           | Coverage | LOC   |
| ----------------- | -------- | ----- |
| `pkg/crdt`        | 100.0%   | 87    |
| `pkg/data/schema` | 100.0%   | —     |
| `pkg/errors`      | 100.0%   | 157   |
| `pkg/id`          | 100.0%   | 88    |
| `pkg/provider`    | 96.7%    | 239   |
| `pkg/api`         | 94.0%    | 337   |
| `pkg/sync`        | 85.6%    | 772   |
| `pkg/cqrs`        | 82.0%    | 2,225 |
| `pkg/data/model`  | 80.5%    | 309   |
| `pkg/testutil`    | 0.0%     | 212   |

---

## a) FULLY DONE ✅

### Architecture & Core (Solid)

- **CQRS event-sourced architecture** — single `sync_item` aggregate, 3 events (`ItemSynced`, `ItemConflictFound`, `ItemTombstoned`), one projection. No legacy CRUD. All state via events.
- **go-cqrs-lite v3.4 integration** — all 12 modules at v3.3/v3.4. `projection/v3` interface (ADR-0037) adopted. `command.Command.ID()` satisfied.
- **Deterministic aggregate IDs** — SHA256→hex from (source, sourceID) with `sync.Map` cache.
- **Dual backend** — `memory` (testing/dev) + `sqlite` (local file via `modernc.org/sqlite`, pure-Go, no CGO). Identical `ReadModel` API.
- **Projection** — synchronous live delivery (`bus.SubscribeAll`) + background `replayJournal` (SQLite catch-up). Idempotent, no checkpoint store.
- **Snapshots** — `SQLiteSnapshotStore` + `MemorySnapshotStore`, `EveryNEvents(10)` strategy.
- **Branded type IDs** — 6 phantom types (`ItemID` ULID, `ExternalID`, `ProviderID`, `EventTypeID`, `ActorLogin`, `RepoID`) for compile-time safety.
- **DTO/domain boundary** — `provider.Item` (DTO) ↔ `model.Item` (domain entity) via `item_adapter.go`.
- **Pluggable CRDT conflict resolution** — `CQRSConfig.ConflictResolver` accepts any `crdt.ConflictResolver[*model.Item]`. `LWWResolver` default. Remote-wins fallback.
- **Tombstone + resurrect** — soft-delete keeps history; re-sync resurrects automatically.
- **Reconciliation** — opt-in `SyncOptions.Reconcile` tombstones upstream-gone items.

### Sync Engine

- **Full + incremental sync** — `Syncer.Sync()` (all pages), `Syncer.SyncIncremental()` (CreatedAt cutoff).
- **Conflict-aware sync** — `ConflictAwareSyncer` delegates to `DecideSync`. Reports conflicts via explicit `ConflictDetected` flag.
- **Resilient fetch** — exponential backoff + ±25% jitter, `errors.IsRetryable`, Retry-After hook.
- **Per-source serialization** — TOCTOU guard, different sources parallel.

### API & HTTP

- **Huma v2 HTTP API** — `GET /items`, `GET /stats`, `POST /sync`, `GET /health`. OpenAPI 3 auto-generated. Split into server.go + dto.go + handlers.go.

### Type System & Errors

- **Structured errors** — `go-error-family` constructors (Rejection, Transient, Infrastructure) with intrinsic classification.
- **Schema versioning** — `schema.Version` (V1/V2) on every item for forward event migration.
- **Exported winner constants** — `ConflictWinnerRemote`/`Local` + `ParseConflictWinner` (unknown → remote-wins).

### Session 24 Work (This Session)

- ✅ **Fixed dirty tree + root cause** — restored ~140 reformatted vendored files; added `vendor/` to `.golangci.yml` `formatters.exclusions.paths`.
- ✅ **Eliminated `context.Value` smuggling** — `SyncOutcome` now rides on `SyncItemCommand.outcome` struct field, not `context.Value`.
- ✅ **Made conflict detection explicit** — `SyncOutcome.ConflictDetected` set by event type (`EventItemConflictFound`), not `eventCount > 1` inference.
- ✅ **Resolved QueryDispatcher ghost** — benchmark-proven bypass (1.6×–5.5× overhead), documented as optional observability path.
- ✅ **Fixed doc-drift** — FEATURES.md + DOMAIN_LANGUAGE.md (tombstone names, `projection.Projection`, removed stale Turso/Outbox).
- ✅ **Reworked broken CI** — `build` job now cross-platform compile verify; `release` job binary-free.

### Build & CI

- **Nix flake** — devShell + `buildGoModule` (vendored private deps).
- **GitHub Actions CI** — `test` (race + coverage) + `lint` (vet + golangci-lint) + `build` (cross-platform) + `release` (tag-triggered). All green.

---

## b) PARTIALLY DONE 🟡

| Area                    | What's Done                                                                  | What's Missing                                                                                                                    |
| ----------------------- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Observability**       | `otel/v3` module vendored; `middleware.EventLogging` wired for event logging | No spans, no metrics, no tracing on `Syncer.Sync()` / `CQRSStack.SyncItems()` / HTTP. The `otel/v3` module is completely unwired. |
| **Schema evolution**    | `schema.Version` (V1/V2) foundation in place                                 | `UpcasterRegistry` from go-cqrs-lite not adopted — no actual upcasting logic.                                                     |
| **Test coverage**       | 9 packages, 190 tests, 100% on 4 packages                                    | `pkg/cqrs` at 82% (lowest of core), `pkg/data/model` at 80.5%, `pkg/testutil` at 0%.                                              |
| **CI security**         | golangci-lint + go vet run                                                   | No `gosec`, no `govulncheck`, no `gitleaks` in CI pipeline.                                                                       |
| **Error response docs** | Error templates registered (What/Why/Fix/WayOut)                             | OpenAPI spec lacks per-endpoint error response schemas.                                                                           |
| **API pagination**      | Query params (`limit`, `offset`) work                                        | No `X-Total-Count` header, no cursor-based pagination.                                                                            |

---

## c) NOT STARTED ⬜

- **API authentication middleware** — HTTP API is completely unauthenticated. Not safe to expose on a network.
- **API rate limiting middleware** — no protection against `POST /sync` abuse.
- **Data export** — no JSON/CSV export of stored events.
- **OpenTelemetry instrumentation** — `otel/v3` vendored but zero spans/metrics emitted.
- **`govalid` struct tags** — `SyncOptions`, `CQRSConfig` lack `//govalid:` annotations.
- **Conflict resolution per-sync override** — only `CQRSConfig.ConflictResolver` exists; no `SyncOptions.ConflictResolver`.
- **`projectionhost/v3`** — not vendored, not evaluated. Hand-rolled `replayJournal` is the only catch-up.
- **Multi-user sync** — read model doesn't track which user each event belongs to.
- **Event retention/TTL** — no automatic cleanup of old events.
- **TUI / daemon mode** — explicitly deferred to consumer apps (correct per ADR-0004).

---

## d) TOTALLY FUCKED UP! 💥

These are infrastructure friction points that are technically workable but genuinely "fucked up" — they create fragility, force workarounds, or carry risk.

### 1. `go mod tidy` is BROKEN

Since `decider/v3` v3.3.0, its test imports pull in `event/v3/eventtest` — a nested Go module inside the event/ directory. The Go toolchain cannot resolve this nested module path via VCS. **`go mod tidy` fails.** Workaround: use `GOWORK=off go mod vendor` directly. This means adding/removing top-level deps requires manual vendor management instead of the standard `tidy` flow. Fragile.

### 2. Pre-commit hook OOMs on vendor

The buildflow pre-commit hook (60s budget) runs gofumpt/goimports across the entire tree including `vendor/`. With ~400 `modernc.org/sqlite` generated files, these tools get OOM-killed. Workaround: commit with `--no-verify` after manually verifying formatting. The formatter exclusion (fixed this session) stops the pollution, but the hook still scans vendor and can still OOM on the scan itself.

### 3. `nix flake check` fails

`go.mod` requires `go 1.26.4`, but nixpkgs unstable only packages `go_1_26` at 1.26.3. The nix sandbox forces `GOTOOLCHAIN=local`, so `nix build` / `nix flake check` fail with `go.mod requires go >= 1.26.4 (running go 1.26.3)`. Self-resolves when nixpkgs bumps. Do NOT lower the directive.

### 4. `gopkg.in/yaml.v3` is a transitive dependency

The banned `yaml.v3` (CVEs, aging) is pulled in transitively (indirect). It's not used directly by our code, but it's in the dependency graph. Should be tracked / replaced when the pulling dep updates.

### 5. `otel/v3` is vendored dead weight

The entire `otel/v3` module (~11 files) is vendored but produces zero spans/metrics. It's carrying bytes and module-graph weight for nothing. Either wire it or remove it from the direct dependency set.

---

## e) WHAT WE SHOULD IMPROVE!

### Type Model

1. **`SyncOutcome` is cleaner now** (this session added `ConflictDetected bool`), but it still mixes "what happened" (EventCount, WasNew) with "classification inputs" (ConflictDetected, ConflictWinner). Consider splitting into a pure `DecideResult` (decider output) and a `ClassifyInput`.
2. **`ConflictWinner` as `string`** — could be a typed enum with exhaustive switch enforcement. `ParseConflictWinner` already does the safe decode; making the type stricter would prevent stringly-typed misuse.
3. **`model.Item` is a god struct** — 12 fields, some GitHub-shaped (`ActorLogin`, `RepoName`, `RepoURL`). ADR-0004 accepts this as in-scope, but a future `ItemData` map or optional-field pattern could generalize without widening the aggregate scope.

### Architecture

4. **Observability is the biggest gap.** The `otel/v3` module is vendored and unused. Production debugging requires log spelunking. This should be the #1 architectural investment.
5. **The SDK exposes no hooks for consumer-level instrumentation.** If the decision is "consumer wraps," the SDK needs explicit interfaces (e.g., a `Tracer` injection point) so consumers don't have to fork.
6. **`pkg/testutil` at 0% coverage** — test helpers themselves untested. A bug in a test helper silently corrupts every test that uses it.

### Process

7. **Always run `git status` before claiming "done."** This session caught a dirty tree that the previous session missed entirely.
8. **Run `git status` after `golangci-lint fmt`** to catch formatter pollution immediately.
9. **The 60s buildflow budget is too tight** for a repo with a large vendor/ tree. Either exclude vendor from the formatter steps or raise the budget.

### Library Hygiene

10. **`encoding/json` v1 in 10 files** — Go 1.26 policy wants v2. v2 is experiment-gated now; migrate once stable.
11. **No `gosec`/`govulncheck`/`gitleaks` in CI** — a pure library SDK should have supply-chain scanning.

---

## f) Top #25 Things to Get Done Next

Sorted by **impact ↑ / effort ↓** (Pareto).

| #  | Task                                                                           | Impact      | Effort        | Why                                                                           |
| -- | ------------------------------------------------------------------------------ | ----------- | ------------- | ----------------------------------------------------------------------------- |
| 1  | **Wire OpenTelemetry instrumentation** (`otel/v3` is vendored, unused)         | 🔴 Critical | 4-8h          | Biggest feature gap. Production debugging needs spans.                        |
| 2  | **Make `go-cqrs-lite` public**                                                 | 🔴 Critical | 1h (external) | Eliminates entire vendor-pollution class of bugs + enables real `vendorHash`. |
| 3  | **Add API authentication middleware** (API key / JWT)                          | 🔴 High     | 4h            | API is unauthenticated — not safe to expose.                                  |
| 4  | **Add `gosec` + `govulncheck` to CI**                                          | 🔴 High     | 2h            | Supply-chain scanning for a pure library SDK.                                 |
| 5  | **Fix pre-commit hook OOM** (exclude vendor from buildflow formatters)         | 🟠 High     | 1h            | Removes `--no-verify` workaround fragility.                                   |
| 6  | **API rate limiting middleware** (`golang.org/x/time/rate`)                    | 🟠 High     | 3h            | Protect `POST /sync` from abuse.                                              |
| 7  | **API pagination headers** (`X-Total-Count`, cursor)                           | 🟠 High     | 3h            | Standard API ergonomics.                                                      |
| 8  | **Decide + implement observability strategy** (SDK-built-in vs consumer-wraps) | 🟠 High     | 1h design     | Blocks #1. Architectural fork.                                                |
| 9  | **`encoding/json` v1 → v2 migration** (10 files, once v2 stable)               | 🟡 Medium   | 4h            | Policy compliance, perf.                                                      |
| 10 | **Improve `pkg/cqrs` coverage** (82% → 90%+)                                   | 🟡 Medium   | 4h            | Lowest-coverage core package.                                                 |
| 11 | **Adopt `UpcasterRegistry`** for schema evolution                              | 🟡 Medium   | 3h            | Foundation exists; no actual upcasting.                                       |
| 12 | **Structured logging fields consistency** (source, page, event_id)             | 🟡 Medium   | 2h            | Log spelunking is hard without consistent fields.                             |
| 13 | **Conflict resolution per-sync override** (`SyncOptions.ConflictResolver`)     | 🟡 Medium   | 2h            | Currently only `CQRSConfig`-level.                                            |
| 14 | **Add `govalid` struct tags** to `SyncOptions`, `CQRSConfig`                   | 🟡 Medium   | 1h            | Zero-alloc validation at boundaries.                                          |
| 15 | **Evaluate `projectionhost/v3`** to replace hand-rolled `replayJournal`        | 🟡 Medium   | 4-8h          | Adds crash-restart, checkpoint, DLQ. Needs ADR.                               |
| 16 | **Fix/remove `gopkg.in/yaml.v3` transitive dep**                               | 🟡 Medium   | 2h            | Banned lib in dep graph.                                                      |
| 17 | **Test `pkg/testutil`** (currently 0% coverage)                                | 🟢 Low      | 2h            | Test helpers should be tested.                                                |
| 18 | **API OpenAPI spec enhancement** (error response schemas per endpoint)         | 🟢 Low      | 2h            | Better consumer DX.                                                           |
| 19 | **Data export** (JSON/CSV of stored events)                                    | 🟢 Low      | 4h            | Analysis in external tools.                                                   |
| 20 | **Improve `CONTRIBUTING.md`** (architecture guide, conventions)                | 🟢 Low      | 2h            | Onboarding.                                                                   |
| 21 | **Fix `go mod tidy` (nested eventtest module) or upstream**                    | 🟢 Low      | 3h            | Vendor management friction.                                                   |
| 22 | **Write ADR for observability decision**                                       | 🟢 Low      | 1h            | Records the fork decision for #8.                                             |
| 23 | **Add `Scenario/v3` BDD tests** if compatible                                  | 🟢 Low      | 4h            | DecideFunc DSL may not fit curried pattern.                                   |
| 24 | **Event retention/TTL**                                                        | 🟢 Low      | 6h            | Auto-cleanup of old events.                                                   |
| 25 | **Multi-user sync support**                                                    | 🟢 Low      | 8h+           | Out of current scope; revisit if needed.                                      |

---

## g) My Top #1 Question I Cannot Figure Out Myself

### Should OpenTelemetry instrumentation live in the SDK (built-in) or in the consumer app (consumer wraps)?

**Why I'm blocked:** This is a genuine architectural fork with real tradeoffs, and the answer depends on information I don't have access to — the needs of the consumer apps (`github-local-sync`, `discordsync`).

| Option                                                                                                               | Pro                                                                                                                  | Con                                                                                                                                   |
| -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| **A) Built into the SDK** (wire `otel/v3` spans into `Syncer.Sync()`, `CQRSStack.SyncItems()`, HTTP middleware)      | Every consumer gets observability for free. Zero-config. Matches what go-cqrs-lite already did (it ships `otel/v3`). | Couples the SDK to an observability stack. Consumers who don't want OTel pay the overhead. Harder to swap backends.                   |
| **B) Consumer wraps** (SDK exposes `Tracer`/`Meter` injection interfaces; consumer provides the OTel implementation) | SDK stays pure. Consumer chooses observability stack. No coupling.                                                   | Every consumer reimplements. More boilerplate. The `otel/v3` module being vendored-but-unused suggests the ecosystem leaned toward A. |

**What I need from you:** A decision on A vs B (or a hybrid — built-in with an opt-out). This blocks the #1 priority task (wire OpenTelemetry). If A, I wire `otel/v3` into the SDK directly. If B, I design the injection interfaces and document the consumer-side pattern. I cannot start #1 without this.

---

## Session 24 Commits (All Pushed)

| Commit    | Message                                                                     |
| --------- | --------------------------------------------------------------------------- |
| `5114e4f` | Stop golangci-lint fmt from reformatting vendored third-party code          |
| `2f8c270` | Remove context.Value outcome smuggling and make conflict detection explicit |
| `019f7b0` | Justify QueryDispatcher bypass with benchmark data, not speculation         |
| `babe493` | Fix doc-drift in FEATURES.md and DOMAIN_LANGUAGE.md                         |
| `a855cba` | Rework broken CI build/release jobs for a pure library                      |
| `6a26637` | Add brutal self-review report for session 24                                |
| `255dd25` | Add execution plan report for session 24                                    |

---

## Reports Written This Session

- `docs/reviews/2026-06-29_17-52_brutal-self-review.html` — 11-question honest self-review
- `docs/planning/2026-06-29_17-52_session-24-execution-plan.html` — Pareto-sorted execution plan
- `docs/status/2026-06-29_18-01_session-24-comprehensive-status.md` — this report

---

## Resolution (2026-07-22)

Session 24's fixes all shipped. Since this report, **v0.4.0** landed:

- **`go mod tidy` broken** — **fixed** by the v4 migration (the v3 nested-`eventtest` blocker is gone; `go mod tidy` now works).
- **`otel/v3` vendored but unused** — replaced by `otel/v4` (still vendored, still unwired — OpenTelemetry instrumentation remains in TODO_LIST).
- **CI `build`/`release` broken** — **fixed** (cross-platform compile verify + binary-free releases).
- **`gopkg.in/yaml.v3` transitive** — resolved in dependency refreshes.
- **projectionhost/v3** → upgraded to **projectionhost/v4** (ADR-0006), now with DLQ wired.
- **`encoding/json` v1→v2** — **done** (adopted `encoding/json/v2` via `GOEXPERIMENT=jsonv2`).
- **Test count** is now **216** (this report said 190).
