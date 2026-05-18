# ROADMAP.md

**Project:** go-localsync  
**Last Updated:** 2026-05-17

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

---

## 🔧 TECHNICAL DEBT

### Architecture

- [ ] **Consolidate conflict detection**  
       `ConflictAwareSyncer` and `DecideSync` both independently detect conflicts using the same `HasChanged()` but different truth sources (read model vs event store). The decider should be the single authority. `SyncItems` should return per-item results so the sync layer can observe conflicts without duplicating detection.

- [ ] **Adopt projection.Runner**  
       Replace custom `Projector` with go-cqrs-lite's `projection.Runner` for replay + checkpointing.

- [ ] **Wire error taxonomy**  
       Use go-cqrs-lite's `event.RegisterClassification` for proper CLI exit codes instead of generic 1.

- [ ] **Adopt command.Dispatcher**  
       Use typed command dispatch from go-cqrs-lite instead of raw `SyncItems` method.

### Code Quality

- [ ] **Unify test framework**  
       1 file uses Ginkgo, 6 files use testify. Standardize on one approach (stdlib recommended).

- [ ] **CLI tests**  
       Zero test coverage for 240-line `main.go` — flag parsing, signal handling, exit codes.

- [ ] **Push/Pull tests**  
       `CQRSStack.Push()` and `Pull()` untested.

---

## ❓ OPEN QUESTIONS

1. **End-to-end testing** — Do we need a real GitHub PAT for CI integration tests, or are mocks sufficient?
2. **Multi-user sync** — Should the read model track which user each event belongs to?
3. **Event retention/TTL** — Automatic cleanup of old events? Configurable?
4. **Conflict resolution policy** — Currently hard-coded to "remote wins". Should this be configurable (local-wins, manual resolution)?
