# Session 8 — go-cqrs-lite v2 Migration & Dependency Upgrade

**Date:** 2026-06-03 16:40
**Branch:** master
**Status:** ✅ COMPLETE — all 9 packages, 235 tests green

---

## Summary

Migrated go-localsync from go-cqrs-lite v1 (monolithic `core/*` modules) to v2 (independent versioned sub-modules). Upgraded all dependencies to latest. Removed features that no longer exist in v2 (outbox pattern, Turso remote sync).

---

## Work Completed

### a) FULLY DONE ✅

| Task                               | Files                                        | Impact                                                                  |
| ---------------------------------- | -------------------------------------------- | ----------------------------------------------------------------------- |
| go.work updated                    | `go.work`                                    | Removed `core`, `saga`; added 13 v2 sub-modules                         |
| go.mod migrated to v2              | `go.mod`, `go.sum`                           | 11 direct v2 deps, 0 replace directives, 0 pseudo-versions              |
| All source imports migrated        | 8 `pkg/cqrs/*.go` files                      | `core/*` → `event/v2`, `command/v2`, `decider/v2`, `query/v2`, `id/v2`  |
| All test imports migrated          | 6 test files                                 | Same migration pattern                                                  |
| `codec/v2.JSONCodec` adopted       | `decider.go`, `projection.go`, `stack.go`    | Replaced `event.JSONCodec`                                              |
| `snapshot/v2.EveryNEvents` adopted | `stack.go`                                   | Replaced `event.EveryNEvents`                                           |
| Outbox pattern removed             | `stack.go`, `stack_adapters.go`, `runner.go` | Removed `OutboxPublisher`, `WithOutbox`, outbox goroutine               |
| Turso sync removed                 | `stack.go`, `store_factory.go`               | Removed `TursoSyncDB`, `OpenTursoSync`, `Push()`, `Pull()`              |
| SQLite driver migrated             | `go.mod`, test files                         | `tursogo` → `modernc.org/sqlite` (pure-Go)                              |
| `OpenTurso` → `OpenSQLite`         | `store_factory.go`, tests                    | Renamed in v2 storage module                                            |
| CLI flags cleaned                  | `cmd/examples/github-sync/main.go`           | Removed `--push`, `--pull` flags and usage                              |
| `pushpull_test.go` deleted         | Test file                                    | Tests for removed Push/Pull features                                    |
| Direct bus subscription            | `runner.go`                                  | Replaces `InMemoryRunner` — synchronous `bus.SubscribeAll(proj.Handle)` |
| go-error-family v0.2→v0.3          | `go.mod`                                     | Required by go-cqrs-lite v2                                             |
| go-branded-id v0.1→v0.3            | `go.mod`                                     | Now explicitly tracked                                                  |
| All third-party deps upgraded      | `go.mod`                                     | `go get -u ./...`                                                       |
| AGENTS.md updated                  | `AGENTS.md`                                  | Session 8 section, dependency table, architecture notes                 |

### b) PARTIALLY DONE ⚠️

| Task                                         | What's Left                                                | Severity |
| -------------------------------------------- | ---------------------------------------------------------- | -------- |
| `CQRSConfig.RemoteURL` / `AuthToken`         | Dead code — fields exist but nothing uses them             | MEDIUM   |
| `TestCQRSStack_OutboxPoller_PublishesEvents` | Test renamed behavior but not name                         | LOW      |
| Docs (README, FEATURES, ROADMAP)             | Still describe Outbox, Push/Pull, InMemoryRunner as active | LOW      |

### c) NOT STARTED ❌

| Task                                      | Priority | Notes                                                  |
| ----------------------------------------- | -------- | ------------------------------------------------------ |
| Lint with golangci-lint v2                | HIGH     | Project uses v2 config but only v1 binary is installed |
| `CQRSConfig` cleanup — remove dead fields | MEDIUM   | RemoteURL, AuthToken are unused                        |
| README.md update                          | MEDIUM   | Still references Turso sync, outbox                    |
| FEATURES.md update                        | MEDIUM   | Still lists Outbox, Push/Pull as features              |
| ROADMAP.md update                         | LOW      | References removed features                            |
| Turso readmodel DDL migration             | LOW      | Verify schema still works with v2 storage              |
| flake.nix update                          | LOW      | May need modernc.org/sqlite in build inputs            |

### d) TOTALLY FUCKED UP 💥

Nothing. All 235 tests pass, build is clean, no import cycles.

---

## Dependency State

