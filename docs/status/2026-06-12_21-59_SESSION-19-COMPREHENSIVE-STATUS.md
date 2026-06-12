# Status Report — go-localsync

**Date:** 2026-06-12 21:59
**Session:** 19
**Branch:** master (up to date with origin)
**Latest commit:** `80f799a style: fix wsl_v5 lint warnings in model/item.go validateIdentity`
**Build:** `go build ./...` — PASS
**Tests:** 285 test functions, all PASS
**Lint:** golangci-lint v2 — 0 issues (SA5012 panics are known linter bug)
**go vet:** PASS

---

## Project At a Glance

| Metric                      | Value                                                                         |
| --------------------------- | ----------------------------------------------------------------------------- |
| Production code             | 4,980 lines (52 files)                                                        |
| Test code                   | 7,758 lines (46 files)                                                        |
| Test:prod ratio             | 1.56:1                                                                        |
| Packages                    | 10 production packages + 1 example CLI                                        |
| Test functions              | 285                                                                           |
| Exported types              | 68                                                                            |
| Interfaces                  | 5 (`Provider`, `SyncStore`, `ReadModel`, `ConflictResolver[T]`, `ItemReader`) |
| Direct dependencies         | 20                                                                            |
| Commits since June 11       | 13                                                                            |
| TODO/FIXME/HACK in code     | **0**                                                                         |
| Files over 300 lines (prod) | **0**                                                                         |

### Coverage by Package

| Package                    | Coverage | Tests |
| -------------------------- | -------- | ----- |
| `pkg/data/model`           | 100.0%   | 14    |
| `pkg/data/schema`          | 100.0%   | 0     |
| `pkg/errors`               | 100.0%   | 9     |
| `pkg/id`                   | 100.0%   | 12    |
| `pkg/crdt`                 | 96.2%    | 57    |
| `pkg/api`                  | 93.9%    | 15    |
| `pkg/provider`             | 90.9%    | 2     |
| `pkg/providers/github`     | 84.4%    | 32    |
| `pkg/sync`                 | 85.4%    | 24    |
| `pkg/cqrs`                 | 86.4%    | 98    |
| `cmd/examples/github-sync` | 12.3%    | 14    |

### Package Size (Production Lines)

| Package                | Files          | Lines |
| ---------------------- | -------------- | ----- |
| `pkg/cqrs`             | 19             | 1,891 |
| `pkg/crdt`             | 6              | 614   |
| `pkg/data`             | 2 sub-packages | 243   |
| `pkg/sync`             | 4              | 465   |
| `pkg/providers/github` | 3              | 492   |
| `pkg/api`              | 4              | 335   |
| `pkg/testutil`         | 3              | 205   |
| `pkg/errors`           | 2              | 157   |
| `pkg/provider`         | 1              | 156   |
| `pkg/id`               | 1              | 88    |

---

## a) FULLY DONE

These items are complete, tested, committed, and pushed.

### Session 18–19 Work (June 12)

1. **CRDT Operation.Deserialize validation** — `DeserializeOperation` now validates ID, NodeID, Type after JSON unmarshal. Prevents silent zero-value operations from entering the system. `operation.go:66-80`.
2. **Health endpoint probes store** — `GET /health` now calls `store.Count()` and returns 503 on failure instead of always returning 200. `handlers.go:16-33`.
3. **slog→charm.log in runner.go** — Replaced `"log/slog"` with `charm.land/log/v2` in projection runner goroutine error path. Only `stack.go` retains slog (bridges to `*slog.Logger` for `middleware.EventLogging`).
4. **Context propagation in factory functions** — `newSQLiteReadModel`, `createSQLiteStore`, `createStoreAndBus`, `createReadModel` all accept `ctx context.Context` parameter instead of internal `context.Background()`. Tests updated. Remaining `context.Background()` calls are in appropriate places (entry points, goroutine roots, oauth2 client).
5. **FetchOptions.Source type consistency** — `client.go` uses `opts.Source.Get()` where string is expected (GitHub API, error wrapping). All test files wrap source strings with `id.NewProviderID()`.
6. **Item.Validate collects all errors** — `errors.Join` returns all validation errors at once instead of stopping at first.
7. **wsl_v5 lint fix** — Added blank line above `if` in `model/item.go` `validateIdentity`.

