# Status Report — Whole-TODO-List Sweep Part 2: Docs Cluster, Provider ETag, Upstream Issue, Final Gauntlet

**Generated:** 2026-09-06 08:05 CEST
**Session scope:** Continuation of the owner's "execute the ENTIRE TODO_LIST" directive. Started from the v0.6-enactment state (previous report: `2026-09-06_06-19_V06-ENACTMENT-AND-TODO-SWEEP.md`), executed the remaining docs/tooling/provider/audit clusters, drove every implementable TODO to done, and ran the full verification gauntlet.
**Commit note:** The auto-commit daemon committed and pushed all session work in heuristic chunks (HEAD `dfdfb93` at report time, tree clean, branch == origin/master). No manual commits.
**Verification headline:** `go test -race ./...` exit 0 (401 tests / 11 pkgs + 35 provider), `golangci-lint` 0 issues, both cqrs-lint gates clean, `nix flake check` green locally after an emergency vendorHash re-pin (see d1/d2), dprint + actionlint + doc-counts green. **Master CI was red twice during the session and one red leg (vendorHash) has a fix sitting uncommitted at report time — see d1/d2 and the 🔴 banner below.**

> 🔴 **ACTION REQUIRED:** the vendorHash re-pin (`flake.nix` → `sha256-ZCX5pAXML5c+rKjqFNvk1C4VM+lqxzWN50sTShl0E2A=`) is on disk and locally verified but NOT yet committed/pushed at report time. Until the next daemon commit lands and CI re-runs, master's nix leg stays red. Verify with `gh run list` before trusting "green".

---

## Headline

