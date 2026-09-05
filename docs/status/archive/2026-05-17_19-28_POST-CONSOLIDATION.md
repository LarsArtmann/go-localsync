# Post-Consolidation Status Report

**Date:** 2026-05-17 19:28\
**Session:** Consolidation cleanup + conflict detection fix\
**Projects:** go-localsync, go-cqrs-lite

---

## Executive Summary

Executed 7 commits eliminating technical debt and fixing a real production bug. Net change: **-935 lines** (502 added, 1437 removed). The project is now on a single CQRS architecture with no legacy paths, accurate documentation, and a consolidated conflict detection system. All 107 tests pass, go vet clean, build clean.

---

## a) FULLY DONE

| Area                             | Detail                                                                                                       |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| CQRS architecture                | Full event-sourced stack via go-cqrs-lite: Decider + ReadModel + Projection + Turso                          |
| Legacy CRUD deletion             | `pkg/storage/`, `internal/database/`, `internal/db/`, `sql/` — all gone                                      |
| CLI on CQRS                      | `main.go` uses `cqrs.NewCQRSStack()` exclusively                                                             |
| Deterministic aggregate IDs      | SHA256→ULID from (source, sourceID) for idempotency                                                          |
| Conflict detection consolidation | Decider is single authority; `ConflictAwareSyncer` delegates to `SyncItems` results                          |
| HasChanged time comparison bug   | Fixed `!=` to `.Equal()` — was causing false conflict detections                                             |
| cockroachdb/errors removal       | Replaced with stdlib `fmt.Errorf` + `%w`; now only indirect via go-cqrs-lite/storage                         |
| go mod tidy                      | Unused deps removed, missing deps added                                                                      |
| Stale docs deleted               | `CQRS_MIGRATION_PLAN.md`, `PROJECT_SPLIT_EXECUTIVE_REPORT.md`, `BDD_TESTS_REVIEW.md`, `PARTS.md`, `justfile` |
| AGENTS.md rewrite                | Complete rewrite reflecting actual codebase (was dangerously outdated)                                       |
| ROADMAP.md update                | Removed deleted code references, added current tech debt                                                     |
| SchemaVersion serialization      | Already fixed in go-cqrs-lite `a760426` (Pebble + SQL + SQLite + Outbox)                                     |
| Branded IDs                      | 6 phantom types for compile-time safety                                                                      |
| GitHub provider                  | Full implementation with pagination, rate limiting, retry (35 tests)                                         |
| Turso remote sync                | Push/Pull via TursoSyncDB                                                                                    |
| go-cqrs-lite integration         | 21 test packages pass, zero lint issues                                                                      |

### Test Coverage

| Package                    | Tests | Status                                                    |
| -------------------------- | ----- | --------------------------------------------------------- |
| `pkg/cqrs`                 | 46    | ✅ Decider (17), ReadModel (10), Stack (11), Turso RM (8) |
| `pkg/providers/github`     | 35    | ✅ Client (19 unit + 16 BDD)                              |
| `pkg/sync`                 | 11    | ✅ Syncer (4) + ConflictAware (7)                         |
| `pkg/types`                | 10    | ✅ ID construction, roundtrip, zero, equal                |
| `pkg/errors`               | 4     | ✅ Sentinel errors, wrapping                              |
| `pkg/provider`             | 1     | ✅ Item validation                                        |
| `cmd/examples/github-sync` | 0     | ⬜ No tests                                               |
| `pkg/testhelpers`          | 0     | ⬜ Helper package                                         |

**Total: 107 test cases across 6 test packages.** Test:Code ratio 0.92:1.

### Code Metrics

| Metric               | Value                   |
| -------------------- | ----------------------- |
| Production files     | 17 (2,638 lines)        |
| Test files           | 10 (2,421 lines)        |
| Build                | Clean                   |
| Vet                  | Clean                   |
| Lint                 | 0 issues (125+ linters) |
| go.mod direct deps   | 10                      |
| go.mod indirect deps | ~35                     |
| TODO/FIXME/HACK      | 0                       |
| Dead code            | None                    |

---

## b) PARTIALLY DONE

| Area                   | What's Done                           | What's Missing                                                    |
| ---------------------- | ------------------------------------- | ----------------------------------------------------------------- |
| Documentation accuracy | AGENTS.md, ROADMAP.md updated         | FEATURES.md, TODO_LIST.md, README.md still reference deleted code |
| Error handling         | cockroachdb/errors removed            | Error taxonomy from go-cqrs-lite not wired (generic exit codes)   |
| Test framework         | Identified 6 testify + 1 ginkgo files | Not unified; both frameworks coexist                              |

---

## c) NOT STARTED

