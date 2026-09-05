# Comprehensive Status Report — 2026-06-28

**Session:** "Superb SDK — Session 21: Tombstone Pivot + Resilience + Docs Honesty"
**Date:** 2026-06-28 09:51
**Branch:** master
**Test status:** ✅ All 188 tests pass (`go test ./... -count=1`)
**Build status:** ✅ `go build ./...` green
**Lint status:** ✅ `golangci-lint run ./...` — 0 issues

---

## a) FULLY DONE (✅ Shipped, tested, committed)

### P2 — Tombstone Pivot (soft-delete replaces hard-delete)

Committed in `8c0847f`. The aggregate no longer nils out the item on delete — it records a
`Tombstone{Reason, At}` on the existing `*model.Item`, preserving full history.

| ID   | Task                                                                                                                                                                                        |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| P2.1 | `model.Tombstone` + `TombstoneReason` (`upstream_gone`, `user_hidden`, `redacted`) + `ParseTombstoneReason`; `Item.Tombstone` field + `IsTombstoned()`; `ItemFilter.IncludeTombstoned`      |
| P2.2 | Read-model tombstoning: memory (flag-set) + SQLite (`tombstoned`/`tombstone_reason`/`tombstoned_at` columns, idempotent `migrateSyncItems`, resurrect on upsert)                            |
| P2.3 | Event/command rename: `DeleteItemCommand`→`TombstoneItemCommand`, `EventItemDeleted`→`EventItemTombstoned`, `decideDelete`→`decideTombstone`; `SyncItemState` drops the `Deleted bool` flag |
| P2.4 | `SyncStore.Reconcile(ctx, source, seenKeys)` + `SyncOptions.Reconcile` (opt-in) + `SyncResult.Tombstoned`                                                                                   |
| P2.5 | Query filtering: tombstoned items excluded from `List`/`Count`/`GetTypes` by default                                                                                                        |

### P3 — Dead-Code Removal + Event Fold

| ID   | Task                                                                                                                                                                                                                                         | Commit    |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| P3.2 | **Delete the dead CRDT cluster**: `vectorclock.go`, `operation.go`, `types.go`, `doc.go` + all their tests. `Conflict[T]` simplified to `Local`/`Remote`/`Timestamp`; `LWWResolver` is timestamp-only. Package went from ~700 lines to ~100. | `8c0847f` |
| P3.3 | Fold `ItemDeleted` into tombstone semantics — a sync event always means "live", so resurrection is implicit via projection upsert. No V2→V3 schema bump needed.                                                                              | `8c0847f` |

### P4 — Resilience (retry, serialization, error classification)

| ID   | Task                                                                                                                             | Commit    |
| ---- | -------------------------------------------------------------------------------------------------------------------------------- | --------- |
| P4.1 | `fetchItems` retry with exponential backoff + ±25% jitter (`pkg/sync/retry.go`)                                                  | `8c0847f` |
| P4.2 | Consults `errors.IsRetryable` — permanent errors surface immediately, only transient errors retry                                | `8c0847f` |
| P4.3 | Per-source mutex (`lockSource`) — orders concurrent syncs of the same source (TOCTOU guard); different sources run in parallel   | `8c0847f` |
| P4.4 | `retryAfterer` interface — forward-compatible Retry-After hook for providers                                                     | `8c0847f` |
| P4.5 | Lock-free internals (`runSync`/`runSyncIncremental`) — avoids re-entrant mutex deadlock when incremental falls back to full sync | `8c0847f` |

### P5 — Docs Honesty + Example + ADR

| ID   | Task                                                                                                                                                                                                                                       |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| P5.1 | **README honesty rewrite**: reframe as single-writer pull mirror (not multi-device sync); drop CRDT/VectorClock marketing; fix the broken `NewLWWResolver()` signature; correct test counts (188); tombstone/reconciliation/retry sections |
| P5.2 | **AGENTS.md honesty update**: `pkg/crdt` row, delete→tombstone rename, reconciliation, retry, per-source lock, schema columns, test table                                                                                                  |
| P5.3 | **ADR-0003 revised**: `ConflictResolver.Resolve` now returns `(T, error)`; vector-clock machinery formally retired                                                                                                                         |
| P5.4 | **ADR-0004 update note**: findings reference pre-tombstone vocabulary (historical record preserved)                                                                                                                                        |
| P5.5 | **ADR-0005 (new)**: records the tombstone-over-delete decision (why, the data-model rules, consequences)                                                                                                                                   |
| P5.6 | **`pkg/cqrs/example_test.go` (new)**: runnable `ExampleSyncer` showing provider → stack → sync → read-model loop                                                                                                                           |

