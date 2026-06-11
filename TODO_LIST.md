# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-06-11 (session 14)
**Status:** Active Development
**Tests:** 270+ passing, 11 packages | **Lint:** 0 issues (golangci-lint v2)

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

- [ ] **Improve `cmd/examples/github-sync` coverage (12.3%)**
      **Source:** `cmd/examples/github-sync/`
      **Description:** Core CLI logic (main.go: `runSync`, `runStats`, signal handling) is untested.
      **Context:** Lowest coverage in the project. Helpers are tested; main flow is not.

- [ ] **Test Turso read model with real database file**
      **Source:** `pkg/cqrs/sqlite_readmodel_test.go`
      **Description:** Currently only tests with `:memory:` SQLite. Verify file-based persistence works across restarts.
      **Context:** In-memory tests don't catch file I/O or locking issues.

- [ ] **Improve `pkg/data/model` coverage (68.4%)**
      **Source:** `pkg/data/model/`
      **Description:** Coverage dropped after removing ProviderItem tests. Add more Item.Validate edge cases.
      **Context:** Below project average of ~83%.

---

## 🟡 MEDIUM PRIORITY

### Testing & Coverage

- [ ] **Improve `pkg/api` coverage (76.6%)**
      **Source:** `pkg/api/server_test.go`
      **Description:** Add error path tests for store failures, malformed requests, edge cases.
      **Context:** Happy paths are tested; error handling gaps remain.

- [ ] **Real GitHub PAT smoke test**
      **Source:** `cmd/examples/github-sync/`
      **Description:** Verify actual API sync works end-to-end with a real token.
      **Context:** All testing is mock-based. Never verified with real GitHub API.

### Code Quality

- [ ] **Extract shared `testutil` package**
      **Source:** Multiple test files
      **Description:** `TestItem()` helper duplicated across 4+ test files. Extract to `internal/testutil/`.
      **Context:** DRY violation. Changes to test item factory require editing multiple files.

- [ ] **Clean `nolint:ireturn` in store_factory**
      **Source:** `pkg/cqrs/store_factory.go`
      **Description:** 3 functions return interfaces. Consider if ireturn lint is worth suppressing.
      **Context:** Low priority, but worth a quick evaluation.

### Features & UX

- [ ] **OpenTelemetry instrumentation**
      **Source:** `pkg/sync/sync.go`, `pkg/cqrs/stack.go`, `pkg/api/server.go`
      **Description:** Add spans for `Syncer.Sync()`, `CQRSStack.SyncItems()`, HTTP middleware.
      **Context:** No observability. Production debugging requires log spelunking.

- [ ] **Add structured logging fields**
      **Source:** `pkg/sync/sync.go`, `pkg/providers/github/client.go`
      **Description:** Add consistent context fields (username, page, event_id) to all log statements.
      **Context:** Improve debuggability when filtering logs for specific users or events.

---

## 🟢 LOWER PRIORITY

### API Enhancements

- [ ] **API authentication middleware** (API key or JWT)
- [ ] **API pagination headers** (`X-Total-Count`, cursor-based)
- [ ] **API rate limiting middleware** (prevent `POST /sync` abuse)
- [ ] **API OpenAPI spec enhancement** (error response schemas per endpoint)

### Code Quality

- [ ] **Add `govalid` struct tags to `AppConfig`, `SyncOptions`, `CQRSConfig`**
- [ ] **Document file split naming convention** in CONTRIBUTING.md
- [ ] **Adopt `middleware.CommandRetry`** from go-cqrs-lite (API mismatch currently blocks adoption)
- [ ] **Adopt `UpcasterRegistry`** from go-cqrs-lite for schema evolution
- [ ] **Adopt `catalog/`** from go-cqrs-lite for AsyncAPI/OpenAPI/D2 generation

### Documentation

- [ ] **Add ADR: CQRS adoption decision** (`docs/adr/0001-cqrs-adoption.md`)
- [ ] **Add ADR: Branded ID migration** (`docs/adr/0002-branded-ids.md`)
- [ ] **Add ADR: CRDT integration strategy** (`docs/adr/0003-crdt-integration.md`)
- [ ] **Improve CONTRIBUTING.md** — add architecture guide, file size limits, testing requirements

---

## ✅ COMPLETED (Sessions 3–14)

