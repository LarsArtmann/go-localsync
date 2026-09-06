# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-09-06 (docs-health sweep: executed items removed — their record lives in [CHANGELOG.md](CHANGELOG.md) under v0.6.0; fresh harvest from the [16:28 execution report](docs/status/archive/2026-09-06_16-28_TODO-EXECUTION-SWEEP-SELF-REVIEW.md) §f and still-open review findings)
**Tests:** 431 test functions across 11 packages, plus 35 in the standalone `provider/github` module — all passing (race-clean) | **Latest release:** v0.5.0 + `provider/github/v0.1.0` (v0.6.0 enacted, untagged)

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

> **Scope note:** go-localsync is deliberately a **single-aggregate Item sync SDK**. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). The tasks below improve the SDK _within its current scope_; do not add tasks that widen it without revisiting that decision.

---

## 🔴 HIGH PRIORITY

### Release path (v0.6.0) — owner actions

- [ ] **Owner: cut the v0.6.0 tag** — v0.6 is enacted and fully staged (CHANGELOG migration section, README/FEATURES rows, suite race-green, CI green on master incl. the new provider lint leg, run 34039472270); tag, then run `./scripts/verify-release.sh v0.6.0` (CONTRIBUTING checklist). Everything else in the v0.6 window is done. Post-tag follow-ups: provider re-pin (below) + the v0.7 shim-removal window (ROADMAP Open Question 6).
- [ ] **Owner: `SSH_PRIVATE_KEY` repo secret** so the library cqrs-lint CI leg runs (deploy key with read access to the private `larsartmann/go-finding` module; until then the step skips with a notice and the gate runs locally from the devShell). Alternative endgame: make `go-finding` public and delete all SSH machinery.
- [ ] **Owner: structural vendorHash ↔ daemon decision** — the daemon's dep auto-refresh broke the flake twice on 2026-09-06 alone; pick a mode: stop daemon dep-refreshes, make the refresh re-pin, or accept CI-fail-fast-and-repin. ([08:05 report](docs/status/archive/2026-09-06_08-05_TODO-SWEEP-P2-DOCS-PROVIDER-ETAG-GAUNTLET.md) §g3)
- [ ] **Owner: route the buildflow leftovers** — stale preflight binary, dead gomod-freshness cache mount, and the dense-growing 2.6 GB db belong to the buildflow tool, not this repo; route them to the buildflow project's tracker or drop them. ([16:28 report](docs/status/archive/2026-09-06_16-28_TODO-EXECUTION-SWEEP-SELF-REVIEW.md) §f-5, closing its §d-3 "tracked nowhere" gap)

### Post-tag follow-ups (blocked on the v0.6.0 tag)

- [ ] **`provider/github`: migrate to the v0.6 vocabulary + wire `CacheHits`** — the nested module pins `go-localsync v0.5.0` (proven via standalone `GOWORK=off` build), so `SourceID`/`StreamID` adoption is a post-release follow-up, not a blocker for the tag. Same re-pin window wires `FetchResult.CacheHits` from the ETag stats delta (core field shipped in v0.6.0; provider cannot compile against it until the pin moves).

## 🟡 MEDIUM PRIORITY

### Correctness & test depth (harvested 2026-09-06 from the 16:28 report §f + open review findings; verified open against code)

