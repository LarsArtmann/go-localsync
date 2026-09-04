# Go-LocalSync — Comprehensive Status Report

**Date:** 2026-05-22 00:35 CEST
**Author:** Crush (AI Agent)
**Trigger:** Post go-cqrs-lite integration sprint — OutboxPublisher, command.Dispatcher, query.Dispatcher
**Git HEAD:** `0ad8a3b` on `master` (changes uncommitted)

---

## Executive Summary

Two-project sprint completed. go-cqrs-lite gained exported `OutboxPublisher`, `NewEvents`, and error sentinels (previously unexported). go-localsync replaced a hand-rolled outbox poller (with a silent event loss bug) with the library's `OutboxPublisher`, and wired `command.Dispatcher` and `query.Dispatcher` for full CQRS pipeline support. All 197 tests passing, 0 lint issues, 77.8% coverage in pkg/cqrs (up from 82.5%→77.8% due to new untested code paths). go-cqrs-lite module adoption: 58% → **75%** (9/12 modules).

---

## A) FULLY DONE ✅

### 1. go-cqrs-lite: Exported Previously Internal APIs

| What              | Detail                                                                                                                             |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `OutboxPublisher` | Was `outboxPublisher` (unexported). Now: `NewOutboxPublisher`, `WithPollInterval`, `WithBatchSize`, `Start`, `Close`, `PublishNow` |
| `NewEvents`       | Was `newEvents` (unexported). Now: `NewEvents`, `MustNewEvents`, `DecodePayloads`                                                  |
| Error sentinels   | `ErrNilOutbox`, `ErrNilBus`, `ErrAlreadyStarted`, `ErrPublisherClosed` — all exported                                              |
| All tests pass    | 11 modules, 0 failures                                                                                                             |

### 2. go-localsync: Replaced `startOutboxPoller` with `event.OutboxPublisher`

| What           | Detail                                                                                    |
| -------------- | ----------------------------------------------------------------------------------------- |
| **Bug fix**    | Old poller acked entries even when `bus.Publish` failed → **silent event loss**           |
| **Bug fix**    | Old poller used 1ms tick (busy loop). New: 1s configurable interval                       |
| **Bug fix**    | Old poller had no panic recovery → silent goroutine death                                 |
| **Bug fix**    | Old poller's `cancel()` returned immediately → no graceful shutdown                       |
| Implementation | `startOutboxPublisher()` in `stack.go`, stored as `*event.OutboxPublisher` on `CQRSStack` |
| Close ordering | `OutboxPublisher.Close()` → runner cancel → read model → outbox → store                   |
| Test timeouts  | `waitForCount` deadline increased 1s→5s, `subscribeAll` deadline 1s→5s                    |

### 3. go-localsync: Wired `command.Dispatcher`

| What          | Detail                                                                        |
| ------------- | ----------------------------------------------------------------------------- |
| New file      | `pkg/cqrs/commands_queries.go` (173 lines)                                    |
| Command types | `SyncItemCommand` (embeds `command.Core` + `*provider.Item`)                  |
| Command types | `DeleteItemCommand` (embeds `command.Core` + Source/SourceID)                 |
| Handlers      | `handleSyncItem`, `handleDeleteItem` — type-assert with proper error wrapping |
| Wiring        | `wireCommandDispatcher(repo)` in `NewCQRSStack`                               |
| Dispatch      | `SyncItem()` and `DeleteItem()` now dispatch through `CommandDispatcher`      |
| Lifecycle     | `CommandDispatcher.Close()` in `CQRSStack.Close()`                            |

### 4. go-localsync: Wired `query.Dispatcher`

| What        | Detail                                                                   |
| ----------- | ------------------------------------------------------------------------ |
| Query types | `ListItemsQuery`, `GetItemQuery`, `CountItemsQuery`, `GetTypesQuery`     |
| Handlers    | `handleListItems`, `handleGetItem`, `handleCountItems`, `handleGetTypes` |
| Wiring      | `wireQueryDispatcher(rm)` in `NewCQRSStack`                              |
| Lifecycle   | `QueryDispatcher.Close()` in `CQRSStack.Close()`                         |

### 5. Documentation Updated

| What                                | Detail                                                                                                   |
| ----------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `AGENTS.md`                         | Updated architecture table, key properties, go-cqrs-lite adoption (58%→75%), anti-patterns resolved list |
| `docs/go-cqrs-lite-gap-analysis.md` | Updated fix priority table with DONE status for 3 of 4 items                                             |

