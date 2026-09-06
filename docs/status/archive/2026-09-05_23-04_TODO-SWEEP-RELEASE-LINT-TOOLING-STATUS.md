# Status Report — TODO_LIST Execution Sweep: Release Verification, Lint/CI Tooling, devShell

**Generated:** 2026-09-05 23:04 CEST
**Session scope:** Execute + verify the open TODO_LIST items (release integrity, library cqrs-lint CI leg, `exhaustruct_v5` migration, provider README verification, dprint devShell) plus anything discovered on the way.
**Format note:** Status-report skill default is a styled HTML dashboard; the user explicitly requested `.md` — honored (one-off override, not propagated).
**Commit note:** The auto-commit daemon committed and **pushed** all session work during the session (`HEAD == origin/master` at `9625b1b`, tree clean). No manual commits were made per house rules.

---

## Headline

All actionable TODO items from the pasted list were executed or deliberately dispositioned. **CI is green on the pushed changes** — run `33991823805` (23:01 CEST) exercised both the `exhaustruct_v5` config and the new secret-gated library cqrs-lint step end-to-end: the gate **skipped correctly** (no `SSH_PRIVATE_KEY` yet) and the notice step ran. One **pre-existing hidden breakage** was found and fixed: `nix build` / `nix flake check` had been silently red since the 21:53 dependency refresh (stale `vendorHash`).

**Verification gauntlet (all green):** `go build` · `go test ./...` (11 pkgs) · internal `cqrs-lint --strict` · library `cqrs-lint v4.8.1` (1 known suppression, clean) · `golangci-lint run ./...` (0 issues, no deprecation warning) · `dprint check` · provider module standalone (`GOWORK=off`) · `nix flake check` · CI run `33991823805` (success, Lint job steps verified individually).

---

## a) FULLY DONE

| # | Item                                                                    | Evidence                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| - | ----------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **Release integrity verified (post-public-flip)** — 🔴 TODO item closed | `git ls-remote --tags origin`: annotated `v0.5.0` + `provider/github/v0.1.0` pushed; `gh release list`: both Releases exist (provider is Latest); proxy serves `@v/list` and correct `@latest` for core (`v0.5.0`) and provider (`v0.1.0`). **No bump release needed.**                                                                                                                                                                                                                                   |
| 2 | **`exhaustruct` → `exhaustruct_v5` migration** (`.golangci.yml`)        | All 6 references migrated (enable list, `settings.exhaustruct.exclude` → `settings.exhaustruct_v5.ignore-patterns`, 4 exclusion rules). Schema confirmed against golangci-lint v2.13.2 source (`ExhaustructV5Settings` via Sourcegraph). `config verify` OK; full run → `0 issues`, deprecation warning gone. Confirmed `gomodguard`/`wsl` already on `_v2`/`_v5` names — no other deprecated linters in use.                                                                                             |
| 3 | **dprint added to devShell** (`flake.nix`)                              | `nix develop -c dprint --version` → 0.56.1; `dprint check` green after formatting the provider README table (realigned columns).                                                                                                                                                                                                                                                                                                                                                                          |
| 4 | **`provider/github/README.md` verified against the FetchPages rebuild** | Every claim cross-checked in code: FetchPages delegation, page-1 sequential probe, pool default 3, short-page stop, `MinRemaining` 10 / `MaxWait` 15m, retries 3 with 1s→30s backoff, error mapping incl. `ErrForbidden`→`ErrRateLimited`, original error preserved via `errors.Join`, PAT smoke test (`GITHUB_PAT`, `-run TestLivePAT` prefix-matches `TestLivePAT_Smoke`, torvalds, 1 page). Two precision fixes applied (page-1 probe named; `WithBaseURL` returns `(client, error)` — not chainable). |
| 5 | **Stale `vendorHash` found & re-pinned** (pre-existing breakage)        | `nix flake check` failed with hash mismatch; root-caused: `go.mod`/`go.sum` changed 21:53 (`e9a9565`) after the hash was pinned 16:22 (`858108f`). Re-pinned to the reported hash → `nix flake check`: "all checks passed".                                                                                                                                                                                                                                                                               |
| 6 | **Docs sync**                                                           | `TODO_LIST.md` pruned (4 done items removed; CI item rewritten to the remaining owner action); `CHANGELOG.md` [Unreleased] +6 entries; `AGENTS.md`: cqrs-lint gate #2 rewritten (secret-gated), exhaustruct gotcha updated to v2.13/`ignore-patterns` semantics, public-repo bullet corrected, **new gotcha**: `vendorHash` drifts on dependency refreshes.                                                                                                                                               |
| 7 | **CI gate logic live-verified (skip path)**                             | Run `33991823805` Lint job: `exhaustruct_v5` golangci step ✅, internal cqrs-lint ✅, library step **skipped** (secret absent — correct), notice step ✅, job success. Also fixed a latent bug from the old step: git-config cleanup ran last with `\|\| true`, masking linter failures — now captures `rc` and exits with it.                                                                                                                                                                            |

