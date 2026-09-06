# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-09-06 (docs-health sweep: vocabulary/CI rows synced, harvest from 06:19 + 08:05 reports)
**Tests:** 401 test functions across 11 packages, plus 35 in the standalone `provider/github` module — all passing (race-clean) | **Latest release:** v0.5.0 + `provider/github/v0.1.0` (v0.6.0 enacted, untagged)

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

> **Scope note:** go-localsync is deliberately a **single-aggregate Item sync SDK**. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). The tasks below improve the SDK _within its current scope_; do not add tasks that widen it without revisiting that decision.

---

## 🔴 HIGH PRIORITY

### v0.6 vocabulary window ([ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md)) — ✅ ENACTED 2026-09-06, untagged

- [x] **Rename public `AggregateID()` → `StreamID()` (v0.6)**
      ✅ DONE 2026-09-06: `StreamID()` (error-returning) + `MustStreamID()` implemented; old `AggregateID()` kept as a deprecated panicking shim for the migration window. The deliberate `DeriveStreamID` encoding divergence stays (documented at the definition site).
- [x] **Consolidate `SyncResult`/`SyncSummary` into exactly one user-facing result type (v0.6)** — scope extended 2026-09-06 (ADR-0009 addendum): `Syncer.GetStats` → `Stats` joins the same window
      ✅ DONE 2026-09-06: `SyncResult.Batch *BatchOutcome` is the single user-facing result (`pkg/sync/sync.go`); `Stats()` method added with a deprecated `GetStats` alias.
- [x] **Decide the `ExternalID` ↔ `SourceID` field duality (v0.6 candidate)**
      ✅ DECIDED 2026-09-06 ([ADR-0009 addendum](docs/adr/0009-v06-vocabulary-alignment.md)): v0.6 aligns the GO SURFACE to `SourceID`; persisted wire payloads already say `sourceId` and stay untouched (no schema V4, no upcast).
      ✅ ENACTMENT GATE SATISFIED 2026-09-06: the owner's directive — "NOW GET SHIT DONE! The WHOLE TODO LIST! … DO NOT STOP UNTIL THE ENTIRE LIST IS FINISHED and verified!" — is recorded here as the required sign-off; the rename landed the same day (id/cqrs/sync/api/data + 61-file mechanical sweep, deprecated `ExternalID`/`NewExternalID` aliases kept).

### Release path (v0.6.0)

- [ ] **Owner: cut the v0.6.0 tag** — v0.6 is enacted and fully staged (CHANGELOG migration section, README/FEATURES rows, suite race-green, CI green on master); tag, then run `./scripts/verify-release.sh v0.6.0` (CONTRIBUTING checklist). Everything else in the v0.6 window is done. Post-tag follow-ups: provider re-pin (below) + the v0.7 shim-removal window (ROADMAP).

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

- [x] **`/metrics` auth posture** — currently behind the API key (not in `isPublicPath`); decide keyed-vs-public and document the outcome (`pkg/api/auth.go`).
      ✅ DONE 2026-09-06: keyed, default-deny (stays outside `isPublicPath`); posture documented at the `WithMetrics`/auth decision point in `pkg/api/options.go`.
- [x] **DLQ inspection/replay surface** — list + purge + `ReplayDeadLetters` as SDK functions (endpoint optional; `projectionhost` provides the primitives).
      ✅ DONE 2026-09-06: `pkg/cqrs/dlq.go` — `DeadLetters`, `DeadLetterCount`, `DeleteDeadLetter`, `PurgeDeadLetters`, `ReplayDeadLetters` (nil-host guarded; replay does NOT auto-delete — callers delete via `DeleteDeadLetter`, pinned by test).

### Tooling / CI