### Sessions 4–17 Historical Work (Completed)

- **Session 4:** SyncStore interface seam, SyncAction/ItemSyncResult moved to seam, testhelpers deduplicated
- **Session 5:** HTTP API (4 endpoints), CLI server mode, JSON output, error templates, flake.nix
- **Session 6:** CRDT conflict resolution wired as pluggable strategy, 13 new tests
- **Session 7:** conflict_aware.go extracted, CLI helpers, domain language documented
- **Session 8:** go-cqrs-lite v2 migration (11 modules), turso→sqlite rename, dead config removed
- **Session 9:** Correctness fixes (rows.Err, nil Item on delete, indexes, panic→error)
- **Session 10:** SyncItems through command pipeline, SyncOutcome, compile-time assertion, runner error logging
- **Session 11:** Data module types (Item, Key, schema.Version), MockProvider consolidation
- **Session 12:** Deduplication pass (96→73 assertion groups)
- **Session 13:** Dead types removed, ConflictStrategy CLI, HasChanged tests, benchmarks, graceful shutdown
- **Session 14:** Dead Get\*() methods removed, ItemFilter moved to model, concurrent access tests, mapSyncError tests
- **Session 15:** File splits (3 large files → 9 focused), ADR-001/002/003, API error path tests
- **Session 16:** Deep audit, WaitForCount fix, go.mod version fix, UpdatedAt validation
- **Session 17:** UpdatedAt validation, WaitForCount busy-spin, go.mod fix, item/limit audit

---

## b) PARTIALLY DONE

### Context.Background() Reduction (60% done)

**Status:** Factory functions cleaned up. Remaining 6 call sites are appropriate.

| Location                                    | Justification                                  | Verdict    |
| ------------------------------------------- | ---------------------------------------------- | ---------- |
| `pkg/providers/github/client.go:37`         | oauth2.NewClient has no ctx param — acceptable | Keep       |
| `pkg/cqrs/runner.go:38`                     | Goroutine root context for projection runner   | Acceptable |
| `pkg/cqrs/stack.go:68`                      | Stack constructor entry point                  | Acceptable |
| `cmd/examples/github-sync/helpers.go:43,48` | CLI entry point — no request context available | Acceptable |
| `cmd/examples/github-sync/helpers.go:173`   | Shutdown timeout context — correct usage       | Acceptable |
| `cmd/examples/github-sync/main.go:161`      | Main goroutine root context                    | Acceptable |

### Logging Standardization (90% done)

- `charm.land/log/v2` used in 11 production files
- `log/slog` only in `pkg/cqrs/stack.go` — required bridge for `middleware.EventLogging`
- No `fmt.Println` or `log.Println` anywhere in production code
- **Remaining:** Consider contributing an adapter to go-cqrs-lite that accepts `charm.land/log/v2` natively

### nolint Directives (95% documented)

13 `nolint` directives in production code, all with inline explanations. Clean and justified.

---

## c) NOT STARTED

### High-Impact Features

1. **Second provider** (GitLab, Jira, etc.) — SDK is generic but only GitHub exists. Zero code.
2. **OpenTelemetry instrumentation** — No spans, metrics, or traces anywhere. Production debugging requires log spelunking.
3. **API authentication** — No auth middleware. Unsafe to expose on a network.
4. **API rate limiting** — No protection against POST /sync abuse.
5. **Daemon/background mode** — No cron, systemd, or scheduler integration. Manual execution only.
6. **Multi-user sync** — CLI accepts single `-user`. No multi-user support.
7. **Data export** — No JSON/CSV export of stored events.
8. **Real-time sync protocol** — `SyncRequest`/`SyncResponse` types exist in `pkg/crdt/` but are unused. Multi-node sync is planned, not implemented.

### Code Quality

9. **`cmd/examples/github-sync` coverage (12.3%)** — Main flow (`runSync`, `runStats`, signal handling) is untested. Functions call `os.Exit()` which needs process-level isolation.
10. **Test framework unification** — 1 file uses Ginkgo, rest uses stdlib. Not urgent but inconsistent.
11. **`go:generate` directives** — None exist. No code generation pipeline.
12. **CONTRIBUTING.md architecture guide** — Basic file exists but lacks architecture guide, file size limits, testing requirements.