## b) PARTIALLY DONE

| # | Item                                     | Done                                                                                                                                                 | Missing                                                                                                                                                                                                                                            |
| - | ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **Library cqrs-lint CI leg**             | Step restored, secret-presence auto-gating (`HAS_GO_FINDING_KEY` job env), skip notice, exit-code fix — all pushed and CI-verified on the skip path. | The **run path** cannot execute until the owner adds the `SSH_PRIVATE_KEY` deploy-key secret — `larsartmann/go-finding` verified still **private** (`gh repo view`). Goal "gate runs in CI" remains unrealized until then.                         |
| 2 | **dprint "format check parity with CI"** | Tool installed, green locally, formatting applied.                                                                                                   | The TODO's **premise doesn't match reality**: `ci.yml` has no formatting job at all (only golangci formatters for Go). There is nothing to be "at parity" with for json/yaml/md/dockerfile. I should have challenged the premise before executing. |
| 3 | **README external-claims verification**  | All claims about _our_ code verified against source.                                                                                                 | Kit-side claims trusted, not source-verified: "empty token = unauthenticated (60 req/h)" and "retry on 429 and idempotent 5xx" were not checked in `go-github-kit v0.3.0` source.                                                                  |

## c) NOT STARTED

| # | Item                                                                                                                              | Why                                                                                                                                     |
| - | --------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **v0.6 vocabulary window** (ADR-0009): `AggregateID()`→`StreamID()`, `SyncResult`/`SyncSummary` consolidation, panic→error return | Deliberately **awaiting the breaking release** ("Why not now" is explicit in ADR-0009). ADR presence re-verified; untouched.            |
| 2 | **govalid struct tags**                                                                                                           | Closed/pivoted 2026-09-05; correctly left struck-through.                                                                               |
| 3 | **`buildflow --build-mode full`** and local **race suite**                                                                        | Skipped: changes were config/docs-only; the individual gauntlet legs were run instead. Should be run once to bless the session (see f). |
| 4 | **pkg.go.dev indexing check**                                                                                                     | Proxy verified; pkg.go.dev indexer not (can lag proxy).                                                                                 |
| 5 | **CONTRIBUTING.md staleness check**                                                                                               | Not opened. Likely still describes the library gate as "local-only" — now stale after the workflow change.                              |
| 6 | **AGENTS dependency-version table re-verification**                                                                               | Last night's go.mod refresh (`e9a9565`) may have moved versions (event v4.9.0 etc.); table not re-checked.                              |
| 7 | **HARVEST of this report's backlog into TODO_LIST/ROADMAP**                                                                       | Awaiting instructions (per status-report skill, section f is HARVEST input).                                                            |

## d) TOTALLY FUCKED UP

Nothing I shipped is broken — but honesty per category:

1. **`nix build` / `nix flake check` were silently red for ~an hour+ of commits** (stale `vendorHash` from the auto-committed dependency refresh). Nobody noticed because **CI has no nix leg** — the vendor workaround removal traded one blind spot for another. Fixed this session; the _systemic_ gap (no CI nix check, no drift guard) remains open.
2. **The CI library-gate TODO's actual goal is still unmet** — after my change, CI _contains_ the gate but cannot _run_ it (secret missing). By design, but it means two sessions in a row have ended with this item not truly done.
3. **My process stumbles (minor, own):** filesystem-wide `find` that had to be killed; a failed `multiedit` (non-unique substring) costing a round trip; edited the README before `dprint fmt` (extra pass); executed the dprint task before questioning its "CI parity" premise.
4. **11 pre-existing gopls `stdversion` warnings** (`json.Marshal*` requires go1.27, go.mod says 1.26) — unaddressed, undocumented as known noise anywhere. LSP-only noise (builds pass with `GOEXPERIMENT=jsonv2`), but every session pays attention-cost for it.

## e) WHAT WE SHOULD IMPROVE

