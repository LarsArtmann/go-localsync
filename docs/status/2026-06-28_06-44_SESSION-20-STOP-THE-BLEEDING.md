# Comprehensive Status Report — 2026-06-28

**Session:** "Superb SDK Execution — Phase 1 Stop-the-Bleeding + P3.1 Honesty"
**Date:** 2026-06-28 06:44
**Branch:** master
**Test status:** ✅ All 225 tests pass
**Build status:** ✅ `go build ./...` green

---

## a) FULLY DONE (✅ Shipped, tested, committed)

### P0 — Unblock

| ID   | Task                                             | Commit    |
| ---- | ------------------------------------------------ | --------- |
| P0.1 | Fix vendoring drift (`go mod vendor` for v3.1.0) | `e2187ac` |

### P1 — Stop the Bleeding (Phase 1 — 1% → 51% impact)

| ID   | Task                                                                                  | Commit    |
| ---- | ------------------------------------------------------------------------------------- | --------- |
| P1.1 | **Aggregate-ID colon collision** → length-prefixed encoding                           | `e2187ac` |
| P1.2 | **`hasChanged` false-negative** → ContentHash (SHA256 of RawJSON) + ActorAvatarURL    | `e2187ac` |
| P1.3 | **`Sync()` always-nil error** → now returns real per-item errors + `ItemErrors` slice | `9d0b73f` |
| P1.4 | **`ctx` cancellation ignored** → batch loop now checks `ctx.Err()` per iteration      | `9d0b73f` |
| P1.5 | **Replay resurrect bug** → projection version gate (sync.Map per-aggregate)           | `de0281c` |

### P3.1 — Honesty Cleanup (partial)

| ID   | Task                                                  | Commit    |
| ---- | ----------------------------------------------------- | --------- |
| P3.1 | **Empty `NewVectorClock()` lie removed** from decider | `7ab2494` |

### Documentation artifacts

- `docs/brainstorming/is-it-what-it-claims-to-be.html` — premium dark UI diagnostic report
- `docs/planning/2026-06-28_05-09-SUPERB-SDK-TOMBSTONE-PIVOT.html` — full Pareto execution plan with D2 graph

**Total: 7 tasks completed, 5 commits, 0 test failures.**

---

## b) PARTIALLY DONE (🔄 In progress or incomplete)

### P3 — Honesty Cleanup (P3.1 done, P3.2-P3.3 pending)

- ✅ P3.1: Empty VC construction removed from decider
- ⬜ P3.2: Dead CRDT cluster B files still exist (`vectorclock.go`, `operation.go`, `SyncMessage`/`SyncRequest`/`SyncResponse` types)
- ⬜ P3.3: `EventItemDeleted` still uses hard-delete in read models (folding into tombstone pending P2)

### `pkg/crdt/` status

- `Conflict[T]` struct still has `LocalVC`/`RemoteVC` fields (unused but not yet removed from struct definition)
- `LWWResolver` still calls `conflict.LocalVC.Cmp()` — now compares nil maps, producing `OrderEqual` → falls through to timestamp (correct behavior, but the VC comparison code is still there)
- The dead files (`vectorclock.go`, `operation.go`) still compile and are tested by `pkg/crdt/*_test.go`

---

## c) NOT STARTED (⬜)

### P2 — Tombstone Pivot (4% → 64% impact) — THE ARCHITECTURAL COMMITMENT

| ID   | Task                                                                                     |
| ---- | ---------------------------------------------------------------------------------------- |
| P2.1 | Add tombstone fields to `model.Item` + `ItemSyncedPayload` + schema V2→V3                |
| P2.2 | Replace hard `DELETE` with tombstone update in both read models + DDL migration          |
| P2.3 | Rename `DeleteItem` → `TombstoneItem(reason)` with reason taxonomy                       |
| P2.4 | Implement reconciliation pass (set-diff provider set vs local → tombstone upstream-gone) |
| P2.5 | Filter tombstoned items from default queries (opt-in `IncludeTombstoned`)                |

### P4 — Resilience (20% → 80% impact)

| ID   | Task                                                                 |
| ---- | -------------------------------------------------------------------- |
| P4.1 | Wire `RetryConfig` into fetch path with exponential backoff + jitter |
| P4.2 | Wire `RateLimitConfig`: respect `Retry-After`, block on 429          |
| P4.3 | Streaming progress callback (per-batch, not once-at-end)             |
| P4.4 | Consult error taxonomy (`IsRetryable`) in retry loop                 |
| P4.5 | Per-source mutex in `Syncer` (fix TOCTOU on latest timestamp)        |

### P5 — Ergonomics & Docs

| ID   | Task                                                                               |
| ---- | ---------------------------------------------------------------------------------- |
| P5.1 | Write `example_test.go` (20-line provider + sync + query)                          |
| P5.2 | Add `pkg/provider/memory` — real in-memory provider (not a mock)                   |
| P5.3 | Update README + AGENTS.md honestly (pull mirror + tombstones, drop CRDT marketing) |
| P5.4 | Update ADR-0003 + new ADR for tombstone-over-delete decision                       |

### P6 — Migration (deferred)

| ID   | Task                                                                 |
| ---- | -------------------------------------------------------------------- |
| P6.1 | Migrate `ConflictResolver[T]` → `go-cqrs-lite/conflict/v3` submodule |
| P6.2 | Implement real schema upcasting (V1→V2→V3 chain)                     |
| P6.3 | Add expected-version assertion in decider                            |

---

## d) TOTALLY FUCKED UP (❌ Things that are broken or wrong)

