# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-05-29
**Status:** Active Development
**Tests:** 241 passing, 9 packages | **Lint:** 0 issues (golangci-lint v2)

## Overview

Actionable tasks for the next 2–4 weeks. Items are organized by priority.

---

## 🔴 HIGH PRIORITY

### Blocking Issues

- [ ] **Resolve go-cqrs-lite upstream WIP**
      **Source:** `pkg/cqrs/stack.go`, `go.mod`
      **Description:** `Sink→EventSink` rename + `Source` type collision in go-cqrs-lite upstream.
      **Context:** Blocks dependency upgrades. Current pseudo-versions work but cannot update until upstream settles.

### Testing & Quality

- [ ] **Add integration test for full sync pipeline**
      **Source:** `pkg/sync/`, `pkg/cqrs/`, `pkg/api/`
      **Description:** Provider → CQRS → read model → API round-trip. End-to-end verification.
      **Context:** All testing is mock-based. No test verifies the full pipeline works together.

- [ ] **Test concurrent read model access**
      **Source:** `pkg/cqrs/memory_readmodel.go`
      **Description:** MemoryReadModel uses `sync.RWMutex` but has no concurrent access tests.
      **Context:** Race conditions could hide in production under load.

- [ ] **Improve `cmd/examples/github-sync` coverage (10.3%)**
      **Source:** `cmd/examples/github-sync/`
      **Description:** Core CLI logic (main.go: `runSync`, `runStats`, signal handling) is untested.
      **Context:** Lowest coverage in the project. Helpers are tested; main flow is not.

- [ ] **Test `mapSyncError()`**
      **Source:** `pkg/api/server.go`
      **Description:** Table-driven tests for all 6 error→HTTP status mappings.
      **Context:** Error-to-HTTP mapping is critical for API correctness.

### Architecture

- [ ] **Add CLI flag for conflict resolver**
      **Source:** `cmd/examples/github-sync/main.go`
      **Description:** `--conflict-strategy=remote-wins|lww|custom` flag to configure `CQRSConfig.ConflictResolver`.
      **Context:** CRDT resolver is wired into `DecideSync` but no CLI flag exposes it. Users always get default remote-wins.

---

## 🟡 MEDIUM PRIORITY

### Testing & Coverage

- [ ] **Add table-driven tests for `HasChanged`**
      **Source:** `pkg/cqrs/decider.go`
      **Description:** Edge cases in field comparison (UpdatedAt, Type, ActorLogin, RepoName, RepoURL).
      **Context:** HasChanged is the gate for all conflict detection. Subtle bugs hide here.

- [ ] **Verify `ConflictAwareSyncer` handles `ActionConflictLocal` in integration test**
      **Source:** `pkg/sync/conflict_aware.go`
      **Description:** Only unit tests cover local-wins path. No integration test with real CQRS stack + resolver.
      **Context:** Local-wins is a new code path that has never been tested end-to-end.

- [ ] **Performance benchmarks**
      **Source:** `pkg/cqrs/`, `pkg/sync/`
      **Description:** Benchmark `SyncItems` with 1k/10k/100k items. Profile memory usage.
      **Context:** No performance data exists. Unknown how the system scales.

- [ ] **Test Turso read model with real database file**
      **Source:** `pkg/cqrs/turso_readmodel_test.go`
      **Description:** Currently only tests with `:memory:` SQLite. Verify file-based persistence works across restarts.
      **Context:** In-memory tests don't catch file I/O or locking issues.

- [ ] **Improve `pkg/api` coverage (76.3%)**
      **Source:** `pkg/api/server_test.go`
      **Description:** Add error path tests for store failures, malformed requests, edge cases.
      **Context:** Happy paths are tested; error handling gaps remain.

- [ ] **Real GitHub PAT smoke test**
      **Source:** `cmd/examples/github-sync/`
      **Description:** Verify actual API sync works end-to-end with a real token.
      **Context:** All testing is mock-based. Never verified with real GitHub API.

