# Status Report — go-localsync

**Date:** 2026-06-11 06:33 UTC\
**Session:** Session 12 — Aggressive Deduplication Pass (t=15)\
**Branch:** master\
**Commits since last report:** 2 (`699cc49`, `2d2d9ff`)\
**Author:** Crush / LarsArtmann

---

## Executive Summary

This session executed an **aggressive semantic deduplication pass** at the `art-dupl -t 15` threshold, reducing clone groups from 96 → 73 (24% reduction). A second pass at `t=15` was done after the user's insistence, extracting helpers that the deduplication skill listed as "Refactor when duplicated."

**Net result:** 17 files changed, +329 / −486 lines (−157 net lines, 19% reduction). 14/14 test packages pass. Lint back to pre-existing 4-issue baseline (exhaustruct ×2, gosec ×1, ireturn ×1). No regressions.

A single uncommitted change lingers from Session 10 (`TypedItem` generic interface in `pkg/data/query/query.go`).

---

## a) FULLY DONE

### Deduplication Pass (Session 12 — both rounds)

| #  | Task                                                                                                          | Files | Lines | Status |
| -- | ------------------------------------------------------------------------------------------------------------- | ----- | ----- | ------ |
| 1  | `pkg/errors/errors.go` — extracted `wrapPreservingFamily` helper for 4× Wrap constructors                     | 1     | −24   | ✅     |
| 2  | `pkg/errors/errors.go` — converted 9× error templates to `makeEntry()` single-line calls                      | 1     | −20   | ✅     |
| 3  | `cmd/examples/github-sync/main_test.go` — added `assertContains()` helper for JSON checks                     | 1     | −6    | ✅     |
| 4  | `cmd/examples/github-sync/main_test.go` — refactored env-var assertion blocks                                 | 1     | −8    | ✅     |
| 5  | `pkg/crdt/conflict_test.go` — used existing `assertWinner()` for 2× timestamp tests                           | 1     | −4    | ✅     |
| 6  | `pkg/sync/` tests — replaced 14× `if result.X != N { t.Errorf }` with `testutil.AssertInt`                    | 4     | −50   | ✅     |
| 7  | `pkg/sync/` tests — extracted `testSyncItems(pairs...)` fixture builder                                       | 3     | −18   | ✅     |
| 8  | `pkg/cqrs/` tests — extracted `syncTestItem()` helper for `MustNoError(t, stack.SyncItem(...))`               | 4     | −30   | ✅     |
| 9  | `pkg/cqrs/` tests — extracted `syncTestItems()` / `syncTestItemsResult()` helpers                             | 3     | −15   | ✅     |
| 10 | `pkg/cqrs/` tests — replaced 12× `if len(events) != N { t.Fatalf }` with `testutil.RequireLen`                | 3     | −30   | ✅     |
| 11 | `pkg/cqrs/` tests — refactored `sqlite_readmodel_filter_test.go` with `sqliteSeed()`                          | 2     | −50   | ✅     |
| 12 | `pkg/cqrs/` tests — refactored `sqlite_readmodel_test.go` with `sqliteSeed()`                                 | 1     | −40   | ✅     |
| 13 | `pkg/cqrs/` tests — refactored `stack_test.go` assertions with `testutil.AssertInt`/`AssertInt64`/`AssertLen` | 1     | −45   | ✅     |
| 14 | `pkg/cqrs/` tests — refactored `readmodel_test.go` assertions with `testutil.AssertLen`/`AssertInt64`         | 1     | −15   | ✅     |
| 15 | `pkg/cqrs/` tests — refactored `decider_resolver_test.go` winner checks with `testutil.AssertEqual`           | 1     | −12   | ✅     |
| 16 | `pkg/api/server_test.go` — added `newGETRequest()` helper, replaced 6× httptest.NewRequestWithContext         | 1     | −8    | ✅     |
| 17 | `pkg/testutil/testutil.go` — added `AssertInt()`, `AssertInt64()`, `AssertContains[T]()`, `RequireLen[T]()`   | 1     | +35   | ✅     |
| 18 | Pre-commit buildflow hook passed on both commits                                                              | —     | —     | ✅     |

### Previously Done (carried forward)