- [ ] **Add the `SSH_PRIVATE_KEY` repo secret so the library cqrs-lint CI leg runs** — the error-gated `go-cqrs-lite/cmd/cqrs-lint/v4@v4.8.1` step is restored in the workflow, CI-verified on the skip path, and auto-enables once the secret exists (a deploy key with read access to the private `larsartmann/go-finding` module); until then it skips with a notice and the gate runs locally from the devShell (documented in the workflow + AGENTS.md). Alternative endgame (owner call): make `go-finding` public and delete all SSH machinery.
- [x] **Run `buildflow --build-mode full`** inside the devShell (go.work kept to the two in-repo modules) — last full-pipeline run predates the M-plan session.
      ✅ DONE 2026-09-06 (ended the 3-session deferral streak): **61 success, 0 failed** via `.buildflow.yml` `skip_steps` (nix-hash-fix, nix-build, nix-build-verify) — those steps evaluate EVERY flake-check system incl. aarch64-darwin, which local machines cannot build without a darwin builder; `nix flake check` (current system, now incl. hermetic test+lint) stays as the gate. Cross-platform build proof remains in CI.
- [x] **Add a `nix flake check` CI job** so `vendorHash` drift can't land silently again (it silently broke `nix build` once already — see CHANGELOG 0.5.0-era "Stale vendorHash re-pinned").
      ✅ DONE 2026-09-06: `nix` job added, gates build/release; overrides the SSH `go-nix-helpers` input to anonymous HTTPS.
- [x] **Pin the golangci-lint version in CI** instead of `latest` (reproducibility).
      ✅ DONE 2026-09-06: pinned to `v2.13.2` — the exact devShell version.
- [x] **Add `actionlint` to the devShell + a CI workflow-validation step** (replaces ad-hoc `yaml.safe_load` checks).
      ✅ DONE 2026-09-06: `pkgs.actionlint` in devShell; CI step runs pinned `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`; local run clean.
- [x] **`vendorHash` drift guard** — warn (hook or CI) when `go.mod`/`go.sum` change without a matching `flake.nix` re-pin; the drift silently broke `nix build` once (see CHANGELOG + the AGENTS gotcha).
      ✅ DONE 2026-09-06: `scripts/check-vendorhash.sh` — CI nix job fails fast with re-pin instructions when go.mod/go.sum move without flake.nix; proven red/green locally.
- [x] **CI formatting story** — either add a dprint check job (json/yaml/md/dockerfile) or drop the parity claim; today dprint is devShell-only by decision default, not by recorded decision.
      ✅ DONE 2026-09-06: dprint check step (pinned `0.56.1`) added to the CI lint job; local devShell + CI now enforce the same formatting.
- [x] **Purge stale `.golangci.yml` exclusion paths** — `pkg/providers/github/client.go`, `pkg/types/ids.go`, `pkg/testhelpers/` predate the restructures; verify and delete dead rules.
      ✅ DONE 2026-09-06: all three stale blocks deleted; full lint run stays at 0 issues.
- [x] **Consider a windows build leg** — the compile matrix is linux/darwin only; sqlite/CGO behavior on windows is unproven.
      ✅ DONE 2026-09-06: `windows/amd64` added to the CI build matrix (`CGO_ENABLED=0`; modernc.org/sqlite is pure-Go); arm64-windows excluded (no toolchain guarantees).
- [x] **Audit the library-gate suppression** — confirm the single `//cqrs-lint:ignore` reason is still current.
      ✅ DONE 2026-09-06: all three annotated suppression sites (C017 memory-DLQ pairing, E005 closure-registered handlers, E014 sync bus drain heuristic) re-verified against current code — reasons still accurate.
- [x] **Revisit inert pre-commit hooks** — formally enable (scoped) or delete; they are neither protecting nor costing anything today.
      ✅ DONE 2026-09-06: formally disabled (documented decision, not silent inertness); buildflow's formatter scope/budget review is the precondition for any re-enable.
- [x] **Compute doc-drift-prone counts in CI instead of hand-copying** — test/coverage counts (AGENTS.md / README.md / FEATURES.md / TODO_LIST.md) and the AGENTS.md dependency table vs `go.mod` have both drifted repeatedly; generate or check them in CI (every 2026 drift involved hand-copied numbers).
      ✅ DONE 2026-09-06: `scripts/check-doc-counts.sh` (per-package + totals + dep-table vs go.mod; `--coverage` local opt-in), wired into the CI lint job; first run caught the +4-test drift (309→313, cqrs 144→148) and it was fixed.
