# Cross-Project CQRS Integration Status Report

**Date:** 2026-05-17 18:28\
**Session:** Deep Architecture Review + SchemaVersion Fix Verification\
**Projects:** go-localsync, go-cqrs-lite

---

## Executive Summary

Completed a deep review of the CQRS/go-cqrs-lite integration across both projects. The SchemaVersion serialization bug described in the 2026-05-17 05:47 status report was **already fixed** in go-cqrs-lite commit `a760426`. Legacy CRUD code (`pkg/storage/`, `internal/database/`, `internal/db/`, `sql/`) has been **fully deleted** — AGENTS.md was severely outdated. The go-localsync CLI **already uses CQRSStack exclusively**. Both projects have **zero test failures** and **zero lint issues**.

The main remaining work is: `go mod tidy`, removing cockroachdb/errors, updating stale documentation, and adding missing test coverage for CLI/Push/Pull.

---

## a) FULLY DONE

| Area                           | Status | Detail                                                                                               |
| ------------------------------ | ------ | ---------------------------------------------------------------------------------------------------- |
| SchemaVersion serialization    | ✅     | Fixed in go-cqrs-lite `a760426` — Pebble + SQL + SQLite paths all preserve schema_version            |
| CQRS decider pattern           | ✅     | `Decider[SyncItemState]` with pure Fold + Decide, 17 tests                                           |
| Deterministic aggregate IDs    | ✅     | SHA256→ULID from (source, sourceID), idempotent                                                      |
| Event-sourced read model       | ✅     | MemoryReadModel + TursoReadModel, both with filter/pagination                                        |
| CQRSStack wiring               | ✅     | Store+Bus+Repo+ReadModel, automatic projection via bus subscription                                  |
| CLI uses CQRS                  | ✅     | `main.go` uses `cqrs.NewCQRSStack()`, no legacy storage imports                                      |
| Legacy CRUD deleted            | ✅     | `pkg/storage/`, `internal/database/`, `internal/db/`, `sql/` — all gone                              |
| Conflict-aware sync            | ✅     | `ConflictAwareSyncer` with LWW remote-wins                                                           |
| Branded IDs                    | ✅     | 6 types: ItemID, ExternalID, ProviderID, ActorID, RepoID, EventTypeID                                |
| GitHub provider                | ✅     | Full implementation with pagination, rate limiting, retry (35 tests)                                 |
| Sync orchestration             | ✅     | `Syncer` + `ConflictAwareSyncer` glue between Provider and CQRS stack                                |
| Turso remote sync              | ✅     | Push/Pull exposed via CQRSStack, TursoSyncDB integration                                             |
| Event upcasting infrastructure | ✅     | go-cqrs-lite has version-sorted chaining with cycle detection (now that schema_version is preserved) |
| go-cqrs-lite all modules       | ✅     | 21 test packages pass, zero lint issues                                                              |

---

## b) PARTIALLY DONE

| Area                     | What's Done                                                                                                         | What's Missing                                                        |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| go mod tidy              | Identified changes needed (version bumps, remove unused `golang/protobuf`, `matttproud/golang_protobuf_extensions`) | Not committed yet — affects both go.mod and go.sum                    |
| AGENTS.md accuracy       | This report identifies all inaccuracies                                                                             | Not yet updated — still references deleted packages                   |
| Test framework migration | 1 file uses Ginkgo (`client_bdd_test.go`), 6 files use testify                                                      | Mixed frameworks; pre-commit hooks use buildflow (not a testify ban)  |
| Error taxonomy wiring    | go-cqrs-lite provides 5 error families                                                                              | go-localsync doesn't wire them — domain errors get generic exit codes |
| CLI tests                | CLI is well-structured with `exitCodeForError()`                                                                    | No tests for `cmd/examples/github-sync/` at all                       |

---

## c) NOT STARTED