1. **Verify external claims at source before marking docs "verified"** (kit-side README claims today were trusted, not read).
2. **Challenge task premises before executing** — the dprint "parity" premise was discovered false mid-task, not before it.
3. **CI must watch what we build with** — add a `nix flake check` leg; today's vendorHash drift was invisible to CI.
4. **Pin golangci-lint in CI** (`version: latest` today) — config migrations like `exhaustruct_v5` land the hard way when the runner's version floats.
5. **Use `actionlint`** (devShell + CI) instead of `python yaml.safe_load` for workflow validation.
6. **Automate release verification** — today's tag/proxy/`@latest` checks were manual `gh`/fetch one-offs; script them into a checklist.
7. **Guard `vendorHash` drift** — any go.mod/go.sum auto-commit should warn that flake.nix needs re-pinning (the new AGENTS gotcha documents it; automation would prevent it).
8. **Document LSP-vs-build divergence** (jsonv2 stdversion warnings) so future sessions don't re-debug it.
9. **Keep running the verification gauntlet even for "boring" changes** — it caught the vendorHash breakage.

## f) NEXT — up to 50 things to get done (impact-ordered; HARVEST fuel)

**P0 — unblock CI truth / release trust**

1. Owner: add `SSH_PRIVATE_KEY` secret (deploy key, read access to `go-finding`) → watch the library gate's first green run.
2. Verify the run path of the gate end-to-end once the secret exists (skip path already proven).
3. Check **pkg.go.dev** indexing for both modules (proxy already verified).
4. Inspect GitHub Release **bodies** for `v0.5.0` / `provider/github v0.1.0` (existence checked, content not).
5. Update **CONTRIBUTING.md** gate documentation (likely still "local-only").
6. Re-verify **AGENTS dependency-version table** against post-refresh go.mod.
7. Run `buildflow --build-mode full` once to bless the session's changes.
8. Local `go test -race ./...` (+ provider leg) to re-confirm race-clean post-refresh.
9. Add **`nix flake check` CI leg** — would have caught the vendorHash drift.
10. **Pin golangci-lint version** in `golangci-lint-action` (drop `version: latest`).
11. Add **actionlint** to devShell + a CI workflow-validation step.

**P1 — docs & config debt**
12. Decide the **CI formatting story**: add a dprint check job (json/yaml/md/dockerfile) or drop the "parity" claim.
13. Purge **stale `.golangci.yml` exclusion paths** (`pkg/providers/github/client.go`, `pkg/types/ids.go`, `pkg/testhelpers/` — verify existence, delete dead rules).
14. Verify kit-side README claims in `go-github-kit` source (empty-token behavior; 429/idempotent-5xx retry); annotate if wrong.
15. Document gopls stdversion warnings as known GOEXPERIMENT noise (or act on the go directive).
16. Run **gitleaks + govulncheck** locally (security-leg parity, post-changes).
17. Script the **release-verification checklist** (tags, releases, proxy list, `@latest`) — codify this session's manual checks.
18. **vendorHash drift guard** — hook/CI warning when go.mod/go.sum change without flake update.
19. Confirm the `::notice::` skip message renders readably in the Actions UI (logic verified, cosmetics not).
20. Audit the single **library-gate suppression** still necessary (`//cqrs-lint:ignore` reasons current?).
21. Check **FEATURES.md** tooling/CI rows reflect the new state (secret-gated gate, dprint in devShell).
22. HARVEST this list into `TODO_LIST.md` (actionable) + `ROADMAP.md` (ideas) — docs-health HARVEST mode.
23. Grep docs for other stale "LOCAL-ONLY gate" mentions (exclude historical status snapshots).

**P2 — v0.6 preparation (decided, ADR-0009)**
24. Branch + enact `AggregateID()` → `StreamID()` with deprecated alias.
25. v0.6: `SyncResult`/`SyncSummary` consolidation (ADR-0009 shape).
26. v0.6: `AggregateID` panic fallback → error return.
27. v0.6: CHANGELOG migration-section skeleton.
28. v0.6: release checklist (tag → proxy → pkg.go.dev → release body) executed on the real release.

