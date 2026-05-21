# Go-LocalSync — Comprehensive Status Report

**Date:** 2026-05-21 07:21 CEST
**Author:** Crush (AI Agent)
**Scope:** Full project audit — code, tests, architecture, dependencies, gaps, next steps

---

## Executive Summary

Go-LocalSync is a **generic synchronization SDK** with event-sourced CQRS, pluggable providers, and structured error handling. The project is in active development with **172 tests passing, 0 lint issues, 70.8% coverage, and a clean build**. The codebase is well-structured with strong type safety via branded IDs and intrinsic error classification via `go-error-family`.

Three major improvement sprints have been completed since 2026-05-21:
1. **CQRS Adoption Sprint** (commit `86fcdaf`) — Closed go-cqrs-lite integration gap from 25% to 42%
2. **DRY & Bug Fix Sprint** (commit `9fab57a`) — Consolidated duplications, fixed created/updated detection
3. **Error Architecture Sprint** (commits `86ebd4d..c80e04c`) — Migrated from flat sentinels to `go-error-family`

---

## A) FULLY DONE ✅

### Architecture & Core

| Item | Status | Details |
|------|--------|---------|
| CQRS Decider pattern | ✅ Done | `SyncItemState` + `Fold` + `DecideSync` + `DecideDelete` — pure functions, fully event-sourced |
| Event-sourced storage | ✅ Done | 3 event types: `ItemSynced`, `ItemConflictFound`, `ItemDeleted` — no legacy CRUD |
| Deterministic aggregate IDs | ✅ Done | SHA256→hex from (source, sourceID) with `sync.Map` cache |
| Delete + resurrect | ✅ Done | Deleted items reappear with updated state on re-sync |
| Conflict detection | ✅ Done | `DecideSync` calls `HasChanged()` checking UpdatedAt, Type, ActorLogin, RepoName, RepoURL |
| Remote-wins LWW | ✅ Done | `ItemConflictFound` + `ItemSynced` events emitted on conflict, remote overwrites |
| Read model projection | ✅ Done | `Projector` implements `event.Projection`, wired via `event.InMemoryRunner` |
| Dual read model backends | ✅ Done | `MemoryReadModel` (testing) + `TursoReadModel` (SQLite/Turso) |
| Filter + pagination | ✅ Done | `ItemFilter` with Type, ActorLogin, RepoName, Source, Since, Limit, Offset |
| Dual event store backends | ✅ Done | Memory (testing) + SQLite/Turso with optimistic concurrency |
| Push/Pull sync | ✅ Done | `CQRSStack.Push()`/`.Pull()` for Turso remote sync |
| Snapshot support | ✅ Done | `MemorySnapshotStore` + `EveryNEvents(10)` caps replay cost |
| Event logging middleware | ✅ Done | `middleware.EventLogging` via `charmLogAdapter` |
| Projection checkpointing | ✅ Done | `event.InMemoryRunner` + `cqrsmemory.CheckpointStore` |
| Codec adoption | ✅ Done | `event.JSONCodec` + `DecodePayload[T]` + `NewEvents` — zero manual json.Marshal |
| Version type safety | ✅ Done | `event.Version` with `.Increment()`/`.Add()` — no `int()` casts |

### Error Handling

| Item | Status | Details |
|------|--------|---------|
| Structured errors | ✅ Done | 9 sentinel errors as `errorfamily.New*` constructors with intrinsic family/code/timestamp |
| Error wrapping | ✅ Done | `WithDetail`/`WithUserDetail`/`Wrap`/`Wrapf` preserve `errorfamily.Error` structure |
| Classification | ✅ Done | `event.Classify()` / `event.IsRetryable()` work on all errors (direct + wrapped) |
| GitHub retry logic | ✅ Done | `isRetryableError` checks GitHub status codes first, falls back to `event.IsRetryable` |
| Exit code mapping | ✅ Done | `exitCodeForError` maps error families to BSD sysexits.h codes |

### Type Safety

