# Comprehensive Status Report — 2026-06-29

**Session:** 23 — Status snapshot (no code changes this session; verification + reporting only)
**Date:** 2026-06-29 15:48 CEST
**Branch:** `master`
**Head:** `7abba09` — Sync vendor documentation and normalize markdown table formatting across project docs
**Working tree:** ✅ **clean** (no uncommitted changes)
**Build:** ✅ `go build ./...` green
**Tests:** ✅ **190 test functions / 300 test runs pass** (`go test ./... -count=1`), 9 packages
**Lint:** ✅ `golangci-lint run ./...` — **0 issues**

> **Honest framing:** This session produced **no code**. It is a verification + reporting pass. The codebase is exactly where session 22 left it (`0f97b62`); only two doc/formatting commits landed since (`6b1376a`, `7abba09`). So this report is less "what we built" and more "what is actually true right now, and where the lies are." There are several lies (stale docs). They are named below.

---

## a) FULLY DONE (✅ Shipped, tested, committed)

### The "Superb SDK" arc (sessions 20 → 21 → 22) — all merged on `master`

| Area                            | Status | Evidence                                                                                                                                     |
| ------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------- |
| **Data-loss stops**             | ✅     | aggregate-ID collision fix, `hasChanged` content-hash, real per-item errors, ctx cancellation, projection version-gate (`e2187ac`→`de0281c`) |
| **Tombstone soft-delete model** | ✅     | live → tombstoned → resurrect lifecycle, opt-in guarded reconciliation (`8c0847f`, `c294ab3`)                                                |
| **Dead-code removal**           | ✅     | deleted the CRDT cluster (`vectorclock`/`operation`/`types`); `pkg/crdt` now only `conflict.go` + `errors.go` (7 tests, 100% coverage)       |
| **Resilient fetch**             | ✅     | retry+backoff+jitter, `IsRetryable`, per-source mutex, Retry-After hook, configurable via `sync.WithRetry(...)` / `sync.WithLogger(...)`     |
| **Conflict surface honesty**    | ✅     | `ConflictResult` now mirrors `SyncResult` (carries `ItemErrors`, `Tombstoned`); `classify()` extracted                                       |
| **Runnable examples**           | ✅     | `ExampleSyncer` (happy path) + `ExampleSyncer_tombstoneResurrect` (lifecycle)                                                                |
| **Scope decision recorded**     | ✅     | ADR-0004 (defer multi-aggregate); ADR-0005 (tombstone-over-delete)                                                                           |
| **Green gate**                  | ✅     | build + 190 tests + 0 lint issues, reproducible offline via committed `vendor/`                                                              |

### Verification done THIS session (re-confirmed, not new work)

- `go build ./...` → exit 0
- `go test ./... -count=1` → all 9 packages `ok`
- `golangci-lint run ./...` → 0 issues
- `go.work` absent (correct — must stay off disk for buildflow)
- Working tree clean
- ADRs 0001–0005 all present and referenced

---

## b) PARTIALLY DONE (🔄)

| Item                    | State | The honest gap                                                                                                                                                                                                                      |
| ----------------------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Tombstone lifecycle** | 🔄    | Works end-to-end, but **no purge/TTL** — tombstoned rows accumulate forever (ADR-0005 "Future").                                                                                                                                    |
| **Test inventory docs** | 🔄    | `TODO_LIST.md` + `FEATURES.md` still claim **224 tests**. Reality: **190 functions / 300 runs**. This is a live doc/reality split-brain (see §d).                                                                                   |
| **Coverage floor**      | 🔄    | `pkg/data/model` 80.5% and `pkg/cqrs` 82.1% sit just under a comfortable 85–90% target. The other 7 packages are 94–100%.                                                                                                           |
| **CHANGELOG**           | 🔄    | **~~Frozen at v0.1.0~~ → fixed — CHANGELOG maintained through v0.5.0.** Does not mention v0.2, v0.3, the tombstone pivot, CRDT removal, or any session-20→22 work. A consumer reading it would believe `VectorClock`/`Operation[T]`/`SyncMessage` still ship — **they were deleted**. |
| **Retry surface**       | 🔄    | `WithRetry` is public, but only `fetchItems` is retried — the store path (`SyncItems`) is not. Intentional, but undocumented.                                                                                                       |
| **Examples**            | 🔄    | Happy-path + tombstone lifecycle covered. **No `ConflictResolver` example** — the third headline capability has no demo.                                                                                                            |