**P3 — quality / coverage / hygiene**
29. Raise `cmd/cqrs-lint` coverage (56.4% — lowest in repo).
30. Raise `pkg/data/model` coverage (84.9% — lowest package).
31. Re-run benchmark suite; refresh AGENTS numbers post-refresh.
32. Sweep `errors.As` → `errors.AsType` / run erraudit (go-error-modernization pass).
33. Revisit inert pre-commit hooks: formally enable (scoped) or delete.
34. `provider/github`: consider exposing `PerPage` instead of hardcoded 100 in `FetchAll`'s `newFetchOptions`.
35. Spot-check `retryAfterer` (Retry-After) hook still wired post provider extraction.
36. Owner decision: **make `go-finding` public** → delete all SSH machinery permanently.
37. Dead-link sweep across README/CONTRIBUTING/docs (docs-health VERIFY).
38. Evaluate a **windows build leg** (matrix is linux/darwin; sqlite/CGO on windows unproven).
39. Review `.golangci.yml` `run.go: 1.26.7` pin cadence (tie to toolchain bumps).
40. PR-template/dependabot reminder: dependency bumps must re-pin `vendorHash`.
41. Consider `go-github-kit` version check (v0.3.0 still latest?) at next provider touch.
42. Root README sales-page refresh once v0.6 lands.
43. Confirm the `provider/github` standalone nix check idea (GOWORK=off leg as a flake check).
44. Consolidate TODO_LIST strikethrough/govalid notes into ROADMAP "reopen-only" entries.
45. Next session start: `git status` + `gh run list` ritual — verify daemon commits pushed and CI stayed green overnight.

_(45 items — 5 slots intentionally left empty; items 36 and 1 are owner-gated.)_

## g) Questions I cannot figure out myself

1. **`SSH_PRIVATE_KEY` / go-finding:** Will you add the deploy-key secret to activate the CI gate — or is making `go-finding` public (or shipping a go-finding-free cqrs-lint build) the intended endgame? The gate's design depends on which future you want.
2. **v0.6 timing:** Should I start the ADR-0009 breaking work on a branch now, or does "awaiting the breaking release" mean you want to call the window first (I won't know when that is)?
3. **CI formatting:** Does your cross-repo convention include a dprint CI job that this repo should adopt (making "parity" real), or should dprint stay devShell-only and the TODO's parity clause be dropped?

## Resolution (2026-09-06 docs-policy sweep)

Every §f forward item closed or routed — archive criterion met. Bucket verdicts:

- **Shipped (2026-09-06, v0.6 enactment session):** 3, 5, 6, 7 (buildflow 61/0), 9, 10, 11, 12, 13, 17, 18, 20, 21, 22, 24, 25, 26, 27, 31, 32, 33, 38, 44. Items 8 (`go test -race`) and 28 (checklist on the real release) are standing gates: race re-runs each session, 28 executes at the v0.6.0 tag.
- **Routed to TODO_LIST:** 1–2 (owner-gated `SSH_PRIVATE_KEY` / go-finding-public decision), 14 (kit-side claims verify — open), 15 (gopls stdversion doc — open, planned for the AGENTS gotcha pass), 29–30 (coverage floors — open), 36 (owner: make go-finding public — folded into item 1's alternative), 42 (README refresh — migration section shipped 2026-09-06; final sales-page polish at tag time).
- **Moot / closed by events:** 4 (Release bodies → `verify-release.sh` now checks Releases+proxy+pkg.go.dev in one command), 16 (gitleaks+govulncheck run in CI's security leg; local parity optional), 19 (skip-path notice verified in CI run 33991823805), 23 (vocabulary sweep executed 2026-09-06 — living docs clean, snapshots untouched), 39 (go-directive cadence documented in AGENTS gotcha), 40 (superseded by `check-vendorhash.sh` failing CI instead of reminding PRs), 43 (superseded: CI `provider` job already builds the module standalone with `GOWORK=off`, which is the same isolation a flake check would add), 45 (process note, adopted as session ritual).
- **Small unowned ideas (never committed, kept for the record):** 34 (`PerPage` option on `FetchAll`), 35 (`retryAfterer` spot-check — verified wired post-extraction during the v0.6 sweep), 37 (dead-link sweep — covered opportunistically by docs-health VERIFY runs; not scheduled).

§g question 3 (CI formatting) was answered 2026-09-06: dprint check is now a CI lint-job step (pinned 0.56.1). §g question 2 was answered by the owner's "do the whole TODO list" directive — the v0.6 window opened and was enacted the same day. §g question 1 remains owner-gated (see TODO_LIST).

---

_Point-in-time snapshot — generated 2026-09-05 23:04 CEST from this session only. Historical truth belongs to the timestamp; current truth belongs to the code._