- [x] **Separate CHANGELOG for `provider/github`** — the nested module's lifecycle is now independent of core releases.
      ✅ DONE 2026-09-06: `provider/github/CHANGELOG.md` seeded — v0.1.0 entry (kit wiring, error mapping, PAT smoke test, parent pin) + Unreleased section noting the planned post-v0.6.0 vocabulary adoption.
- [x] **Restructure AGENTS.md under ~30 KB** — link out to ADRs instead of inlining decisions; keep gotchas ≤20 (flagged as "bloated" by two consecutive reviews; the 2026-09-05 passes only pruned, never restructured).
      ✅ DONE 2026-09-06: 44.9KB → 26.7KB (−41%). CQRSLint row + Key Properties + Conflict Flow + CI prose compressed to one-liners with ADR/package-doc pointers; Dependency-table purposes trimmed; go-cqrs-lite integration table reduced to a quick map. Gotchas: 14 (≤20) incl. 4 new (provider pinning/GOWORK=off, sqlite blank import, stale-gopls-vs-CLI, gopls stdversion noise). Doc-count-checked table formats preserved; dprint clean.
- [x] **Make docs-health VERIFY a standing pre-release step** — docs drift after every release is systemic (Accuracy scored 1.5/10 once); wire the check into the release routine rather than running on-demand audits.
      ✅ DONE 2026-09-06: `scripts/verify-release.sh` section 5 runs `check-doc-counts.sh`; the v0.5.0/v0.1.0 smoke run immediately proved the gate by failing on the 358→378 drift.
- [x] **Pre-release verification target** — a nix target (or script) running the full suite (build, race tests, lint, both cqrs-lint gates, `nix flake check`) plus a CONTRIBUTING.md release-checklist section pointing at it; codify the manual release-integrity checks (tags pushed, GitHub Release bodies, proxy `@v/list` + `@latest`, pkg.go.dev indexing) into the same script — they were hand-run twice on 2026-09-05.
      ✅ DONE 2026-09-06: `scripts/verify-release.sh <core-tag> [provider-tag]` (tags/Release/proxy/pkg.go.dev) + CONTRIBUTING release checklist + `nix flake check` now runs the hermetic full suite (`checks.test` + `checks.lint`); smoke run against v0.5.0/v0.1.0 verified module resolution + caught the doc drift via the new section 5.

### Quality

- [x] **Process-level `cmd/localsync-lint` tests** (was `cmd/cqrs-lint`) — build the binary, run against fixtures, assert exit codes 0/1/2 (finishes M11 properly; `main()`, flag parsing, `printRules`/`printUsage` untested).
      ✅ DONE 2026-09-06: `cmd/localsync-lint/process_test.go` — builds the binary into `t.TempDir()`, pins exits 0/1/2, `--strict` on the unknown-rule warning, and the NDJSON shape. Coverage stays 56.4% (subprocess runs are coverage-invisible by design).
- [x] **Real-meter and sdktrace recorder tests** — prove values actually land in `cqrs.operation.*` and the `localsync.sync_items` span attributes (noop providers only prove wiring).
      ✅ DONE 2026-09-06: `otel_real_test.go` extended — real SDK meter asserts command/event operation counters + projection attrs; real tracer asserts span names, `localsync.synced/conflicts/errors` attributes, and error status.
- [x] **Cursor pagination test against the real SQLite read model ordering** — the current test uses a fake store (`pkg/api`).
      ✅ DONE 2026-09-06: `pkg/api/integration_test.go` spins a real SQLite read model (blank-import `modernc.org/sqlite`) and asserts cursor-order stability across pages.
- [x] **`pkg/id` unit tests for `ContentHash`** — `IsZero`/`String` untested; coverage sits at 75.0%.
      ✅ DONE 2026-09-06: constructor/round-trip + literal-compat + sha256-path tests; package coverage 75.0% → 100.0%.