### Regression tests

| File                          | Covers                                                                                                                  |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `pkg/cqrs/regression_test.go` | aggregate-ID collision, `hasChanged` content-hash, projection version-gate resurrect, tombstone→reconcile→upstream-gone |
| `pkg/sync/regression_test.go` | error propagation, reconcile opt-in/off-by-default                                                                      |
| `pkg/sync/retry_test.go`      | transient-error retry, permanent-error no-retry                                                                         |

**Total this session: 2 commits (`8c0847f` + the P5/lint commit), 0 test failures, lint clean.**

---

## b) PARTIALLY DONE (🔄 In progress or incomplete)

| Item                    | State                                                                                                                                                                                                        |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Tombstone lifecycle** | Tombstone + resurrect + reconcile all work, but there is **no purge/TTL** for tombstoned rows — they accumulate forever. Documented as "future" in ADR-0005.                                                 |
| **Example coverage**    | `ExampleSyncer` shows the happy path. No example yet for tombstone→resurrect or plugging in a `ConflictResolver`.                                                                                            |
| **Retry configuration** | Retry works with `provider.DefaultRetryConfig`, but there is **~~no public setter~~ → done — WithRetry shipped (session 22, ca0844c)** — consumers can't tune `MaxAttempts`/backoff without editing the SDK. |

---

## c) NOT STARTED

| Item                             | Notes                                                                                |
| -------------------------------- | ------------------------------------------------------------------------------------ |
| Tombstone purge/TTL job          | Needs a real storage-cost signal from `github-local-sync` first (ADR-0005 "Future"). |
| Public retry-config API          | `NewSyncer` should gain an options struct or a `WithRetry(...)` setter.              |
| Schema upcasters / observability | Carried over from ADR-0004's deferred data-module work; tracked in TODO_LIST.md.     |
| Tombstone/reconcile CLI surface  | `github-local-sync` (consumer) exposes flags; the SDK just needs the API (done).     |

---

## d) TOTALLY FUCKED UP / RISKS (⚠️)

| Risk                                   | Severity     | Detail                                                                                                                                                                                                                                                                                                                          |
| -------------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Stale LSP flooding false positives** | Nuisance     | `gopls` + `golangci_lint_ls` continuously report "inconsistent vendoring" and "mockSyncStore missing method Reconcile". **Confirmed false** — `vendor/modules.txt` is at v3.1.0, `go build`/`go test`/`golangci-lint run` all green. LSP cannot be restarted (`lsp_restart` fails). Verify via the Go toolchain, never the LSP. |
| **nixpkgs Go lag**                     | Pre-existing | `go.mod` requires `go 1.26.4`; nixpkgs unstable ships `go_1_26` 1.26.3. `nix build` / `nix flake check` fail in the sandbox (`GOTOOLCHAIN=local`). Native gate is green. Self-resolves when nixpkgs bumps. **Do not lower the directive.**                                                                                      |
| **Reconcile footgun**                  | Mitigated    | `SyncOptions.Reconcile` with a _partial_ fetch would wrongly tombstone still-present items. Mitigated by opt-in default + loud doc comments, but a careless caller can still trip it.                                                                                                                                           |

---

## e) WHAT WE SHOULD IMPROVE

1. **Expose retry config publicly** — `NewSyncer` options struct (`WithRetry`, `WithLogger`). Currently hard-coded.
2. **Tombstone purge policy** — at minimum a `PurgeTombstonesBefore(t)` on the read model; ideally a TTL.
3. **More example tests** — tombstone→resurrect, conflict-resolver injection. `ExampleSyncer` alone undersells the SDK.
4. **Kill the LSP vendoring false positive** — likely needs a `vendor/modules.txt` regeneration or an LSP cache clear; investigate so future sessions aren't drowned in noise.
5. **Reconcile safety net** — consider a `CompleteFetch` marker on `FetchResult` so reconcile can't run on a partial fetch even if the caller sets the flag.
6. **Syncer retry is fetch-only** — `SyncItems` (the per-item store path) isn't retried; only `fetchItems` is. Document this explicitly.
7. **`pkg/sync` name collision** — the package is named `sync`, forcing `stdsync "sync"` aliasing. Consider renaming the package to `synclib` (breaking) to remove the footgun.

---

## f) Top 25 next tasks (Pareto-ordered)