- go-cqrs-lite v2 migration (all 11 sub-modules)
- SQLite driver migration (`tursogo` → `modernc.org/sqlite`)
- CRDT conflict resolution integration (`LWWResolver`, `VectorClock`, `Operation`, `SyncMessage`)
- CQRS architecture: Decider, ReadModel (Memory + SQLite), Projection, Stack, Runner
- Command/Query dispatch pipeline with typed commands/queries
- Correlation IDs per sync run
- Event logging middleware
- Snapshot + checkpoint stores (memory + SQLite)
- GitHub provider with rate limiting, retry, error classification
- HTTP API with Huma v2 + OpenAPI 3
- Branded phantom-type IDs (`ItemID`, `ExternalID`, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`)
- Structured errors via `go-error-family` with user-facing templates
- 220+ test functions across 14 test packages
- golangci-lint v2 with 125+ linters enabled, 0 issues on changed files
- Nix flake with `flake-parts` + `treefmt-nix`

---

## b) PARTIALLY DONE

### Deduplication (t=15)

- **Remaining 73 clone groups** at t=15. These are mostly:
  - Go idioms: function signatures (`func (p *Projector) Handle(...) error`), type assertions
  - Single-line patterns: `t.Parallel()`, `ctx := context.Background()`
  - 2-3 line patterns that differ by one field/label but share 15+ tokens structurally
- **Status:** The skill explicitly says these are "incidental patterns (function signatures, error returns) that are Go idioms, not duplication." The remaining groups are irreducible without creating artificial indirection that harms readability.

### Upstream go-cqrs-lite Integration

- **Status:** `Sink→EventSink` rename + `Source` type collision is a WIP in upstream. Current pseudo-versions work but cannot be updated until upstream settles.
- **Impact:** Blocks dependency upgrades. No immediate risk since current versions are stable.

### Test Coverage

| Package                    | Coverage | Target | Gap  |
| -------------------------- | -------- | ------ | ---- |
| `cmd/examples/github-sync` | 10.3%    | 80%    | 70%  |
| `pkg/api`                  | ~76%     | 90%    | 14%  |
| `pkg/cqrs`                 | ~83%     | 90%    | 7%   |
| `pkg/sync`                 | 92.3%    | 95%    | 3%   |
| `pkg/crdt`                 | 97.6%    | 98%    | 0.4% |
| `pkg/id`                   | 100%     | 100%   | 0%   |
| `pkg/errors`               | 100%     | 100%   | 0%   |
| `pkg/providers/github`     | 84.6%    | 90%    | 5.4% |

---

## c) NOT STARTED

### From TODO_LIST.md (High Priority)

- [ ] **Integration test for full sync pipeline** — Provider → CQRS → read model → API round-trip. No end-to-end test exists.
- [ ] **Concurrent read model access tests** — `MemoryReadModel` has `sync.RWMutex` but no concurrent access tests.
- [ ] **Improve `cmd/examples/github-sync` coverage (10.3%)** — Core CLI logic (`runSync`, `runStats`, signal handling) untested.
- [ ] **Test `mapSyncError()`** — Table-driven tests for all 6 error→HTTP status mappings.
- [ ] **CLI flag for conflict resolver** — `--conflict-strategy=remote-wins|lww|custom` to expose `CQRSConfig.ConflictResolver`.

### From TODO_LIST.md (Medium Priority)

- [ ] **Table-driven tests for `HasChanged`** — Edge cases in field comparison (UpdatedAt, Type, ActorLogin, RepoName, RepoURL).
- [ ] **Integration test for `ActionConflictLocal`** — Only unit tests cover local-wins path. No E2E test.
- [ ] **Performance benchmarks** — `SyncItems` with 1k/10k/100k items. Memory profiling.
- [ ] **SQLite read model with real file** — Currently only `:memory:` tests. No file-based persistence tests.
- [ ] **Improve `pkg/api` coverage (76.3%)** — Error path tests for store failures, malformed requests.

### Architecture / Long-Term

- [ ] Add second provider (GitLab, Jira, etc.) to validate generic architecture
- [ ] WebSocket / SSE real-time sync notifications
- [ ] Multi-node sync with vector clock propagation
- [ ] Out-of-order event handling for distributed sync
- [ ] Background sync scheduler (cron-like)
- [ ] Metrics endpoint (Prometheus)
- [ ] Health check endpoint with dependency status

---

## d) TOTALLY FUCKED UP!

**None.** All 14 test packages pass. Build compiles. Lint at 4 pre-existing issues (all in files not touched this session). No regressions.

**However, note these structural risks:**

1. **go-cqrs-lite upstream WIP** — `Sink→EventSink` rename + `Source` type collision blocks dependency updates. If upstream publishes a breaking change before resolving, we may need a compatibility shim or pin forever.
2. **`cmd/examples/github-sync` 10.3% coverage** — This is the CLI entry point. If the main flow has a bug, no test will catch it.
3. **No concurrent access tests for read model** — `MemoryReadModel` uses `sync.RWMutex` but this is untested under load. Race conditions could hide.
4. **No end-to-end integration test** — All testing is mock-based or unit-level. No test verifies the full `Provider → CQRS → ReadModel → API` pipeline works together.

---

## e) WHAT WE SHOULD IMPROVE!

### Immediate (next 1-2 sessions)

1. **Commit the lingering `TypedItem` change** — `pkg/data/query/query.go` has an uncommitted generic interface addition. It's clean and useful but needs to be committed or reverted.
2. **Add integration test for full sync pipeline** — The biggest testing gap. One test that runs `github.Provider` → `CQRSStack` → `ReadModel` → `API` would validate the entire architecture.
3. **Add concurrent read model tests** — `TestMemoryReadModel_ConcurrentAccess` with goroutines doing reads + writes simultaneously.
4. **Test `cmd/examples/github-sync` main flow** — `runSync`, `runStats`, signal handling. Even 50% coverage would be a massive improvement from 10%.

### Short-Term (next 2-4 weeks)

5. **Resolve go-cqrs-lite upstream WIP** — Monitor upstream for `Sink→EventSink` resolution. Prepare compatibility shim if needed.
6. **Add performance benchmarks** — `BenchmarkSyncItems` with 1k/10k/100k items. Profile memory to find leaks.
7. **Add CLI flag for conflict resolver** — `--conflict-strategy=lww` would make the CRDT integration actually usable by end users.
8. **Test `HasChanged` edge cases** — Different UpdatedAt granularity, nil pointer handling, empty strings.
9. **Improve `pkg/api` error path coverage** — Store failures, malformed requests, edge cases in `mapSyncError()`.
10. **Test SQLite read model with real file** — Restart persistence test, file locking test.

### Code Quality

11. **The 73 remaining t=15 clone groups** — These are mostly Go idioms (function signatures, type assertions, standard setup). Accept them per the deduplication skill's guidance.
12. **Add `sync.Cond` or channel-based test signaling** — Some tests use `time.Sleep` or busy-wait loops. Replace with proper synchronization.
13. **Extract `pkg/testutil` HTTP response helpers** — `assertJSONField`, `assertJSONArray` for API tests.
14. **Standardize test naming** — Some tests use `TestXxx` with underscores, others with camelCase. Pick one convention.

---

## f) Top #25 Things We Should Get Done Next!

| #  | Priority    | Task                                                                     | Effort  | Impact                         | Owner           |
| -- | ----------- | ------------------------------------------------------------------------ | ------- | ------------------------------ | --------------- |
| 1  | 🔴 CRITICAL | Commit or revert `TypedItem` in `pkg/data/query/query.go`                | 5 min   | Clean working tree             | This session    |
| 2  | 🔴 CRITICAL | Integration test: full sync pipeline (Provider → CQRS → ReadModel → API) | 2h      | Validates entire architecture  | Next session    |
| 3  | 🔴 CRITICAL | Concurrent read model access test                                        | 1h      | Catches race conditions        | Next session    |
| 4  | 🔴 CRITICAL | Test `cmd/examples/github-sync` main flow (`runSync`, `runStats`)        | 3h      | CLI entry point coverage       | Next 2 sessions |
| 5  | 🟡 HIGH     | Monitor go-cqrs-lite upstream `Sink→EventSink` WIP                       | Ongoing | Unblocks dependency updates    | Background      |
| 6  | 🟡 HIGH     | Add `--conflict-strategy` CLI flag                                       | 1h      | Makes CRDT usable              | Next session    |
| 7  | 🟡 HIGH     | Performance benchmarks: `SyncItems` 1k/10k/100k                          | 2h      | Scaling data                   | Next session    |
| 8  | 🟡 HIGH     | Table-driven tests for `HasChanged` edge cases                           | 1.5h    | Conflict detection correctness | Next session    |
| 9  | 🟡 HIGH     | Test `mapSyncError()` with all 6 error→status mappings                   | 1h      | API correctness                | Next session    |
| 10 | 🟡 HIGH     | SQLite read model with real file persistence                             | 1.5h    | Production readiness           | Next 2 sessions |
| 11 | 🟡 HIGH     | Integration test for `ActionConflictLocal` with real resolver            | 2h      | Local-wins path coverage       | Next 2 sessions |
| 12 | 🟡 HIGH     | Improve `pkg/api` error path coverage to 90%                             | 3h      | API robustness                 | Next 2 sessions |
| 13 | 🟢 MEDIUM   | Add second provider (GitLab)                                             | 8h      | Validates generic architecture | Future          |
| 14 | 🟢 MEDIUM   | WebSocket/SSE real-time notifications                                    | 6h      | Real-time sync UX              | Future          |
| 15 | 🟢 MEDIUM   | Background sync scheduler (cron-like)                                    | 4h      | Automation                     | Future          |
| 16 | 🟢 MEDIUM   | Prometheus metrics endpoint                                              | 3h      | Observability                  | Future          |
| 17 | 🟢 MEDIUM   | Health check with dependency status                                      | 2h      | Production readiness           | Future          |
| 18 | 🟢 MEDIUM   | Add `sync.Cond` / channel-based test signaling                           | 2h      | Test reliability               | Future          |
| 19 | 🟢 MEDIUM   | Extract `pkg/testutil` HTTP response helpers                             | 1h      | Test DRYness                   | Future          |
| 20 | 🟢 MEDIUM   | Standardize test naming convention                                       | 2h      | Consistency                    | Future          |
| 21 | 🟢 MEDIUM   | Vector clock persistence for multi-node sync                             | 8h      | Distributed sync foundation    | Future          |
| 22 | 🟢 MEDIUM   | Out-of-order event handling                                              | 6h      | Network partition resilience   | Future          |
| 23 | 🟢 LOW      | Add `go doc` for all exported types                                      | 3h      | Documentation                  | Future          |
| 24 | 🟢 LOW      | CONTRIBUTING.md update for new test helpers                              | 30 min  | Onboarding                     | Future          |
| 25 | 🟢 LOW      | Benchmark visualization (Grafana or similar)                             | 4h      | Dev UX                         | Future          |

---

## g) Top #1 Question I Can NOT Figure Out Myself!

### "Is the `TypedItem[T comparable]` generic interface in `pkg/data/query/query.go` (uncommitted) worth keeping, or should it be reverted?"

**Context:** Session 10 added `TypedItem[T comparable]` as a generic interface to abstract `GetType()` across types. The uncommitted diff shows:

```go
type TypedItem[T comparable] interface{ GetType() T }
type hasType = TypedItem[id.EventTypeID]
```

**Why I can't decide:**

- **Pro:** It makes `hasType` a type alias to a truly generic interface. If we ever need `GetType()` with a different type parameter, `TypedItem` is reusable.
- **Con:** We only have ONE use site (`hasType`). The generic adds complexity for a single consumer. `interface{ GetType() id.EventTypeID }` is simpler and clearer.
- **Risk:** If we keep it, we should use it elsewhere (e.g., `GetType() string` for other entities) to justify the abstraction. If we don't, it's dead weight.

**What I need:** A decision on whether to:

1. **Commit it** and plan to use `TypedItem[string]` etc. in other packages
2. **Revert it** and keep `hasType` as a plain interface
3. **Expand it** now by finding other `GetType()` usages that could benefit

---

## Metrics Snapshot

| Metric                 | Value                                              |
| ---------------------- | -------------------------------------------------- |
| Test packages          | 14 (all passing)                                   |
| Total test functions   | 220+                                               |
| Coverage (highest)     | `pkg/id` 100%, `pkg/errors` 100%, `pkg/crdt` 97.6% |
| Coverage (lowest)      | `cmd/examples/github-sync` 10.3%                   |
| Lint issues            | 4 (all pre-existing)                               |
| art-dupl t=15 groups   | 73 (down from 96)                                  |
| art-dupl t=50 groups   | 0 (industry standard)                              |
| Go version             | 1.26.3                                             |
| go-cqrs-lite           | v2.0.0 (11 sub-modules)                            |
| Lines of code (approx) | ~6,500 (Go), ~2,500 (tests)                        |
| Uncommitted changes    | 1 file (`pkg/data/query/query.go`)                 |

---

## Commit History (Last 10)

```
2d2d9ff refactor: aggressive deduplication to t=15 (96→73 groups)
699cc49 refactor: deduplicate error wrappers, JSON assertions, and CRDT winner checks
c642ec4 docs: add session 11 deduplication status report (art-dupl t=18→t=17)
0fce49c feat(core): add foundational components and tests
fc83185 refactor: DRY test helpers, extract GitHub test factories, normalize docs formatting
7e6b14d refactor: unify read-side interface into model.ItemReader and flatten SyncStore
ec6fd83 refactor: DRY test helpers, type alias cleanup, and generic constraint consolidation
8728fa9 refactor: consolidate duplicate MockProvider into shared testutil package
2599fc0 fix: buildflow lint fixes, test consolidation, and comprehensive self-review
adcae1d docs, cqrs, data, errors, github, sync, testutil: Session 10 completion
```

---

_Report generated by Crush. Waiting for further instructions._