Every implementable item on TODO_LIST.md is now done or explicitly owner-gated — the list went from ~25 open items to **3** (one owner-gated secret call, one historical struck pivot, one deliberate post-tag follow-up). The v0.6 breaking window is fully documented and staged for the tag. The provider module gained ETag conditional caching. One upstream library gap was verified at source level and filed (go-cqrs-lite#21). The adoption audit re-scored **78 → 93/100**.

---

## a) FULLY DONE

| # | Item | Evidence |
| - | ---- | -------- |
| 1 | **TODO_LIST refresh: ~30 items ticked with evidence notes** | Every tick carries a `✅ DONE 2026-09-06:` note with file/test pointers; the ADR-0009 ExternalID→SourceID enactment gate is recorded with the owner's verbatim directive ("NOW GET SHIT DONE! The WHOLE TODO LIST! …") as the sign-off. |
| 2 | **Doc-count truth restored (multiple iterations)** | 358→378→386→401 test counts + per-package rows + provider 31→35 + coverage columns (cqrs 87.7→85.1, model →100.0, api →96.4) across AGENTS/README/FEATURES/TODO_LIST; `check-doc-counts.sh` green in both modes after each code change. |
| 3 | **CHANGELOG restructured to v0.6.0** | `[Unreleased]` → `[v0.6.0] - Unreleased` with a 9-row **Migration (v0.5.0 → v0.6.0)** table (SourceID, StreamID, BatchOutcome, Stats, DTO `sourceId`, 499/504, TombstoneItem variadic, MaxPages validation, sentinel interface-typing) + all session entries. |
| 4 | **FEATURES.md rows 68-71** | DLQ SDK surface, tombstone-on-read-path DTO, typed attribute write-helpers, v0.6 vocabulary row; rows 60 (OTel spans), 62 (rate limiter + headers), 67 (CLI 23 tests) refreshed; stale "8 tests" claim fixed. |
| 5 | **README v0.6 migration quick-reference** | 6-row old→new table + "no data migration" note, placed before Quick Start. |
| 6 | **Vocabulary sweep (ADRs/docs)** | Only ADR-0009 referenced `AggregateID` (intact by design); living docs audited clean; `docs/go-cqrs-lite-gap-analysis.md` got a vocabulary-version banner + 2 current-tense refs updated; point-in-time status/planning docs deliberately untouched. |
| 7 | **AGENTS.md restructure: 44.9KB → 26.7KB (−41%)** | CQRSLint row + Key Properties + Conflict Flow + CI prose compressed to one-liners with ADR/package pointers; dep-table purposes trimmed; integration table → quick map. Gotchas: **14** (≤20) incl. 4 new: provider pinning/GOWORK=off, sqlite driver blank-import, stale-gopls-vs-CLI authority, gopls stdversion GOEXPERIMENT noise. Checker-parsed table formats preserved. |
| 8 | **Docs policy cluster decided + executed** | Policy recorded in `docs/status/README.md` (§5 HTML artifacts: linked deliverables stay, superseded dashboards archive; §6 dprint scope: `docs/status/**` + `docs/planning/**` excluded — snapshots frozen). 3 June HTML dashboards `git mv`'d to archive; both undated planning files classified with banners (era-closed audit w/ 2025→2026 date typo; superseded by go-branded-id/ADR-0002) and moved to `planning/archived/`; the 23:04 report got a full Resolution appendix (all 45 §f items shipped/routed/moot) and is archived. |
| 9 | **provider/github CHANGELOG seeded** | v0.1.0 entry (kit wiring, error mapping, PAT smoke, parent pin) + Unreleased with the ETag addition and the claims-verification note. |
| 10 | **ROADMAP cleanup** | Export-JSON/CSV struck (SHIPPED + remaining streaming idea), benchmark + /metrics suggestions struck (SHIPPED), fuzz-idea vocabulary → `StreamID`, ADR-0009 status → **ENACTED**, Last Updated refreshed. |
| 11 | **provider/github ETag conditional requests** | Kit v0.3.0 already ships the kernel-level conditional GET cache (verified in module-cache source); wired via `Client.WithETagCache(githubkit.ETagOptions)` (derive-safe, off by default) + `ETagStats()`. 4 tests: unchanged→cache hit (Hits=1), changed→refetch, disabled-by-default, derive-preservation. Provider suite green (35 tests). |
| 12 | **go-github-kit claims verified at source** | Against the module-cache source of v0.3.0 (the code actually compiled): empty PAT → `resolveToken` returns "" and `New` skips `WithAuthToken` (client.go) → unauthenticated (GitHub-documented 60 req/h); retry = 429 any method + idempotent 5xx, Retry-After overrides backoff (transport.go:166-170). Both README lines now carry `verified:` annotations. |
| 13 | **Watermill causation-metadata issue filed upstream** | All 5 `verify-before-filing` gates passed — the gap is WIDER than the TODO claimed: `eventToMessage` (protocol.go:53-96) drops `CorrelationID`, scalar `CausationID`, AND typed `Causation` while `buildMetadata` (:225-245) parses the scalars; no newer release; no existing issue. Filed as [go-cqrs-lite#21](https://github.com/LarsArtmann/go-cqrs-lite/issues/21) with source quotes + minimal fix + consumer impact; AGENTS + CHANGELOG link it. |
| 14 | **Deep-dive audit re-scored: 78 → 93/100** | Re-score appendix added to the report HTML: stat cards recomputed (15/16 fully leveraged, 0 missed, 0 anti-patterns), every scored finding cluster closed with evidence links, residual deductions enumerated (eventtest upstream-blocked ~4, testutil tier deliberate ~2, DeriveStreamID documented divergence ~1). Honest basis note: composite judgment, not fake-derived rubric. |
| 15 | **API-hardening cluster fully closed: log-level control** | `CQRSConfig.LogLevel` (string, construction-validated via `Validate()` → `InvalidField("logLevel")`, applies to stack-owned event logger only — consumer loggers keep control, pinned by test) + `api.WithLogLevel(log.Level)` (typed option, global-fallback documented). 8 tests. |
| 16 | **Coverage floors raised** | `pkg/data/model` 87.1% → **100.0%** (IsTombstoned, ParseTombstoneReason safe-default branches, NewTombstone UTC, Tombstone.IsZero discriminant, WithIncludeTombstoned copy semantics — 5 tests). `cmd/localsync-lint` 35.2% → **64.8%** in-process (splitRuleList, parseRuleSelection, printers, github/json/text formats, quiet-silence, suppressed-by-rule, verbose status — 9 tests); `main()` stays process-tested by design. |
| 17 | **Full verification gauntlet green** | Final state: build ok · `go test ./...` 11/11 ok · `go test -race ./...` exit 0 (×2 full runs + 3× cqrs stress) · golangci 0 issues · internal gate clean · library gate "No findings" · dprint green · actionlint green · doc-counts green (both modes) · gofmt/goimports clean · provider standalone ok. |

## b) PARTIALLY DONE

| # | Item | Done | Missing |
| - | ---- | ---- | ------- |
| 1 | **Master CI green** | The doc-count fix commit triggered a fresh run; the two earlier reds are understood (stale counts in the ETag commits — my fixes landed later; vendorHash drift from the daemon's dep refresh). | The in-progress run failed on the **nix leg (vendorHash mismatch)**; the re-pin is on disk, `nix flake check` passes locally, but the fix is **uncommitted** at report time. Next daemon commit + CI run must be watched to green. |
| 2 | **Deep-dive "re-run the full 100-point audit"** | Re-score appendix with recomputed stat cards and per-finding resolutions (the ≥90 target met). | A full per-dimension re-audit of all 16 leveraged categories (fresh HTML, re-verified against the library's current master) was not rebuilt — the appendix reuses the 2026-09-05 category analysis plus today's evidence. |
| 3 | **cmd/localsync-lint coverage** | 35.2% → 64.8% in-process; every pure helper now unit-tested. | `main()` itself stays 0% in-process (subprocess harness covers the contract by design). Extracting a testable `run(args, stdout, stderr) int` would push it higher — not done. |
| 4 | **provider README ETag documentation** | Feature bullet with semantics + `verified:` claim annotations. | No usage snippet in the README's Usage section (WithETagCache example) — the config table also lacks the new option row. |
| 5 | **TODO_LIST open-item hygiene** | Down to 3 open items, each with a stated gate. | The 3 remaining are all owner-gated or post-tag (see c). |

## c) NOT STARTED

| # | Item | Why |
| - | ---- | --- |
| 1 | **`SSH_PRIVATE_KEY` secret / make `go-finding` public** | Owner call (recurring since 2026-09-05; §g). |
| 2 | **v0.6.0 tag + release run** | Owner timing call (§g); everything staged (CHANGELOG migration, verify-release.sh, checklist). |
| 3 | **provider/github v0.6 vocabulary migration** | Deliberate post-tag follow-up — the module pins released `go-localsync v0.5.0` (tracked as a TODO_LIST item). |
| 4 | **eventtest adoption** | Upstream module still has no released version (ROADMAP open question; unchanged). |
| 5 | **SARIF `--format=sarif`, directives doc page, rules C0011-C0015** (cqrs-lint M19) | Not attempted this session — M19 remainder deliberately left after the M18+coverage closure; not on the critical path for v0.6.0. |
| 6 | **Upstream PR for go-cqrs-lite#21** | Issue filed with the minimal-fix sketch; a PR against the library repo is a separate work item (and the repo is the owner's — sequencing wanted). |

## d) TOTALLY FUCKED UP

1. **Master CI was red for ~50 minutes and I did not notice.** Two runs failed (05:00Z: ETag commit with stale root-doc counts; 05:07Z: same drift) while I was fixing those very counts locally. I ran `check-doc-counts.sh` repeatedly but never ran the **`gh run list` session ritual** (item 45 of the previous report, adopted as a standing rule!) until report time. The drift gate did its job in CI; my push-side awareness failed. Lesson encoded in f1.
2. **vendorHash drift — again — with the fix uncommitted at report time.** The daemon's dependency auto-refresh (go-retry v0.4.0→v0.5.0 at 07:45, otel indirect→direct promotion at 07:06) moved `go.sum` without re-pinning `flake.nix` — the documented gotcha, now triggered by the daemon itself, not human edits. CI's nix job failed fast with the new hash (guard worked as designed); I re-pinned + verified locally, but the red window on master persists until the next commit. Systemic issue: **the daemon refreshes deps on a cadence that will keep breaking the flake** (§g question 3).
3. **An unexplained `-race` flake in `pkg/cqrs` (run 1 of the final gauntlet) — never root-caused.** The failing test name was lost (I only captured the tail via job_output; no DATA RACE was reported, so likely a timing-sensitive wait under full parallel load). 3 subsequent full-suite runs + 3× stress runs are clean, but "didn't reproduce" is not a diagnosis, and I destroyed the evidence by not capturing the full log on first failure.
4. **Six-plus compiler round trips from writing test code against unverified helper APIs** — the exact anti-pattern the `verify-external-claims` skill warns about, applied to MY OWN codebase: invented `testutil.AssertTrue/False` (don't exist), `discardEventLogger`, `timeValueOf`; imported `encoding/json` where the provider module uses `encoding/json/v2` (`json.MarshalWrite` undefined); assumed import alias `charmlog` when the file imports `charm.land/log/v2` as `log`; called `NewCQRSStack(ctx, cfg)` with a context arg (signature is `(cfg)`); one sed delimiter collision. Reading the existing helpers FIRST (one grep) would have made each a zero-cost edit.
5. **"Files fixed locally" ≠ "pipeline green"** — I treated the doc-count fixes as done per-iteration without connecting that the daemon's commit cadence meant CI was validating older snapshots. The verify-release.sh smoke test had ALREADY demonstrated this exact failure mode in the morning (it failed on the 358→378 drift); I fixed the numbers but not the *loop* (fix → commit → CI-confirm).

## e) WHAT WE SHOULD IMPROVE

1. **Session-start AND session-end rituals must include `gh run list`** — start: is master green from yesterday's pushes? end: are MY pushes green? Cheap, catches exactly the two red windows of this session.
2. **Verify helper APIs before writing tests** — grep the package's existing test helpers before writing a new test file; treat "the compiler would catch it" as wasting a round trip, not as a safety net.
3. **Check-doc-counts should offer a `--fix` mode** — four manual count-sync loops this session (each touching 3-4 files in lockstep). The checker already knows old/new values; it could rewrite the claims.
4. **The vendorHash↔daemon interaction needs a structural fix** — either the daemon stops auto-refreshing deps, or its refresh step must re-pin the flake (or the flake should derive the vendor hash). Ask-the-owner material (§g3).
5. **Capture full logs for failing runs immediately** — the race-flake diagnosis died with the truncated tail. `go test -race ./... 2>&1 | tee /tmp/...` should be the default for gauntlet runs.
6. **dprint after every markdown edit batch, not reactively** — five separate "N not formatted" surprises this session; a fmt-per-batch habit removes them.
7. **The AGENTS gotcha for vendorHash should name the daemon explicitly** — today's incident proves the trigger is usually the daemon, not human dep work; the current wording implies human action.
8. **New-feature README coverage should be one pass** — ETag got a feature bullet + tests but no Usage snippet/config row; finishing the user-facing surface in the same edit batch avoids follow-up passes.
9. **Prefer dedicated loggers in library code** — `WithLogLevel` mutating the caller's (possibly global) logger is documented but inherently side-effecting; a per-server/per-stack logger would be cleaner API hygiene for a future minor.
10. **Keep pairing new flags with their validation tests in the same commit** — done right this session (LogLevel + tests together); make it the standing rule it already implicitly is.

## f) NEXT — up to 50 things to get done (impact-ordered)

**P0 — restore + prove green, then release**

1. Commit/push the vendorHash re-pin (daemon) and watch the next CI run to green — `gh run list` until `nix` + `lint` legs pass.
2. Owner: decide v0.6.0 tag timing (§g1) — everything (migration docs, verify-release.sh, gauntlet) is staged; tag → run `scripts/verify-release.sh v0.6.0` → GitHub Release → proxy/pkg.go.dev checks.
3. Owner: `SSH_PRIVATE_KEY` secret vs make `go-finding` public (§g2) — activates the library cqrs-lint CI leg; the skip-path is CI-proven.
4. Re-run the verify-release.sh smoke test post-re-pin to confirm the docs-consistency leg goes green end-to-end.
5. Post-tag: provider/github v0.6 vocabulary migration (re-pin parent to v0.6.0, `ExternalID`→`SourceID` etc., standalone suite green, tag `provider/github/v0.2.0`).
6. Post-tag: strike the v0.6 migration shims' TODO notes and plan the v0.7 shim-removal window (deprecated `ExternalID`/`AggregateID`/`GetStats`).

**P1 — CI truth / flake hunt**

7. Root-cause the pkg/cqrs `-race` flake: captured-log stress loop (`-race -count=20` with full logs) targeting the waitForCount-family timings under load.
8. Structural vendorHash↔daemon fix per §g3 decision (daemon stops dep-refresh / refresh re-pins / flake derives hash).
9. `check-doc-counts.sh --fix` mode (auto-rewrite drifted claims; keep CI in check-only mode).
10. Add a CI job-summary step that posts the three gate badges (test/lint/nix) per run for at-a-glance state.
11. Local `gitleaks` + `govulncheck` parity run (security-leg confidence; CI already owns them).
12. Confirm `::notice::` skip-message rendering in the Actions UI (logic verified; cosmetics never eyeballed).
13. Provider leg: add golangci-lint to the provider CI job (currently build+race only).
14. Dependabot/renovate reminder wiring: dependency PRs must mention vendorHash re-pin (or let the nix job fail-fast, which it now provably does — document that as the guard).

**P2 — v0.6 polish**

15. provider README: `WithETagCache` usage snippet + config-table row; `ETagStats` example.
16. Wire ETag into `FetchAll`'s revalidation story (page-1 probe could skip on 304) — measure the saving first; only if it pays.
17. Surface `ETagStats` in `FetchResult` (optional field) so consumers see cache behavior per run.
18. CHANGELOG: fill the v0.6.0 date at tag time; reconcile section order (there are two `### Added` blocks inside v0.6.0 — merge for cleanliness).
19. AGENTS.md: drop the v0.6-migration vocabulary note once shims are removed (v0.7).
20. README sales-page: final pass post-v0.6 (feature bullets for DLQ surface, rate limiter, log level, ETag).

**P3 — library contributions**

21. Upstream PR for go-cqrs-lite#21 (eventToMessage mapping + round-trip test) — unblocks dropping our custom-fallback note.
22. Upstream issue (or PR): watermill `MessageToEvent` reconstructing typed `Causation` when the new keys exist.
23. Check `go-github-kit` for a newer version than v0.3.0 at next provider touch; if the ETag/claims surface changed, re-verify annotations.
24. eventtest: adopt for stack tests the moment a version is tagged (ROADMAP watcher).

**P4 — quality / coverage**

25. Extract `run(args, stdout, stderr) int` in cmd/localsync-lint; in-process tests for main-flow; push coverage >85%.
26. SARIF `--format=sarif` (M19 remainder).
27. Directives/rules doc page (M19 remainder).
28. New rules C0011-C0015 (M19 remainder; candidates from review backlogs).
29. Scenario DSL for projection tests (currently decider-only).
30. `pkg/sync` retry/backoff edge tests (jitter bounds, Retry-After override path).
31. DLQ HTTP endpoints (`GET /dead-letters`, `POST /dead-letters/replay`) — optional per the original decision; only with owner buy-in on API surface growth.
32. `pkg/api` request-ID middleware + echo header (debuggability).
33. SQLite: opt-in WAL/pragma configuration knob on CQRSConfig.
34. API `/stats`: source/type filter params (read model already supports filtering).
35. Fuzz tests for the localsync-lint directive parser + `StreamID` derivation (ROADMAP raw idea).
36. BDD/Ginkgo sync-flow suite (ROADMAP raw idea; low priority while table tests are green).

**P5 — docs / hygiene**

37. Annotate+archive the 02:40 and 06:25 status reports (both should now be fully routed).
38. Harvest the previous 23:04 report's §f leftovers — verify none remain un-routed (done today; re-check after this report).
39. Add the log-level knobs to CONTRIBUTING.md's config snippets.
40. docs/benchmarks.md: refresh numbers post-log-level/ETag changes (no expected impact; prove it).
41. `nix flake check --all-systems` triage: document which systems are intentionally unsupported and why the check omits them.
42. AGENTS.md: gotcha for "daemon dep refreshes break vendorHash — re-pin from the CI error string" (make today's incident the canonical example).
43. Verify the gap-analysis banner's relative ADR link renders correctly from `docs/` (used `adr/0009-...`).
44. FEATURES: add the log-level row (65+ feature rows — keep numbering stable).
45. TODO_LIST: convert the two remaining struck/historical items into a "closed" bucket or ROADMAP notes for list hygiene.

**P6 — someday / raw ideas (do not start without owner)**

46. Streaming export for very large journals (the struck Export theme's remainder).
47. Bundled Prometheus exporter on top of `WithMetricsHandler` (the struck /metrics remainder).
48. Per-page `ETag` reuse in provider pagination (upstream kit feature request if useful).
49. Multi-error aggregation semantics for partial sync (errors.Join of per-item failures).
50. Second provider implementation (GitLab/Jira) — validates the interface against a different API shape (ROADMAP theme).

## g) Questions I cannot figure out myself

1. **v0.6.0 tag: now or later?** Everything is staged (code enacted and verified, CHANGELOG migration section, verify-release.sh, README/FEATURES docs). Do you want the tag cut from current master as soon as CI is green again — and if so, should the release run through `scripts/verify-release.sh v0.6.0` with you watching, or should I prepare the tag as a draft for your manual push?
2. **go-finding: deploy-key secret or public?** The library cqrs-lint CI leg has been skip-gated for three sessions. Adding the `SSH_PRIVATE_KEY` secret (read-only deploy key) activates it as-is; making `larsartmann/go-finding` public would delete all SSH machinery permanently. Which endgame do you want?
3. **The auto-commit daemon's dependency refreshes broke the flake twice before and again today (go-retry bump at 07:45).** Should I (a) tune the daemon to stop auto-refreshing dependencies entirely, (b) make its refresh step always pair go.mod/go.sum changes with a vendorHash re-pin, or (c) leave accept-and-repin as the intended mode (CI's nix leg catches it in ~2 minutes)? This decides whether the vendorHash gotcha stays a recurring tax.

---

_Point-in-time snapshot — generated 2026-09-06 08:05 CEST from this session only. Historical truth belongs to the timestamp; current truth belongs to the code._