| #  | Task                                                                      | Impact  |
| -- | ------------------------------------------------------------------------- | ------- |
| 1  | Public retry-config API (`NewSyncer` options)                             | 🔴 High |
| 2  | Tombstone purge/TTL (`PurgeTombstonesBefore`)                             | 🔴 High |
| 3  | Investigate + fix LSP vendoring false positive                            | 🟠 Med  |
| 4  | `CompleteFetch` marker to harden reconcile                                | 🟠 Med  |
| 5  | Example: tombstone→resurrect round-trip                                   | 🟠 Med  |
| 6  | Example: plug in a `ConflictResolver`                                     | 🟠 Med  |
| 7  | Retry the per-item store path, not just fetch                             | 🟠 Med  |
| 8  | Rename `pkg/sync` → `pkg/synclib` (breaking)                              | 🟠 Med  |
| 9  | Bump coverage on `pkg/data/model` (80.5%) and `pkg/cqrs` (81.9%)          | 🟡 Low  |
| 10 | Add metrics/observability to the sync loop                                | 🟡 Low  |
| 11 | Schema upcasters (ADR-0004 carry-over)                                    | 🟡 Low  |
| 12 | `GetRateLimit` nilnil — decide sentinel vs keep contract                  | 🟡 Low  |
| 13 | Document that retry is fetch-only                                         | 🟡 Low  |
| 14 | Integration test: SQLite reconcile round-trip                             | 🟡 Low  |
| 15 | Make `lockSource` map cleanup-safe (currently grows unbounded per source) | 🟡 Low  |
| 16 | CONTRIBUTING.md: add the tombstone/reconcile guidance                     | 🟡 Low  |
| 17 | OpenAPI: expose tombstone fields in `/items` response                     | 🟡 Low  |
| 18 | `ParseTombstoneReason` test for empty string                              | 🟡 Low  |
| 19 | Bench: projection replay cost at 10k events                               | 🟡 Low  |
| 20 | Decide whether `Reconcile` should be best-effort or fail-loud             | 🟡 Low  |
| 21 | `flake.nix`: vendor offline-build path still needs `go-cqrs-lite` public  | 🟡 Low  |
| 22 | Add a CHANGELOG entry for the tombstone pivot                             | 🟡 Low  |
| 23 | Reconsider `ActionTombstoned` counting in `SyncResult`                    | 🟡 Low  |
| 24 | Fuzz `AggregateID` delimiter encoding                                     | 🟡 Low  |
| 25 | Cut v0.4.0 once retry-config + purge land                                 | 🟡 Low  |

---

## g) #1 Question

**Should the retry configuration be tunable by consumers, and if so, what's the API shape?**

Right now `NewSyncer` hard-codes `provider.DefaultRetryConfig` with no override. For a
"superb SDK" the consumer must control `MaxAttempts` and backoff (a rate-limited GitHub
mirror needs very different retry behavior than a LAN GitLab). The two viable shapes:

- **Options struct**: `NewSyncer(p, store, opts *SyncerOptions)` where `opts` carries
  logger + retry config (breaking, cleanest).
- **Builder/functional options**: `sync.New(p, store).WithRetry(cfg).WithLogger(l)` — the
  idiomatic Go pattern, non-breaking to extend.

My recommendation: **functional options** (`WithRetry`, `WithLogger`) on a new `New(...)`
constructor, keeping `NewSyncer` as a thin deprecated wrapper. This is the lowest-friction
path that doesn't penalize existing callers.

---

## Verification

```
go build ./...                     ✅
go test ./... -count=1             ✅ 188 tests, 9 packages
golangci-lint run ./... --timeout=4m ✅ 0 issues
```

---

## Resolution (2026-09-05 docs-health sweep)

All forward-looking items in this report are closed as of 2026-09-05 (verified against the tree at `9625b1b`: go-localsync v0.5.0, 309 core tests / 11 packages, CI green, both cqrs-lint gates clean).

- **Shipped since:** Nearly everything closed within 24h; retry tuning shipped (WithRetry); purge/TTL remains an ADR-0005 Future tracked in ROADMAP open questions.
- **Superseded/moot:** anything tied to the Turso backend, committed `vendor/`, go-cqrs-lite v2/v3 WIP, or the pre-de-githubify domain model — all removed or reshaped by ADR-0005/0006/0007 and the go-cqrs-lite v4 migration.
- **Routed:** ideas that still matter live in [TODO_LIST.md](../../TODO_LIST.md) or [ROADMAP.md](../../ROADMAP.md); deliberately deferred work is recorded in the ADRs.
- **Policy:** bucket closure per this directory's [README](README.md); the worst now-false claims are struck inline above.

_Report fully resolved → archived 2026-09-05._