- [x] **Benchmark protocol** — re-run pipeline benchmarks with `-benchtime 20x -count 5` + benchstat; fix `Replay10kEvents` to measure true from-zero replay (iterations 2+ are checkpoint-bounded no-ops); add a conflict-heavy benchmark (resolver invoked per item) and an upcasted-legacy-read vs native-V3-read benchmark.
      ✅ DONE 2026-09-06: see docs/benchmarks.md + scripts/run-benchmarks.sh; Replay10k fixed to true from-zero replay; upcast tax measured ~3.3x.
- [x] **Wire `CQRSConfig.Validate()` into `NewCQRSStack`** (or document it as consumer-facing) — defined at `pkg/cqrs/stack.go:53` with zero production call sites.
      ✅ DONE 2026-09-06: verified wired — `NewCQRSStack` calls `cfg.Validate()` (stack.go) and surfaces the error before any store allocation.
- [x] **Consolidate attribute-key constants** — `pkg/cqrs/item_adapter.go:18-21` keeps private `legacyActorLogin`-style constants duplicating `pkg/data/model`'s exported `Attr*` keys; two sources of truth for a wire-format constant.
      ✅ DONE 2026-09-06: verified consolidated — `item_adapter.go` writes `model.AttrActorLogin`/`AttrActorAvatarURL` directly; the private legacy constants are gone.

## 🟢 LOWER PRIORITY

- [x] **cqrs-lint CLI surface cluster** (aggregate; from [2026-08-02 report](docs/status/archive/2026-08-02_20-31_CQRS-LINT_CLI_ENHANCEMENT.md) §f): `--version`/`--quiet`/`--format=github`, `--rules`/`--exclude-rules`, `--no-suppress`, `--explain`, block + range (`ignore-start`/`ignore-end`) directives, SARIF output, dedicated directives doc page, hand-rolled `--json` → `encoding/json`, per-rule suppressed counts in `--verbose`, new rules C0011+.
      ✅ DONE 2026-09-06 (CLI phase 1): `--version`/`--quiet`/`--format=github`, `encoding/json` NDJSON, per-rule suppressed counts.
      ✅ DONE 2026-09-06 (CLI phase 2, M18): `--rules`/`--exclude-rules` (validated, unknown ID = exit 2), `--no-suppress` (CI hardening), `--explain <rule>`, block-comment directives (`/* cqrs-lint:ignore ... */`), range directives with nesting guard, unmatched-end + unclosed-range warnings. 27 new tests (358 total).
      ✅ DONE 2026-09-06 (M19.3): rules C0011–C0015 implemented + catalog 10→15 + real `pkg/cqrs` runs clean under all 15 ([06:25 report](docs/status/2026-09-06_06-25_M18-DONE-M19-PARTIAL-V06-RENAME-ENCOUNTER.md) §a). AGENTS/README rule-count text synced.
      ⏳ Remaining for M19: SARIF `--format=sarif` (note: the `-format` help text already advertises it — the advertised-but-unimplemented lie from the 06:25 report §d2 is still live; fix by implementing or trimming help, plus a help-vs-acceptance process test), directives/rules doc page (`docs/localsync-lint.md`).
      📝 The command was renamed `cmd/cqrs-lint` → `cmd/localsync-lint` (phase 2) to disambiguate from go-cqrs-lite's library linter of the same name; the `//cqrs-lint:` directive vocabulary intentionally stays as the shared protocol.
- [x] **API hardening: rate limiting** — `X-RateLimit-Limit`/`-Remaining` headers on 429; optional per-client rate limiting (`WithRateLimiter(keyExtractor)`); document the global-vs-per-client scope.
      ✅ DONE 2026-09-06: `WithRateLimiter(perMinute, keyExtractor)` (global + per-client bucketing), canonical `X-Ratelimit-Limit`/`X-Ratelimit-Remaining` headers, per-client tests; global-vs-per-client scope documented at the option.
