# Comprehensive Execution Plan — go-localsync

**Date:** 2026-05-28 10:48 CEST  
**Session:** Brutal Self-Review → Full Execution Sprint  
**Principle:** Pareto — 1% → 51%, 4% → 64%, 20% → 80%

---

## Pareto Breakdown

### 1% Tasks → 51% of Value

| #   | Task                                               | Why 51%?                                                   |
| --- | -------------------------------------------------- | ---------------------------------------------------------- |
| 1   | Fix `exhaustruct` warnings (3 locations)           | Zero-lint build = immediate developer happiness + CI green |
| 2   | HTTP API: `GET /items` with filtering              | The #1 customer request: "how do I query my data?"         |
| 3   | HTTP API: `GET /stats`                             | Most-used endpoint after listing items                     |
| 4   | Integrate `LWWResolver` into `ConflictAwareSyncer` | Makes the renamed `pkg/crdt` package actually useful       |

### 4% Tasks → 64% of Value

| #   | Task                                                     | Why +13%?                                                |
| --- | -------------------------------------------------------- | -------------------------------------------------------- |
| 5   | OpenTelemetry: instrument `Syncer.Sync()`                | Production debugging without logs-only archaeology       |
| 6   | Add `VectorClock` to `provider.Item` + persist in events | Foundation for multi-node conflict detection             |
| 7   | HTTP API: `POST /sync` trigger                           | Customers want to trigger sync via HTTP, not just CLI    |
| 8   | Write actual `docs/DOMAIN_LANGUAGE.md`                   | New contributors can't onboard without domain vocabulary |

### 20% Tasks → 80% of Value

| #   | Task                                                   | Why +16%?                                    |
| --- | ------------------------------------------------------ | -------------------------------------------- |
| 9   | HTTP API: `GET /health` + server tests                 | Production deployment requires health checks |
| 10  | OpenTelemetry: HTTP middleware + trace propagation     | End-to-end request tracing                   |
| 11  | Add `NodeID` to `ConflictAwareSyncer` constructor      | Enables multi-node vector clock incrementing |
| 12  | `errorfamily.RegisterTemplate` for user-facing errors  | Currently errors have no What/Why/Fix/WayOut |
| 13  | Add `coverage/` directory + `.gitignore`               | Clean repo root, proper artifact management  |
| 14  | Add `reportProgress` test + `printSyncResultJSON` test | Close remaining coverage gaps in `pkg/sync`  |
| 15  | Add ADR for CQRS adoption + branded ID migration       | Architecture decisions must be documented    |

---

## Complete Task Inventory (All Tasks ≤ 12 min)

Sorted by **Impact / Effort** ratio (highest first), then by **Customer Value**, then by **Dependency Order**.

### Phase 1: The 1% — Immediate Wins (51% value)

| Rank | ID  | Task                                                                      | File(s)                      | Effort | Impact  | Cust. Value | Blocked By |
| ---- | --- | ------------------------------------------------------------------------- | ---------------------------- | ------ | ------- | ----------- | ---------- |
| 1    | E1  | Fix `exhaustruct`: `sync.go:144` use `ItemFilter{}.WithLimit(1)`          | `pkg/sync/sync.go`           | 5 min  | 🔥 High | Low         | —          |
| 2    | E2  | Fix `exhaustruct`: `sync.go:181` use `ItemFilter{}.WithType()`            | `pkg/sync/sync.go`           | 5 min  | 🔥 High | Low         | —          |
| 3    | E3  | Fix `exhaustruct`: `sync.go:198` use `ItemFilter{}` zero value explicitly | `pkg/sync/sync.go`           | 5 min  | 🔥 High | Low         | —          |
| 4    | E4  | Fix `exhaustruct`: `stack.go:288` use `ItemFilter{}` builder              | `pkg/cqrs/stack.go`          | 5 min  | 🔥 High | Low         | —          |
| 5    | C1  | Add `VectorClock` field to `provider.Item` struct                         | `pkg/provider/provider.go`   | 8 min  | 🔥 High | Medium      | —          |
| 6    | C2  | Add `NodeID` field to `ConflictAwareSyncer` + constructor                 | `pkg/sync/conflict_aware.go` | 10 min | 🔥 High | Medium      | —          |
| 7    | C3  | Persist `VectorClock` in `ItemSynced` event payload                       | `pkg/cqrs/events.go`         | 10 min | 🔥 High | Medium      | C1         |
| 8    | C4  | Update `DecideSync` to include `VectorClock` in event metadata            | `pkg/cqrs/decider.go`        | 8 min  | 🔥 High | Medium      | C1         |
| 9    | S1  | Wire `LWWResolver` into `ConflictAwareSyncer.SyncWithConflictDetection`   | `pkg/sync/conflict_aware.go` | 12 min | 🔥 High | High        | C2, C3     |
| 10   | A1  | Add `github.com/danielgtaylor/huma/v2` dependency                         | `go.mod`                     | 3 min  | 🔥 High | High        | —          |
| 11   | A2  | Create `pkg/api/server.go` with stdlib `net/http` router                  | `pkg/api/server.go`          | 10 min | 🔥 High | High        | A1         |
| 12   | A3  | Implement `GET /items` with `ItemFilter` query params                     | `pkg/api/server.go`          | 12 min | 🔥 High | High        | A2         |
| 13   | A4  | Implement `GET /stats` returning total/types/counts                       | `pkg/api/server.go`          | 8 min  | 🔥 High | High        | A2         |

