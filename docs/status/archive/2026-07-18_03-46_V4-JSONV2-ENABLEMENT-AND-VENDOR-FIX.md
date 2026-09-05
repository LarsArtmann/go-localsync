# Status Report — 2026-07-18 03:46 CEST

**Session scope:** Fix `buildflow` failure (was 33/42, 4 failed steps + 3 blocking findings) after commit `1bd8ea9` bumped go-cqrs-lite to v4.

**Final state:** `buildflow` passes **35/35** in 51.9s inside the fixed devShell.

---

## a) FULLY DONE ✅

### Root cause diagnosed

The 4 failed steps (`test-race`, `go-fix`, `govalid-generate`, `go-auto-upgrade`) all traced to one root cause: go-cqrs-lite **v4** uses `encoding/json/v2`, which Go 1.26 gates behind the `goexperiment.jsonv2` build tag (experimental until ~Go 1.27). The v4 bump (`1bd8ea9`) shipped **without enabling it**. The sibling go-cqrs-lite repo enables it in 3 places (flake devShell `GOFLAGS`, `buildGoModule.preBuild`, `.golangci.yml build-tags`); go-localsync had it only in `.golangci.yml`. Decision: **enable jsonv2** (the v4 bump was deliberate — CBOR `TimeUnixDynamic` nanosecond fix + `event.Instant`/`WallTime`), not revert to v3.

### Code changes applied & verified

| File                               | Change                                                                                                                                                                                      | Verified                                                                                                        |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `flake.nix`                        | Added `goTags`/`tagFlags` let-binding; `GOFLAGS = tagFlags` on both devShells; `preBuild = 'export GOEXPERIMENT=jsonv2'` on both `buildGoModule` packages; `${tagFlags}` in the `test` app  | ✅ `nix build .#default` exit 0; `nix build .#cqrs-lint` exit 0; devShell `GOFLAGS` confirmed via `nix develop` |
| `.golangci.yml`                    | Added `cqrs.ItemSyncedPayload` to the exhaustruct exclusion regex (line 219)                                                                                                                | ✅ `golangci-lint run` → 0 issues                                                                               |
| `pkg/sync/sync.go:502`             | `lockAndValidate` named returns `(release func(), err error)` → `(func(), error)`                                                                                                           | ✅ builds, lint clean                                                                                           |
| `pkg/cqrs/item_adapter.go:120`     | Removed redundant `//nolint:exhaustruct` (now config-excluded)                                                                                                                              | ✅ `nolintlint` clean                                                                                           |
| `vendor/github.com/larsartmann/**` | `git add -f` of **297 untracked** files (go-cqrs-lite v4 + go-branded-id)                                                                                                                   | ✅ nix sandbox build now finds them                                                                             |
| `go.mod` / `go.sum`                | `go mod tidy` removed unused `eventtest`/`schema/v4` entries (tidy now works on v4 — the v3 nested-eventtest blocker is gone)                                                               | ✅ tidy exit 0                                                                                                  |
| `AGENTS.md`                        | v4 dep versions in table; new `GOEXPERIMENT=jsonv2` gotcha; vendor force-tracking warning; resolved nixpkgs-Go-lag gotcha (nixpkgs `go_1_26` now 1.26.4); `/v3` → `/v4` current-module refs | ✅ no stale v3 current-version refs remain                                                                      |

### Gates passed (final run)

- `go build -tags=goexperiment.jsonv2 ./...` ✅
- `go test -tags=goexperiment.jsonv2 ./... -count=1` ✅ (all packages ok)
- `golangci-lint run ./... --timeout=5m` → **0 issues** ✅
- `nix build .#default` ✅
- `nix build .#cqrs-lint` ✅
- **Full `buildflow` inside `nix develop`: 35/35 passed in 51.9s** ✅

### Secondary discovery & fix

During verification, a **second failure** surfaced: `nix build .#default` failed with `cannot find module providing package ... import lookup disabled by -mod=vendor`. Root cause: `.gitignore` line 69 is `vendor/`; the other (public) vendor deps were already force-tracked, but the **`larsartmann/*` deps were not**. Nix flakes only include git-tracked files in the flake source, so the untracked vendor sources were silently dropped from the sandbox. Fixed by `git add -f vendor/github.com/larsartmann/`. This was masked in the original failing run by `nix-hash-fix:fix-vendor-inconsistency` papering over it; standalone `nix build` exposed it.