| Area                       | Description                                                      | Impact                                                                         |
| -------------------------- | ---------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| Remove cockroachdb/errors  | Replace with stdlib `fmt.Errorf` + `%w`                          | Removes 6 transitive deps (pebble, redact, tokenbucket, fifo, logtags, sentry) |
| CLI test coverage          | `cmd/examples/github-sync/` has 0 tests                          | High — it's the main entry point                                               |
| Push/Pull test coverage    | `CQRSStack.Push()` and `Pull()` untested                         | Medium — Turso remote sync is a key feature                                    |
| Adopt projection.Runner    | go-cqrs-lite has `projection.Runner` with replay + checkpointing | Replaces custom Projector                                                      |
| Adopt command.Dispatcher   | go-cqrs-lite has typed command dispatch                          | Could replace raw `SyncItems` method                                           |
| Wire error taxonomy        | Use `event.RegisterClassification` for proper HTTP exit codes    | Users get generic 500s instead of specific error codes                         |
| Testify → stdlib migration | 6 test files use testify                                         | Would eliminate 1 dependency                                                   |
| README/FEATURES update     | Reference deleted packages, outdated architecture                | New developers will be confused                                                |

---

## d) TOTALLY FUCKED UP

| Issue                             | Severity  | Detail                                                                                                                                                                                                                                                                                                                        |
| --------------------------------- | --------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| AGENTS.md severely outdated       | 🟡 MEDIUM | References `pkg/storage/`, `internal/database/`, `internal/db/`, `sql/` — all deleted. Claims "dual storage path" but only CQRS exists. Claims "Phase 4 blocked" but legacy is already gone. Claims pre-commit hooks "ban testify" but they use buildflow. Claims golangci-lint v1/v2 mismatch. Claims go toolchain mismatch. |
| cockroachdb/errors still imported | 🟡 MEDIUM | Pulls in pebble, redact, tokenbucket, fifo, logtags, sentry — 6 heavy transitive deps for a convenience wrapper around `fmt.Errorf` + `%w`                                                                                                                                                                                    |
| go.mod has unused + missing deps  | 🟢 LOW    | `golang/protobuf` and `matttproud/golang_protobuf_extensions` unused; `munnerz/goautoneg` and `go.yaml.in/yaml/v2` missing                                                                                                                                                                                                    |
| No CLI tests                      | 🟡 MEDIUM | 240-line main.go with signal handling, error mapping, backend selection — zero test coverage                                                                                                                                                                                                                                  |

---

## e) WHAT WE SHOULD IMPROVE

### Architecture & Type Models

1. **`SyncItemState` wraps `*provider.Item` directly** — this is clean. The decider pattern avoids duplicate structs. However, `provider.Item` carries `json.RawMessage` (RawJSON) which is serialized into event payloads — consider whether we need a leaner event payload struct.

2. **`ItemFilter` uses `*types.SomeID` for optional fields** — This is a good pattern (nil = no filter). But `Since *time.Time` could use the same optionality approach.

3. **`pkg/sync` duplicates conflict detection that the decider already does** — `ConflictAwareSyncer.Get()` + `HasChanged()` does the same check as `DecideSync`. This is a real split-brain: the sync layer and the decider both independently decide what constitutes a conflict.

4. **`SyncItemsDDL` and `SyncItemsIndexes` are exported constants** but only used within `turso_readmodel.go`. Should be unexported.

5. **Events use `int64` unix nanoseconds** — The `unixNano`/`fromUnixNano` helpers in `events.go` are a good pattern for deterministic serialization, but `time.Time` is the natural Go type. Consider using `time.Time` in payloads with a custom JSON marshaler.

### Library Usage

6. **cockroachdb/errors → stdlib** — go-cqrs-lite already did this (Session 54, net -169 lines). The only feature we actually use is `errors.Is()` compatibility, which stdlib `fmt.Errorf("%w")` provides.

7. **Consider `slog` over `charm.land/log/v2`** — stdlib structured logging is sufficient for an SDK. But charm.land/log is fine if already in use.

8. **`go-cqrs-lite/storage` version is `v0.0.0-00010101000000-000000000000`** — This is a placeholder version via local replace. Needs a real tagged release.

---

## f) TOP 25 THINGS TO DO NEXT