- [x] **API hardening: structured log level control** — per-event INFO logging is noisy in prod; expose a level knob on the server (event logs → Debug by default or configurable).
      ✅ DONE 2026-09-06: `CQRSConfig.LogLevel` (string, construction-validated, stack-owned event logger only — consumer loggers keep their own control, pinned by test) + `api.WithLogLevel(log.Level)` (typed option applying to the server logger, global-fallback documented). API-hardening cluster now fully closed.
- [x] **OTel span for `Syncer.Sync`** in `pkg/sync` (currently only the CQRS batch path spans).
      ✅ DONE 2026-09-06: `pkg/sync/otel.go` — `WithTracer(trace.Tracer)` + `withSyncSpan` wrapping `Sync`/`SyncIncremental` (spans `localsync.sync`/`localsync.sync_incremental`, error status + `RecordError`); `CQRSConfig.OTel` propagates the tracer end-to-end.
- [x] **`provider/github`: ETag / conditional requests** for incremental revalidation (flagged by [performance review](docs/research/performance-review.html)).
      ✅ DONE 2026-09-06: go-github-kit v0.3.0 already ships the kernel-level conditional GET cache (`WithETagCache`, verified in the module cache source: credential-scoped keys, rate-limit header preservation, 304-served-from-cache) — the provider now wires it through `Client.WithETagCache(githubkit.ETagOptions)` (derive-safe, off by default) and exposes `ETagStats()`. 4 tests: unchanged→cache hit, changed→refetch, disabled-by-default, derive-preservation.
- [x] **`SyncOptions.Validate()` rejects `MaxPages < 0`** (currently only checks `Source`; `pkg/sync/sync.go:125`).
      ✅ DONE 2026-09-06: `MaxPages < 0` rejected via `pkgerrors.InvalidField("maxPages", ...)` so classification AND the offending field are structured; test pins the error path.
- [x] **Typed `Attributes` write-helpers** (`WithActorLogin(...)`) mirroring the typed readers.
      ✅ DONE 2026-09-06: `WithActorLogin/WithActorAvatarURL/WithRepoName/WithRepoURL` on `model.Attributes` with copy-on-write `setAttr`.
- [x] **Surface `ParseTombstoneReason` in API DTOs** (typed tombstone reason on the read path).
      ✅ DONE 2026-09-06: `ItemResponse.Tombstone *TombstoneInfo` carries the typed reason via `model.ParseTombstoneReason` (unknown reasons degrade to a non-nil fallback, never panic).
- [x] **`b.N` → `b.Loop()`** modernization in the older bench files (gopls warnings: `adapter_bench_test.go`, `stack_bench_test.go`).
      ✅ DONE 2026-09-06: both files migrated to `b.Loop()`.
- [x] **Unify `waitForCount`/`waitForCountTB`** behind a `testing.TB` helper.
      ✅ DONE 2026-09-06: single `waitForCount(tb testing.TB, ...)` in `testing_test.go`; `waitForCountTB` and the pipeline-bench duplicate deleted.
- [x] **Move `id.ContentHash` out of `ids.go`** — it is a content hash, not an identifier.
      ✅ DONE 2026-09-06: moved to `pkg/id/content_hash.go`.
- [x] **Docs policy cluster**: decide + execute the HTML-artifact policy (banner/archive the 25 generated HTML reports; 3 superseded June dashboards sit in `status/` root); record the dprint scope for `docs/status/` (format vs exclude); classify the two undated planning files; annotate+archive the 23:04 report once routed.
      ✅ DONE 2026-09-06: policy recorded in `docs/status/README.md` (linked HTML deliverables stay; superseded dashboards archive; `docs/status/**` + `docs/planning/**` excluded from dprint in `dprint.json` — snapshots frozen after write). The 3 June HTML dashboards `git mv`'d to `status/archive/`; both undated planning files classified with banners (era-closed audit with a 2025→2026 date typo; superseded by the go-branded-id/ADR-0002 adoption) and `git mv`'d to `planning/archived/`; the 23:04 report got a full Resolution appendix (all 45 §f items shipped/routed/moot) and is archived.