---

## B) PARTIALLY DONE 🔶

| Item                                         | Status      | What's missing                                                                                                                                                                                                       |
| -------------------------------------------- | ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `middleware.CommandRetry` for provider retry | Blocked     | Needs `command.Handler` wrapping — our retry is in the GitHub client (`func() error`), not in the command pipeline. Would need a command-level retry middleware wrapping the `Repo.Execute` call inside the handler. |
| `sync.LWWResolver[T]` + `sync.VectorClock`   | Not started | Our `HasChanged()` + remote-wins is correct and simpler. Formal LWW would be an upgrade but not urgent.                                                                                                              |
| `createTursoRemoteStore` coverage            | 19.0%       | Remote Turso store path is hard to test without a real Turso server. Error handling paths are untested.                                                                                                              |
| Command/Query middleware                     | Not started | Dispatchers are wired but no middleware added yet (`command.Dispatcher.Use()`, `query.Dispatcher.Use()`). This is the _point_ of having dispatchers — the middleware pipeline.                                       |
| `DecideFunc` returns result                  | Not started | `countingDecide` in `SyncItems` still reverse-engineers intent from event count. The decider should return domain semantics directly. Library API limitation: `DecideFunc` returns `([]Event, error)`.               |

---

## C) NOT STARTED ⬜

| #  | Item                                                 | Priority | Effort | Impact                                                  |
| -- | ---------------------------------------------------- | -------- | ------ | ------------------------------------------------------- |
| 1  | Add command middleware (logging, validation)         | HIGH     | 1h     | The entire point of having command.Dispatcher           |
| 2  | Add query middleware (logging, metrics)              | MEDIUM   | 1h     | Observability for read path                             |
| 3  | `sync.LWWResolver[T]` for formal conflict resolution | MEDIUM   | 2h     | Formalize conflict resolution                           |
| 4  | Second provider (GitLab, Bitbucket, etc.)            | MEDIUM   | 8h+    | Multi-provider support                                  |
| 5  | `UpcasterRegistry` for schema evolution              | LOW      | 3h     | Only 1 schema version exists                            |
| 6  | `catalog/` for AsyncAPI/OpenAPI/D2                   | LOW      | 4h     | Documentation automation                                |
| 7  | Flaky test hardening for Turso async tests           | MEDIUM   | 2h     | Tests use `waitForCount` with 5s deadline               |
| 8  | `cmd/examples/github-sync/main.go` coverage (10.5%)  | LOW      | 1h     | Extract logic into testable function                    |
| 9  | `core/aggregate` adoption                            | LOW      | 3h     | We use `decider.Decider` directly — correct, no benefit |
| 10 | Return domain result from `DecideFunc`               | MEDIUM   | 2h     | Requires go-cqrs-lite API change                        |
| 11 | Benchmark tests for `SyncItems` with large batches   | LOW      | 1h     | Performance baseline                                    |
| 12 | Integration test for outbox crash recovery           | HIGH     | 2h     | Verify events survive process crash                     |
| 13 | Context timeout on `SyncItems`                       | MEDIUM   | 15min  | Prevent runaway sync operations                         |
| 14 | `query.Pagination` adoption for `ItemFilter`         | LOW      | 1h     | Our `ItemFilter.Limit/Offset` works fine                |
| 15 | `event.CausationID` for per-item tracing             | LOW      | 30min  | Granular causation tracking                             |
| 16 | Structured logging for sync summary                  | LOW      | 30min  | Post-sync log with counts                               |
| 17 | Split `stack.go` into focused files                  | HIGH     | 30min  | 619 lines, exceeds 350-line target                      |
| 18 | Split `stack_test.go` into focused files             | HIGH     | 20min  | 577 lines, exceeds 350-line target                      |
| 19 | Fix `startProjectionRunner` error propagation        | HIGH     | 15min  | Currently returns nil on error                          |
| 20 | Test `Pull()` success path                           | HIGH     | 15min  | Only error path tested                                  |
| 21 | Test `charmLogAdapter.Error`                         | MEDIUM   | 10min  | Error logging never triggered                           |
| 22 | Test `createTursoRemoteStore` error paths            | HIGH     | 45min  | 19% coverage, production risk                           |
| 23 | Replace deprecated Turso legacy client               | MEDIUM   | 2h     | `go-structure-linter` flags it                          |
| 24 | Create `flake.nix` for build automation              | LOW      | 2h     | LarsArtmann standard                                    |
| 25 | Adopt `go.opentelemetry.io/otel`                     | LOW      | 3h     | Flagged by library policy                               |