### Direct Dependencies

| Module                       | Version | Change                                       |
| ---------------------------- | ------- | -------------------------------------------- |
| `go-cqrs-lite/event/v2`      | v2.0.0  | NEW (was `core` v1.6.0)                      |
| `go-cqrs-lite/command/v2`    | v2.0.0  | NEW (was `core` v1.6.0)                      |
| `go-cqrs-lite/decider/v2`    | v2.0.0  | NEW (was `core` v1.6.0)                      |
| `go-cqrs-lite/query/v2`      | v2.0.0  | NEW (was `core` v1.6.0)                      |
| `go-cqrs-lite/id/v2`         | v2.0.0  | NEW (was `core/pkg/id` v1.6.0)               |
| `go-cqrs-lite/codec/v2`      | v2.0.0  | NEW (was in `core`)                          |
| `go-cqrs-lite/snapshot/v2`   | v2.0.0  | NEW (was in `core`)                          |
| `go-cqrs-lite/memory/v2`     | v2.0.0  | Was v1.6.0                                   |
| `go-cqrs-lite/middleware/v2` | v2.0.0  | Was v1.6.0                                   |
| `go-cqrs-lite/projection/v2` | v2.0.0  | Was v1.6.0                                   |
| `go-cqrs-lite/storage/v2`    | v2.0.0  | Was pseudo-version                           |
| `go-branded-id`              | v0.3.0  | Was v0.1.0                                   |
| `go-error-family`            | v0.3.0  | Was v0.2.0                                   |
| `modernc.org/sqlite`         | v1.51.0 | NEW (replaces `turso.tech/database/tursogo`) |

### Removed Dependencies

| Module                        | Reason                                         |
| ----------------------------- | ---------------------------------------------- |
| `go-cqrs-lite/core`           | Dissolved into independent v2 modules          |
| `go-cqrs-lite/saga`           | Removed in v2                                  |
| `turso.tech/database/tursogo` | Remote sync removed, local SQLite uses modernc |
| `go-cqrs-lite/storage` (v1)   | Replaced by `storage/v2`                       |

---

## Test Results

```
ok  github.com/larsartmann/go-localsync/cmd/examples/github-sync  0.003s
ok  github.com/larsartmann/go-localsync/pkg/api                   0.005s
ok  github.com/larsartmann/go-localsync/pkg/cqrs                  0.015s
ok  github.com/larsartmann/go-localsync/pkg/crdt                  0.002s
ok  github.com/larsartmann/go-localsync/pkg/errors                0.002s
ok  github.com/larsartmann/go-localsync/pkg/id                    0.002s
ok  github.com/larsartmann/go-localsync/pkg/provider              0.002s
ok  github.com/larsartmann/go-localsync/pkg/providers/github      0.015s
ok  github.com/larsartmann/go-localsync/pkg/sync                  0.003s
```

**235 tests passing**, 0 failures, 9/9 packages green.

---

## What Could Be Better — Self-Review

### Mistakes Made

1. **Took 3 iterations on `runner.go`** — First tried using `projection.Runner` for memory backend (wrong — it needs a journal). Then tried async bus subscription via goroutine (race condition). Finally landed on direct synchronous `bus.SubscribeAll` which is the correct pattern for memory backends.

2. **Missed `RemoteURL`/`AuthToken` cleanup** — These `CQRSConfig` fields are dead code after removing Turso sync. Should have cleaned them up during the migration.

3. **Missed renaming `TestCQRSStack_OutboxPoller_PublishesEvents`** — The outbox is gone; this test now validates projection subscription, not outbox polling.

4. **Didn't update README, FEATURES, ROADMAP** — These docs still describe removed features as if they're active.

5. **Didn't run lint** — golangci-lint v2 config requires v2 binary which isn't installed. Should have flagged this earlier.

### Architecture Observations

1. **`CQRSConfig` is stale** — `RemoteURL` and `AuthToken` are dead fields. The struct should be simplified.
2. **`store_factory.go` still has `createTursoStore`** — It works (SQLite) but the naming is misleading since Turso remote sync is gone.
3. **`pkg/cqrs/turso_readmodel.go`** — Still named "turso" but now uses plain SQLite. The model itself is fine; the naming is a historical artifact.
4. **go-cqrs-lite v2 has `otel/v2` as an indirect dep** — We already planned OpenTelemetry instrumentation (deferred in session 5). The v2 modules come with OTel support built-in.

---

## Top #25 Things to Do Next (Sorted by Impact × Effort)

