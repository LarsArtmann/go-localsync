# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-09-05 (evening docs-health sweep)
**Tests:** 309 test functions across 11 packages, plus 31 in the standalone `provider/github` module — all passing (race-clean) | **Latest release:** v0.5.0 + `provider/github/v0.1.0`

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

> **Scope note:** go-localsync is deliberately a **single-aggregate Item sync SDK**. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). The tasks below improve the SDK _within its current scope_; do not add tasks that widen it without revisiting that decision.

---

## 🔴 HIGH PRIORITY

### v0.6 vocabulary window (decided, awaiting the breaking release — [ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md))

- [ ] **Rename public `AggregateID()` → `StreamID()` (v0.6)**
      **Source:** `pkg/cqrs/aggregate_id.go` — returns `cqrsid.StreamID` since v0.4.1 but kept the old name for API stability
      **Description:** Breaking rename for v0.6, decided and recorded in [ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md) — with the deliberate `DeriveStreamID` encoding divergence (keep ours; documented at the definition site) and the `AggregateID` panic-fallback → error-return conversion in the same window.
- [ ] **Consolidate `SyncResult`/`SyncSummary` into exactly one user-facing result type (v0.6)**
      **Source:** ADR-0009 decision #2; `pkg/sync/types.go` (`Stats` also renames there — vague noun per the [naming review](docs/reviews/2026-07-19_01-25_naming-review.html))
- [ ] **Decide the `ExternalID` ↔ `SourceID` field duality (v0.6 candidate)**
      **Source:** `provider.Item.ExternalID` vs event-payload `SourceID string` (`pkg/cqrs/events.go:32,52,61` vs `pkg/provider/provider.go`). Touches versioned event payloads, so [ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md) does not cover it yet — disposition (align + upcast, or document) needs an ADR addendum. Raised by the session-26/27 self-reviews, never dispositioned.

### Correctness hardening (verified open against code 2026-09-05)

- [ ] **Disposition "resurrect bypasses conflict resolution"** — the decider emits `ItemSynced` events without invoking the resolver when resurrecting a tombstoned item (`pkg/cqrs/decider.go:109`); either document as by-design with a test pinning it, or route resurrections through `resolveConflict`. Flagged by [is-it-what-it-claims-to-be](docs/brainstorming/is-it-what-it-claims-to-be.html).
- [ ] **Audit memory-store legacy-event mutation** — the upcaster registry stamps schema version in place; SQLite decodes fresh instances (safe) but the memory backend stores shared pointers (`docs/status/2026-09-05_22-30_SUPERB-REFERENCE-CONSUMER-EXECUTION.md` §b). Clone unconditionally or document the caveat.
- [ ] **Pin the upcaster chain semantics** — comment + test explaining WHY the V1→V2→V3 double application is safe, so a library change can't silently break it.
- [ ] **Deterministic race-regression for the upcaster path** — sync + replay concurrently under `-race` (the fixed data race from 2026-09-05 has no standing regression test).

## 🟡 MEDIUM PRIORITY

### Decisions needed (owner input welcome)

- [ ] **`/metrics` auth posture** — currently behind the API key (not in `isPublicPath`); decide keyed-vs-public and document the outcome (`pkg/api/auth.go`).
- [ ] **DLQ inspection/replay surface** — list + purge + `ReplayDeadLetters` as SDK functions (endpoint optional; `projectionhost` provides the primitives).

### Tooling / CI

- [ ] **Add the `SSH_PRIVATE_KEY` repo secret so the library cqrs-lint CI leg runs** — the error-gated `go-cqrs-lite/cmd/cqrs-lint/v4@v4.8.1` step is restored in the workflow and auto-enables once the secret exists (a deploy key with read access to the private `larsartmann/go-finding` module); until then it skips with a notice and the gate runs locally from the devShell (documented in the workflow + AGENTS.md).
- [ ] **Run `buildflow --build-mode full`** inside the devShell (go.work kept to the two in-repo modules) — last full-pipeline run predates the M-plan session.
- [ ] **Add a `nix flake check` CI job** so `vendorHash` drift can't land silently again (it silently broke `nix build` once already — see CHANGELOG 0.5.0-era "Stale vendorHash re-pinned").
- [ ] **Pin the golangci-lint version in CI** instead of `latest` (reproducibility).
- [ ] **Add a golangci-lint leg for `provider/github`** — the standalone CI job builds + race-tests but does not lint.
- [ ] **Compute test/coverage counts in CI instead of hand-copying** — the counts drifted across AGENTS.md / README.md / FEATURES.md / TODO_LIST.md multiple times; generate or check them (every drift in 2026 involved hand-copied numbers).
- [ ] **Separate CHANGELOG for `provider/github`** — the nested module's lifecycle is now independent of core releases.

