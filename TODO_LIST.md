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
- [ ] **Consolidate `SyncResult`/`SyncSummary` into exactly one user-facing result type (v0.6)** — scope extended 2026-09-06 (ADR-0009 addendum): `Syncer.GetStats` → `Stats` joins the same window
      **Source:** ADR-0009 decision #2; `pkg/sync/types.go` (`Stats` also renames there — vague noun per the [naming review](docs/reviews/2026-07-19_01-25_naming-review.html))
- [x] **Decide the `ExternalID` ↔ `SourceID` field duality (v0.6 candidate)**
      ✅ DECIDED 2026-09-06 ([ADR-0009 addendum](docs/adr/0009-v06-vocabulary-alignment.md)): v0.6 aligns the GO SURFACE to `SourceID` (`id.ExternalID` → `id.SourceID` + field renames, deprecated aliases); the persisted wire payloads already say `sourceId` and stay untouched (no schema V4, no upcast). **Enactment gate: requires the owner's sign-off recorded here before the v0.6 branch enacts it.**
      **Origin:** raised by the session-26/27 self-reviews.

### Correctness hardening (verified open against code 2026-09-05)

- [x] **Disposition "resurrect bypasses conflict resolution"** — the decider emits `ItemSynced` events without invoking the resolver when resurrecting a tombstoned item (`pkg/cqrs/decider.go:109`); either document as by-design with a test pinning it, or route resurrections through `resolveConflict`. Flagged by [is-it-what-it-claims-to-be](docs/brainstorming/is-it-what-it-claims-to-be.html).
      ✅ DONE 2026-09-06: pinned as by-design ([ADR-0005 addendum](docs/adr/0005-tombstone-over-delete.md)) — a tombstoned local is a deleted marker and a sync event is the only path back to live; pinned by `TestDecideSync_ResurrectTombstonedItem_BypassesResolver` + branch comment in `decider.go`.
- [x] **Audit memory-store legacy-event mutation** — the upcaster registry stamps schema version in place; SQLite decodes fresh instances (safe) but the memory backend stores shared pointers (`docs/status/2026-09-05_22-30_SUPERB-REFERENCE-CONSUMER-EXECUTION.md` §b). Clone unconditionally or document the caveat.
      ✅ DONE 2026-09-06: CLONE chosen and implemented — legacy-versioned events (stamp 1/2) now always rebuild a private copy in `upcastItemSyncedToV3` (the anomalous Attributes-present shape previously passed the stored pointer back for in-place stamping); true V3 events keep the zero-cost pass-through.
- [x] **Pin the upcaster chain semantics** — comment + test explaining WHY the V1→V2→V3 double application is safe, so a library change can't silently break it.
      ✅ DONE 2026-09-06: WHY comment on `upcastItemSyncedToV3` + `TestUpcaster_ChainSemantics_V1ToFoldedV3` (fold-once, identity preserved, idempotent re-transform).
- [x] **Deterministic race-regression for the upcaster path** — sync + replay concurrently under `-race` (the fixed data race from 2026-09-05 has no standing regression test).
      ✅ DONE 2026-09-06: `TestUpcaster_ConcurrentReadsDuringSync` — 100 anomalous legacy streams, 4 barrier-start readers with shifted visit orders + live V3 writer; verified it FAILS against the old logic (3 DATA RACEs) and is 5× `-race`-clean with the fix.

## 🟡 MEDIUM PRIORITY

### Decisions needed (owner input welcome)

- [ ] **`/metrics` auth posture** — currently behind the API key (not in `isPublicPath`); decide keyed-vs-public and document the outcome (`pkg/api/auth.go`).
- [ ] **DLQ inspection/replay surface** — list + purge + `ReplayDeadLetters` as SDK functions (endpoint optional; `projectionhost` provides the primitives).

### Tooling / CI

- [ ] **Add the `SSH_PRIVATE_KEY` repo secret so the library cqrs-lint CI leg runs** — the error-gated `go-cqrs-lite/cmd/cqrs-lint/v4@v4.8.1` step is restored in the workflow, CI-verified on the skip path, and auto-enables once the secret exists (a deploy key with read access to the private `larsartmann/go-finding` module); until then it skips with a notice and the gate runs locally from the devShell (documented in the workflow + AGENTS.md). Alternative endgame (owner call): make `go-finding` public and delete all SSH machinery.
- [ ] **Run `buildflow --build-mode full`** inside the devShell (go.work kept to the two in-repo modules) — last full-pipeline run predates the M-plan session.
- [x] **Add a `nix flake check` CI job** so `vendorHash` drift can't land silently again (it silently broke `nix build` once already — see CHANGELOG 0.5.0-era "Stale vendorHash re-pinned").
  ✅ DONE 2026-09-06: `nix` job added, gates build/release; overrides the SSH `go-nix-helpers` input to anonymous HTTPS.
- [x] **Pin the golangci-lint version in CI** instead of `latest` (reproducibility).
  ✅ DONE 2026-09-06: pinned to `v2.13.2` — the exact devShell version.