| Area                          | Description                                          | Impact                              |
| ----------------------------- | ---------------------------------------------------- | ----------------------------------- |
| CLI tests                     | Zero coverage for 240-line main.go                   | HIGH — main entry point             |
| Push/Pull tests               | `CQRSStack.Push()` and `Pull()` untested             | MEDIUM — key Turso feature          |
| Adopt projection.Runner       | go-cqrs-lite has `projection.Runner` with replay     | MEDIUM — replaces custom Projector  |
| Adopt command.Dispatcher      | Typed command dispatch from go-cqrs-lite             | LOW — nice-to-have                  |
| Wire error taxonomy           | `event.RegisterClassification` for proper exit codes | MEDIUM — users get generic 500s     |
| Testify → stdlib migration    | 6 test files use testify                             | LOW — works fine, just inconsistent |
| Update FEATURES.md            | References deleted packages                          | LOW — outdated                      |
| Update TODO_LIST.md           | References deleted packages                          | LOW — outdated                      |
| Update README.md              | References `just sqlc`, deleted packages             | MEDIUM — user-facing                |
| Real GitHub PAT smoke test    | End-to-end verification with real API                | MEDIUM                              |
| go-cqrs-lite v0.1.0-alpha tag | Library is unversioned                               | HIGH — consumers can't pin          |
| go-cqrs-lite CONTRIBUTING.md  | Referenced in README but doesn't exist               | LOW                                 |

---

## d) TOTALLY FUCKED UP

Nothing currently fucked up. The session fixed the two most critical issues:

1. ~~HasChanged used `!=` for time.Time~~ — **Fixed** in `0f48545`. Was a production bug causing false conflict detections after event roundtrip.
2. ~~Split-brain conflict detection~~ — **Fixed** in `0f48545`. ConflictAwareSyncer and DecideSync independently detected conflicts with different truth sources.

No remaining critical issues.

---

## e) WHAT WE SHOULD IMPROVE

### Architecture

1. **FEATURES.md / TODO_LIST.md / README.md still reference deleted code** — These are user-facing docs that will confuse new developers. Quick wins.

2. **No CLI tests** — The 240-line `main.go` has signal handling, error mapping, flag parsing. Zero tests. This is the highest-impact testing gap.

3. **Push/Pull untested** — Turso remote sync is a key differentiator. Should have at least basic tests.

4. **docs/status/ has 28 files** — 23 are from Feb-Apr. Most are session snapshots that are no longer relevant. Consider archiving older ones.

5. **go-cqrs-lite/storage version is a placeholder** — `v0.0.0-00010101000000-000000000000` with local replace. Needs a real tagged release.

### Type Models

6. **Events use `int64` unix nanoseconds** — The `unixNano`/`fromUnixNano` helpers work but `time.Time` is the natural Go type. A custom JSON marshaler would be cleaner and eliminate the entire class of time comparison bugs.

7. **`SyncAction` is a string type** — Could be a typed enum with methods for better type safety.

### Library Usage

8. **Testify + Ginkgo coexist** — One BDD file uses Ginkgo, six use testify. Standard library `t.Errorf`/`t.Fatal` would eliminate both dependencies and be consistent with go-cqrs-lite.

---

## f) TOP 25 THINGS TO DO NEXT

