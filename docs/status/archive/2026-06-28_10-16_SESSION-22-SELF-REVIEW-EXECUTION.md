# Comprehensive Status Report — 2026-06-28

**Session:** "Superb SDK — Session 22: Self-Review Plan Execution"
**Date:** 2026-06-28 10:16 CEST
**Branch:** `master` (pushed to `origin`)
**Head:** `0f97b62` — Add a tombstone→resurrect example test
**Test status:** ✅ **193 tests pass** (`go test ./... -count=1`), 9 packages
**Build status:** ✅ `go build ./...` green
**Lint status:** ✅ `golangci-lint run ./...` — **0 issues**

---

## a) FULLY DONE (✅ Shipped, tested, committed, pushed)

### This session's work — executing the brutal-self-review plan (5 commits + the review report)

Every issue found in the [brutal self-review](../reviews/2026-06-28_09-58_brutal-self-review.html)
was investigated, fixed, tested, and committed — no item was hand-waved.

| # | Task                                 | Commit    | Detail                                                                                                                                                                                                                                                                                                                                  |
| - | ------------------------------------ | --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | **Reconcile data-loss guard**        | `c294ab3` | `reconcile()` now refuses to tombstone when `FetchResult.HasMore == true`. A partial (still-paginating) fetch with `Reconcile=true` would previously have silently tombstoned every not-yet-fetched item. Added `HasMore` field to `testutil.MockProvider` + a regression test. **This was the #1 finding — a real data-loss footgun.** |
| 2 | **Configurable retry**               | `ca0844c` | Added functional-options constructor `sync.New(p, store, sync.WithRetry(cfg), sync.WithLogger(l))`. `NewSyncer` is a thin backwards-compatible wrapper (no SA1019 deprecation noise). Consumers can now tune backoff per deployment. `TestNew_WithRetry` proves injection lands.                                                        |
| 3 | **lockSource documented**            | `56ccab6` | Investigated the "leak": it is a **bounded per-source cache** (source set = provider/user IDs), NOT a leak. Refcount cleanup was rejected because it would re-introduce the exact TOCTOU race the lock prevents. Comment now states this tradeoff so a future reader doesn't "fix" it into a race.                                      |
| 4 | **ConflictResult split-brain fixed** | `3159d60` | Added `ItemErrors []ItemSyncResult` + `Tombstoned int` to `ConflictResult` so its surface mirrors `SyncResult`. The conflict path previously dropped per-item error detail and never ran reconciliation — now both are wired. Extracted `classify()` helper to stay under funlen. Added `TestConflictAwareSyncer_RetainsItemErrors`.    |
| 5 | **Tombstone→resurrect example**      | `0f97b62` | Added `ExampleSyncer_tombstoneResurrect` demonstrating the full soft-delete lifecycle: sync (live) → `TombstoneItem` (hidden) → sync again (resurrected). The headline feature now has a runnable demo, not just an ADR.                                                                                                                |
| — | **Brutal self-review report**        | `9308843` | HTML report at `docs/reviews/2026-06-28_09-58_brutal-self-review.html`: 4 real issues, 0 ghost systems, 2 split-brain smells, prioritized plan.                                                                                                                                                                                         |

### Cumulatively DONE across sessions 20 → 21 → 22 (the whole "Superb SDK" arc)

| Phase                     | What                                                                                                                 | Commits              |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------- | -------------------- |
| **P1** Stop-the-bleeding  | aggregate-ID collision, `hasChanged` content-hash, real error propagation, ctx cancellation, projection version-gate | `e2187ac`→`de0281c`  |
| **P2** Tombstone pivot    | soft-delete model, event rename, read-model tombstoning, opt-in reconciliation                                       | `8c0847f`            |
| **P3** Dead-code removal  | deleted vectorclock/operation/types (CRDT cluster), simplified `Conflict[T]`                                         | `8c0847f`            |
| **P4** Resilience         | retry+backoff+jitter, `IsRetryable`, per-source mutex, Retry-After hook, lock-free internals                         | `8c0847f`            |
| **P5** Docs honesty       | README/AGENTS reframed as single-writer pull mirror; ADR-0003 revised, ADR-0005 (tombstone) added; `example_test.go` | `6e87c0f`, `0f97b62` |
| **P6** Self-review + plan | brutal self-review + 5-step execution (this session)                                                                 | `9308843`→`0f97b62`  |