- [ ] **Unify test framework**
      **Source:** 6 testify files + 1 Ginkgo file
      **Description:** Replace testify assertions and Ginkgo BDD with stdlib `t.Errorf`/`t.Fatal`.
      **Context:** Inconsistent test frameworks. go-cqrs-lite uses stdlib throughout.

### Code Quality

- [ ] **Doc comments for exported types**
      **Source:** `pkg/id/`, `pkg/errors/`, `pkg/crdt/`
      **Description:** ~18 exported types/sentinels missing godoc comments across 3 packages.
      **Context:** `go doc` returns empty for these. Hinders API discoverability.

- [ ] **Remove `CQRSStack.GetTypes` duplicate**
      **Source:** `pkg/cqrs/stack_adapters.go`
      **Description:** `GetTypes()` and `GetItemTypes()` are identical — both call `s.ReadModel.GetTypes(ctx)`. Consolidate to one.
      **Context:** Two names for the same method. `GetItemTypes` satisfies `SyncStore` interface; `GetTypes` is used by CLI stats.

- [ ] **Extract shared `testutil` package**
      **Source:** Multiple test files
      **Description:** `TestItem()` helper duplicated across 4+ test files. Extract to `internal/testutil/`.
      **Context:** DRY violation. Changes to test item factory require editing multiple files.

- [ ] **Split `pkg/sync/sync.go` (348 lines)**
      **Source:** `pkg/sync/sync.go`
      **Description:** Extract `SyncStore` interface, `SyncAction` constants, and types to separate files.
      **Context:** Single file contains interface, constants, types, and core Syncer logic. Near 350-line soft limit.

### Features & UX

- [ ] **OpenTelemetry instrumentation**
      **Source:** `pkg/sync/sync.go`, `pkg/cqrs/stack.go`, `pkg/api/server.go`
      **Description:** Add spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, HTTP middleware. `go.opentelemetry.io/otel` is already an indirect dependency.
      **Context:** No observability. Production debugging requires log spelunking.

- [ ] **Add structured logging fields**
      **Source:** `pkg/sync/sync.go`, `pkg/providers/github/client.go`
      **Description:** Add consistent context fields (username, page, event_id) to all log statements.
      **Context:** Improve debuggability when filtering logs for specific users or events.

- [ ] **Graceful shutdown for API server**
      **Source:** `cmd/examples/github-sync/helpers.go`
      **Description:** Use `http.Server.Shutdown(ctx)` instead of hard close. Drain in-flight requests.
      **Context:** Current implementation calls `http.ListenAndServe` with no shutdown handling.

---

## 🟢 LOWER PRIORITY

### API Enhancements

- [ ] **API authentication middleware** (API key or JWT)
- [ ] **API pagination headers** (`X-Total-Count`, cursor-based)
- [ ] **API rate limiting middleware** (prevent `POST /sync` abuse)
- [ ] **API OpenAPI spec enhancement** (error response schemas per endpoint)

### Code Quality

- [ ] **Add `NewItemFilter()` default constructor** — `ItemFilter` has no zero-value constructor
- [ ] **Clean `nolint:ireturn` in store_factory** — 3 functions return interfaces
- [ ] **Add `govalid` struct tags to `AppConfig`, `SyncOptions`, `CQRSConfig`**
- [ ] **Enforce file size limit in CI** — add `find . -name "*.go" -exec wc -l` check
- [ ] **Document file split naming convention** in CONTRIBUTING.md
- [ ] **Adopt `middleware.CommandRetry`** from go-cqrs-lite (API mismatch currently blocks adoption)
- [ ] **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution
- [ ] **Adopt `catalog/`** from go-cqrs-lite for AsyncAPI/OpenAPI/D2 generation

### Documentation

- [ ] **Add ADR: CQRS adoption decision** (`docs/adr/0001-cqrs-adoption.md`)
- [ ] **Add ADR: Branded ID migration** (`docs/adr/0002-branded-ids.md`)
- [ ] **Add ADR: CRDT integration strategy** (`docs/adr/0003-crdt-integration.md`)
- [ ] **Add `pkg/crdt/example_test.go`** showing LWWResolver with `*provider.Item`
- [ ] **Improve CONTRIBUTING.md** — add architecture guide, file size limits, testing requirements