### Phase 2: The 4% — Foundation Layer (64% value)

| Rank | ID  | Task                                                       | File(s)                                 | Effort | Impact | Cust. Value | Blocked By |
| ---- | --- | ---------------------------------------------------------- | --------------------------------------- | ------ | ------ | ----------- | ---------- |
| 14   | O1  | Add `go.opentelemetry.io/otel` dependencies                | `go.mod`                                | 3 min  | High   | High        | —          |
| 15   | O2  | Create `pkg/telemetry/tracer.go` with tracer provider init | `pkg/telemetry/tracer.go`               | 10 min | High   | Medium      | O1         |
| 16   | O3  | Instrument `Syncer.Sync()` with span + attributes          | `pkg/sync/sync.go`                      | 8 min  | High   | Medium      | O2         |
| 17   | O4  | Instrument `Syncer.SyncIncremental()` with span            | `pkg/sync/sync.go`                      | 8 min  | High   | Medium      | O2         |
| 18   | O5  | Instrument `CQRSStack.SyncItems()` with span               | `pkg/cqrs/stack.go`                     | 8 min  | High   | Medium      | O2         |
| 19   | O6  | Add OTel HTTP middleware to API server                     | `pkg/api/server.go`                     | 8 min  | High   | High        | A2, O2     |
| 20   | A5  | Implement `POST /sync` trigger endpoint                    | `pkg/api/server.go`                     | 12 min | High   | High        | A2         |
| 21   | A6  | Implement `GET /health` health check endpoint              | `pkg/api/server.go`                     | 5 min  | Medium | High        | A2         |
| 22   | D1  | Write actual `docs/DOMAIN_LANGUAGE.md` with real terms     | `docs/DOMAIN_LANGUAGE.md`               | 12 min | Medium | Medium      | —          |
| 23   | T1  | Add `reportProgress` callback test                         | `pkg/sync/sync_test.go`                 | 5 min  | Medium | Low         | —          |
| 24   | T2  | Add `printSyncResultJSON` test                             | `cmd/examples/github-sync/main_test.go` | 5 min  | Medium | Low         | —          |
| 25   | T3  | Add `printVersion` with `bytes.Buffer` test                | `cmd/examples/github-sync/main_test.go` | 5 min  | Medium | Low         | —          |

### Phase 3: The 20% — Polish & Production (80% value)

