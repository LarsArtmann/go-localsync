# ROADMAP.md

**Project:** go-localsync  
**Last Updated:** 2026-05-25

## Overview

Aspirational features and improvements with no fixed timeline. These are planned for future phases.

---

## ✅ COMPLETED

- [x] **CQRS migration** — Full event-sourced architecture via go-cqrs-lite. Legacy CRUD deleted.
- [x] **Deterministic aggregate IDs** — SHA256→ULID from (source, sourceID) for idempotency.
- [x] **Conflict-aware sync** — `DecideSync` detects conflicts and emits `ItemConflictFound` events.
- [x] **Provider architecture** — Generic `Provider` interface with GitHub implementation.
- [x] **Branded type IDs** — 6 phantom types for compile-time safety.
- [x] **Turso backend** — SQLite/Turso event store + read model with remote Push/Pull sync.
- [x] **cockroachdb/errors removal** — Replaced with stdlib `fmt.Errorf` + `%w`.
- [x] **Database migration system** — Replaced by go-cqrs-lite/storage schema management.
- [x] **Zero lint issues** — golangci-lint v2 with 125+ linters, 0 issues.
- [x] **Dead code removal** — `types.SourceID` (unused), `AggregateID` sync.Map cache.
- [x] **AggregateID double-computation eliminated** — Uses `cmd.AggregateID()` from command metadata.
- [x] **Domain language documented** — `docs/DOMAIN_LANGUAGE.md` with full glossary.

---

## 🟢 FUTURE PHASES

### Enhanced Features

- [ ] **Build TUI with Bubble Tea**  
       Interactive terminal UI for browsing events, filtering, and real-time sync.  
       Effort: ~2h. Low priority.

- [ ] **Support multiple user sync**  
       Accept multiple `-user` flags or user list from file.  
       Requires read model schema to track which user each event belongs to.

- [ ] **Implement daemon/background mode**  
       Run as cron job or systemd service for periodic sync.

### Data & Export

- [ ] **Create HTTP API endpoint**  
       REST API for querying events (GET /events, /stats, /types).  
       Would use cqrs-htmx library. Effort: ~4h.

- [ ] **Add export to JSON/CSV**  
       Export stored events to file formats (`-export json` or `-export csv`).

- [x] **Conflict detection consolidation** — Decider is single authority; `ConflictAwareSyncer` delegates to `SyncItems` results.
- [x] **HasChanged time comparison fix** — Fixed `!=` to `.Equal()` — was causing false conflict detections.

---

## 🔧 TECHNICAL DEBT

### Architecture

- [x] **Adopt projection.Runner** (Completed)  
       Replaced custom `Projector` with go-cqrs-lite's `projection.Runner` for replay + checkpointing.

- [x] **Wire error taxonomy** (Completed)  
       Uses go-cqrs-lite's `event.RegisterClassification` for proper CLI exit codes.

- [x] **Adopt command.Dispatcher** (Completed)  
       Typed command dispatch via `SyncItemCommand`/`DeleteItemCommand` through `command.Dispatcher`.

### Code Quality

- [ ] **Unify test framework**  
       1 file uses Ginkgo, 6 files use testify. Standardize on one approach (stdlib recommended).

- [x] **CLI tests** (Completed)  
       `main_test.go` covers exitCodeForError, LoadConfig, env defaults.

- [ ] **Push/Pull tests**  
       `CQRSStack.Push()` and `Pull()` untested.

---

## ❓ OPEN QUESTIONS

1. **End-to-end testing** — Do we need a real GitHub PAT for CI integration tests, or are mocks sufficient?
2. **Multi-user sync** — Should the read model track which user each event belongs to?
3. **Event retention/TTL** — Automatic cleanup of old events? Configurable?
4. **Conflict resolution policy** — Currently hard-coded to "remote wins". Should this be configurable (local-wins, manual resolution)?