- [ ] **Red/green proof for the goroutine-leak poll fix** — the race-flake fix (poll-to-baseline) is hardening + clean-stress-verified but never reproduced against the old logic; reintroduce the single-sample logic on a scratch branch, demonstrate the flake under synthetic sibling load, show the fix holds. (16:28 §f-4; the repo's own convention from the upcaster race fix.)
- [ ] **Extend `check-doc-counts.sh` claim coverage** — TODO_LIST header counts and the `docs/localsync-lint.md` rule-table titles are unchecked today; the README per-package test/coverage table is also outside the gate (it drifted 157/36/35/33-64.8% vs the real 168/42/42/47-95.5% while the gate stayed green; fixed manually in this sweep). (16:28 §f-7 + this sweep's finding)
- [ ] **`--fix` hardening** — remind to run `dprint fmt` when rewriting markdown table cells (width-preserving padding covers same-width rewrites only), and unit-test the en-dash claim variant (`C0001–C0015`) that README uses. (16:28 §f-9)
- [ ] **SARIF golden-file snapshot test** — pin the full SARIF document against one fixture; extend assertions to `informationUri` and rule `shortDescription` text. (16:28 §f-11 + §f-19)
- [ ] **`localsync-lint --list --format=json`** — machine-readable rule list so doc tables can be generated/checked instead of hand-copied; add the title-drift check to CI. (16:28 §f-13 + §f-25)
- [ ] **Auth × rate-limit middleware ordering test** — does an unauthenticated request spend a per-client token? The order is currently only implicitly correct. (16:28 §f-14)
- [ ] **Consolidate the wait helpers** — `waitForCount` (pkg/cqrs), `waitForLiveCount` (pkg/api), `waitForExportedCount` export-poll; one predicate-based `WaitFor` in `pkg/testutil`, with a cheaper signal than re-exporting the whole journal per poll. (16:28 §f-15 + §f-16)
- [ ] **Consolidate the two conflict-classification switches** — `classifyAction` (`pkg/cqrs/stack_adapters.go:20`) and `ConflictAwareSyncer.classify` (`pkg/sync/conflict_aware.go:95`) categorize the same outcomes independently; one shared mapping. ([session-26 plan](docs/planning/2026-06-29_19-29_session-26-execution-plan.html) + [19:29 brutal review](docs/reviews/2026-06-29_19-29_brutal-self-review.html))
- [ ] **Surface reconciliation failures** — a persistent-store failure during `Reconcile` is log-only today (`pkg/sync/sync.go:305` warns and returns); decide whether it belongs in `SyncResult`/the partial-sync error. ([22:04 brutal review](docs/reviews/2026-06-29_22-04_brutal-self-review.html))
- [ ] **Multi-key API auth** — `WithAPIKey` accepting a set or verifier so the per-client `APIKeyClient` recipe works for real fleets; document the extractor pairing. (16:28 §f-21)
- [ ] **Idle-baseline benchmark run** — both protocol runs to date executed under load-average ~20 (noise-dominated); rerun idle (or `taskset`/`nice`-pinned) and add the idle comparison pair for the upcast benchmark. (16:28 §f-6 + §f-34)
- [ ] **Scenario-DSL specs against the SQLite read model** — projection specs are memory-only today. (16:28 §f-18)

## 🟢 LOWER PRIORITY

- [ ] **API niceties (post-v0.6, small)**: request-ID middleware + echo header; SQLite opt-in WAL/pragma knob on `CQRSConfig`; `/stats` source/type filter params (read model already supports filtering). ([08:05 report](docs/status/archive/2026-09-06_08-05_TODO-SWEEP-P2-DOCS-PROVIDER-ETAG-GAUNTLET.md) §f32-34)
- [ ] **`MemoryReadModel` defensive copies** — `Get` returns the stored `*model.Item` pointer and `Upsert` stores the caller's (`pkg/cqrs/memory_readmodel.go:40,96`); document the shared-pointer contract or copy defensively. ([22:04 brutal review](docs/reviews/2026-06-29_22-04_brutal-self-review.html))
- [ ] **`configureSQLitePool` dead param** — ignores its `dbPath` argument (`_ = dbPath`, `pkg/cqrs/store_factory.go:146`) and carries a duplicated doc comment; fix or remove. ([16:30 brutal review](docs/reviews/2026-06-29_16-30_brutal-self-review.html))
- [ ] **Redundant `tombstoned` column** — the SQLite DDL keeps `tombstoned INTEGER` alongside `tombstone_reason` (`pkg/cqrs/sqlite_readmodel.go:25-26`); derive one from the other at the next schema touch. ([22:28 brutal review](docs/reviews/2026-06-29_22-28_brutal-self-review.html))
- [ ] **Table-drive the `run()` exit-code matrix** in `cmd/localsync-lint` tests (0/1/2 × flag variants) to shrink boilerplate. (16:28 §f-24)
- [ ] **CI niceties** — deep-link failing steps from the job-summary badge table (16:28 §f-20); upload SARIF as a CI artifact or code-scanning result when the linter runs in CI (16:28 §f-12).
- [ ] **`check-doc-counts.sh` self-test harness** — the helper functions are live-tested only; a tiny sh harness would pin `fix_number` pick semantics. (16:28 §f-35)
- [ ] **SARIF schema example in `docs/localsync-lint.md`** — one rendered sample document. (16:28 §f-36)
- [ ] **Race-flake deeper hunt — only if it flakes again** — instrument + synthetic parallel-stack load to convert the goroutine-leak hypothesis into a captured failure. (16:28 §f-23, explicitly conditional)