| #  | Priority | Project      | Task                                                                                                                                          | Impact | Effort |
| -- | -------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------ |
| 1  | 🔴 P0    | go-localsync | **`go mod tidy`** — Remove unused deps, add missing ones                                                                                      | HIGH   | 5min   |
| 2  | 🔴 P0    | go-localsync | **Update AGENTS.md** — Remove all references to deleted packages, fix outdated claims                                                         | HIGH   | 30min  |
| 3  | 🔴 P0    | go-cqrs-lite | **Tag v0.1.0-alpha** — First public release, storage as experimental                                                                          | HIGH   | 30min  |
| 4  | 🟡 P1    | go-localsync | **Remove cockroachdb/errors** — Replace with stdlib `fmt.Errorf` + `%w`                                                                       | MEDIUM | 1h     |
| 5  | 🟡 P1    | go-localsync | **Unexport `SyncItemsDDL`/`SyncItemsIndexes`** — Only used internally                                                                         | LOW    | 5min   |
| 6  | 🟡 P1    | go-localsync | **Fix split-brain conflict detection** — Consolidate `ConflictAwareSyncer` + `DecideSync`                                                     | HIGH   | 2h     |
| 7  | 🟡 P1    | go-localsync | **Add CLI tests** — Test `exitCodeForError`, flag parsing, signal handling                                                                    | MEDIUM | 2h     |
| 8  | 🟡 P1    | go-localsync | **Add Push/Pull tests** — Test CQRSStack remote sync methods                                                                                  | MEDIUM | 1h     |
| 9  | 🟡 P1    | go-localsync | **Wire error taxonomy** — Use go-cqrs-lite error classification for exit codes                                                                | MEDIUM | 1h     |
| 10 | 🟡 P1    | go-localsync | **Update README.md** — Reflect CQRS-only architecture                                                                                         | MEDIUM | 30min  |
| 11 | 🟡 P1    | go-localsync | **Update FEATURES.md** — Remove legacy storage references                                                                                     | MEDIUM | 15min  |
| 12 | 🟡 P1    | go-localsync | **Update TODO_LIST.md** — Mark completed items, add new ones                                                                                  | LOW    | 15min  |
| 13 | 🟡 P1    | go-localsync | **Delete stale docs** — Remove `CQRS_MIGRATION_PLAN.md`, `PROJECT_SPLIT_EXECUTIVE_REPORT.md`, `BDD_TESTS_REVIEW.md`, `PARTS.md`, `ROADmap.md` | LOW    | 5min   |
| 14 | 🟡 P1    | go-localsync | **Delete `justfile`** — AGENTS.md says "justfile is deprecated"                                                                               | LOW    | 1min   |
| 15 | 🟢 P2    | go-localsync | **Adopt `projection.Runner`** — Replace custom Projector with go-cqrs-lite's                                                                  | MEDIUM | 2h     |
| 16 | 🟢 P2    | go-localsync | **Testify → stdlib** — Migrate 6 test files from testify to stdlib `t.Errorf`/`t.Fatal`                                                       | LOW    | 3h     |
| 17 | 🟢 P2    | go-localsync | **Unify test framework** — Either all Ginkgo or all stdlib, not mixed                                                                         | LOW    | 4h     |
| 18 | 🟢 P2    | go-localsync | **Use `time.Time` in event payloads** — Replace int64 unix nanoseconds                                                                        | MEDIUM | 1h     |
| 19 | 🟢 P2    | go-cqrs-lite | **PostgreSQL integration tests** — Test against real PG, not just mocks                                                                       | MEDIUM | 4h     |
| 20 | 🟢 P2    | go-cqrs-lite | **Implement Saga/Process Manager** — Design exists, 18h estimate                                                                              | HIGH   | 18h    |
| 21 | 🟢 P2    | go-cqrs-lite | **CONTRIBUTING.md** — Referenced in README but doesn't exist                                                                                  | LOW    | 1h     |
| 22 | 🟢 P2    | go-cqrs-lite | **TransactionalStore implementation** — Atomic save+outbox in single DB tx                                                                    | HIGH   | 4h     |
| 23 | 🟢 P2    | go-cqrs-lite | **Document Pebble storage** — Add to README, FEATURES.md                                                                                      | LOW    | 30min  |
| 24 | 🟢 P2    | go-localsync | **Real GitHub PAT smoke test** — Verify end-to-end with real API                                                                              | MEDIUM | 1h     |
| 25 | 🟢 P2    | go-localsync | **Add JSON output flag** — Structured output for CLI                                                                                          | LOW    | 1h     |

---