### Upstream Adoption

13. **`middleware.CommandRetry`** from go-cqrs-lite — API mismatch blocks adoption
14. **`UpcasterRegistry`** from go-cqrs-lite — For schema evolution
15. **`catalog/`** from go-cqrs-lite — AsyncAPI/OpenAPI/D2 generation
16. **`signing/v2`** — Ed25519 event signing for multi-node trust
17. **`otel/v2`** — OpenTelemetry integration from go-cqrs-lite

---

## d) TOTALLY FUCKED UP

### Nothing Is Actually Fucked

The codebase is in the best shape it has ever been. Build passes, all 285 tests pass, zero lint issues, zero TODO comments, no files over 300 lines. However, there are things that deserve honest calling out:

### Honest Problems

1. **Session 18 left a broken build between sessions** — The previous session removed the `"log/slog"` import from `stack.go` but `newSlogLogger()` still needed it. The build was broken at conversation start. This was fixed in session 19 but it should never have happened. Lesson: **always run `go build ./...` before ending a session.**

2. **Session 18 bundled everything into one commit** — Six distinct changes (CRDT validation, health probe, slog refactor, context propagation, ProviderID fix, test fix) were committed as a single monolithic commit `80409a9`. This makes git bisect and blame harder. Lesson: **commit each logical change separately.**

3. **`cmd/examples/github-sync` at 12.3% coverage** — This is the only entry point into the entire system and it's barely tested. The main sync/stats/server flows are completely untested. This is a risk.

4. **CRDT `pkg/crdt/` is 614 lines but no real multi-node sync** — The package implements VectorClock, Operation[T], SyncMessage, ConflictResolver — all the building blocks for distributed sync. But none of it is actually used for real multi-node sync. The only real use is `LWWResolver` which just compares timestamps. The rest is speculative infrastructure. This is YAGNI until there's a concrete multi-node use case.

5. **Only one provider** — The entire SDK architecture is designed for pluggable providers, but only GitHub exists. The provider interface has never been validated with a second implementation. There may be hidden assumptions.

6. **No real GitHub API smoke test** — All testing is mock-based. The system has never been verified against a real GitHub PAT. Mock-passing ≠ working.

### Panic Calls in Production Code

6 `panic()` calls remain. All are in initialization/factory paths (must-construct helpers), not in runtime hot paths:

- `pkg/cqrs/commands.go:25` — `mustNewCommand` helper (programmer error if fails)
- `pkg/cqrs/queries.go:23` — `mustNewQuery` helper (programmer error if fails)
- `pkg/crdt/operation.go:86` — `MustNewOperation` (explicitly named Must\*)
- `pkg/crdt/types.go:21,51` — `MustNewVectorClock`, `MustNewSyncMessage`
- `pkg/testutil/testutil.go:119` — `BuildPair` argument validation

These are acceptable Go convention for "this should never happen" paths.

---

## e) WHAT WE SHOULD IMPROVE

### Architecture

1. **Extract `pkg/crdt` into its own repository** — The CRDT types (VectorClock, Operation, ConflictResolver) are generic data structures with no dependency on go-localsync. They could be reused by other projects. Currently they're coupled to this repo's release cycle.
2. **Consider removing `pkg/testutil`** — Only 205 lines, used by github provider tests. Could be inlined as unexported test helpers. A public testutil package creates an API surface commitment.
3. **The `pkg/data` package is split into `data/model` and `data/schema`** — Only 243 lines total. Consider merging into a single `data` package or flattening into `model`.

### Code Quality

4. **Add integration test that exercises the full stack** — Provider → Syncer → CQRSStack → ReadModel → API handler. Currently each layer is tested in isolation. A full-stack integration test would catch wiring bugs.
5. **Standardize error construction** — Some packages use `fmt.Errorf`, others use `go-error-family` constructors. Be consistent about when to use which.
6. **Add `String()` methods to all branded IDs** — `pkg/id` has 6 branded types. `ItemID` has a `String()` method, but others may not. Consistent stringer support aids logging.

### Testing