| Item                                                                      | Session | Date       |
| ------------------------------------------------------------------------- | ------- | ---------- |
| CLI tests (`exitCodeForError`, `LoadConfig`, env defaults)                | 3       | 2026-05-25 |
| Wire error taxonomy via `event.RegisterClassification`                    | 3       | 2026-05-25 |
| Adopt `projection.Runner` from go-cqrs-lite                               | 3       | 2026-05-25 |
| Adopt `command.Dispatcher` with typed commands                            | 3       | 2026-05-25 |
| HTTP API server (`GET /items`, `GET /stats`, `POST /sync`, `GET /health`) | 5       | 2026-05-28 |
| CLI server mode (`-server`, `-port`)                                      | 5       | 2026-05-28 |
| Error templates (`RegisterErrorTemplates` for all 9 codes)                | 5       | 2026-05-28 |
| JSON output flag (`-json`)                                                | 5       | 2026-05-28 |
| `flake.nix` with devShell + buildGoModule                                 | 5       | 2026-05-28 |
| `reportProgress` callback test                                            | 5       | 2026-05-28 |
| `printSyncResultJSON` test                                                | 5       | 2026-05-28 |
| API server tests (8 tests, all endpoints)                                 | 5       | 2026-05-28 |
| CRDT wired as pluggable conflict resolution strategy                      | 6       | 2026-05-29 |
| `ActionConflictLocal` SyncAction                                          | 6       | 2026-05-29 |
| `resolveConflict` helper + `conflictMeta` struct                          | 6       | 2026-05-29 |
| `CQRSConfig.ConflictResolver` field + wiring                              | 6       | 2026-05-29 |
| 13 new CRDT/conflict tests (decider + stack + classify)                   | 6       | 2026-05-29 |
| `CONTRIBUTING.md` (basic)                                                 | 6       | 2026-05-29 |
| `conflict_aware.go` extracted from `sync.go`                              | 7       | 2026-05-29 |
| CLI helpers extracted to `helpers.go`                                     | 7       | 2026-05-29 |
| Fix `exhaustruct` warnings (ItemFilter builder)                           | 7       | 2026-05-29 |
| Domain language documented (`docs/DOMAIN_LANGUAGE.md`)                    | 7       | 2026-05-29 |
| go-cqrs-lite v2 migration (all 11 modules)                               | 8       | 2026-06-03 |
| Turso→SQLite rename across 11 files                                       | 8       | 2026-06-03 |
| Dead config removed (RemoteURL, AuthToken, Push/Pull flags)               | 8       | 2026-06-03 |
| SyncItems through command pipeline                                        | 10      | 2026-06-10 |
| Compile-time SyncStore assertion                                          | 10      | 2026-06-10 |
| Consistent not-found semantics                                            | 10      | 2026-06-10 |
| NewServer simplified                                                      | 10      | 2026-06-10 |
| Runner errors logged via `slog.Error`                                     | 10      | 2026-06-10 |
| Foundational data module types (Item, Key, schema.Version)                | 11      | 2026-06-11 |
| Read-side interface unified into `model.ItemReader`                       | 11      | 2026-06-11 |
| MockProvider consolidation into testutil                                  | 11      | 2026-06-11 |
| Orphaned packages deleted (data/query, data/repo, data/transform)         | 13      | 2026-06-11 |
| Dead types removed (ProviderItem, ItemView, StatsView)                    | 13      | 2026-06-11 |
| ConflictStrategy CLI flag wired to CQRS stack                             | 13      | 2026-06-11 |
| HasChanged edge case tests (7 subtests)                                   | 13      | 2026-06-11 |
| ActionConflictLocal integration test with LWW                             | 13      | 2026-06-11 |
| SyncItems benchmarks (1/10/100 items)                                     | 13      | 2026-06-11 |
| Doc comments for error sentinels, ClockOrder, String methods              | 13      | 2026-06-11 |
| E2E API filter/pagination test                                            | 13      | 2026-06-11 |
| ConflictAwareSyncer coupling fixed (named field + Close)                  | 13      | 2026-06-11 |
| Graceful shutdown with signal handling                                    | 13      | 2026-06-11 |
| Deduplication pass (96→73 assertion groups)                               | 12      | 2026-06-11 |
| Dead Get*() methods removed from model.Item                               | 14      | 2026-06-11 |
| ItemFilter moved from pkg/provider to pkg/data/model                      | 14      | 2026-06-11 |
| pkg/sync/sync.go split into types.go + sync.go                           | 14      | 2026-06-11 |
| Concurrent access tests for MemoryReadModel (3 tests)                     | 14      | 2026-06-11 |
| mapSyncError table-driven tests (6 mappings)                              | 14      | 2026-06-11 |
| CRDT example_test.go (LWWResolver with model.Item)                        | 14      | 2026-06-11 |
| Benchmarks modernized to b.Loop()                                         | 14      | 2026-06-11 |

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
- [x] Dual storage backend (memory + SQLite)
- [x] Concurrent access verified (race detector clean)
- [x] Dead code removed (orphaned packages, unused types)
- [x] ItemFilter in correct package (model, not provider)
- [ ] Real GitHub API sync verified with PAT
- [ ] OpenTelemetry instrumentation
- [ ] API authentication
- [ ] All HIGH priority items complete

---

## ❓ OPEN QUESTIONS

1. **Multi-user sync** — Should the read model track which user each event belongs to?
2. **Event retention/TTL** — Automatic cleanup of old events? Configurable?
3. **`github-local-sync` vs `go-localsync`** — Thin CLI skin, independent with shared SDK, or deprecated/merged?
4. **Decide CRDT package fate** — Keep in repo, extract to own repo, or wire deeper?