---

## b) PARTIALLY DONE 🟡

### `hierarchical-errors` tool — dismissed without full audit

The tool reported **3708 findings** of "Function returns generic error interface instead of specific error type." I classified these as Go-idiomatic noise (returning `error` IS the Go convention; concrete-typed errors are an anti-pattern) and confirmed the tool is **non-gating** (its `:detect` step shows ✔ in buildflow output). **However**, the tool's full description is "panics, swallowed errors, bare error returns" — three sub-checks. I only reasoned about the "bare error returns" subset. I did **not** spot-check whether any of the 3708 findings are _real_ defects (an actually-swallowed error, or a panic) hiding among the noise. The buildflow step passes regardless, so this is a quality gap, not a blocking one.

### AGENTS.md "go.work" gotcha — not re-validated this session

The gotcha says go.work must not be on disk during buildflow. I confirmed `go.work` does not exist at session start, but I did not re-test the go.work-detection behavior end-to-end. The gotcha text is unchanged and presumed still accurate.

---

## c) NOT STARTED ⬜

### Workflow steps from AGENTS.md I did NOT run this session

- **Step 5**: `golangci-lint fmt ./...` (formatter pass) — I ran `golangci-lint run` (the linter) but not `fmt` (the formatter). My edits may have formatting drift.
- **Step 6**: `go run ./cmd/cqrs-lint -pkg pkg/crs` (the CQRS architectural gate) — I verified `nix build .#cqrs-lint` succeeds and the binary runs, but did **not** invoke it against `pkg/cqrs` as the dev workflow specifies. The buildflow run includes a `cqrs-lint` check via nix, so it likely passed — but I did not confirm this explicitly.
- `go vet` — not run.
- `gofumpt -l pkg/` and `goimports -l pkg/` — not run on my edited files (`pkg/sync/sync.go`, `pkg/cqrs/item_adapter.go`).
- Pre-commit hook behavior — not tested. AGENTS.md documents a known OOM on vendor/; with 297 newly-tracked vendor files, the hook may be _more_ prone to OOM. Unknown.

### Commit

**Nothing was committed.** The user did not ask for a commit. All changes are staged/unstaged in the working tree, ready for review.

---

## d) TOTALLY FUCKED UP 💥

### Honest self-critique — what I got wrong or did suboptimally

1. **Introduced a regression mid-session, then fixed it.** My first flake.nix edit passed its targeted tests, but running `go mod vendor` (which I triggered for a different reason) combined with the pre-existing `.gitignore vendor/` gap broke `nix build .#default`. I caught this only because I ran the _full_ buildflow rather than stopping at "test-race passes." If I had stopped early, I would have shipped a broken nix build. Lesson: the vendor force-tracking gap was a **latent bug** that my work exposed; it should have been part of the original v4 bump commit.

2. **Investigative artifact left behind, then cleaned up.** I generated `.buildflow.yml` via `buildflow config init` purely to inspect the schema (looking for a way to disable the noisy hierarchical-errors tool). I then deleted it. This was correct, but I should have inspected the schema via `--help`/docs first rather than materializing a config file that changed effective defaults (`max_concurrency: 32 → 4`).

3. **Dismissed 3708 findings on reasoning alone.** "They're Go-idiomatic" is plausible but unverified. A 5-minute spot-check of the highest-severity subset (filter for `critical` and look for `panic`/swallowed-error patterns) would have converted guess into knowledge.

4. **Did not run the project's own documented workflow end-to-end.** AGENTS.md spells out steps 1–7. I ran buildflow (which covers most of it) but skipped `golangci-lint fmt` and the native `cqrs-lint` invocation. "Buildflow passed" ≇ "the documented workflow passed."

5. **flake.nix `lint` app not updated.** I added `${tagFlags}` to the `test` app but not the `lint` app. Rationale: golangci-lint reads tags from `.golangci.yml build-tags`, not `GOFLAGS`, so it doesn't need the flag. This is correct, but the asymmetry looks inconsistent and a future reader may "fix" it by adding the flag redundantly. Could be documented or made symmetric for clarity.