**0 ghost systems found** — verified `Store()`, `Stats`, `SyncSummary`, `ActionUnchanged`/`ActionError`, `pkg/testutil` are all live.

---

## b) PARTIALLY DONE (🔄 In progress or incomplete)

| Item                        | State | Why it's partial                                                                                                                                                                                    |
| --------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tombstone lifecycle**     | 🔄    | Tombstone + resurrect + guarded reconcile all work, but there is **no purge/TTL** — tombstoned rows accumulate forever. Documented as "future" in ADR-0005.                                         |
| **Example coverage**        | 🔄    | `ExampleSyncer` (happy path) + `ExampleSyncer_tombstoneResurrect` (lifecycle). No example yet for **plugging in a `ConflictResolver`** — the third headline capability.                             |
| **Retry surface**           | 🔄    | `WithRetry` is now public, but the per-item `SyncItems` store path is **not retried** — only `fetchItems` is. This is intentional (store errors aren't transient in the same way) but undocumented. |
| **Test inventory accuracy** | 🔄    | `TODO_LIST.md` header still says "**224 tests**" and was last updated 2026-06-27 — it's stale (actual: 193). `FEATURES.md` is thin.                                                                 |
| **Coverage floor**          | 🔄    | Two packages sit just under the 80% target conventionally: `pkg/data/model` (80.5%) and `pkg/cqrs` (82.1%). The rest are 94–100%.                                                                   |

---

## c) NOT STARTED

| Item                                  | Notes                                                                                                                                                                                                                                      |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Tombstone purge/TTL job**           | Needs a real storage-cost signal from `github-local-sync` first (ADR-0005 "Future").                                                                                                                                                       |
| **`pkg/sync` → `pkg/synclib` rename** | Kills the stdlib `sync` collision footgun. **Breaking** for `github-local-sync`; deferred pending consumer coordination (see Top #1 question).                                                                                             |
| **OpenTelemetry instrumentation**     | `go-cqrs-lite` v3 ships an `otel/v3` module; no spans exist in go-localsync today. Tracked in TODO_LIST.md.                                                                                                                                |
| **Schema upcasters**                  | Carried over from ADR-0004's deferred data-module work.                                                                                                                                                                                    |
| **CI build/release rework**           | The `.github/workflows/ci.yml` `build` job cross-compiles `./cmd/examples/github-sync` which was removed. ~~Only failing piece of CI~~ → fixed — CI reworked in v0.4.0; fully green today (`test` + `lint` pass). Tracked in TODO_LIST.md. |
| **Make `go-cqrs-lite` public**        | The one private dep; its privacy forces committed `vendor/` + `vendorHash = null`. Making it public drops the workaround.                                                                                                                  |

---

## d) TOTALLY FUCKED UP (⚠️ — none are code defects; all are environmental/external)

| Issue                                  | Severity        | Detail                                                                                                                                                                                                                                                                                                                              |
| -------------------------------------- | --------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Stale LSP flooding false positives** | 🔴 Nuisance     | `gopls` + `golangci_lint_ls` continuously report "inconsistent vendoring" and "mockSyncStore missing Reconcile". **Confirmed false** — `vendor/modules.txt` is at v3.1.0, `go build`/`go test`/`golangci-lint run` all green. `lsp_restart` fails. Verify via the Go toolchain, NEVER the LSP. This wastes real time every session. |
| **nixpkgs Go lag**                     | 🟠 External     | `go.mod` requires `go 1.26.4`; nixpkgs unstable ships `go_1_26` 1.26.3. `nix build` / `nix flake check` fail in the sandbox (`GOTOOLCHAIN=local`). Native gate is green. **Do not lower the directive** (deliberate bump). Self-resolves when nixpkgs catches up.                                                                   |
| **CI build/release jobs broken**       | 🟠 Pre-existing | `.github/workflows/ci.yml` `build` job references deleted `./cmd/examples/github-sync`. The `release` job depends on it. `test` + `lint` pass. Needs a library-appropriate rework.                                                                                                                                                  |
| **Reconcile footgun (now guarded)**    | ✅ Was a defect | This WAS a data-loss footgun — a partial fetch with `Reconcile=true` would tombstone live items. **Fixed this session** (`c294ab3`). Listed here for honesty about what existed.                                                                                                                                                    |

---

## e) WHAT WE SHOULD IMPROVE

1. **Fix the stale LSP** — it drowns every session in false "missing Reconcile" / "inconsistent vendoring" errors. Likely needs a `vendor/modules.txt` regeneration or LSP cache clear. Investigate so future sessions aren't noise-drowning.
2. **Refresh `TODO_LIST.md` + `FEATURES.md`** — both are stale (224 tests claimed; real is 193). Use the `todo-list-builder` / `features-audit` skills.
3. **Close the coverage soft spots** — `pkg/data/model` (80.5%) tombstone paths, `pkg/cqrs` (82.1%) resolver/error branches.
4. **Add a `ConflictResolver` example** — the third headline capability has no runnable demo.
5. **Add observability** — OTel spans for `Sync`, `SyncItems`, HTTP middleware. No production debuggability today.
6. **Document that retry is fetch-only** — the store path isn't retried; make this explicit.
7. **Resolve the `pkg/sync` naming collision** — the only structural smell left (see #1 question).
8. **Rework CI** — the broken `build`/`release` jobs are the only red signal in the pipeline.
9. **Make `go-cqrs-lite` public** — eliminates the vendored-private-dep workaround entirely.
10. **Add a tombstone purge policy** — `PurgeTombstonesBefore(t)` + optional TTL, once storage cost is real.

---

## f) Top #25 things to do next (Pareto-ordered: impact ÷ effort)

| #  | Task                                                                                       | Impact  | Effort          |
| -- | ------------------------------------------------------------------------------------------ | ------- | --------------- |
| 1  | **Decide & coordinate `pkg/sync` → `pkg/synclib` rename** (blocking architecture question) | 🔴 High | High (breaking) |
| 2  | **Fix the stale LSP** (regenerate vendor manifest / clear cache)                           | 🔴 High | Low             |
| 3  | **Refresh `TODO_LIST.md` + `FEATURES.md`** (use the skills)                                | 🟠 Med  | Low             |
| 4  | **Add `ConflictResolver` example test**                                                    | 🟠 Med  | Low             |
| 5  | **Bump `pkg/data/model` coverage** (tombstone paths → 90%+)                                | 🟠 Med  | Low             |
| 6  | **Document retry is fetch-only** (doc comment on `fetchItems`)                             | 🟡 Low  | Low             |
| 7  | **OpenTelemetry: spans for `Sync` + `SyncItems`**                                          | 🟠 Med  | Med             |
| 8  | **Tombstone purge: `PurgeTombstonesBefore(t)`** on the read model                          | 🟠 Med  | Med             |
| 9  | **Rework CI `build`/`release` jobs** for a pure-library flow                               | 🟠 Med  | Med             |
| 10 | **Make `go-cqrs-lite` public** + drop committed `vendor/`                                  | 🟠 Med  | Med             |
| 11 | **Schema upcasters** (ADR-0004 carry-over)                                                 | 🟡 Low  | Med             |
| 12 | **`CompleteFetch` type** as an alternative reconcile guard (defense in depth)              | 🟡 Low  | Med             |
| 13 | **Bump `pkg/cqrs` coverage** (resolver/error branches → 90%+)                              | 🟡 Low  | Med             |
| 14 | **HTTP `/items` response: expose tombstone fields**                                        | 🟡 Low  | Low             |
| 15 | **`flake.nix`: real `vendorHash`** once go-cqrs-lite is public                             | 🟡 Low  | Low             |
| 16 | **CHANGELOG entry** for the tombstone pivot + self-review fixes                            | 🟡 Low  | Low             |
| 17 | **Retry the store path** (or document why not)                                             | 🟡 Low  | Med             |
| 18 | **Benchmark: projection replay cost at 10k events**                                        | 🟡 Low  | Med             |
| 19 | **Fuzz `AggregateID` delimiter encoding**                                                  | 🟡 Low  | Med             |
| 20 | **Structured logging consistency** (source/page/event_id fields)                           | 🟡 Low  | Low             |
| 21 | **`ParseTombstoneReason` edge-case tests** (empty string)                                  | 🟡 Low  | Low             |
| 22 | **Decide `Reconcile` semantics**: best-effort vs fail-loud                                 | 🟡 Low  | Low             |
| 23 | **Cut v0.4.0** once purge + OTel land                                                      | 🟡 Low  | Low             |
| 24 | **`CONTRIBUTING.md`**: add tombstone/reconcile guidance                                    | 🟡 Low  | Low             |
| 25 | **`gosec` + `govulncheck`** wired into CI                                                  | 🟡 Low  | Low             |

---

## g) #1 Question I can NOT figure out myself

### Should we rename `pkg/sync` → `pkg/synclib` now, and break the one consumer?

`pkg/sync` is named `sync`, which collides with the Go stdlib `sync` package.
Every file in it must alias the standard library (`stdsync "sync"`). I hit this
footgun myself this session, and it is a self-inflicted wound baked into the
import path — every external consumer hits it too.

**The fix is mechanical** (rename package, update imports), but it is
**breaking** for the sole real consumer:
[`github-local-sync`](https://github.com/larsartmann/github-local-sync), which
imports `pkg/sync` in many places.

I cannot resolve this alone because it depends on **information I don't have**:

1. **Can you update `github-local-sync` in lockstep?** If yes, the rename is
   cheap and high-value. If the consumer is deployed/frozen, the rename lands
   as a breaking change with no co-evolution path.
2. **Are there other consumers** beyond `github-local-sync`? The SDK docs claim
   it's the reference consumer, but I can't see who else depends on the import
   path `github.com/larsartmann/go-localsync/pkg/sync`.

**My recommendation if you say "go":** rename to `pkg/synclib`, cut it as
**v0.4.0** (we're pre-1.0, breaks are expected), and update `github-local-sync`
in the same PR cycle. If you say "not yet", I'll leave it documented and move
to OTel + purge instead.

---

## Verification at report time

```
go build ./...              ✅
go test ./... -count=1      ✅ 193 tests / 9 packages
golangci-lint run ./...     ✅ 0 issues
git status                  ✅ clean (all pushed to origin/master)
```

### Test inventory (verified this report)

| Package           | Tests   | Coverage     |
| ----------------- | ------- | ------------ |
| `pkg/cqrs`        | 95      | 82.1%        |
| `pkg/sync`        | 31      | 85.6%        |
| `pkg/crdt`        | 8       | 100.0%       |
| `pkg/id`          | 12      | 100.0%       |
| `pkg/errors`      | 9       | 100.0%       |
| `pkg/provider`    | 10      | 96.7%        |
| `pkg/api`         | 14      | 94.0%        |
| `pkg/data/model`  | 10      | 80.5%        |
| `pkg/data/schema` | 4       | 100.0%       |
| **Total**         | **193** | **~91% avg** |

---

## Resolution (2026-09-05 docs-health sweep)

All forward-looking items in this report are closed as of 2026-09-05 (verified against the tree at `9625b1b`: go-localsync v0.5.0, 309 core tests / 11 packages, CI green, both cqrs-lint gates clean).

- **Shipped since:** The HasMore guard, WithRetry, and ConflictResult parity shipped; CI was reworked in v0.4.0 and is fully green; the synclib rename was rejected 2026-07-22; go-cqrs-lite went public 2026-09-05.
- **Superseded/moot:** anything tied to the Turso backend, committed `vendor/`, go-cqrs-lite v2/v3 WIP, or the pre-de-githubify domain model — all removed or reshaped by ADR-0005/0006/0007 and the go-cqrs-lite v4 migration.
- **Routed:** ideas that still matter live in [TODO_LIST.md](../../TODO_LIST.md) or [ROADMAP.md](../../ROADMAP.md); deliberately deferred work is recorded in the ADRs.
- **Policy:** bucket closure per this directory's [README](README.md); the worst now-false claims are struck inline above.

_Report fully resolved → archived 2026-09-05._