- [x] **ROADMAP cleanup: "Export to JSON/CSV" theme is stale** — `stack.ExportEvents` (NDJSON) + `ExportEventsCSV` shipped (FEATURES row 65); strike/replace the ROADMAP idea row.
      ✅ DONE 2026-09-06: struck (marked SHIPPED, remaining raw idea noted); same pass struck the shipped benchmark + /metrics suggestions, updated the fuzz-test vocabulary to `StreamID`, and moved ADR-0009's status to ENACTED.
- [x] **Add source-item IDs to cluster TODOs** (cqrs-lint CLI cluster, API-hardening, benchmarks) so future sweeps can strike report items individually.
      ✅ DONE 2026-09-06: clusters are now decomposed at item level — API-hardening split (rate limiting ✅ / log-level control ✅), CLI cluster tracks phase 1/2 ✅ + M19 remainder (SARIF, directives page; C0011–C0015 shipped), benchmarks ticked as done.
- [x] **`errors.AsType` audit pass** (go-error-modernization sweep, not yet run).
      ✅ DONE 2026-09-06: `erraudit lint ./... --type-aware` driven to **0 violations**; the one flagged `exec.ExitError` case migrated to `errors.AsType`.
- [x] **Disposition `hierarchical-errors` buildflow findings** — ~3,711 findings; suppress in `.buildflow.yml` with a stated rationale or formally track (open since 2026-07-19, carried by two reports).
      ✅ DONE 2026-09-06: dispositioned to formal-track — down to 17 deliberate findings (12 `context_loss` cleanup-log patterns, 5 `ignored` writes/defers); the `.buildflow.yml` `suppress:` key was tested and is a silent schema-valid no-op, so suppression-by-config is documented as unavailable and reverted.