6. **LSP diagnostics went stale and I didn't force-refresh.** After my edits, the LSP continued showing 2 already-fixed warnings. I verified via actual `golangci-lint run` (source of truth), so no harm — but I could have called `lsp_restart` to keep the IDE view honest for the user.

---

## e) WHAT WE SHOULD IMPROVE 📈

### Process / repo hygiene

1. **The v4 bump commit (`1bd8ea9`) was incomplete.** It bumped versions without enabling `GOEXPERIMENT=jsonv2` or force-tracking the new vendor files. The CI/buildflow should have caught this at commit time. **Recommendation:** the pre-commit hook (or a CI gate) should run `nix build .#default` and a single `go build -tags=goexperiment.jsonv2 ./...` before allowing dep bumps. The current pre-commit hook is skipped (not executable per AGENTS.md) — this is the real root cause of the regression shipping.
2. **`.gitignore vendor/` + force-tracking is a footgun.** The pattern relies on every contributor remembering to `git add -f` after every `go mod vendor`. A repo-level check (buildflow step or pre-commit) that verifies `vendor/modules.txt` entries have corresponding tracked files would prevent silent nix sandbox breakage.
3. **No `.buildflow.yml` in this repo.** The sibling go-cqrs-lite has one (tunes `max_concurrency: 4`, `todo_min_severity`, excludes). Go-localsync runs on defaults. Worth adding to make buildflow behavior explicit and reproducible.
4. **AGENTS.md "go mod tidy fails on v3" gotcha is now obsolete** — I removed it and noted tidy works on v4. Good, but the _nested-eventtest module_ itself still exists in the v4 source; it's just no longer pulled into go-localsync's graph. If a future dep change re-introduces it, the old failure mode returns. The real fix lives upstream (go-cqrs-lite should publish `eventtest` as a proper module or move it out of `event/`).

### Technical debt surfaced