---

## c) NOT STARTED (the untouched session-22 backlog)

Session 22 produced a 25-item Pareto backlog (`docs/status/2026-06-28_10-16_SESSION-22-SELF-REVIEW-EXECUTION.md`, §f). **None of it has been started.** The high-impact items still open:

1. `pkg/sync` → `pkg/synclib` rename (kills the stdlib `sync` collision) — **blocking, breaking, needs consumer coordination**
2. Fix the stale LSP false positives (vendor manifest regen / cache clear)
3. Refresh `TODO_LIST.md` + `FEATURES.md` (the 224→190 lie)
4. `ConflictResolver` example test
5. Bump `pkg/data/model` coverage (tombstone paths)
6. OpenTelemetry spans for `Sync` / `SyncItems` / HTTP
7. Tombstone purge: `PurgeTombstonesBefore(t)`
8. CI `build`/`release` rework for a pure library
9. Make `go-cqrs-lite` public → drop committed `vendor/`
10. Schema upcasters (ADR-0004 carry-over)

Also untouched: HTTP `/items` tombstone fields exposure, `flake.nix` real `vendorHash`, structured logging fields, fuzz `AggregateID` delimiter.

---

## d) TOTALLY FUCKED UP (⚠️)

No code defects. The fucked-up things are **documentation that lies** and **environmental noise**.

| Issue                                              | Severity        | The truth                                                                                                                                                                                                                                                                                                             |
| -------------------------------------------------- | --------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`TODO_LIST.md` / `FEATURES.md` claim 224 tests** | 🔴 Doc lie      | Real: **190 test functions**. The number drifted down because the CRDT cluster deletion removed ~45 tests and nothing updated the headers. A new contributor reads "224" and trusts it.                                                                                                                               |
| **`CHANGELOG.md` frozen at v0.1.0**                | 🔴 Doc lie      | Top entry still advertises `VectorClock`, `Operation[T]`, `SyncMessage`, `crdt.ConflictResolver[T]` with `LWWResolver` — **all removed or restructured**. The tombstone pivot, data-loss fixes, and CRDT deletion are invisible to anyone reading the changelog. This is the single most misleading file in the repo. |
| **`pkg/cqrs` coverage 82.1%**                      | 🟠 Soft spot    | Lowest-coverage package and the most complex one. Resolver-error and store-factory branches under-tested.                                                                                                                                                                                                             |
| **Stale LSP flooding false positives**             | 🟠 Nuisance     | `gopls`/`golangci_lint_ls` keep reporting "inconsistent vendoring" / missing methods. **Confirmed false** by the toolchain. Drowns every session in noise.                                                                                                                                                            |
| **CI `build`/`release` jobs broken**               | 🟠 Pre-existing | `build` cross-compiles deleted `./cmd/examples/github-sync`; `release` depends on it. Only `test`+`lint` pass. The one red signal in CI.                                                                                                                                                                              |
| **nixpkgs Go lag**                                 | 🟡 External     | `go.mod` needs `go 1.26.4`; nixpkgs ships 1.26.3. `nix build`/`nix flake check` fail under `GOTOOLCHAIN=local`. **Do not lower the directive.** Self-resolves when nixpkgs catches up.                                                                                                                                |

---

## e) WHAT WE SHOULD IMPROVE