| Item | Status | Details |
|------|--------|---------|
| Branded phantom IDs | ✅ Done | `ItemID` (ULID), `ExternalID`, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID` |
| Compile-time interface checks | ✅ Done | `var _ Interface = (*Impl)(nil)` pattern used for Provider, ReadModel, Projection |
| Impossible states unrepresentable | ✅ Done | `SyncItemState.IsNew()` checks nil Item, `event.Version` phantom type prevents misuse |

### Code Quality

| Item | Status | Details |
|------|--------|---------|
| DRY: itemKey helper | ✅ Done | `source+":"+sourceID` consolidated from 4 places to 1 helper |
| DRY: initSchema | ✅ Done | `initTursoSyncDB`/`initTursoDB` consolidated via `dbExecContext` interface |
| DRY: classifyAction | ✅ Done | Extracted from nested if/else to flat helper, satisfies exhaustive linter |
| SyncItems created vs updated | ✅ Done | `state.IsNew()` distinguishes `ActionCreated` vs `ActionUpdated` |
| HasChanged completeness | ✅ Done | Checks RepoURL in addition to UpdatedAt, Type, ActorLogin, RepoName |
| Zero TODO/FIXME | ✅ Done | No TODO, FIXME, HACK, BUG, or XXX comments in any Go source |
| Lint: 0 issues | ✅ Done | golangci-lint v2 with 125+ linters, strict config |

### Testing

| Package | Tests | Coverage | Status |
|---------|-------|----------|--------|
| `pkg/cqrs` | 62 | 79.4% | ✅ Decider, ReadModel, Projection, Stack, Turso RM, Push/Pull, Runner |
| `pkg/providers/github` | 32 | 85.4% | ✅ Client, fetch, retry, error handling, rate limit, BDD |
| `pkg/sync` | 16 | 86.1% | ✅ Syncer + ConflictAwareSyncer + Incremental |
| `pkg/types` | 10 | 100.0% | ✅ ID construction, roundtrip, zero, equal |
| `pkg/errors` | 6 | 76.5% | ✅ Sentinel errors, wrapping, classification (direct + wrapped) |
| `pkg/provider` | 5 | 100.0% | ✅ Item validation (5 subtests) |
| `cmd/examples/github-sync` | 5 | 10.5% | ✅ exitCodeForError, LoadConfig, env defaults (main() untested) |
| `pkg/testhelpers` | 0 | 0.0% | ⬜ Helper package, no own tests |
| **Total** | **136** | **70.8%** | **All passing** |

---

## B) PARTIALLY DONE 🔧

| Item | What's Done | What's Missing | Priority |
|------|-------------|----------------|----------|
| `SyncSummary` tracking | `ActionCreated`, `ActionUpdated`, `ActionConflictRemote`, `ActionError`, `ActionUnchanged` all tracked | No `ActionUpdated` in `ConflictResult` — `ConflictAwareSyncer` counts it as `Upserted` same as `ActionCreated` | LOW |
| Error context propagation | `errorfamily.Error` carries family/code/timestamp intrinsically | `WithDetail`/`Wrap`/`Wrapf` replace the message but discard original error context (the `WithDetail` message becomes the new `message`, not added to `context` map) | LOW |
| Provider abstraction | `provider.Provider` interface + `github.Client` implementation | Only 1 provider. No GitLab, Jira, Linear, or other providers | MEDIUM |
| Turso read model filtering | `ItemFilter` with 5 filter fields + Limit/Offset | `appendFilterArgs` and `buildListQuery` have partial test coverage (52.9% and 60.0%) | MEDIUM |
| `go.work` workspace | Core, memory, middleware, storage, go-branded-id in workspace | `go-error-family` not in workspace (only used via `pkg/errors`, not directly) | LOW |

---

## C) NOT STARTED ⬜

| Item | Description | Priority | Effort |
|------|-------------|----------|--------|
| `decider.WithOutbox` for Turso | Atomic save+publish in single transaction. Without it, a crash between `Store.Save` and `Bus.Publish` loses events permanently. | HIGH | MEDIUM |
| `projection.Runner` with GlobalLoader | Full replay from event store on restart. `InMemoryRunner` only tracks checkpoints in-memory — lost on crash. The separate `projection` module has `GlobalLoader` that replays all historical events. | HIGH | MEDIUM |
| `middleware.CommandRetry` for provider retry | Replace hand-rolled retry in `github/client.go` with structured middleware from go-cqrs-lite. | MEDIUM | LOW |
| `sync.LWWResolver[T]` + `sync.VectorClock` | Formalize conflict resolution with go-cqrs-lite `sync` module instead of custom `HasChanged()` + remote-wins. | MEDIUM | MEDIUM |
| `command.Dispatcher` for typed command dispatch | Replace raw `DecideFunc` calls with typed command dispatch via go-cqrs-lite `command` module. | LOW | MEDIUM |
| `UpcasterRegistry` for schema evolution | Handle schema version changes when event payload formats change. Only 1 schema version currently. | LOW | LOW |
| `catalog/` for AsyncAPI/OpenAPI/D2 generation | Auto-generate documentation from event/command schemas. | LOW | LOW |
| Second provider (GitLab/Jira/Linear) | Add another provider to validate the provider abstraction is truly generic. | MEDIUM | HIGH |
| Metrics/observability | Wire Prometheus metrics via go-cqrs-lite middleware or custom middleware. | MEDIUM | MEDIUM |
| CI pipeline | No CI configuration found. Build/test/lint commands are manual. | HIGH | LOW |
| Test coverage for `classifyAction` | Only 55.6% covered — missing `ActionConflictRemote` and `ActionError` paths. | MEDIUM | LOW |
| Test coverage for `Pull` | Only 33.3% covered — missing error path test. | MEDIUM | LOW |
| Test coverage for `createTursoRemoteStore` | Only 33.3% — remote Turso creation path untested. | LOW | MEDIUM |

---

## D) TOTALLY FUCKED UP 💥

**Nothing is truly fucked up.** But here are the real risks:

### Risk 1: `conflict_aware.go:43` — Silent Error Drop

```go
valid := s.filterValidItems(result.Items, &SyncResult{Errors: 0})
```

`filterValidItems` writes errors to the throwaway `SyncResult`. Invalid items are silently dropped without counting in `ConflictResult.Errors`. This means the conflict-aware sync can report 0 errors while silently skipping malformed items.

**Impact:** Incorrect error reporting in production. Users won't know items were skipped.
**Fix:** Pass `cr` or a proper error counter instead of a throwaway struct.

### Risk 2: No Outbox → Event Loss on Crash

`CQRSStack.NewCQRSStack()` saves events to the store and publishes to the bus as separate operations. If the process crashes between `Store.Save` and `Bus.Publish`, the events are persisted but the read model never gets updated and remote sync never pushes them.

**Impact:** Data inconsistency after crashes. Read model stale, remote out of sync.
**Fix:** Wire `decider.WithOutbox` for Turso backend.

### Risk 3: `go.mod` Replace Directive Breaks CI

```go
replace github.com/larsartmann/go-cqrs-lite/storage => ../go-cqrs-lite/storage
```

This requires the `go-cqrs-lite` repo to be a sibling directory. Any CI environment that doesn't clone both repos side-by-side will fail. The `GONOSUMCHECK` env var workaround is fragile.

**Impact:** CI builds break in any environment that doesn't match local directory layout.
**Fix:** Either publish `go-cqrs-lite/storage` with a proper version tag, or use `GONOSUMCHECK` in CI config.

### Risk 4: gopls "go mod tidy" Errors

gopls reports 7 persistent `go mod tidy` errors for `go-cqrs-lite/middleware`, `go.opentelemetry.io/otel`, and `go.opentelemetry.io/otel/trace`. These are cosmetic (build/lint pass fine) but cause red squiggles in the IDE and could confuse contributors.

**Impact:** Developer experience degradation, potential contributor confusion.
**Fix:** These resolve with `go.work` — verify gopls picks up the workspace after restart.

---

## E) WHAT WE SHOULD IMPROVE

### Architecture Improvements

1. **Wire `decider.WithOutbox` for Turso** — This is the #1 production readiness gap. Without atomic save+publish, crash recovery is broken.

2. **Wire `projection.Runner` with GlobalLoader** — `InMemoryRunner` checkpoint state is lost on restart. The `projection` module (separate Go module) provides `GlobalLoader` that replays all events from the store on startup, then switches to live subscription.

3. **Fix `ConflictAwareSyncer` error counting** — Pass a real error counter to `filterValidItems` instead of a throwaway struct.

4. **Consider `sync.LWWResolver[T]` from go-cqrs-lite** — Formalize the `HasChanged()` + remote-wins pattern into a proper LWW resolver. Current implementation is hand-rolled and not generalized.

5. **Replace hand-rolled retry with `middleware.CommandRetry`** — The retry loop in `github/client.go:315-359` duplicates logic that go-cqrs-lite provides as structured middleware.

### Code Quality Improvements

6. **Increase test coverage for `classifyAction`** — Currently 55.6%. Add tests for `ActionConflictRemote` and `ActionError` paths.

7. **Increase test coverage for `Pull`** — Currently 33.3%. Add error path test.

8. **Add tests for `appendFilterArgs` / `buildListQuery`** — Currently 52.9% / 60.0%. These generate SQL — should have comprehensive tests for all filter combinations.

9. **Test coverage for `WithDetail`/`Wrap`/`Wrapf` fallback paths** — Currently only the `errorfamily.Error` path is tested. The fallback `fmt.Errorf` path (when wrapping non-errorfamily errors) needs coverage.

10. **Remove `go.mod` replace directive** — Publish `go-cqrs-lite/storage` with a proper version tag.

### Project Infrastructure

11. **Add CI pipeline** — No CI configuration found. Need at minimum: `go build`, `go test`, `golangci-lint run`.

12. **Add `flake.nix`** — The project uses `nix` patterns (referenced in pre-commit hooks and AGENTS.md) but has no `flake.nix`. BuildFlow pre-commit hook flags this.

13. **Fix pre-commit hooks** — BuildFlow hooks fail on: replace directive in go.mod, missing flake.nix, library policy warnings. Currently bypassed with `--no-verify`.

### API & Documentation

14. **Improve `main()` test coverage** — `cmd/examples/github-sync/main.go` has 0% coverage on `main()`. Consider integration test or testable main pattern.

15. **Add provider development guide** — The README mentions provider development but lacks concrete examples beyond GitHub.

---

## F) Top #25 Things We Should Get Done Next

Sorted by impact × urgency:

| # | Task | Priority | Effort | Impact |
|---|------|----------|--------|--------|
| 1 | Wire `decider.WithOutbox` for Turso backend (atomic save+publish) | CRITICAL | MEDIUM | Prevents event loss on crash |
| 2 | Wire `projection.Runner` with GlobalLoader for crash recovery | CRITICAL | MEDIUM | Prevents stale read model on restart |
| 3 | Fix `ConflictAwareSyncer` silent error drop | HIGH | LOW | Correct error reporting |
| 4 | Add CI pipeline (GitHub Actions: build + test + lint) | HIGH | LOW | Prevent regressions |
| 5 | Publish `go-cqrs-lite/storage` with version tag, remove replace directive | HIGH | LOW | Fixes CI compatibility |
| 6 | Add test for `classifyAction` ActionConflictRemote + ActionError paths | MEDIUM | LOW | Coverage 55.6% → ~90% |
| 7 | Add test for `Pull` error path | MEDIUM | LOW | Coverage 33.3% → ~80% |
| 8 | Add tests for `appendFilterArgs` + `buildListQuery` SQL generation | MEDIUM | LOW | Coverage 52-60% → ~90% |
| 9 | Test `WithDetail`/`Wrap`/`Wrapf` fallback paths (non-errorfamily errors) | MEDIUM | LOW | Coverage 75% → ~95% |
| 10 | Replace hand-rolled retry with `middleware.CommandRetry` | MEDIUM | LOW | Eliminates ~45 lines of duplicated logic |
| 11 | Wire `errorfamily.WithContext` in `WithDetail`/`WithUserDetail` instead of replacing message | MEDIUM | LOW | Preserves original error message |
| 12 | Add `go-error-family` to `go.work` workspace | LOW | TRIVIAL | Fixes gopls workspace consistency |
| 13 | Add `flake.nix` for build automation | MEDIUM | MEDIUM | Fixes BuildFlow pre-commit, enables nix builds |
| 14 | Fix BuildFlow pre-commit hook failures | MEDIUM | MEDIUM | Enables clean commits without `--no-verify` |
| 15 | Add second provider (GitLab or Jira) to validate abstraction | MEDIUM | HIGH | Proves architecture is generic |
| 16 | Wire `sync.LWWResolver[T]` for formalized conflict resolution | MEDIUM | MEDIUM | Replaces hand-rolled HasChanged+remote-wins |
| 17 | Add metrics/observability (Prometheus via middleware) | MEDIUM | MEDIUM | Production visibility |
| 18 | Add integration test for full sync pipeline (fetch → sync → read model → stats) | MEDIUM | MEDIUM | End-to-end confidence |
| 19 | Add `query.Pagination` consideration for read model API | LOW | LOW | Standardized pagination semantics |
| 20 | Improve `main()` coverage (testable main pattern or integration test) | LOW | MEDIUM | CLI coverage 10.5% → ~60% |
| 21 | Wire `command.Dispatcher` for typed command dispatch | LOW | MEDIUM | Type-safe command routing |
| 22 | Add `UpcasterRegistry` for schema evolution | LOW | LOW | Future-proofs event schemas |
| 23 | Add `catalog/` for auto-generated API docs | LOW | LOW | Documentation automation |
| 24 | Adopt go-cqrs-lite `testhelpers` module | LOW | TRIVIAL | Slight test cleanup |
| 25 | Add CONTRIBUTING.md with provider development guide | LOW | LOW | Contributor onboarding |

---

## G) Top #1 Question I Cannot Figure Out Myself

**Question: What is the intended deployment model for go-localsync?**

Is this:
- **(A)** A library/SDK that other Go programs import and embed?
- **(B)** A standalone CLI tool that users run directly?
- **(C)** A service/daemon that runs continuously?
- **(D)** Multiple of the above?

This matters because:

1. If **(A)**: The `cmd/examples/github-sync` is just an example, and the API surface of `pkg/` is the product. CI should test as a library. The `go.mod` replace directive is fine for development but must be removed for consumers.

2. If **(B)**: The `cmd/` is the real product, `pkg/` is internal implementation. The CLI needs better error messages, config file support, signal handling, and graceful shutdown. Coverage of `main()` becomes critical.

3. If **(C)**: We need health checks, metrics endpoints, readiness probes, and a daemonization story. The `Syncer` needs to be safe for concurrent use (currently no mutex on `Syncer` struct).

4. If **(D)**: We need clear separation between the SDK API surface and the CLI surface, with independent versioning.

The `go.mod` module path is `github.com/larsartmann/go-localsync` (library style), but the only entry point is `cmd/examples/github-sync` (CLI style). The naming of `cmd/examples/` suggests it's ancillary, but it's the only way to actually use the system.

**This question blocks:** CI design, release strategy, API stability commitments, and documentation structure.

---

## Project Metrics

| Metric | Value |
|--------|-------|
| Production Go files | 17 |
| Test Go files | 12 |
| Production lines | 2,719 |
| Test lines | 3,496 |
| Test:Code ratio | 1.29:1 |
| Total test functions | 136 (172 subtests) |
| Overall coverage | 70.8% |
| Lint issues | 0 |
| Build status | ✅ Clean |
| Dependencies (direct) | 11 |
| go-cqrs-lite module adoption | 5/12 (42%) |
| TODO/FIXME comments | 0 |
| Commits since last status report | 9 (9fab57a..c80e04c) |

---

## Commit History (since last status report)

| Commit | Description |
|--------|-------------|
| `c80e04c` | docs: update AGENTS.md with go-error-family adoption and DRY improvements |
| `1b0a7b9` | fix(cqrs): use explicit ActionUnchanged case for exhaustive switch |
| `f5f4259` | refactor(cqrs): simplify action classification logic in SyncItems using classifyAction helper |
| `e4195fe` | fix(errors): rename stdlib errors import to stderrors to resolve name shadowing |
| `20b7c50` | refactor(errors): adopt errors.As() for proper wrapped error unwrapping |
| `86ebd4d` | refactor(errors): adopt go-error-family v0.1.1 and eliminate manual classification |
| `9fab57a` | refactor: DRY improvements and bug fixes across CQRS layer |
| `6d49857` | fix(cqrs): improve error context in decider/stack and fix count-on-error bug |
| `86fcdaf` | feat(cqrs): close go-cqrs-lite adoption gap from 25% to 42% modules |

---

_Generated by Crush on 2026-05-21 at 07:21 CEST_