| #  | Priority | Project      | Task                                                                     | Impact | Effort |
| -- | -------- | ------------ | ------------------------------------------------------------------------ | ------ | ------ |
| 1  | 🔴 P0    | go-localsync | **Update README.md** — Remove deleted code refs, justfile, sqlc          | HIGH   | 30min  |
| 2  | 🔴 P0    | go-localsync | **Update FEATURES.md** — Remove legacy storage references                | MEDIUM | 15min  |
| 3  | 🔴 P0    | go-localsync | **Update TODO_LIST.md** — Remove completed/stale items                   | LOW    | 15min  |
| 4  | 🔴 P0    | go-cqrs-lite | **Tag v0.1.0-alpha** — First public release, storage experimental        | HIGH   | 30min  |
| 5  | 🟡 P1    | go-localsync | **Add CLI tests** — Test exitCodeForError, flag parsing, signal handling | HIGH   | 2h     |
| 6  | 🟡 P1    | go-localsync | **Add Push/Pull tests** — Test Turso remote sync methods                 | MEDIUM | 1h     |
| 7  | 🟡 P1    | go-localsync | **Wire error taxonomy** — go-cqrs-lite error classification → exit codes | MEDIUM | 1h     |
| 8  | 🟡 P1    | go-localsync | **Adopt projection.Runner** — Replace custom Projector                   | MEDIUM | 2h     |
| 9  | 🟡 P1    | go-localsync | **Migrate testify → stdlib** — 6 test files                              | LOW    | 3h     |
| 10 | 🟡 P1    | go-localsync | **Archive old status docs** — 23 stale files in docs/status/             | LOW    | 5min   |
| 11 | 🟡 P1    | go-cqrs-lite | **CONTRIBUTING.md** — Referenced in README but missing                   | LOW    | 1h     |
| 12 | 🟢 P2    | go-localsync | **Use time.Time in event payloads** — Replace int64 unix nanoseconds     | MEDIUM | 1h     |
| 13 | 🟢 P2    | go-localsync | **Make SyncAction a typed enum** — Better type safety                    | LOW    | 30min  |
| 14 | 🟢 P2    | go-localsync | **Real GitHub PAT smoke test** — End-to-end verification                 | MEDIUM | 1h     |
| 15 | 🟢 P2    | go-localsync | **Add JSON output flag** — Structured output for CLI                     | LOW    | 1h     |
| 16 | 🟢 P2    | go-localsync | **Add structured logging fields** — Consistent context                   | LOW    | 1h     |
| 17 | 🟢 P2    | go-cqrs-lite | **PostgreSQL integration tests** — Real PG, not just mocks               | MEDIUM | 4h     |
| 18 | 🟢 P2    | go-cqrs-lite | **Implement Saga/Process Manager** — Design done, 18h estimate           | HIGH   | 18h    |
| 19 | 🟢 P2    | go-cqrs-lite | **TransactionalStore implementation** — Atomic save+outbox               | HIGH   | 4h     |
| 20 | 🟢 P2    | go-cqrs-lite | **Document Pebble storage** — Add to README, FEATURES.md                 | LOW    | 30min  |
| 21 | 🟢 P2    | go-cqrs-lite | **Consolidate CatalogMeta** — Share between event/command/query          | LOW    | 2h     |
| 22 | 🟢 P2    | cqrs-htmx    | **Add DecodeRequest[T]** — Access body + \*http.Request                  | HIGH   | 4h     |
| 23 | 🟢 P2    | go-localsync | **Build TUI with Bubble Tea** — Interactive event browser                | LOW    | 2h     |
| 24 | 🟢 P2    | go-localsync | **Daemon/background mode** — Periodic sync as service                    | MEDIUM | 2h     |
| 25 | 🟢 P2    | go-localsync | **HTTP API endpoint** — REST via cqrs-htmx                               | MEDIUM | 4h     |

---

## g) TOP QUESTION

**Should we keep the `ConflictAwareSyncer` as a separate type, or fold its logging/metrics role into `Syncer` directly?**

Now that conflict detection is fully in the decider, `ConflictAwareSyncer` is just a thin wrapper that:

1. Calls `filterValidItems` (which `Syncer` already does)
2. Calls `SyncItems` (which `Syncer` already calls)
3. Maps `SyncAction` → `ConflictResult` metrics (trivial)
4. Logs conflict info (could be in Syncer)

The `ConflictAwareSyncer` type adds a `ConflictResult` struct with `Fetched/Upserted/Skipped/Conflicts/Errors` — but `SyncSummary` from `SyncItems` already has this data. We could:

- **Option A**: Delete `ConflictAwareSyncer`, add a `SyncWithDetails()` method to `Syncer` that returns the `SyncSummary` directly
- **Option B**: Keep it as a convenience wrapper for CLI users who want the `ConflictResult` format

The CLI `main.go` uses `--conflict-aware` flag to choose between them. If we delete it, we just change the default behavior.

---

## Session Stats

| Metric                    | Value                              |
| ------------------------- | ---------------------------------- |
| Commits                   | 8 (including this report)          |
| Net LOC change            | -935 (502 added, 1437 removed)     |
| Files changed             | 18                                 |
| Bugs fixed                | 1 (HasChanged time comparison)     |
| Architecture issues fixed | 1 (split-brain conflict detection) |
| Stale files deleted       | 5 (-1,031 lines)                   |
| Dependencies simplified   | 1 (cockroachdb/errors → stdlib)    |
| Docs rewritten            | 2 (AGENTS.md, ROADMAP.md)          |
| Test failures             | 0 (all 107 pass)                   |

---

_Generated by Crush — 2026-05-17_

---

## Resolution (2026-09-05)

The §d strikethroughs closed the same-session bugs; everything else: Turso/CLI items moot (v0.2.0), dispatcher/middleware/snapshots shipped by v0.1.0-v0.4.0, testify/stdlib unification overtaken by the test-framework maturation (ginkgo indirect only), upstream go-cqrs-lite items live in that repo. PAT smoke test is tracked in TODO_LIST (provider/github). No live items remain.