7. **Increase `cmd/examples/github-sync` coverage** — Extract testable logic from `main()`, use subprocess tests for `os.Exit()` paths.
8. **Add property-based tests for CRDT** — VectorClock merge is a CRDT — it should satisfy mathematical properties (commutativity, associativity, idempotency). Quick/rapidcheck tests would catch subtle bugs.
9. **Add chaos/fault-injection tests** — What happens when the SQLite file is corrupted? When the event store returns partial writes? These edge cases are untested.

### Documentation

10. **AGENTS.md is 476 lines** — It has become a changelog rather than enduring context. Sessions 4–18 are historical. Consider archiving old sessions and keeping only the architecture tables and current state.
11. **No CONTRIBUTING.md architecture guide** — New contributors (or AI sessions) need to understand the layered architecture at a glance.
12. **Too many status reports in `docs/status/`** — 37 status reports across 6 weeks. Most are historical. Consider archiving old ones.

### Dependency Management

13. **go-cqrs-lite is pinned to pseudo-versions** — Uses `go.work` for local development. CI uses `GONOSUMCHECK`. This works but prevents reproducible builds without the workspace.
14. **20 direct dependencies** — Reasonable for the feature set, but each is a maintenance commitment. Evaluate if all are still needed.

---

## f) Top #25 Things We Should Get Done Next

### Tier 1: High Impact, Low Effort (< 1 hour each)