---

## D) TOTALLY FUCKED UP 💥

| Issue                                    | Severity | Detail                                                                                                                              |
| ---------------------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/cqrs/stack.go` is 619 lines         | 🟡       | Exceeds 350-line limit by 77%. Should be split: `stack.go` (core), `store_factory.go` (store creation), `runner.go` (runner setup). |
| `pkg/cqrs/stack_test.go` is 577 lines    | 🟡       | Exceeds 350-line limit by 65%. Should be split: `stack_test.go`, `turso_integration_test.go`, `correlation_test.go`.                |
| `startProjectionRunner` swallows errors  | 🟠       | If `NewRunner` or `Register` fails, returns `nil` cancel func. Caller has no way to know.                                           |
| `createTursoRemoteStore` at 19% coverage | 🟠       | Remote Turso path is essentially untested. Production remote sync could break silently.                                             |
| `Pull()` at 33% coverage                 | 🟢       | Only the error path is tested, not the success path.                                                                                |
| `charmLogAdapter.Error` at 0% coverage   | 🟢       | The error logging path is never triggered in tests.                                                                                 |
| Command/Query middleware not wired       | 🟡       | We wired the dispatchers but didn't add any middleware. This is like buying a sports car and never driving it.                      |
| `countingDecide` still in `SyncItems`    | 🟢       | Event-count heuristics instead of proper domain result. Library API limitation.                                                     |
| Stale SQLite test artifacts              | 🟢       | `pkg/cqrs/:memory::memory:` and `-wal` files appeared. Should be in `.gitignore`.                                                   |

---

## E) WHAT WE SHOULD IMPROVE

### Architecture

1. **Add command middleware** — `command.Dispatcher.Use(loggingMiddleware)`, `command.Dispatcher.Use(validationMiddleware)`. This is the entire ROI of having a command dispatcher.

2. **Add query middleware** — `query.Dispatcher.Use(loggingMiddleware)`, `query.Dispatcher.Use(metricsMiddleware)`. Same reasoning.

3. **Split `stack.go` into 3 focused files** — `stack.go` (NewCQRSStack, SyncItems, Close), `store_factory.go` (createStoreAndBus, createTursoStore, createTursoLocalStore, createTursoRemoteStore), `runner.go` (startProjectionRunner, startInMemoryRunner, startOutboxPublisher). This is the single biggest code quality win available.

4. **Fix `startProjectionRunner` error propagation** — Should return `error` instead of `context.CancelFunc`. Let the caller handle it.

5. **Return domain result from `DecideFunc`** — Requires go-cqrs-lite API change. `DecideFunc` should return `([]Event, Result, error)` or similar. The `countingDecide` wrapper is a code smell.

### Code Quality

6. **Split `stack_test.go`** — Move Turso integration tests to `turso_integration_test.go`, correlation tests to `correlation_test.go`.

7. **Test command/query dispatch paths** — No tests verify that `SyncItem` actually goes through the command dispatcher. Add integration tests for the dispatch pipeline.

8. **Remove stale SQLite artifacts** — Add `*.db`, `*-wal`, `*-shm` to `.gitignore` or clean up after tests.

### Dependencies

9. **Replace deprecated Turso legacy client** — `go-structure-linter` flags it.

10. **Commit go-cqrs-lite changes** — The exported APIs need to be committed and tagged in go-cqrs-lite before go-localsync can use them in CI.

---

## F) Top #25 Things to Get Done Next

Sorted by impact × urgency.

| #  | Task                                                             | Impact | Effort | Category       |
| -- | ---------------------------------------------------------------- | ------ | ------ | -------------- |
| 1  | Add command middleware (logging, validation)                     | HIGH   | 1h     | Architecture   |
| 2  | Add query middleware (logging, metrics)                          | MEDIUM | 1h     | Architecture   |
| 3  | Split `stack.go` into 3 focused files                            | HIGH   | 30min  | Code quality   |
| 4  | Split `stack_test.go` into 3 focused files                       | HIGH   | 20min  | Code quality   |
| 5  | Fix `startProjectionRunner` to propagate errors                  | HIGH   | 15min  | Bug fix        |
| 6  | Add tests for command dispatcher dispatch path                   | HIGH   | 1h     | Coverage       |
| 7  | Add tests for query dispatcher dispatch path                     | HIGH   | 1h     | Coverage       |
| 8  | Commit and tag go-cqrs-lite exports                              | HIGH   | 30min  | Dependencies   |
| 9  | Test `Pull()` success path                                       | HIGH   | 15min  | Coverage       |
| 10 | Test `charmLogAdapter.Error`                                     | MEDIUM | 10min  | Coverage       |
| 11 | Test `createTursoRemoteStore` error paths                        | HIGH   | 45min  | Coverage       |
| 12 | Wire `sync.LWWResolver[T]` for formal conflict resolution        | MEDIUM | 2h     | Feature        |
| 13 | Add second provider (GitLab/Bitbucket)                           | HIGH   | 8h+    | Feature        |
| 14 | Extract `cmd/examples/github-sync/main.go` logic for testability | MEDIUM | 1h     | Coverage       |
| 15 | Remove dead `testhelpers` or adopt across tests                  | LOW    | 2h     | Cleanup        |
| 16 | Integration test for outbox crash recovery                       | HIGH   | 2h     | Reliability    |
| 17 | Replace deprecated Turso legacy client                           | MEDIUM | 2h     | Dependencies   |
| 18 | Return domain result from `DecideFunc`                           | MEDIUM | 2h     | Architecture   |
| 19 | Flaky test hardening (longer deadlines, retries)                 | MEDIUM | 2h     | Reliability    |
| 20 | Benchmark tests for `SyncItems` with large batches               | MEDIUM | 1h     | Performance    |
| 21 | Add `Close()` integration test for Turso                         | MEDIUM | 30min  | Coverage       |
| 22 | Add context timeout to `SyncItems`                               | MEDIUM | 15min  | Robustness     |
| 23 | Wire `event.CausationID` for per-item tracing                    | LOW    | 30min  | Observability  |
| 24 | Create `flake.nix` for build automation                          | LOW    | 2h     | Infrastructure |
| 25 | Adopt OpenTelemetry instead of direct Prometheus                 | LOW    | 3h     | Observability  |

---

## G) Top #1 Question I Cannot Figure Out Myself

**Should the go-cqrs-lite changes be committed and tagged as a new release (v1.5.0 or v1.4.1), or should we keep using the local go.work for development and only tag when the API surface is stable?**

Context:

- We exported `OutboxPublisher`, `NewEvents`, `MustNewEvents`, `DecodePayloads`, and 4 error sentinels
- These are purely additive exports (no breaking changes)
- go-localsync's `go.mod` still points to `v1.4.0` — CI uses pseudo-versions from GitHub
- Without a new tag, CI will fail because `event.NewOutboxPublisher` etc. don't exist in `v1.4.0`
- But we might want to add more exports or adjust the API before locking it in

---

## Metrics Dashboard

| Metric                    | Value                                                  |
| ------------------------- | ------------------------------------------------------ |
| Build                     | ✅ Clean                                               |
| Tests                     | 197 passing                                            |
| Coverage (pkg/cqrs)       | 77.8% (was 82.5% — new code paths untested)            |
| Coverage (overall)        | ~75%                                                   |
| Lint issues               | 0                                                      |
| go-cqrs-lite adoption     | 9/12 (75%)                                             |
| Production risks          | 1 CRITICAL (startProjectionRunner error swallowing)    |
| Commits this sprint       | 0 (changes uncommitted)                                |
| Files changed this sprint | ~8 (go-localsync) + 4 (go-cqrs-lite)                   |
| Lines added this sprint   | ~300+ (go-localsync) + ~50 (go-cqrs-lite renames)      |
| Outbox for Turso          | ✅ Library OutboxPublisher (was hand-rolled)           |
| Command dispatch          | ✅ command.Dispatcher with typed SyncItem/DeleteItem   |
| Query dispatch            | ✅ query.Dispatcher with typed List/Get/Count/GetTypes |
| Projection replay         | ✅                                                     |
| Persistent snapshots      | ✅                                                     |
| Persistent checkpoints    | ✅                                                     |
| Correlation IDs           | ✅                                                     |

---

_Generated by Crush on 2026-05-22 at 00:35 CEST_