- [x] **Add `actionlint` to the devShell + a CI workflow-validation step** (replaces ad-hoc `yaml.safe_load` checks).
  ✅ DONE 2026-09-06: `pkgs.actionlint` in devShell; CI step runs pinned `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`; local run clean.
- [x] **`vendorHash` drift guard** — warn (hook or CI) when `go.mod`/`go.sum` change without a matching `flake.nix` re-pin; the drift silently broke `nix build` once (see CHANGELOG + the AGENTS gotcha).
  ✅ DONE 2026-09-06: `scripts/check-vendorhash.sh` — CI nix job fails fast with re-pin instructions when go.mod/go.sum move without flake.nix; proven red/green locally.
- [ ] **CI formatting story** — either add a dprint check job (json/yaml/md/dockerfile) or drop the parity claim; today dprint is devShell-only by decision default, not by recorded decision.
- [ ] **Purge stale `.golangci.yml` exclusion paths** — `pkg/providers/github/client.go`, `pkg/types/ids.go`, `pkg/testhelpers/` predate the restructures; verify and delete dead rules.
- [ ] **Consider a windows build leg** — the compile matrix is linux/darwin only; sqlite/CGO behavior on windows is unproven.
- [ ] **Audit the library-gate suppression** — confirm the single `//cqrs-lint:ignore` reason is still current.
- [ ] **Revisit inert pre-commit hooks** — formally enable (scoped) or delete; they are neither protecting nor costing anything today.
- [x] **Compute doc-drift-prone counts in CI instead of hand-copying** — test/coverage counts (AGENTS.md / README.md / FEATURES.md / TODO_LIST.md) and the AGENTS.md dependency table vs `go.mod` have both drifted repeatedly; generate or check them in CI (every 2026 drift involved hand-copied numbers).
  ✅ DONE 2026-09-06: `scripts/check-doc-counts.sh` (per-package + totals + dep-table vs go.mod; `--coverage` local opt-in), wired into the CI lint job; first run caught the +4-test drift (309→313, cqrs 144→148) and it was fixed.
- [ ] **Separate CHANGELOG for `provider/github`** — the nested module's lifecycle is now independent of core releases.
- [ ] **Restructure AGENTS.md under ~30 KB** — link out to ADRs instead of inlining decisions; keep gotchas ≤20 (flagged as "bloated" by two consecutive reviews; the 2026-09-05 passes only pruned, never restructured).
- [ ] **Make docs-health VERIFY a standing pre-release step** — docs drift after every release is systemic (Accuracy scored 1.5/10 once); wire the check into the release routine rather than running on-demand audits.
- [x] **Pre-release verification target** — a nix target (or script) running the full suite (build, race tests, lint, both cqrs-lint gates, `nix flake check`) plus a CONTRIBUTING.md release-checklist section pointing at it; codify the manual release-integrity checks (tags pushed, GitHub Release bodies, proxy `@v/list` + `@latest`, pkg.go.dev indexing) into the same script — they were hand-run twice on 2026-09-05.
  ✅ DONE 2026-09-06: `scripts/verify-release.sh <core-tag> [provider-tag]` (tags/Release/proxy/pkg.go.dev) + CONTRIBUTING release checklist + `nix flake check` now runs the hermetic full suite (`checks.test` + `checks.lint`); dry-run green against v0.5.0/v0.1.0.

### Quality

- [x] **Process-level `cmd/cqrs-lint` tests** — build the binary, run against fixtures, assert exit codes 0/1/2 (finishes M11 properly; `main()`, flag parsing, `printRules`/`printUsage` untested).
  ✅ DONE 2026-09-06: `cmd/cqrs-lint/process_test.go` — builds the binary into `t.TempDir()`, pins exits 0/1/2, `--strict` on the unknown-rule warning, and the NDJSON shape. Coverage stays 56.4% (subprocess runs are coverage-invisible by design).
- [ ] **Real-meter and sdktrace recorder tests** — prove values actually land in `cqrs.operation.*` and the `localsync.sync_items` span attributes (noop providers only prove wiring).
- [ ] **Cursor pagination test against the real SQLite read model ordering** — the current test uses a fake store (`pkg/api`).
- [x] **`pkg/id` unit tests for `ContentHash`** — `IsZero`/`String` untested; coverage sits at 75.0%.
  ✅ DONE 2026-09-06: constructor/round-trip + literal-compat + sha256-path tests; package coverage 75.0% → 100.0%.
- [ ] **Benchmark protocol** — re-run pipeline benchmarks with `-benchtime 20x -count 5` + benchstat; fix `Replay10kEvents` to measure true from-zero replay (iterations 2+ are checkpoint-bounded no-ops); add a conflict-heavy benchmark (resolver invoked per item) and an upcasted-legacy-read vs native-V3-read benchmark.
- [ ] **Wire `CQRSConfig.Validate()` into `NewCQRSStack`** (or document it as consumer-facing) — defined at `pkg/cqrs/stack.go:53` with zero production call sites.
- [ ] **Consolidate attribute-key constants** — `pkg/cqrs/item_adapter.go:18-21` keeps private `legacyActorLogin`-style constants duplicating `pkg/data/model`'s exported `Attr*` keys; two sources of truth for a wire-format constant.