| Rank | ID   | Task                                                         | File(s)                              | Effort | Impact  | Cust. Value | Blocked By     |
| ---- | ---- | ------------------------------------------------------------ | ------------------------------------ | ------ | ------- | ----------- | -------------- |
| 26   | A7   | Add API server tests (httptest) for all endpoints            | `pkg/api/server_test.go`             | 12 min | Medium  | High        | A3, A4, A5, A6 |
| 27   | A8   | Wire HTTP server into CLI (`--api` flag)                     | `cmd/examples/github-sync/main.go`   | 10 min | Medium  | High        | A2             |
| 28   | O7   | Add trace correlation IDs to sync events                     | `pkg/cqrs/stack.go`                  | 10 min | Medium  | Medium      | O5             |
| 29   | ERR1 | Add `errorfamily.RegisterTemplate` for all 9 sentinel errors | `pkg/errors/templates.go`            | 12 min | Medium  | Medium      | —              |
| 30   | ERR2 | Add user-facing error detail tests                           | `pkg/errors/errors_test.go`          | 8 min  | Medium  | Low         | ERR1           |
| 31   | COV1 | Create `coverage/` directory                                 | `coverage/`                          | 2 min  | Low     | Low         | —              |
| 32   | COV2 | Move `coverage.out` → `coverage/coverage.out`                | `coverage/`                          | 2 min  | Low     | Low         | COV1           |
| 33   | COV3 | Update `.gitignore` for coverage artifacts                   | `.gitignore`                         | 2 min  | Low     | Low         | COV1           |
| 34   | DOC1 | Add ADR: CQRS adoption decision                              | `docs/adr/0001-cqrs-adoption.md`     | 10 min | Low     | Low         | —              |
| 35   | DOC2 | Add ADR: Branded ID migration                                | `docs/adr/0002-branded-ids.md`       | 10 min | Low     | Low         | —              |
| 36   | DOC3 | Add ADR: CRDT integration strategy                           | `docs/adr/0003-crdt-integration.md`  | 10 min | Low     | Low         | S1             |
| 37   | VAL1 | Add `govalid` struct tags to `AppConfig`                     | `cmd/examples/github-sync/config.go` | 10 min | Low     | Low         | —              |
| 38   | VAL2 | Add `govalid` struct tags to `SyncOptions`                   | `pkg/sync/sync.go`                   | 8 min  | Low     | Low         | —              |
| 39   | VAL3 | Add `govalid` struct tags to `CQRSConfig`                    | `pkg/cqrs/stack.go`                  | 8 min  | Low     | Low         | —              |
| 40   | BLD1 | Create `flake.nix` with dev shell                            | `flake.nix`                          | 12 min | Low     | Low         | —              |
| 41   | BLD2 | Add `nix build` target for binary                            | `flake.nix`                          | 10 min | Low     | Low         | BLD1           |
| 42   | INT1 | Create `internal/` directory, move `aggregate_id.go` helpers | `internal/`                          | 12 min | Low     | Low         | —              |
| 43   | C5   | Add `VectorClock` benchmark                                  | `pkg/crdt/benchmark_test.go`         | 8 min  | Low     | Low         | —              |
| 44   | C6   | Add `Operation[T]` serialization benchmark                   | `pkg/crdt/benchmark_test.go`         | 8 min  | Low     | Low         | —              |
| 45   | C7   | Add `SyncMessage` protocol roundtrip test                    | `pkg/crdt/conflict_test.go`          | 8 min  | Low     | Low         | —              |
| 46   | FIN1 | Update `FEATURES.md` with new features + coverage            | `FEATURES.md`                        | 10 min | Medium  | Medium      | All above      |
| 47   | FIN2 | Update `AGENTS.md` with architecture changes                 | `AGENTS.md`                          | 10 min | Medium  | Medium      | All above      |
| 48   | FIN3 | Final `go test ./...` + `golangci-lint run`                  | project                              | 5 min  | 🔥 High | Low         | All above      |
| 49   | FIN4 | `git push`                                                   | git                                  | 2 min  | 🔥 High | Low         | FIN3           |

---

## Dependency Graph (Simplified)

```
E1-E4 (exhaustruct fixes) ──┐
                            ├──→ FIN3 (final verification)
C1-C4 (VectorClock) ────────┤
     ↓                      │
S1 (LWWResolver) ───────────┤
     ↑                      │
C2 (NodeID) ────────────────┤
                            │
A1-A4 (HTTP API core) ──────┤
     ↓                      │
A5-A8 (API extensions) ─────┤
     ↓                      │
O1-O7 (OpenTelemetry) ──────┤
                            │
D1, DOC1-3, ERR1-2 ─────────┘
```

---

## Time Estimates

| Phase     | Tasks                                                                  | Total Time         | Cumulative Value |
| --------- | ---------------------------------------------------------------------- | ------------------ | ---------------- |
| 1 (1%)    | E1-E4, C1-C4, S1, A1-A4                                                | 13 tasks ≈ 110 min | 51%              |
| 2 (4%)    | O1-O6, A5-A6, D1, T1-T3                                                | 12 tasks ≈ 100 min | 64%              |
| 3 (20%)   | A7-A8, O7, ERR1-2, COV1-3, DOC1-3, VAL1-3, BLD1-2, INT1, C5-C7, FIN1-4 | 24 tasks ≈ 210 min | 80%              |
| **Total** | **49 tasks**                                                           | **≈ 420 min (7h)** | **80% Pareto**   |

---

## Execution Rules

1. **Max 12 min per task** — if a task exceeds, split it
2. **Commit after EVERY task** — `git add <files> && git commit -m "..."`
3. **Test after EVERY task** — `go test ./... -count=1`
4. **Lint after EVERY task** — `golangci-lint run ./...`
5. **Push at end of each phase** — `git push`
6. **Parallel where safe** — HTTP API tests can run while CRDT tests run

---

_Generated with Crush_
