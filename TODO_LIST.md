# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-09-05
**Tests:** 232 functions across 10 packages, all passing | **Latest release:** v0.5.0 + `provider/github/v0.1.0`

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

> **Scope note:** go-localsync is deliberately a **single-aggregate Item sync SDK**. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). The tasks below improve the SDK _within its current scope_; do not add tasks that widen it without revisiting that decision.

---

## 🔴 HIGH PRIORITY

### Release integrity (post-public-flip)

- [ ] **Verify tag push + proxy propagation for `v0.5.0` and `provider/github/v0.1.0`**
      **Source:** `git tag`, GitHub Releases, proxy.golang.org
      **Description:** Both tags were created while the repo was private; the repo flipped public the same day. Verify the tags are pushed, GitHub Releases exist, and `proxy.golang.org` serves the versions; if the proxy never fetched them, cut a bump release to re-establish `@latest` (owner decision whether to re-tag).
      **Context:** Raised by the 2026-09-05 docs-health session (status report §g Q1).

### cqrs-lint hardening (the ADR-0004 gate is the least-tested code in the repo)

- [ ] **CLI test coverage for `cmd/cqrs-lint`**
      **Source:** `cmd/cqrs-lint/main.go` (zero test files — verified via `go test ./...`)
      **Description:** ~250 lines of untested code: flag parsing, exit-code contract (0 clean / 1 findings / 2 usage), `emitSummary`/`countFindings`/`emitRuleStatus`. An integration test that builds the binary and runs it against small fixture packages closes the gap.
      **Context:** Open since the cqrs-lint CLI sprint (2026-07-17); re-flagged by the 2026-08-02 enhancement session.

- [ ] **Run cqrs-lint as a CI gate**
      **Source:** `.github/workflows/ci.yml` (no cqrs-lint step — only `nix flake check` runs it via `checks.cqrs-lint`)
      **Description:** Add a workflow step `go run ./cmd/cqrs-lint --strict -pkg pkg/cqrs` so the architectural invariant is enforced on every push, not only in nix environments.

- [ ] **Suppression audit trail + unknown-rule warning**
      **Source:** `internal/cqrslint/finding.go:36` (only `Suppressed bool` — no provenance), `internal/cqrslint/suppress.go`
      **Description:** Add `SuppressedBy`/`SuppressedReason` fields (which directive silenced a finding, and its optional reason — currently parsed and discarded), and warn when a directive names a nonexistent rule (`//cqrs-lint:ignore C9999` silently succeeds today).

### Deep-dive audit follow-ups (2026-09-05)

> From the [go-cqrs-lite utilization audit](docs/research/2026-09-05_go-cqrs-lite-deep-dive.html) (adoption 78/100). Full decomposition: [execution plan](docs/planning/2026-09-05_20-42_SUPERB-REFERENCE-CONSUMER.md).