## 🟢 LOWER PRIORITY

- [ ] ~~**Add `govalid` struct tags**~~ — pivoted 2026-09-05: govalid is a buildflow-internal generator, not a proxy-resolvable module; real `Validate()` methods were implemented instead (`SyncOptions.Validate`, `CQRSConfig.Validate`, `ItemFilter.Validate`). Reopen only if govalid is ever published with a stable tag format.
- [ ] **cqrs-lint CLI surface cluster** (aggregate; from [2026-08-02 report](docs/status/archive/2026-08-02_20-31_CQRS-LINT_CLI_ENHANCEMENT.md) §f): `--version`/`--quiet`/`--format=github`, `--rules`/`--exclude-rules`, `--no-suppress`, `--explain`, block + range (`ignore-start`/`ignore-end`) directives, SARIF output, dedicated directives doc page, hand-rolled `--json` → `encoding/json`, per-rule suppressed counts in `--verbose`, new rules C0011+.
- [ ] **API hardening polish**: `X-RateLimit-Limit`/`-Remaining` headers on 429; optional per-client rate limiting (`WithRateLimiter(keyExtractor)`); document the global-vs-per-client scope; structured log level control (per-event INFO is noisy in prod).
- [ ] **OTel span for `Syncer.Sync`** in `pkg/sync` (currently only the CQRS batch path spans).
- [ ] **`provider/github`: ETag / conditional requests** for incremental revalidation (flagged by [performance review](docs/research/performance-review.html)).
- [ ] **`SyncOptions.Validate()` rejects `MaxPages < 0`** (currently only checks `Source`; `pkg/sync/sync.go:125`).
- [ ] **Typed `Attributes` write-helpers** (`WithActorLogin(...)`) mirroring the typed readers.
- [ ] **Surface `ParseTombstoneReason` in API DTOs** (typed tombstone reason on the read path).
- [ ] **`b.N` → `b.Loop()`** modernization in the older bench files (gopls warnings: `adapter_bench_test.go`, `stack_bench_test.go`).
- [ ] **Unify `waitForCount`/`waitForCountTB`** behind a `testing.TB` helper.
- [x] **Move `id.ContentHash` out of `ids.go`** — it is a content hash, not an identifier.
  ✅ DONE 2026-09-06: moved to `pkg/id/content_hash.go`.
- [ ] **Docs policy cluster**: decide + execute the HTML-artifact policy (banner/archive the 25 generated HTML reports; 3 superseded June dashboards sit in `status/` root); record the dprint scope for `docs/status/` (format vs exclude); classify the two undated planning files; annotate+archive the 23:04 report once routed.
- [ ] **ROADMAP cleanup: "Export to JSON/CSV" theme is stale** — `stack.ExportEvents` (NDJSON) + `ExportEventsCSV` shipped (FEATURES row 65); strike/replace the ROADMAP idea row.
- [ ] **Add source-item IDs to cluster TODOs** (cqrs-lint CLI cluster, API-hardening, benchmarks) so future sweeps can strike report items individually.
- [ ] **`errors.AsType` audit pass** (go-error-modernization sweep, not yet run).
- [ ] **Disposition `hierarchical-errors` buildflow findings** — ~3,711 findings; suppress in `.buildflow.yml` with a stated rationale or formally track (open since 2026-07-19, carried by two reports).
- [ ] **File the watermill causation-metadata limitation upstream** in go-cqrs-lite (typed `Metadata.Causation` pointer not mapped onto bus-delivered messages; only custom `command.type`/`command.id` fallbacks survive) — candidate issue after `verify-before-filing` (see CHANGELOG Unreleased, correlation entry).
- [ ] **Quality: coverage floor raises** — `cmd/cqrs-lint` 56.4% (lowest in repo; process-level tests above cover much of it) and `pkg/data/model` 84.9% (lowest package).
- [ ] **Verify kit-side claims in `go-github-kit` source** before trusting the provider README's "empty token = unauthenticated (60 req/h)" and "retry on 429 and idempotent 5xx" lines (`verify-external-claims`); annotate if wrong.
- [ ] **Document gopls `stdversion` warnings as known GOEXPERIMENT noise** (`json.Marshal*` wants go1.27, `go.mod` says 1.26) so sessions stop re-debugging LSP-only noise.
- [ ] **`TombstoneItem` variadic `...event.Option`** for parity with direct dispatch.
- [ ] **Verify OpenAPI `/sync` 408** — confirm huma maps RequestTimeout consistently with `pkgerrors.HTTPStatus` (499/504 for ctx cancel/deadline may be more accurate).
- [ ] **`AggregateID` → `StreamID` vocabulary sweep in ADRs/docs** — ADR prose still uses the old vocabulary (v0.4.1 leftover; mechanical, do with the v0.6 rename).
- [ ] **Re-run the full 100-point go-cqrs-lite deep-dive audit** to get the true post-M-plan adoption score (the ≥90 target was never re-scored; see [docs/research/2026-09-05_go-cqrs-lite-deep-dive.html](docs/research/2026-09-05_go-cqrs-lite-deep-dive.html)).