5. **`vendorHash = null`** in flake.nix remains (the documented workaround for the private go-cqrs-lite repo). This forces the entire `vendor/` dir to be committed (~3000+ files). The clean fix (make go-cqrs-lite public, use a real `vendorHash`) is still the long-term recommendation. The 297 newly-tracked files increase the cost of every vendor regeneration.
6. **Legacy `ActorLogin`/`RepoName`/`RepoURL` fields on `ItemSyncedPayload`** (ADR-0007) — kept for event upcasting, now config-suppressed rather than refactored. The upcasting logic in `item_adapter.go:98-111` is the real home of this complexity; the fields could eventually be removed once all historical V1/V2 events are aged out of event stores.
7. **`GOEXPERIMENT=jsonv2` is experimental.** It will graduate in Go 1.27+ (per sibling's AGENTS.md). When it does, the `preBuild` exports and `GOFLAGS` can be dropped — but not before. Tracking upstream Go release notes is now a maintenance task.

---

## f) Up to 50 things we should get done next

### Immediate (this session's loose ends)

1. **Commit the changes** — 7 files modified + 297 vendor files staged. Suggest two commits: (a) `feat: enable GOEXPERIMENT=jsonv2 for go-cqrs-lite v4` (flake.nix, .golangci.yml, AGENTS.md), (b) `fix: force-track larsartmann vendor deps for nix flake source` (vendor/**, go.mod, go.sum).
2. Run `golangci-lint fmt ./...` and fix any formatting drift on edited files.
3. Run `gofumpt -l pkg/ && goimports -l pkg/` and fix.
4. Run `go run ./cmd/cqrs-lint -pkg pkg/cqrs` natively to confirm the architectural gate.
5. Run `go vet ./...` with the jsonv2 tag.
6. Spot-check the 3708 `hierarchical-errors` findings: filter for severity=critical, grep for `panic(` and swallowed-error patterns, confirm there are no real defects.
7. Restart the LSP (`lsp_restart`) to clear stale diagnostics.

### Near-term (quality of life)

8. Add a `.buildflow.yml` to go-localsync mirroring the sibling's tuning (concurrency, excludes, todo severity).
9. Make the `flake.nix` `lint` app's tag-handling consistent with `test` (or document why it differs).
10. Add a buildflow step or pre-commit check that verifies every `vendor/modules.txt` package path has ≥1 git-tracked file (catches the silent-nix-breakage class of bug).
11. Make the pre-commit hook executable / functional (AGENTS.md says it's currently skipped) OR formally remove it.
12. Add `nix build .#default --no-link` as a required CI gate (it caught the vendor-tracking bug that buildflow's `nix-hash-fix` was masking).
13. Add `GOEXPERIMENT=jsonv2` awareness to the `crush-config` / devShell onboarding so new sessions inherit it automatically (already done via flake, but verify `nix develop` is the documented entrypoint).

### Documentation

14. Update `docs/adr/` with a new ADR: "ADR-0008: GOEXPERIMENT=jsonv2 adoption for go-cqrs-lite v4" — records _why_ the experiment is enabled, when it can be removed, and the three wiring points.
15. Update `docs/adr/0007*` (the de-GitHubify ADR) to note that the legacy payload fields are now config-suppressed rather than removed.
16. Add a `docs/runbooks/dependency-bump.md` checklist: (1) bump go.mod, (2) `GOWORK=off go mod vendor`, (3) `git add -f vendor/github.com/larsartmann/`, (4) `nix build .#default`, (5) `nix build .#cqrs-lint`, (6) `buildflow` inside `nix develop`, (7) commit.
17. Update `TODO_LIST.md` if it references the v3→v4 migration or the jsonv2 enablement as pending.
18. Update `FEATURES.md` if it lists go-cqrs-lite version prominently.
19. Refresh `CHANGELOG.md` with the v4 + jsonv2 enablement entry.

### Upstream / strategic

20. Proposal to go-cqrs-lite: publish `eventtest` as a top-level module (or move out of `event/`) to kill the nested-module fragility permanently.
21. Proposal to go-cqrs-lite: add a consumer-facing "enable jsonv2" doc section pointing at the 3 wiring points (this session re-derived them from the sibling's flake).
22. Track Go 1.27 release: when `encoding/json/v2` graduates, open a cleanup PR dropping all `goexperiment.jsonv2` tags + `preBuild` exports across both repos.
23. Re-evaluate making go-cqrs-lite public — would eliminate the `vendorHash = null` workaround and the 3000+ committed vendor files entirely.
24. Consider a `go.work` CI mode that _does_ run sibling tests (currently avoided) — now that v4 is stable, sibling test breakage may be less noisy.

### Testing / robustness

25. Add a regression test that asserts `encoding/json/v2` is importable (compile-time guard) so a future toolchain/experiment change fails loudly.
26. Add a CI matrix entry that builds _without_ `GOEXPERIMENT=jsonv2` to confirm it fails fast (documents the hard dependency).
27. Run `govulncheck` with the jsonv2 tag (AGENTS.md mentions govulncheck in CI; confirm it inherits the tag).
28. Run `gitleaks` on the 297 newly-tracked vendor files (defense in depth — they're third-party but should be scanned once).
29. Benchmark: does jsonv2 change event serialization performance materially? (The v4 bump cited CBOR time-precision fix, not perf — but worth measuring.)

### Cleanup

30. Remove the `query/v4` indirect dep if truly unused (AGENTS.md says QueryDispatcher was removed; query/v4 is transitive).
31. Audit other `// indirect` go-cqrs-lite deps (`dedup`, `idempotency`, `metadata`, `scheduling`, `testutil`) — confirm they're genuinely transitive and not dead weight.
32. The `flake.nix` `checks.test` overrides `packages.default` with `doCheck = true` — verify this actually runs tests with the jsonv2 tag in-sandbox (the `preBuild` export should cover it, but unverified).
33. Confirm `treefmt` still excludes `vendor/**` (it does per flake.nix line 114-116) — the 297 new files should not trigger treefmt.
34. Run `nix flake check` standalone (buildflow ran it as a sub-step; a direct run confirms no other-system warnings).
35. Verify the `formatter` flake output still works (`nix fmt` / `treefmt`) after adding 297 vendor files.

36-50. _(Stretch / lower-priority — listed for completeness)_ 36. Add a `make-vendor` or `revendor` nix app that automates `GOWORK=off go mod vendor && git add -f vendor/github.com/larsartmann/`. 37. Promote the `flake.nix` `let goTags` binding to also drive a `vet` app and a `coverage` app (sibling has both; go-localsync only has test/lint/cqrs-lint). 38. Consider adding `pkgs.govulncheck` and `pkgs.gitleaks` to the devShell (sibling has them; go-localsync's devShell is thinner). 39. Document the `GOTOOLCHAIN=local` nix-sandbox behavior in AGENTS.md (referenced in the resolved Go-lag gotcha but not explained). 40. Add a `//go:build goexperiment.jsonv2` compile guard file to fail fast in IDEs that don't read GOFLAGS. 41. Audit whether `internal/cqrslint` needs the jsonv2 tag (it parses `pkg/cqrs` ASTs — probably not, but confirm). 42. Check if `cmd/cqrs-lint` builds without the tag (it has no json/v2 imports directly; it may not need it — if so, the flake `preBuild` on cqrs-lint is unnecessary). 43. Review the `hierarchical-errors` tool's source (if accessible) to understand its 3 sub-checks precisely. 44. File a buildflow feature request: per-tool disable/enable in `.buildflow.yml` (to formally quiet hierarchical-errors if desired). 45. Consider migrating `GOFLAGS = tagFlags` to `GOEXPERIMENT = "jsonv2"` in the devShell env (cleaner intent) — verify mkShell passes it through. 46. Add a `direnv` `.envrc` with `use flake` so the jsonv2 flag is active outside explicit `nix develop` shells. 47. Confirm the CI workflow (GitHub Actions, if present) sets `GOEXPERIMENT=jsonv2` or `GOFLAGS=-tags=goexperiment.jsonv2` — CI may not use the devShell. 48. Update the CI section of AGENTS.md to mention the jsonv2 requirement (currently CI docs don't reference it). 49. Audit the 297 force-tracked vendor files for any that shouldn't be committed (testdata, fixtures, large binaries). 50. Schedule a follow-up to revisit this entire jsonv2 setup when Go 1.27 ships (calendar reminder / issue).

---

## g) Questions I CANNOT figure out myself (max 3)

1. **Commit strategy?** Should I commit as (a) one combined commit, (b) two commits (jsonv2 enablement / vendor force-tracking), or (c) leave it for you to commit? The v4 bump itself is already committed (`1bd8ea9`); my work is the "make it actually build" follow-up. Your call on granularity and message phrasing.

2. **Is making `go-cqrs-lite` public still the plan, or is the committed-`vendor/` workaround now the permanent design?** This determines whether the 297 newly-tracked vendor files are a band-aid (to be removed when public + real `vendorHash`) or the forever-state. Affects whether I should invest in the vendor-consistency buildflow step (#10) or just wait for the public flip.

3. **Is the `hierarchical-errors` tool's 3708 "generic error return" findings something you actually want addressed** (e.g., by adopting concrete-typed errors project-wide), **or is it formally accepted noise?** The sibling go-cqrs-lite likely has the same tool and the same finding volume — if you've already decided it's idiomatic-and-ignored there, I'll mark it permanently dismissed here too. If not, it's a large refactor I should scope separately.

---

## Session metrics

- **Steps in todos:** 8 (all completed)
- **Files modified (non-vendor):** 7 (`flake.nix`, `.golangci.yml`, `go.mod`, `go.sum`, `pkg/sync/sync.go`, `pkg/cqrs/item_adapter.go`, `AGENTS.md`)
- **Files newly tracked (vendor):** 297
- **Tool calls:** ~25
- **buildflow before:** 33/42, 4 failed + 3 findings
- **buildflow after:** 35/35, 0 failed, 51.9s
- **Commits made:** ~~0 (awaiting instruction)~~ **Shipped in `v0.4.0` (2026-07-18).** All changes committed; jsonv2 build tag wired in flake.nix devShells, `.golangci.yml`, and `buildGoModule` preBuild.
---

## Resolution (2026-09-05 docs-health sweep)

All vendor-era guidance is moot (vendor/ removed; real vendorHash + go-standard flake module in v0.5.0, re-pinned 2026-09-05); the jsonv2 enablement it shipped is now wired in devShell + CI + lint config. Track-Go-1.27 stays a watch item. Verified against the 2026-09-05 tree (`9625b1b`: v0.5.0, 309 core tests, CI green). Report fully resolved → archived.
