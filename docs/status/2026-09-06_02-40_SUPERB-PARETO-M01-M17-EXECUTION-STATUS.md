# SUPERB PARETO EXECUTION — M01–M17 session status (2026-09-06 02:40 CEST)

**Session scope:** executed the approved [Pareto plan](../planning/2026-09-06_00-14_SUPERB-PARETO-EXECUTION-PLAN.md) straight through, in its dependency order (M01→M02→M03–M06→M07→M08→M09–M16→M17). **17 of 27 tasks complete.** All quality gates green at session end: build, vet, full race suite (11 packages), internal cqrs-lint strict, golangci-lint 0 issues, `nix flake check` (now incl. hermetic test + lint), doc-count check, vendorHash guard.

**Current state:** master at `3f5bc5e` (local; daemon commits sweep continuously), branch `v0.6-prep` at `60b1cdb` (local only, enactment gated on owner sign-off per ADR-0009). Tests: **331 core / 11 packages (+31 provider/github)**, race-clean.

---

## a) FULLY DONE

| Task                             | What landed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Proof                                                                                                |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| **M01 Resurrect disposition**    | Pinned by-design (upstream truth wins; resolver never arbitrates a resurrection): ADR-0005 addendum, branch comment in `decider.go`, `TestDecideSync_ResurrectTombstonedItem_BypassesResolver` (local-wins resolver recorded 0 invocations, remote applied)                                                                                                                                                                                                                                                                                                                                                                                                                                                 | commit `0bd7354` + daemon sweep                                                                      |
| **M02 Upcaster audit trio**      | Found + fixed a **real residual data race**: legacy-versioned (V1/V2) `ItemSynced` events whose payload already carried `Attributes` were passed through to the registry, which stamped schema version IN PLACE on the stored shared pointer (memory backend serves shared `*ImmutableEvent`s — `slices.Clone` clones the slice, not elements). Fix: always rebuild a private event for legacy-versioned stamps; true-V3 hot path keeps the zero-cost pass-through. WHY-comment on the V1→V2→V3 double-application; chain-semantics pin (fold-once, identity, idempotent re-transform); concurrent replay regression (100 anomalous streams, 4 barrier-start readers, shifted visit orders, live V3 writer) | commit `b92a2c9`; regression **verified to fail old code with 3 DATA RACEs**, 5× `-race` clean after |
| **M03 CI truth cluster**         | New `nix` CI job (`nix flake check` on every push; overrides the flake's SSH `go-nix-helpers` input to anonymous HTTPS; gates build/release); actionlint in devShell (`pkgs.actionlint`) + CI step (pinned `@v1.7.12`); golangci-lint pinned `v2.13.2` = exact devShell version (was `latest`)                                                                                                                                                                                                                                                                                                                                                                                                              | commit `928626d`                                                                                     |
| **M04 Counts-in-CI**             | `scripts/check-doc-counts.sh`: per-package test counts vs AGENTS table, totals across AGENTS/README/FEATURES, dependency table vs `go.mod`; `--coverage` opt-in (±1.0 pt, fresh `go test -cover`); wired into CI `lint` job. **First run caught real drift** (upcaster session's +4 tests: 309→313, pkg/cqrs 144→148) — fixed same session                                                                                                                                                                                                                                                                                                                                                                  | commit `03d6cb3`                                                                                     |
| **M05 vendorHash guard**         | `scripts/check-vendorhash.sh`: go.mod/go.sum moved without flake.nix re-pin → fail with step-by-step re-pin instructions; CI nix job runs it first (PR base sha / push before-sha / `HEAD~1` locally; unresolvable base skips with notice). Proven red→green with a dummy dep touch                                                                                                                                                                                                                                                                                                                                                                                                                         | commit `5b92625`                                                                                     |
| **M06 Pre-release pipeline**     | `scripts/verify-release.sh <core-tag> [provider-tag]`: tags local+origin+ancestry, GitHub Release via `gh`, proxy `@v/list` + `@latest` for BOTH modules, pkg.go.dev indexing (warn-only). CONTRIBUTING.md release checklist. `nix flake check` upgraded to one-command full suite (`enableTestCheck` + `lintAsCheck` join build/format/cqrs-lint). **Dry-run green against live v0.5.0 + v0.1.0**                                                                                                                                                                                                                                                                                                          | commit `4d4224a`                                                                                     |
| **M07 v0.6 decision package**    | ADR-0009 addendum: Go surface aligns to `SourceID` in v0.6 (type + fields, deprecated aliases) while persisted payloads (`json:"sourceId"`) stay untouched — no schema V4/upcast; `GetStats`→`Stats` joins the window; enactment explicitly gated on recorded owner sign-off. Naming-review HTML finding links to the ADR                                                                                                                                                                                                                                                                                                                                                                                   | commit `67f6919`                                                                                     |
| **M09 CLI process tests**        | `cmd/cqrs-lint/process_test.go`: builds the real binary into `t.TempDir()`, pins exit 0 clean / 1 findings / 2 usage, `--strict` on the unknown-rule warning, NDJSON shape (positioned code-level findings, package-level without file/line)                                                                                                                                                                                                                                                                                                                                                                                                                                                                | commit `0a708cb`                                                                                     |
| **M10 Real-meter + sdktrace**    | OTel tests move beyond noop: `ManualReader` asserts `cqrs.operation.count{operation=projection}` increments w/ `status=success`, accumulates, records `dead_lettered` (DLQ path); `tracetest.SpanRecorder` proves `localsync.sync_items` span recorded, internal kind                                                                                                                                                                                                                                                                                                                                                                                                                                       | commit `3d69000`                                                                                     |
| **M11 Cursor vs real SQLite**    | Root-cause fix: `ORDER BY created_at DESC` alone leaves ties unordered → OFFSET paging (the cursor's backing) can duplicate/skip rows on a query-plan change. Now `ORDER BY created_at DESC, item_id ASC` (total order). Pinned by real-file-DB cursor walk: 12 items, deliberate 3-row tie group straddling pages, attribute filter, set-equality + ordering assertions                                                                                                                                                                                                                                                                                                                                    | commit `a546fc5`                                                                                     |
| **M12 ContentHash tests + move** | Constructor/round-trip, zero-value, sha256-hex construction path, literal-compat contract; type moved to `pkg/id/content_hash.go`. pkg/id coverage **75.0% → 100.0%**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | commit `6de5d53`                                                                                     |
| **M13 Benchmark protocol**       | `docs/benchmarks.md` (fixed `-benchtime 20x -count 5`, benchstat, environment caveats, per-bench table); `scripts/run-benchmarks.sh`; **Replay10k fixed to true from-zero replay** (persisted checkpoint wiped per iteration — old version measured stack open/close, replaying nothing); new `BenchmarkConflict_SyncExisting` + `BenchmarkUpcastedLegacyRead` (measured **~3.3× upcast tax**, ~26 ms/200-item conflict batch)                                                                                                                                                                                                                                                                              | commits `149498d`, `fd4e539`                                                                         |
| **M14 Wire Config.Validate**     | `NewCQRSStack` validates before any factory dispatch — bad configs fail fast at the boundary with the classified `ErrUnknownBackend` chain, zero resources allocated                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        | commit `d8b3c96`                                                                                     |
| **M15 Attribute consolidation**  | Adapter's private `legacy*` key constants deleted; `upcastLegacyAttributes` writes `model.Attr*` directly; round-trip test proves adapter output reads back through typed accessors                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | commit `8eb65b2`                                                                                     |
| **M16 buildflow full run**       | **61 success, 0 failed** — 3-session deferral streak ended. Root cause of the streak was environmental: nix-hash-fix/nix-build evaluate EVERY flake-check system (incl. aarch64-darwin — unbuildable locally). `.buildflow.yml` skips exactly those; `nix flake check` stays the gate. Result recorded in the 23:58 report follow-ups + TODO                                                                                                                                                                                                                                                                                                                                                                | commit `b023e0f`                                                                                     |
| **M17 CLI phase 1**              | `--version`, `--quiet` (exit-code-only), `--format=github` (`::error`/`::warning` file/line annotations), `--json` via `encoding/json` + explicitly tagged schema struct, per-rule suppressed counts in `--verbose` (stale-directive radar). 5 new process tests; AGENTS flag docs updated                                                                                                                                                                                                                                                                                                                                                                                                                  | commit `71ee8e9`                                                                                     |

## b) PARTIALLY DONE

1. **M08 v0.6 enactment prep** — all 8 micro-tasks done on branch `v0.6-prep` (`StreamID()` w/ error return + deprecated panic-signature alias, internal callers migrated, `SyncSummary`→alias of `SyncResult`, migration tests, CHANGELOG migration skeleton, full suite + race green). **Deliberately incomplete by design:** enactment awaits owner sign-off; `id.ExternalID`→`id.SourceID` and `GetStats`→`Stats` are recorded but NOT enacted; branch is local-only (unpushed).
2. **cmd/cqrs-lint coverage** — the plan's M09 wanted it RAISED from 56.4%; it now reads **43.3%** because phase-1 grew the CLI surface while its best tests (process-level) are coverage-invisible by design. Documented honestly in the AGENTS table, but the metric regressed and the underlying question (instrumented-subprocess coverage vs accept-and-document) is open.
3. **CI runtime validation** — the new nix job, doc-count step, actionlint step, and vendorHash guard are locally validated (actionlint clean, scripts proven red/green) but have **never executed on a real GitHub runner**. First push exercises them; runtime surprises (Nix install time, override-input behavior under the CI token, `github.event.before` on new branches) are unverified.
4. **Daemon/commit hygiene** — several logical changes landed split across my commits and adjacent `chore: auto-commit` commits (e.g. M02's code in daemon commits, my commit got only TODO_LIST). History is understandable but not the clean per-logical-change story the plan's guards called for.
5. **library cqrs-lint CI gate** — remains error-gated behind the missing `SSH_PRIVATE_KEY` secret (owner action; documented, linked from CI comment).

## c) NOT STARTED (all fully specified in the plan)

- **M18** CLI phase 2: `--rules`, `--exclude-rules`, `--no-suppress`, `--explain`, block-comment + range directives, directive test matrix
- **M19** CLI phase 3: SARIF output, `docs/cqrs-lint.md` rules page, new rules C0011–C0015
- **M20** API hardening: `X-RateLimit-*` headers, per-client limiter option, `/metrics` posture decision (needs owner input)
- **M21** Logging quieting docs + OTel span for `Syncer.Sync`
- **M22** provider/github: verify kit README claims in source, ETag/conditional requests, `PerPage` exposure
- **M23** AGENTS.md restructure to <30KB (link-out to ADRs, ≤20 gotchas)
- **M24** Docs-policy cluster: HTML banner/archive execution, dprint scope for `docs/status/`, 2 undated planning files, stale `.golangci.yml` exclusions, gopls stdversion noise, ROADMAP export-row, 23-04 annotation prep
- **M25** Long-tail A: `SyncOptions.Validate` MaxPages<0, typed Attributes write-helpers, `ParseTombstoneReason` in DTOs, `b.Loop()` migration (4 funcs), `waitForCount` unify
- **M26** Long-tail B: `TombstoneItem` options parity, huma 408 verify, AggregateID→StreamID vocabulary sweep in ADRs, `errors.AsType` audit, hierarchical-errors disposition
- **M27** Long-tail C: 100-point re-audit, pre-commit hooks enable-or-delete, library-gate suppression audit, windows leg eval, provider CHANGELOG, pre-release VERIFY wiring, DLQ inspect/replay surface

## d) TOTALLY FUCKED UP (honest failures this session)

1. **The race regression test was twice wrong before it was right.** First version: single legacy stream, readers-after-writer — tripped **0** DATA RACEs against old code. Second: still 0 — I had forgotten to set `Attributes` on the seeded events, so old code took the always-safe rebuild path and my "regression test" would have caught nothing. Only the third version (100 anomalous streams + barrier start + shifted visit orders, verified against old code: 3 DATA RACEs) earns its name. Lesson applied and now stated plainly: a regression test that has never been seen to fail is a hypothesis, not a test.
2. **verify-release.sh shipped wrong on first dry-run** — I assumed both modules share the release tag despite AGENTS.md documenting the provider's independent versioning. The dry-run failed exactly where it should; fixed to per-module tags. Should have read the constraint I already documented.
3. **check-doc-counts.sh took four debug iterations**: package-boundary off-by-one in the awk (each count attributed to the previous package), wrong number extracted for provider counts (first-number vs last), silent exit-1 in coverage mode (`set -e` + unguarded grep, missing `%` strip), and a wrong-column read. Each caught by actually running it — but the first draft wasn't run against a scratch fixture before wiring.
4. **The first buildflow run failed and my initial config guess was wrong** (`nix.systems` / `steps.*.enabled` — keys the schema doesn't have). I burned a full pipeline run misreading "platform mismatch" before buildflow's own warning pointed at `skip_steps`. Ended green (61/0), but the path there was trial-and-error, not reading-first.
5. **upcast_bench_test.go first draft was garbage** — invented helpers (`event2`, `toEvents`, a nonexistent `pkg/crdt/localsync` import), deleted and rewritten wholesale. Writing-before-reading the module's actual exports.
6. **Stray garbage shipped then removed:** a `StreamID == nil` assertion (vet nilfunc), dead `dir`/`dir2` fixtures in the verbose test, two ineffective nolint directives, a stray tracked `cqrs-lint`-adjacent binary risk (none — but `.buildflow.yml` briefly contained invalid keys). All caught by gates, all fixed; still churn that reading-first avoids.
7. **Coverage metric went the wrong way** (56.4% → 43.3% for cmd/cqrs-lint) and I recorded it only after the phase-1 commit. A plan item said "raise it"; I moved it down and explained why — explaining a regression is not the same as not causing it.
8. **"CI green" claims are locally-proven only.** Every new CI-side claim in this session (nix job, guards, doc-count step) is validated by local equivalents + actionlint, NOT by an observed Actions run. Nothing is known to be broken; nothing is proven to work in situ either.
9. **Daemon-vs-logical commits:** repeatedly `git add -A`'d after the daemon had already swept files, so several M-task commits contain only the residue (e.g. `b92a2c9` = TODO_LIST only; code went in `e756740`/`554649b`). The history reads fine but the plan's "one logical change per commit" was often satisfied by luck, not discipline.

## e) WHAT WE SHOULD IMPROVE

1. **Verify-the-test-fails as a hard rule.** Every regression test gets demonstrated failing against the old code (this caught the race-test fantasy before it shipped).
2. **Read the project's own documentation before designing against its constraints** — the provider-versioning mistake was pre-documented in the very file I was editing.
3. **Run a new script against a scratch fixture before wiring it into CI** (would have caught three of the four check-doc-counts bugs before the "wire it" step).
4. **Treat `set -euo pipefail` + grep pipelines as a silent-exit hazard** — guard every `grep … | head` chain, or the script fails with zero output exactly when someone needs its message.
5. **State unverified claims as unverified.** "CI will be loud" ≠ "CI was loud." Local proof and runtime proof are different tiers.
6. **Coverage honesty:** subprocess-based tests are coverage-invisible; either add an instrumented-subprocess mode for the CLI or stop quoting the number as a quality signal.
7. **Commit before the daemon does** — sweep windows (~minutes) beat my task cadence; commit each logical unit immediately after its gate.
8. **buildflow upstream:** the cross-system nix evaluation is a tool defect for single-builder machines — file the issue (with the skip_steps workaround) upstream.
9. **Add benchstat to the devShell** (`scripts/run-benchmarks.sh` compare mode exits with instructions when it's missing).
10. **Race tests in CI should loop** (`-count=3` on the concurrency regression) — detector trips are probabilistic; one pass is not proof of absence.
11. **go.mod/go.sum edits need an immediate vendorHash check** — today's `go mod tidy` (otel sdk indirect→direct) passed the flake by luck of no content change; make `check-vendorhash.sh` part of the post-tidy reflex.

## f) NEXT 50 (ordered; plan remainder first, then session follow-ups)

**CLI (M18–M19) — M09's harness makes these safe:**

1. `--rules` (run subset) + test
2. `--exclude-rules` + test
3. `--no-suppress` (error on any directive) + CI-hardening test
4. `--explain <ruleID>` + test
5. Block-comment directives (`/* cqrs-lint:ignore … */`)
6. Range directives (`ignore-start`/`ignore-end`) + nesting guard
7. Directive test matrix (valid/invalid/unknown/nested)
8. SARIF output (`--format=sarif`) + schema sanity test
9. `docs/cqrs-lint.md` rules page (all rules + suppression examples)
10. New rules C0011–C0015 (one per micro-task: design, implement, test)

**API + sync surface (M20–M21):**
11. `X-RateLimit-Limit`/`-Remaining` headers on `POST /sync` + tests
12. `WithRateLimiter(keyExtractor)` per-client option + tests
13. Global-vs-per-client limiter docs (options.go + README)
14. `/metrics` posture: implement the owner's decision (keyed vs public)
15. Record the `/metrics` decision ADR-style
16. Document log quieting (middleware logger level) in README/AGENTS
17. Plumb logger level control if missing
18. OTel span for `Syncer.Sync` (`localsync.sync`) + attribute test

**provider/github (M22):**
19. Verify kit claim: empty-token behavior in go-github-kit source
20. Verify kit claim: 429/idempotent-5xx retry; annotate README if wrong
21. ETag spike: do GitHub events endpoints support conditional requests?
22. ETag implementation or documented infeasibility
23. `PerPage` option in `FetchAll` fetch options + tests
24. `provider/github` live-PAT smoke test env documentation pass

**Docs (M23–M24):**
25. AGENTS restructure outline (<30KB, invariants vs links)
26. Extract CQRS internals → ADR/go-cqrs-lite doc links
27. Prune/merge gotchas to ≤20
28. Link-first architecture table
29. Size check + link sweep + lost-context review
30. Execute HTML policy decision (banners and/or archive moves)
31. Record dprint scope for `docs/status/`
32. Classify the 2 undated planning files
33. Purge 3 stale `.golangci.yml` exclusion paths
34. Document gopls stdversion warnings as known GOEXPERIMENT noise
35. Strike shipped "Export to JSON/CSV" row in ROADMAP
36. Prep the 23-04 report for annotation

**Long-tail code (M25–M26):**
37. `SyncOptions.Validate` reject `MaxPages < 0`
38. Typed `Attributes` write-helpers (`WithActorLogin/RepoName/…`)
39. Surface `ParseTombstoneReason` in API DTOs
40. `b.Loop()` migration: `adapter_bench_test.go` + `stack_bench_test.go`
41. Unify `waitForCount`/`waitForCountTB` behind `testing.TB`
42. `TombstoneItem(...event.Option)` parity + test
43. Verify huma 408 mapping vs `pkgerrors.HTTPStatus`
44. ADR/doc vocabulary sweep: `AggregateID`→`StreamID` prose
45. `errors.AsType` audit + safe migrations
46. hierarchical-errors findings: suppress-with-rationale or track

**Closing the loop (M27 + session follow-ups):**
47. 100-point deep-dive re-audit; record delta vs `docs/research/`
48. Pre-commit hooks: enable-scoped or delete; library-gate suppression audit; windows-leg eval
49. Push/observe the next CI run end-to-end (nix job, guards, doc-counts) and file the buildflow cross-system issue upstream; decide `v0.6-prep` branch lifetime (push for review vs keep local)
50. Final v0.6 gap list: DLQ inspect/replay surface, `provider/github` CHANGELOG seeding, standing pre-release VERIFY step wired into the checklist

## g) QUESTIONS FOR YOU (cannot answer these myself)

1. **v0.6 sign-off + branch visibility:** Do you approve the ADR-0009 package as recorded (StreamID rename + error return, SyncResult fold, ExternalID→SourceID surface alignment, GetStats→Stats) so I can finish and enact the `v0.6-prep` branch — and do you want that branch pushed to origin now, or kept local until the window opens?
2. **`/metrics` posture (blocks M20.4):** Should the metrics endpoint stay keyed behind `WithAPIKey`, or become a public path alongside `/health` (standard Prometheus practice, but exposes cardinality/latency shape to anonymous callers)? I will not decide this for you.
3. **CI proof + darwin builders:** May I push master (or a throwaway branch) purely to exercise the new CI jobs end-to-end — and does any darwin build host exist (remote builder or Mac) that would ever make the skipped aarch64-darwin nix checks runnable, or is `skip_steps` their permanent state?

---

**Waiting for instructions.**