## g) TOP QUESTION I CANNOT FIGURE OUT MYSELF

**Should the `ConflictAwareSyncer` conflict detection be consolidated into the CQRS decider, or should both layers independently detect conflicts?**

Current state: `ConflictAwareSyncer.SyncWithConflictDetection()` reads from the ReadModel to check if an item has changed, then calls `Syncer.Sync()` which calls `CQRSStack.SyncItems()` which internally runs `DecideSync` that also checks for changes via `HasChanged()`.

This means every sync goes through **two independent conflict checks**:

1. `ConflictAwareSyncer` reads the read model → compares timestamps
2. `DecideSync` folds events to get state → compares timestamps

Arguments for consolidating into decider only:

- Single source of truth (event-sourced state, not read model)
- Eliminates the read model query per item (performance)
- `ConflictAwareSyncer` becomes just `Syncer` with conflict logging

Arguments for keeping both:

- `ConflictAwareSyncer` can log/warn/notify about conflicts at the sync orchestration layer
- The decider's conflict detection is about producing events, not about user-facing notifications
- Read model query is fast and provides a "last known good state" snapshot

**What's the right architectural boundary here?**

---

## Build & Test Results

### go-cqrs-lite (21 test packages)

```
ok  core/aggregate       0.002s
ok  core/command          0.002s
ok  core/decider          0.007s
ok  core/event            0.019s
ok  core/pkg/dispatcher   0.002s
ok  core/pkg/id           0.003s
ok  core/query            0.002s
ok  memory                0.007s
ok  storage               1.397s
ok  middleware             0.148s
ok  projection            0.127s
ok  sync                  0.003s
ok  catalog/adapters      0.002s
ok  catalog/asyncapi      0.002s
ok  catalog/d2            0.002s
ok  catalog/eventcatalog  0.032s
ok  integration/aggregate 0.019s
ok  integration/command   0.015s
ok  integration/event     0.020s
ok  integration/query     0.018s
```

**All 21 packages PASS.** Zero lint issues.

### go-localsync (6 test packages + 2 no-test packages)

```
ok  pkg/cqrs              0.096s  (46 tests)
ok  pkg/errors            0.003s  (4 tests)
ok  pkg/provider          0.002s  (1 test)
ok  pkg/providers/github  0.014s  (35 tests)
ok  pkg/sync              0.005s  (11 tests)
ok  pkg/types             0.002s  (10 tests)
?   cmd/examples/github-sync   [no test files]
?   pkg/testhelpers            [no test files]
```

**All 6 test packages PASS.** 107 total test cases.

### Code Metrics

| Metric           | go-localsync     | go-cqrs-lite |
| ---------------- | ---------------- | ------------ |
| Production files | 17 (2,699 lines) | ~100+ files  |
| Test files       | 10 (2,431 lines) | ~60+ files   |
| Test:Code ratio  | 0.90:1           | ~0.95:1      |
| Test functions   | 107              | ~500+        |
| TODO/FIXME/HACK  | 0                | 0            |
| Dead code        | None             | None         |
| Lint issues      | 0                | 0            |

---

## Reflection: What I Got Wrong / Could Do Better

1. **I edited files that were already correct** — The SchemaVersion fix was already committed in go-cqrs-lite `a760426`. I should have checked git log first, then verified the fix was complete. Instead I re-applied changes to already-correct files (the edit tool said "Applied" because the find/replace strings matched, but the content was already there).

2. **AGENTS.md trust** — I trusted the AGENTS.md description of the project state, which was severely outdated. The document claimed legacy code existed, dual storage paths, blocked Phase 4, testify-banning hooks — none of which were true. I should have verified every claim against the actual codebase first.

3. **Missing the split-brain conflict detection** — The most significant architectural issue I found: `ConflictAwareSyncer` and `DecideSync` both independently detect conflicts. This is the kind of thing that causes subtle bugs (different conflict resolution outcomes depending on which layer "wins"). I should have caught this earlier.

4. **Not checking go-cqrs-lite git log early enough** — If I had run `git log --oneline -5 -- storage/` in go-cqrs-lite at the start, I would have seen `a760426 fix(storage): complete schema_version column migration` and known the fix was already done.

---

_Generated by Crush — 2026-05-17_