### HIGH Impact, LOW Effort (Do First)

| # | Task                                                                                              | Effort | Impact   |
| - | ------------------------------------------------------------------------------------------------- | ------ | -------- |
| 1 | Remove dead `CQRSConfig.RemoteURL`/`AuthToken` fields + CLI config                                | 30min  | Clean    |
| 2 | Rename `createTursoStore` → `createSQLiteStore` in `store_factory.go`                             | 10min  | Clarity  |
| 3 | Rename `TestCQRSStack_OutboxPoller_PublishesEvents` → `TestCQRSStack_Projection_SubscribesEvents` | 5min   | Clarity  |
| 4 | Rename `turso_readmodel.go` → `sqlite_readmodel.go` (and TursoReadModel → SQLiteReadModel)        | 30min  | Honesty  |
| 5 | Update README.md to remove Turso sync/outbox/Push-Pull references                                 | 30min  | Accuracy |
| 6 | Update FEATURES.md to remove Outbox, Push/Pull, InMemoryRunner                                    | 15min  | Accuracy |
| 7 | Install golangci-lint v2 and run full lint                                                        | 15min  | Quality  |
| 8 | Update `flake.nix` if needed for modernc.org/sqlite                                               | 15min  | Build    |

### HIGH Impact, MEDIUM Effort

| #  | Task                                                                   | Effort | Impact        |
| -- | ---------------------------------------------------------------------- | ------ | ------------- |
| 9  | Adopt `go-cqrs-lite/otel/v2` for OpenTelemetry instrumentation         | 2-3h   | Observability |
| 10 | Adopt `go-cqrs-lite/signing` for event integrity verification          | 2h     | Security      |
| 11 | Review `pkg/errors/` for go-error-family v0.3 API improvements         | 1h     | Error quality |
| 12 | Add integration test for projection replay on restart                  | 1h     | Reliability   |
| 13 | Review `CQRSStack.Close()` — ensure all goroutines are stopped cleanly | 30min  | Correctness   |

### MEDIUM Impact, MEDIUM Effort

| #  | Task                                                                              | Effort | Impact        |
| -- | --------------------------------------------------------------------------------- | ------ | ------------- |
| 14 | Adopt `go-cqrs-lite/schema/v2` for schema versioning of events                    | 2h     | Evolution     |
| 15 | Adopt `middleware.CommandRetry` for provider retry (check v2 API)                 | 1h     | Reliability   |
| 16 | Type model review — consider `event.AggregateRef` instead of separate type+id     | 1h     | Modernize     |
| 17 | Review `SyncItemState` — could use `option.Option[*provider.Item]` instead of nil | 1h     | Type safety   |
| 18 | Add structured logging to `SyncItems` (per-item results)                          | 30min  | Debugging     |
| 19 | Review `store_factory.go` for error wrapping consistency                          | 30min  | Error quality |

### LOWER Priority

| #  | Task                                                                           | Effort | Impact        |
| -- | ------------------------------------------------------------------------------ | ------ | ------------- |
| 20 | Adopt `go-cqrs-lite/catalog` for AsyncAPI/OpenAPI/D2 docs generation           | 3h     | Documentation |
| 21 | Add `pebble` storage backend option (go-cqrs-lite has it)                      | 4h     | Performance   |
| 22 | Review `provider.Item` struct — consider protobuf or codegen for serialization | 2h     | Performance   |
| 23 | Add `go-cqrs-lite/watermill` for message broker integration                    | 4h     | Integration   |
| 24 | Update ROADMAP.md with v2 migration notes                                      | 30min  | Documentation |
| 25 | Add `.goreleaser.yml` update for new binary dependencies                       | 30min  | Release       |

---

## Top #1 Question I Cannot Figure Out Myself

**Should we rename `TursoReadModel` → `SQLiteReadModel` (and the file `turso_readmodel.go` → `sqlite_readmodel.go`)?**

Arguments FOR: The name "Turso" is misleading since we no longer use Turso sync — we use plain SQLite via `modernc.org/sqlite`. The read model has zero Turso-specific code. It's a standard SQLite-backed read model.

Arguments AGAINST: This is a public type used in tests and referenced in docs. Renaming is a breaking change for any consumer. It's also a lot of file churn for what's essentially a naming concern.

**My recommendation:** Rename it. The project is pre-1.0, there are no external consumers, and honesty in naming is more valuable than avoiding churn. But I'd like confirmation before proceeding.