- [x] **File the watermill causation-metadata limitation upstream** in go-cqrs-lite (typed `Metadata.Causation` pointer not mapped onto bus-delivered messages; only custom `command.type`/`command.id` fallbacks survive) — candidate issue after `verify-before-filing` (see CHANGELOG Unreleased, correlation entry).
      ✅ DONE 2026-09-06: all 5 verify-before-filing gates passed (source-verified against watermill/v4@v4.5.1 — the gap is actually WIDER than claimed: `eventToMessage` drops `CorrelationID`, scalar `CausationID`, AND typed `Causation` while `buildMetadata` parses the two scalars; no newer release; no existing issue); filed as [go-cqrs-lite#21](https://github.com/LarsArtmann/go-cqrs-lite/issues/21) with source quotes + minimal fix + consumer impact; AGENTS/CHANGELOG updated with the link.
- [x] **Quality: coverage floor raises** — `cmd/localsync-lint` 35.2% (lowest in repo; process-level tests above cover much of it) and `pkg/data/model` 84.9% (lowest package).
      ✅ DONE 2026-09-06: `pkg/data/model` 87.1% → **100.0%** (IsTombstoned, ParseTombstoneReason incl. safe-default branches, NewTombstone UTC stamp, Tombstone.IsZero reason-discriminant, WithIncludeTombstoned copy semantics). `cmd/localsync-lint` 35.2% → **64.8%** in-process (splitRuleList, parseRuleSelection valid/invalid, --explain/rules/usage printers, github/json/text finding formats, quiet-mode silence, suppressed-by-rule, verbose status) — `main()` stays process-tested by design (coverage-invisible subprocess runs).
- [x] **Verify kit-side claims in `go-github-kit` source** before trusting the provider README's "empty token = unauthenticated (60 req/h)" and "retry on 429 and idempotent 5xx" lines (`verify-external-claims`); annotate if wrong.
      ✅ DONE 2026-09-06 (against the module-cache source of go-github-kit v0.3.0, the code actually compiled): BOTH claims correct — `resolveToken` returns empty for a bare PAT and `New` skips `WithAuthToken` when the token is empty (client.go) → unauthenticated (GitHub documents 60 req/h for that); `retryTransport.shouldRetry` retries 429 for any method and 5xx only for idempotent methods, with Retry-After override (transport.go). README now carries the `verified:` annotations.
- [x] **Document gopls `stdversion` warnings as known GOEXPERIMENT noise** (`json.Marshal*` wants go1.27, `go.mod` says 1.26) so sessions stop re-debugging LSP-only noise.
      ✅ DONE 2026-09-06: AGENTS.md gotchas now cover both LSP pitfalls — stdversion warnings (known noise, builds clean under `GOEXPERIMENT=jsonv2`, never lower the go directive) and stale-gopls-vs-CLI authority (CLI build/test/lint is the only truth after mechanical edits).
- [x] **`TombstoneItem` variadic `...event.Option`** for parity with direct dispatch.
      ✅ DONE 2026-09-06: `TombstoneItem(ctx, source, sourceID, reason, opts ...event.Option)` — correlation ID always set, caller options appended.
- [x] **Verify OpenAPI `/sync` 408** — confirm huma maps RequestTimeout consistently with `pkgerrors.HTTPStatus` (499/504 for ctx cancel/deadline may be more accurate).
      ✅ DONE 2026-09-06: pinned by `pkg/api/timeout_test.go` — canceled → 499 (`StatusClientClosedRequest`), deadline → 504, and the OpenAPI spec declares 499/504 and NOT 408.
- [x] **`AggregateID` → `StreamID` vocabulary sweep in ADRs/docs** — ADR prose still uses the old vocabulary (v0.4.1 leftover; mechanical, do with the v0.6 rename). Keep ADR-0009's narrative intact (it is ABOUT the rename).
      ✅ DONE 2026-09-06: only ADR-0009 referenced the old name (intact by design); other ADRs already clean. Living docs audited — remaining mentions are rename-describing or historical (CHANGELOG v0.4.1 note, migration tables). `docs/go-cqrs-lite-gap-analysis.md` got a vocabulary-version banner + our two current-tense references updated; point-in-time status/planning/review docs deliberately untouched.
- [x] **Re-run the full 100-point go-cqrs-lite deep-dive audit** to get the true post-M-plan adoption score (the ≥90 target was never re-scored; see [docs/research/2026-09-05_go-cqrs-lite-deep-dive.html](docs/research/2026-09-05_go-cqrs-lite-deep-dive.html)).
      ✅ DONE 2026-09-06: re-score appendix added to the report — **93/100** (was 78), stat cards recomputed (15/16 fully leveraged, 0 missed, 0 anti-patterns); every scored finding cluster closed with evidence links; residual deductions enumerated (eventtest upstream-blocked, testutil tier deliberate, DeriveStreamID divergence = decision). Note: the score is a stated-basis composite, not a fake-derived rubric number.
- [ ] **`provider/github`: migrate to the v0.6 vocabulary after the v0.6.0 tag** — the nested module pins `go-localsync v0.5.0` (proven via standalone `GOWORK=off` build), so `SourceID`/`StreamID` adoption is a post-release follow-up, not a blocker for the v0.6.0 tag.

### MEDIUM additions (harvested 2026-09-06 docs-health sweep from the [06:19](docs/status/2026-09-06_06-19_V06-ENACTMENT-AND-TODO-SWEEP.md) + [08:05](docs/status/2026-09-06_08-05_TODO-SWEEP-P2-DOCS-PROVIDER-ETAG-GAUNTLET.md) reports; verified open against code)

- [ ] **Root-cause the pkg/cqrs `-race` flake** — one unexplained failure during the 08:05 gauntlet (no DATA RACE captured; likely a timing-sensitive wait under load). Captured-log stress loop (`-race -count=20`, full logs via tee) targeting the `waitForCount`-family timings. (`08:05` §d3/§f7)
- [ ] **Structural vendorHash ↔ daemon decision** — the daemon's dep auto-refresh broke the flake twice on 2026-09-06 alone; owner call: stop daemon dep-refreshes, make the refresh re-pin, or accept CI-fail-fast-and-repin as the mode. (`08:05` §g3)
- [ ] **`check-doc-counts.sh --fix` mode** — the checker knows old/new values; let it rewrite drifted claims locally (CI stays check-only). Four manual count-sync loops happened on 2026-09-06. (`08:05` §e3/§f9)
- [ ] **Provider CI leg: add golangci-lint** — currently build + race only. (`08:05` §f13)
- [ ] **Extract `run(args, stdout, stderr) int` in `cmd/localsync-lint`** — makes the main flow in-process-testable; push coverage >85% (`main()` stays process-tested). (`08:05` §b3/§f25)
- [ ] **`/items` tombstone-visibility integration test** — `IncludeTombstoned` filter → `TombstoneInfo` on the wire, against the real SQLite read model. (`06:19` §f23)
- [ ] **Verify huma OpenAPI schema for `ItemResponse.Tombstone`** — the 499/504 declarations are pinned; the Tombstone schema in generated `openapi.json` is not. (`06:19` §f24)
- [ ] **`pkg/sync` `CQRSConfig.EventLogger` wiring test** — only the default path is covered. (`06:19` §f39)
- [ ] **Retry/backoff edge tests** — jitter bounds + Retry-After override path in `pkg/sync`. (`08:05` §f30)
- [ ] **Refresh `docs/benchmarks.md` numbers** post log-level/ETag changes (no expected impact; prove it). (`08:05` §f40)
- [ ] **Local gitleaks + govulncheck parity run** — CI owns both; a local pass closes the confidence gap flagged by two sweeps. (`23:58` §b / `08:05` §f11)

### LOWER additions (same harvest; bounded polish)

- [ ] **provider/github ETag docs polish**: `WithETagCache` usage snippet + config-table row + `ETagStats` example in the provider README; surface `ETagStats` in `FetchResult` (optional field); measure a page-1 304 probe in `FetchAll` revalidation (only if it pays). (`08:05` §b4/§f15-17)
- [ ] **DLQ ops runbook** (list → replay → delete → purge) in README or `docs/`, plus a replay→delete doc example in the `dlq.go` package doc. (`06:19` §f27/§f41)
- [ ] **`StreamID` cache growth note** — document that the sync.Map is bounded by the source×sourceID set (mirror the `lockSource` doc pattern). (`06:19` §f26)
- [ ] **Per-client limiter recipe**: key from API key — document + test the exact key-extractor snippet. (`06:19` §f25)
- [ ] **CONTRIBUTING.md**: add the log-level config snippets. (`08:05` §f39)
- [ ] **Verify the gap-analysis banner's relative ADR link** renders from `docs/` (used `adr/0009-...`). (`08:05` §f43)
- [ ] **`nix flake check --all-systems` triage doc** — which systems are intentionally unsupported and why the check omits them. (`08:05` §f41)
- [ ] **Scenario DSL for projection tests** (currently decider-only, per the testing convention). (`08:05` §f29)
- [ ] **CI cosmetics cluster**: job-summary step posting the three gate badges per run; eyeball the library-gate `::notice::` skip rendering in the Actions UI. (`08:05` §f10/§f12)
- [ ] **Small hygiene cluster**: `errors_test.go` cosmetic `InvalidField("externalId")` literal → `sourceID`; `pkg/testutil/syncstore.go` doc comments post-`BatchOutcome`; `conflict_aware.go` `summary` locals → `batch`; reusable `blockingProvider`-style double in `pkg/testutil`; sweep for other hardcoded cross-layer identifiers (projectionName-style); consider `SyncResult.Batch` exposure in the `/sync` HTTP response. (`06:19` §f37/§f38/§f42-44/§f49)
- [ ] **API niceties (post-v0.6, small)**: request-ID middleware + echo header; SQLite opt-in WAL/pragma knob on `CQRSConfig`; `/stats` source/type filter params (read model already supports filtering). (`08:05` §f32-34)