### Quality

- [ ] **Process-level `cmd/cqrs-lint` tests** — build the binary, run against fixtures, assert exit codes 0/1/2 (finishes M11 properly; `main()`, flag parsing, `printRules`/`printUsage` untested).
- [ ] **Real-meter and sdktrace recorder tests** — prove values actually land in `cqrs.operation.*` and the `localsync.sync_items` span attributes (noop providers only prove wiring).
- [ ] **Cursor pagination test against the real SQLite read model ordering** — the current test uses a fake store (`pkg/api`).
- [ ] **`pkg/id` unit tests for `ContentHash`** — `IsZero`/`String` untested; coverage sits at 75.0%.
- [ ] **Benchmark protocol** — re-run pipeline benchmarks with `-benchtime 20x -count 5` + benchstat; fix `Replay10kEvents` to measure true from-zero replay (iterations 2+ are checkpoint-bounded no-ops).
- [ ] **Wire `CQRSConfig.Validate()` into `NewCQRSStack`** (or document it as consumer-facing) — defined at `pkg/cqrs/stack.go:53` with zero production call sites.
- [ ] **Consolidate attribute-key constants** — `pkg/cqrs/item_adapter.go:18-21` keeps private `legacyActorLogin`-style constants duplicating `pkg/data/model`'s exported `Attr*` keys; two sources of truth for a wire-format constant.

## 🟢 LOWER PRIORITY

- [ ] ~~**Add `govalid` struct tags**~~ — pivoted 2026-09-05: govalid is a buildflow-internal generator, not a proxy-resolvable module; real `Validate()` methods were implemented instead (`SyncOptions.Validate`, `CQRSConfig.Validate`, `ItemFilter.Validate`). Reopen only if govalid is ever published with a stable tag format.
- [ ] **cqrs-lint CLI surface cluster** (aggregate; from [2026-08-02 report](docs/status/2026-08-02_20-31_CQRS-LINT_CLI_ENHANCEMENT.md) §f): `--version`/`--quiet`/`--format=github`, `--rules`/`--exclude-rules`, `--no-suppress`, `--explain`, block + range (`ignore-start`/`ignore-end`) directives, SARIF output, dedicated directives doc page, hand-rolled `--json` → `encoding/json`, per-rule suppressed counts in `--verbose`, new rules C0011+.
- [ ] **API hardening polish**: `X-RateLimit-Limit`/`-Remaining` headers on 429; optional per-client rate limiting (`WithRateLimiter(keyExtractor)`); document the global-vs-per-client scope; structured log level control (per-event INFO is noisy in prod).
- [ ] **OTel span for `Syncer.Sync`** in `pkg/sync` (currently only the CQRS batch path spans).
- [ ] **`provider/github`: ETag / conditional requests** for incremental revalidation (flagged by [performance review](docs/research/performance-review.html)).
- [ ] **`SyncOptions.Validate()` rejects `MaxPages < 0`** (currently only checks `Source`; `pkg/sync/sync.go:125`).
- [ ] **Typed `Attributes` write-helpers** (`WithActorLogin(...)`) mirroring the typed readers.
- [ ] **Surface `ParseTombstoneReason` in API DTOs** (typed tombstone reason on the read path).
- [ ] **`b.N` → `b.Loop()`** modernization in the older bench files (gopls warnings: `adapter_bench_test.go`, `stack_bench_test.go`).
- [ ] **Unify `waitForCount`/`waitForCountTB`** behind a `testing.TB` helper.
- [ ] **Move `id.ContentHash` out of `ids.go`** — it is a content hash, not an identifier.
- [ ] **`errors.AsType` audit pass** (go-error-modernization sweep, not yet run).
- [ ] **`TombstoneItem` variadic `...event.Option`** for parity with direct dispatch.
- [ ] **Verify OpenAPI `/sync` 408** — confirm huma maps RequestTimeout consistently with `pkgerrors.HTTPStatus` (499/504 for ctx cancel/deadline may be more accurate).
- [ ] **`AggregateID` → `StreamID` vocabulary sweep in ADRs/docs** — ADR prose still uses the old vocabulary (v0.4.1 leftover; mechanical, do with the v0.6 rename).
- [ ] **Re-run the full 100-point go-cqrs-lite deep-dive audit** to get the true post-M-plan adoption score (the ≥90 target was never re-scored; see [docs/research/2026-09-05_go-cqrs-lite-deep-dive.html](docs/research/2026-09-05_go-cqrs-lite-deep-dive.html)).