| #   | Task                                                  | Why                                                                                                    | Effort |
| --- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------ | ------ |
| 1   | **Smoke test with real GitHub PAT**                   | Never validated against real API. Mock-passing ≠ working.                                              | 30 min |
| 2   | **Add full-stack integration test**                   | Provider→Syncer→CQRS→ReadModel→API in one test. Catches wiring bugs.                                   | 45 min |
| 3   | **Extract testable logic from `main.go`**             | Move `runSync`/`runStats`/`runAPIServer` logic to testable functions. Don't call `os.Exit()` directly. | 45 min |
| 4   | **Archive old sessions from AGENTS.md**               | Move sessions 4–16 to `docs/status/archive/`. Keep only architecture + current state.                  | 20 min |
| 5   | **Archive old status reports**                        | Move pre-June reports to `docs/status/archive/`. Keep last 5.                                          | 10 min |
| 6   | **Add `String()` to all branded IDs**                 | Consistent logging. Check `ExternalID`, `ProviderID`, `ActorID`, `RepoID`, `EventTypeID`.              | 15 min |
| 7   | **Add API pagination headers**                        | `X-Total-Count` header on `GET /items`. Standard REST convention.                                      | 20 min |
| 8   | **Clean up remaining `context.Background()` in cmd/** | Pass ctx from main through helpers instead of creating new backgrounds.                                | 30 min |

### Tier 2: High Impact, Medium Effort (1–4 hours each)

| #   | Task                                                     | Why                                                                                        | Effort |
| --- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------ |
| 9   | **OpenTelemetry instrumentation**                        | No observability. Production debugging is blind. Start with sync + HTTP middleware spans.  | 3 h    |
| 10  | **API authentication middleware**                        | API is unsafe on any network. API key middleware is minimal.                               | 2 h    |
| 11  | **Build a second provider** (GitLab or Jira)             | Validates the provider abstraction isn't GitHub-specific. Forces interface clarity.        | 3 h    |
| 12  | **Increase `cmd/examples/github-sync` coverage to 50%+** | Entry point is barely tested. Subprocess tests for `os.Exit()` paths.                      | 2 h    |
| 13  | **Property-based tests for CRDT VectorClock**            | Verify CRDT mathematical properties (commutativity, associativity, idempotency).           | 2 h    |
| 14  | **CONTRIBUTING.md architecture guide**                   | Layered architecture diagram, file naming conventions, testing requirements.               | 1.5 h  |
| 15  | **Add structured logging fields consistently**           | Add source, user, page, event_id fields to all log statements for filterability.           | 2 h    |
| 16  | **Resolve go-cqrs-lite upstream WIP**                    | `Sink→EventSink` rename + Source type collision blocks upgrades. Coordinate with upstream. | 2 h    |

### Tier 3: Strategic, Higher Effort (4+ hours each)

| #   | Task                                     | Why                                                                                      | Effort |
| --- | ---------------------------------------- | ---------------------------------------------------------------------------------------- | ------ |
| 17  | **Daemon/background mode**               | Periodic sync without manual execution. Cron or systemd integration.                     | 4 h    |
| 18  | **Real-time multi-node sync protocol**   | Use the CRDT infrastructure that already exists. Wire `SyncMessage` over WebSocket/gRPC. | 8 h    |
| 19  | **Extract `pkg/crdt` to own repository** | Generic data structures shouldn't be coupled to go-localsync releases.                   | 4 h    |
| 20  | **Multi-user sync**                      | Accept multiple users, track which user each event belongs to in read model.             | 6 h    |
| 21  | **Data export (JSON/CSV)**               | Export stored events for external analysis. Simple but useful.                           | 3 h    |
| 22  | **Chaos/fault-injection test suite**     | Corrupted SQLite, partial writes, concurrent schema changes. Hardens the system.         | 4 h    |
| 23  | **Build TUI with Bubble Tea**            | Interactive terminal UI for browsing events and real-time sync.                          | 4 h    |
| 24  | **API rate limiting middleware**         | Protect POST /sync from abuse. Token bucket or sliding window.                           | 2 h    |
| 25  | **Adopt `catalog/` from go-cqrs-lite**   | Auto-generate AsyncAPI/OpenAPI specs from event catalog.                                 | 3 h    |

---

## g) Top #1 Question I Cannot Figure Out Myself

**What is the actual target use case for this project?**

The codebase has two very different trajectories baked in:

1. **Local-first personal sync tool** — Single user, local SQLite, GitHub events, CLI-driven. This is what actually works today.
2. **Distributed multi-node sync platform** — CRDT types, VectorClocks, SyncMessages, ConflictResolvers, branded IDs. This is what the infrastructure suggests.

The tension matters because:

- If it's (1), then `pkg/crdt/` is 614 lines of YAGNI. The provider abstraction may be over-engineered. OpenTelemetry and auth are less urgent.
- If it's (2), then the CLI example needs to become a real daemon, multi-user support is critical, and the CRDT package needs real wire protocol work.

**The codebase can't decide, and neither can I.** The architectural investments (CRDT, branded IDs, event sourcing, provider abstraction) suggest ambition far beyond a personal CLI tool. But nothing in the roadmap or featureset actually uses that infrastructure yet.

This is the single most important decision to make because it determines whether items 9, 11, 17, 18, 19, 20 are strategic priorities or speculative waste.

---

## Build & Test Verification

```
$ go build ./...           # PASS
$ go test ./... -count=1   # ALL PASS (285 tests)
$ go vet ./...             # PASS
```

### Test Output

```
ok  github.com/larsartmann/go-localsync/cmd/examples/github-sync  coverage: 12.3%
ok  github.com/larsartmann/go-localsync/pkg/api                   coverage: 93.9%
ok  github.com/larsartmann/go-localsync/pkg/cqrs                  coverage: 86.4%
ok  github.com/larsartmann/go-localsync/pkg/crdt                  coverage: 96.2%
ok  github.com/larsartmann/go-localsync/pkg/data/model            coverage: 100.0%
ok  github.com/larsartmann/go-localsync/pkg/data/schema           coverage: 100.0%
ok  github.com/larsartmann/go-localsync/pkg/errors                coverage: 100.0%
ok  github.com/larsartmann/go-localsync/pkg/id                    coverage: 100.0%
ok  github.com/larsartmann/go-localsync/pkg/provider              coverage: 90.9%
ok  github.com/larsartmann/go-localsync/pkg/providers/github      coverage: 84.4%
ok  github.com/larsartmann/go-localsync/pkg/sync                  coverage: 85.4%
```

---

## Commit History (Last 5)

```
80f799a style: fix wsl_v5 lint warnings in model/item.go validateIdentity
8617217 refactor: provider.Item.Validate collects all errors via errors.Join
80409a9 fix: complete FetchOptions.Source branding, add self-review plan
2e5c481 refactor: collect all validation errors, document stack_adapters query bypass
226adc5 refactor: adopt RegisterTyped APIs, brand SourceID, add String methods, rename validation sentinel
```

---

_Generated: 2026-06-12 21:59_