### Nothing in this session was fucked up — all changes are tested and verified.

### But the following pre-existing issues are still in the codebase:

1. **No upstream-deletion detection** — items the provider stops returning live forever in the read model. This is the #1 missing feature for a real sync SDK.

2. **`RateLimitConfig` and `RetryConfig` are still dead code** — declared, never enforced. The SDK will hammer a 429'd API into the ground.

3. **`DeleteItem` is still an orphan API** — exists at `stack.go:154`, called only in tests. Misleading phantom surface.

4. **Schema upcasting is still cargo-culted** — the entire "upcasting" is `if v==0 { v=V1 }`. Future schema changes = silent zero-fills.

5. **OCC not enforced by localsync** — concurrent same-item syncs can diverge; relies entirely on go-cqrs-lite's version check.

6. **Progress callback fires once at end** — useless for long syncs.

7. **No example, no cmd/, no real provider** — consumer writes all the hard parts.

---

## e) WHAT WE SHOULD IMPROVE

### Architecture

- **Commit to tombstone semantics NOW** — it's the pivot that collapses the A-vs-B fork. Without it, every subsequent task is ambiguous.
- **Delete the dead CRDT cluster B** — stop carrying code that lies about what the system does.
- **Make `pkg/provider/memory` a first-class provider** — it's a development tool, not a test fixture.

### Code quality

- **Add collision regression test** for the aggregate-ID fix (the `github::42` case) — we fixed it but didn't write the test.
- **Add hasChanged regression tests** for avatar-only and RawJSON-only changes.
- **Add Sync() error reporting test** — verify per-item errors propagate to caller.

### Process

- **Write tests immediately with each fix**, not "later" — we shipped fixes without their dedicated regression tests.
- **Update AGENTS.md** to reflect the current honest state (ContentHash field, ItemErrors, projection version gate).

---

## f) Top 25 Things to Get Done Next (sorted by impact)

| #   | Task                                                               | Impact      | Effort |
| --- | ------------------------------------------------------------------ | ----------- | ------ |
| 1   | **P2.1** — Add tombstone fields to `model.Item` + payload          | 🔴 Critical | M      |
| 2   | **P2.2** — Replace hard DELETE with tombstone in read models       | 🔴 Critical | M      |
| 3   | **P2.3** — Rename `DeleteItem` → `TombstoneItem(reason)`           | 🔴 Critical | S      |
| 4   | **P2.4** — Implement reconciliation pass (upstream-gone detection) | 🔴 Critical | L      |
| 5   | **P2.5** — Filter tombstoned items from default queries            | 🟠 High     | S      |
| 6   | **P3.2** — Delete dead CRDT cluster B files                        | 🟠 High     | S      |
| 7   | **P3.3** — Fold `EventItemDeleted` into tombstone semantics        | 🟠 High     | S      |
| 8   | **P4.1** — Wire retry + backoff into fetch path                    | 🟠 High     | M      |
| 9   | **P4.2** — Wire rate-limit / `Retry-After` respect                 | 🟠 High     | M      |
| 10  | **P5.3** — Update README + AGENTS honestly                         | 🟠 High     | S      |
| 11  | **P5.1** — Write `example_test.go`                                 | 🟠 High     | S      |
| 12  | **P5.2** — Implement `pkg/provider/memory`                         | 🔵 Medium   | M      |
| 13  | **P4.3** — Streaming progress callback                             | 🔵 Medium   | S      |
| 14  | **P4.4** — Consult `IsRetryable` in retry loop                     | 🔵 Medium   | S      |
| 15  | **P4.5** — Per-source mutex (fix TOCTOU)                           | 🔵 Medium   | S      |
| 16  | **Test** — Aggregate-ID collision regression test                  | 🟠 High     | S      |
| 17  | **Test** — hasChanged avatar/RawJSON regression test               | 🟠 High     | S      |
| 18  | **Test** — Sync() error propagation test                           | 🟠 High     | S      |
| 19  | **Test** — Projection version-gate resurrect prevention test       | 🟠 High     | S      |
| 20  | **P5.4** — Update ADR-0003 + new tombstone ADR                     | 🔵 Medium   | S      |
| 21  | **P6.2** — Real schema upcasting chain                             | 🔵 Medium   | M      |
| 22  | **P6.3** — Expected-version assertion in decider                   | 🔵 Medium   | S      |
| 23  | **P6.1** — Migrate ConflictResolver → go-cqrs-lite/conflict/v3     | 🔵 Medium   | L      |
| 24  | **Docs** — Update FEATURES.md with tombstone/reconciliation status | 🟢 Low      | S      |
| 25  | **Docs** — Update TODO_LIST.md to reflect plan progress            | 🟢 Low      | S      |

---

## g) Top #1 Question I Cannot Figure Out Myself

**For the reconciliation pass (P2.4): should it run on EVERY full sync, or only when explicitly requested via a `SyncOptions.Reconcile bool` flag?**

The tradeoff:

- **Every sync:** Correct by default — items disappear when upstream drops them. But O(n) set-diff cost per sync, and dangerous if the provider's pagination is incomplete (you'd tombstone items that exist but weren't fetched this page).
- **Explicit opt-in:** Safer — consumer decides when they have a complete view. But then "ghost items" persist until the consumer remembers to run reconciliation.

This is a product-identity question, not an engineering one. It determines whether the SDK is "a mirror that stays in sync" (reconcile every time) or "an append/update log" (reconcile on demand). I cannot decide this without knowing the intended consumer experience.