1. **Stop the doc/reality drift now.** `TODO_LIST.md`, `FEATURES.md`, and especially `CHANGELOG.md` are actively misleading. Fix the test counts and write the missing v0.2/v0.3/tombstone changelog entries. This is low-effort, high-trust.
2. **Close the two coverage soft spots** (`pkg/data/model`, `pkg/cqrs`) — they're the only things under 85%.
3. **Add the `ConflictResolver` example** — the third headline feature has no demo; it's invisible to anyone evaluating the SDK.
4. **Add observability** — zero OTel spans today. `go-cqrs-lite` v3 ships `otel/v3`; wire it.
5. **Decide the `pkg/sync` rename** — the only structural smell; blocking on consumer coordination.
6. **Rework CI** — a library has no binary to cross-compile; the broken `build`/`release` jobs should become a library-appropriate release flow (or be removed).
7. **Make `go-cqrs-lite` public** — removes the committed-`vendor/` + `vendorHash=null` workaround entirely.
8. **Add a tombstone purge policy** once a real storage-cost signal arrives.
9. **Fix or document the LSP noise** so future sessions don't waste time on false positives.

---

## f) Top #25 things to do next (Pareto: impact ÷ effort)

| #  | Task                                                                                                                   | Impact  | Effort          |
| -- | ---------------------------------------------------------------------------------------------------------------------- | ------- | --------------- |
| 1  | **Fix `CHANGELOG.md`** — add v0.2/v0.3 + tombstone pivot + CRDT removal entries (it currently advertises deleted code) | 🔴 High | Low             |
| 2  | **Fix the 224→190 test-count lie** in `TODO_LIST.md` + `FEATURES.md`                                                   | 🔴 High | Low             |
| 3  | **Investigate & silence the stale LSP** (regenerate vendor manifest / clear cache)                                     | 🔴 High | Low             |
| 4  | **Add `ConflictResolver` example test** (third headline feature, no demo)                                              | 🟠 Med  | Low             |
| 5  | **Bump `pkg/data/model` coverage** (tombstone paths → 90%+)                                                            | 🟠 Med  | Low             |
| 6  | **Document retry is fetch-only** (comment on `fetchItems` / store path)                                                | 🟡 Low  | Low             |
| 7  | **Expose tombstone fields on HTTP `/items`**                                                                           | 🟡 Low  | Low             |
| 8  | **`ParseTombstoneReason` edge-case tests** (empty string)                                                              | 🟡 Low  | Low             |
| 9  | **Structured logging fields** (source/page/event_id)                                                                   | 🟡 Low  | Low             |
| 10 | **Bump `pkg/cqrs` coverage** (resolver/error branches → 90%+)                                                          | 🟡 Low  | Med             |
| 11 | **OpenTelemetry: spans for `Sync` + `SyncItems` + HTTP middleware**                                                    | 🟠 Med  | Med             |
| 12 | **Rework CI `build`/`release` jobs** for a pure-library flow                                                           | 🟠 Med  | Med             |
| 13 | **Tombstone purge: `PurgeTombstonesBefore(t)`** on the read model                                                      | 🟠 Med  | Med             |
| 14 | **Make `go-cqrs-lite` public** + drop committed `vendor/`                                                              | 🟠 Med  | Med             |
| 15 | **`flake.nix`: real `vendorHash`** (unblocked by #14)                                                                  | 🟡 Low  | Low             |
| 16 | **Schema upcasters** (ADR-0004 carry-over; `UpcasterRegistry`)                                                         | 🟡 Low  | Med             |
| 17 | **`CompleteFetch` type** as defense-in-depth reconcile guard                                                           | 🟡 Low  | Med             |
| 18 | **Decide `Reconcile` semantics**: best-effort vs fail-loud                                                             | 🟡 Low  | Low             |
| 19 | **Retry the store path** (or document why not)                                                                         | 🟡 Low  | Med             |
| 20 | **Benchmark: projection replay cost at 10k events**                                                                    | 🟡 Low  | Med             |
| 21 | **Fuzz `AggregateID` delimiter encoding**                                                                              | 🟡 Low  | Med             |
| 22 | **`govalid` struct tags** on `SyncOptions`, `CQRSConfig`                                                               | 🟡 Low  | Low             |
| 23 | **Improve `CONTRIBUTING.md`** (architecture guide, conventions)                                                        | 🟡 Low  | Low             |
| 24 | **`SyncOptions.ConflictResolver`** per-sync override                                                                   | 🟡 Low  | Med             |
| 25 | **Decide & coordinate `pkg/sync` → `pkg/synclib` rename**                                                              | 🔴 High | High (breaking) |

> Note: #1, #2, #3 are the **"stop lying"** cluster — lowest effort, highest trust payoff. They should go first, before any new feature work.

---

## g) Top #1 question I can't figure out myself

**"What is the real adoption picture — is `github-local-sync` the only consumer, and does any other consumer actually exist or is realistically planned?"**

I cannot answer this from the code. Everything strategic hinges on it:

- **ADR-0004's scope decision** (stay single-aggregate vs generalise) is only justified if `github-local-sync`-class single-aggregate pull consumers are the audience. If a second real consumer exists with different needs, the deferral deserves re-examination.
- **The entire 25-item backlog ROI** depends on whether this SDK has one user or many. With one consumer, several items (the rename, OTel, upcasters) are easier to justify doing _inside the consumer_; with many, they belong here.
- **The "generic SDK" framing** in README/AGENTS implies multi-consumer intent, but the only evidence is one reference consumer and one rejection (DiscordSync). If github-local-sync is effectively the sole user, the abstraction is extracted-from-one and the framing oversells.

I need to know Lars's actual intent + whether other consumers are real before prioritising architectural work versus just serving the one known consumer excellently.

---

## Snapshot metrics (authoritative, this session)

| Metric                     | Value                                    | Source                                                      |
| -------------------------- | ---------------------------------------- | ----------------------------------------------------------- |
| Test functions             | 190                                      | `grep -rh "^func Test" --include="*_test.go" pkg/ \| wc -l` |
| Test runs (incl. subtests) | 300                                      | `go test ./... -v \| grep -c "^=== RUN"`                    |
| Packages                   | 9 (+ `testutil`, no tests)               | `go test ./...`                                             |
| Lint issues                | 0                                        | `golangci-lint run ./...`                                   |
| Build                      | green                                    | `go build ./...`                                            |
| Working tree               | clean                                    | `git status`                                                |
| Coverage low spots         | `pkg/data/model` 80.5%, `pkg/cqrs` 82.1% | `go test -cover`                                            |
| ADRs                       | 0001–0005                                | `docs/adr/`                                                 |
| `go.work` on disk          | absent (correct)                         | `ls go.work`                                                |

---

## Resolution (2026-07-22)

This session correctly identified doc drift but could not fix it all. Since then:

- **CHANGELOG.md** is no longer frozen at v0.1.0 — it now has `[0.2.0]` through `[0.4.0]` sections.
- **Test count** is **216** (this report said 190; the "224" in docs was also wrong — it counted benchmarks/examples).
- **`VectorClock`/`Operation[T]`/`SyncMessage`** — deleted entirely (CRDT distributed types removed, ADR-0004 scope). `LWWResolver` and `Conflict[T]` remain.
- **`pkg/sync` rename to `pkg/synclib`** — **not done** (accepted tech debt; the stdlib collision is handled with `stdsync` alias).

### 2026-09-05 sweep update

The doc lies named here were fixed: TODO_LIST/FEATURES rebuilt (2026-09-05 passes), CHANGELOG maintained through v0.5.0, CI fully green. Remaining forward items were routed to TODO_LIST.md / ROADMAP.md; stale claims struck inline. Report fully resolved → archived 2026-09-05.