- [ ] **Fix DLQ volatility (C017 — the audit's only ERROR)**
      **Source:** `pkg/cqrs/runner.go:42` — `projectionhost.NewMemoryDeadLetterStore()` paired with the persistent SQLite backend.
      **Description:** Captured poison events are lost on restart, so catch-up re-processes and re-captures them after every crash. Wire `projectionhost.NewSQLiteDeadLetterStore(ctx, db)` in the sqlite branch of `store_factory.go`; keep the memory DLQ for the memory backend. Add a reopen-persistence test.

- [ ] **Correlation + causation metadata on ALL write paths**
      **Source:** `pkg/cqrs/stack.go:193` (`SyncItem` passes `Options: nil`), `TombstoneItemCommand` has no options field; no event carries a CausationID.
      **Description:** Give `TombstoneItemCommand` an `Options` field, default a fresh `CorrelationID` in the single-item paths, and register `decider.WithEnricher(event.CommandCausalityEnricher)` at repository construction so every event names its causing command.

- [ ] **CI leg: library cqrs-lint (error-gated)**
      **Source:** `cmd/cqrs-lint` in go-cqrs-lite (203 consumer-facing rules; found C017 in 101 ms)
      **Description:** Pin `go-cqrs-lite/cmd/cqrs-lint/v4` and run it against `./pkg/...` with `--min-severity error` on every push. Known false positives (E005 handler detection, E014 drain heuristic) are documented in the audit appendix — annotate, don't suppress blindly.

---

## 🟡 MEDIUM PRIORITY

### Observability

- [ ] **OpenTelemetry instrumentation**
      **Source:** `pkg/sync/sync.go`, `pkg/cqrs/stack.go`, `pkg/api/server.go`
      **Description:** Spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, and HTTP middleware. `go-cqrs-lite/otel/v4` (v4.3.0) is already in the module graph as an indirect dep.
      **Context:** No observability today; production debugging requires log spelunking. Open since 2026-05; confirmed unimplemented.

- [ ] **Structured logging fields**
      **Source:** `pkg/sync/sync.go`
      **Description:** Consistent context fields (source, page, event_id) on all log statements.

### API Hardening

- [ ] **API authentication middleware** (API key or JWT) — the HTTP API is currently unauthenticated.
- [ ] **API pagination headers** (`X-Total-Count`, cursor-based).
- [ ] **API rate limiting middleware** (prevent `POST /sync` abuse).
- [ ] **API OpenAPI spec enhancement** (error response schemas per endpoint).

### Code Quality

- [ ] **Improve `pkg/cqrs` coverage (82.4%)**
      **Source:** `pkg/cqrs/`
      **Description:** Add tests for remaining error paths and store-factory branches.

- [ ] **SQLite file-backed integration tests**
      **Source:** `pkg/cqrs/sqlite_readmodel_test.go` (uses `:memory:`, which hides WAL/concurrency behavior)
      **Description:** Round-trip tests against a real file DB (`t.TempDir()`), incl. reconcile + restart-replay.

- [ ] **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution (the `schema.Version` V1→V3 foundation in `pkg/data/schema/` is ready for it; no upcaster exists yet).
- [ ] **Swap hand-rolled validation middleware → `middleware.CommandValidation`** (audit #5) — replaces the 25-line `commandValidationMiddleware` (pkg/cqrs/middleware.go:10-35) and gains opt-in failure logging via `WithLogger`.
- [ ] **Adopt `scenario` DSL + `eventtest` fakes for new decider/projection tests** (audit #6) — library-native BDD (`scenario.Given(...).When(...).Then(...)`); replaces the parked Ginkgo idea without a new dependency.

- [ ] **Rename public `AggregateID()` → `StreamID()`**
      **Source:** `pkg/cqrs/aggregate_id.go:30` — returns `cqrsid.StreamID` since v0.4.1 but kept the old name for API stability
      **Description:** Breaking rename for the next minor/major; aligns the exported vocabulary with upstream.

### Provider module (`provider/github`)

- [ ] **Real GitHub PAT smoke test** — the ported suite is mock-based; one live-API round-trip would prove the kit wiring (token, rate-limit gating, error mapping).

---

## 🟢 LOWER PRIORITY

- [ ] **Add `govalid` struct tags** to `SyncOptions`, `CQRSConfig`.
- [ ] **Improve `CONTRIBUTING.md`** — add architecture guide, file-split conventions, testing requirements (current file is a 25-line stub).
- [ ] **Conflict resolution per-sync override** — `SyncOptions.ConflictResolver` for per-sync strategy (currently only `CQRSConfig.ConflictResolver`).
- [ ] **`ItemFilter.Validate()`** — reject negative `Limit`/`Offset` instead of silently accepting them (data-model-review finding).
- [ ] **Branded `ContentHash` type** — `model.Item.ContentHash` is a bare `string` today (data-model-review finding).
- [ ] **Typed `Attributes` accessors** in `pkg/data/model` (e.g. `ActorLogin()`, `RepoName()` helpers over the map — data-model-review finding).
- [ ] **`SyncResult` vs `SyncSummary` vocabulary alignment** (naming-review finding: two near-synonymous types in `pkg/sync`).
- [ ] **Migrate `exhaustruct` → `exhaustruct_v5` in `.golangci.yml`** — golangci-lint v2.13 flags the old linter as deprecated.
- [ ] **Verify `provider/github/README.md` prose against the `FetchPages` rebuild** — modified during the 2026-09-05 concurrent dependency session; claims not re-checked since `FetchAll` moved onto the kit kernel.
- [ ] **Recompute coverage % + run dprint check** after the 2026-09-05 dependency churn (charm.land/log v2.0.1 landed after the last `go test -cover` run); dprint is not yet in the devShell.
- [ ] **Benchmarks for the full sync pipeline** (current benchmarks cover individual operations only).
- [ ] **Shutdown hygiene cluster** (audit #7/#12, cqrs-lint C023/C009/C007): log the 8 swallowed `Close()` errors (stack.go, store_factory.go), convert the `AggregateID` panic fallback to an error return, inject a clock into the tombstone decider path (replace `time.Now()`).
- [ ] **Document `DeriveStreamID` divergence** (audit #10): `AggregateID` (aggregate_id.go:17-52) and the library's `id.DeriveStreamID` solve the same problem with different encodings — keep ours (data compatibility), pin the divergence in a comment, fold the rename decision into the `AggregateID`→`StreamID` item above.