---

## ✅ COMPLETED (Sessions 3–7)

| Item | Session | Date |
|------|---------|------|
| CLI tests (`exitCodeForError`, `LoadConfig`, env defaults) | 3 | 2026-05-25 |
| Wire error taxonomy via `event.RegisterClassification` | 3 | 2026-05-25 |
| Adopt `projection.Runner` from go-cqrs-lite | 3 | 2026-05-25 |
| Adopt `command.Dispatcher` with typed commands | 3 | 2026-05-25 |
| HTTP API server (`GET /items`, `GET /stats`, `POST /sync`, `GET /health`) | 5 | 2026-05-28 |
| CLI server mode (`-server`, `-port`) | 5 | 2026-05-28 |
| Error templates (`RegisterErrorTemplates` for all 9 codes) | 5 | 2026-05-28 |
| JSON output flag (`-json`) | 5 | 2026-05-28 |
| `flake.nix` with devShell + buildGoModule | 5 | 2026-05-28 |
| `reportProgress` callback test | 5 | 2026-05-28 |
| `printSyncResultJSON` test | 5 | 2026-05-28 |
| API server tests (8 tests, all endpoints) | 5 | 2026-05-28 |
| CRDT wired as pluggable conflict resolution strategy | 6 | 2026-05-29 |
| `ActionConflictLocal` SyncAction | 6 | 2026-05-29 |
| `resolveConflict` helper + `conflictMeta` struct | 6 | 2026-05-29 |
| `CQRSConfig.ConflictResolver` field + wiring | 6 | 2026-05-29 |
| 13 new CRDT/conflict tests (decider + stack + classify) | 6 | 2026-05-29 |
| `CONTRIBUTING.md` (basic) | 6 | 2026-05-29 |
| Push/Pull tests (5 tests: no-DB, local-DB, sync-after) | 7 | 2026-05-29 |
| `conflict_aware.go` extracted from `sync.go` | 7 | 2026-05-29 |
| CLI helpers extracted to `helpers.go` | 7 | 2026-05-29 |
| Fix `exhaustruct` warnings (ItemFilter builder) | 7 | 2026-05-29 |
| Domain language documented (`docs/DOMAIN_LANGUAGE.md`) | 7 | 2026-05-29 |

---

## 📋 COMPLETION CHECKLIST

Before Phase 2 (Production Ready):

- [x] Test coverage for `pkg/cqrs`, `pkg/providers/github`, `pkg/sync`, `pkg/id`, `pkg/errors`, `pkg/crdt`, `pkg/api`
- [x] CI/CD pipeline configured (GitHub Actions)
- [x] go.mod properly formatted (no replace directives)
- [x] Architecture decoupling (domain types, branded IDs) complete
- [x] CQRS migration complete (legacy CRUD deleted)
- [x] Conflict-aware sync engine functional (with pluggable CRDT resolver)
- [x] Error handling migrated to stdlib (cockroachdb/errors removed)
- [x] golangci-lint v2 passing (0 issues)
- [x] CRDT integration wired (LWWResolver, custom resolvers)
- [x] HTTP API functional (4 endpoints)
- [x] Dual storage backend (memory + Turso/SQLite)
- [ ] Real GitHub API sync verified with PAT
- [ ] OpenTelemetry instrumentation
- [ ] API authentication
- [ ] All HIGH priority items complete

---

## ❓ OPEN QUESTIONS

1. **End-to-end testing** — Do we need a real GitHub PAT for CI integration tests, or are mocks sufficient?
2. **Multi-user sync** — Should the read model track which user each event belongs to?
3. **Event retention/TTL** — Automatic cleanup of old events? Configurable?
4. **`github-local-sync` vs `go-localsync`** — Thin CLI skin, independent with shared SDK, or deprecated/merged?
5. **Decide CRDT package fate** — Keep in repo, extract to own repo, or wire deeper?
6. **Should `pkg/sync/sync.go` be split now (348 lines) or only when it crosses 350?**
