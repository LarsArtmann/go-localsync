# Comprehensive Status Report — go-localsync

**Date:** 2026-05-28 06:21  
**Branch:** master  
**Commit:** 219f9a0  
**Author:** Lars Artmann <git@lars.software>

---

## a) FULLY DONE

| # | Item | Evidence |
|---|------|----------|
| 1 | **Zero lint issues** | `golangci-lint run ./...` reports 0 issues |
| 2 | **All tests pass** | `go test ./... -count=1` — 7 packages, all green |
| 3 | **CQRS migration complete** | Full event-sourced architecture, no legacy CRUD |
| 4 | **Branded IDs** | 5 branded types (ItemID, ExternalID, ProviderID, ActorID, RepoID, EventTypeID) |
| 5 | **Dual backend** | Memory + Turso backends with identical ReadModel API |
| 6 | **Conflict-aware sync** | Decider emits ItemConflictFound + ItemSynced events |
| 7 | **Provider abstraction** | Generic Provider interface with GitHub implementation |
| 8 | **Domain language documented** | `docs/DOMAIN_LANGUAGE.md` with full glossary |
| 9 | **Dead code removed** | `types.SourceID` eliminated, `AggregateID` cache removed |
| 10 | **Error taxonomy** | `go-error-family` with 3 families (Rejection, Transient, Infrastructure) |
| 11 | **Upstream doc** | `docs/planning/2026-05-25_UPSTREAM-SUGGESTIONS.md` for go-cqrs-lite team |
| 12 | **CI/CD** | GitHub Actions with 4-job pipeline |
| 13 | **No CGO** | Pure Go build with `CGO_ENABLED=0` |
| 14 | **Outbox pattern** | Turso backend uses `SQLTransactionalStore` for atomic save+publish |
| 15 | **Deterministic aggregate IDs** | SHA256→hex from (source, sourceID) with sync.Map cache removed |

---

## b) PARTIALLY DONE

| # | Item | Status | Gap |
|---|------|--------|-----|
| 1 | **Test coverage** | 73.7% total | `cmd/examples/github-sync` at 10.5%, `pkg/sync` at 77.8% |
| 2 | **CLI** | Functional | Missing: JSON output flag, daemon mode, multi-user sync |
| 3 | **Provider system** | GitHub only | No GitLab, Jira, or other providers |
| 4 | **Conflict resolution** | Remote-wins only | No configurable strategy (local-wins, manual, merge) |
| 5 | **pkg/localsync** | Generic sync primitives | NOT integrated into main sync flow — isolated module |
| 6 | **Read model** | Memory + Turso | No PostgreSQL or other SQL backends |
| 7 | **HTTP API** | None | Must use Go SDK or CLI directly |
| 8 | **Export** | None | No JSON/CSV export of stored events |
| 9 | **Domain language** | Documented | Terms defined but not yet used in code comments |
| 10 | **Logging** | charm.land/log/v2 | Mixed with `log/slog` import in `stack.go` (unused?) |

---

## c) NOT STARTED

| # | Item | Why Not Started |
|---|------|-----------------|
| 1 | **HTTP API** | Out of scope for CLI-focused project |
| 2 | **WebSocket/SSE** | No real-time requirement |
| 3 | **Bubble Tea TUI** | Listed in ROADMAP as low priority |
| 4 | **Multi-provider sync** | Only GitHub exists |
| 5 | **Event retention/TTL** | No requirement yet |
| 6 | **Schema evolution** | Only 1 schema version |
| 7 | **OpenTelemetry tracing** | Not requested |
| 8 | **Metrics endpoint** | Not requested |
| 9 | **Structured JSON output** | Listed in TODO but not implemented |
| 10 | **Real GitHub PAT smoke test** | Requires secrets management |

---

## d) TOTALLY FUCKED UP

| # | Item | What Happened | Fix Needed |
|---|------|--------------|------------|
| 1 | **LSP stale diagnostics** | LSP cache shows deleted `aggregate_id_test.go` errors | Restart LSP or ignore (CLI is clean) |
| 2 | **Log dependency inconsistency** | `stack.go` imports `log/slog` alongside `charm.land/log/v2` | Audit if slog is actually used or just imported |
| 3 | **pkg/localsync is an island** | Generic sync primitives (VectorClock, LWWResolver) are NOT used by `pkg/sync` | Either integrate or delete |
| 4 | **68 fmt.Errorf calls** | Every error is hand-wrapped with `fmt.Errorf("...: %w", err)` | Consider `errors.Wrap` or `go-error-family` wrappers |
| 5 | **Exhaustruct config bloat** | `ItemFilter` has 7 pointer fields, all must be specified | Consider builder pattern or functional options |
| 6 | **Store factory returns interface** | `createReadModel`, `createSnapshotStore`, `createCheckpointStore` return interfaces with `nolint:ireturn` | Design: should return concrete types |

---

## e) WHAT WE SHOULD IMPROVE

### Architecture

1. **Connect `pkg/localsync` to `pkg/sync`** — The generic sync primitives (VectorClock, LWWResolver, Operation) were designed for this project but are orphaned. Either use them or delete them.
2. **Unify logging** — Pick one: `charm.land/log/v2` or `log/slog`. Mixed imports suggest confusion.
3. **Replace `ItemFilter` with functional options** — 7 pointer fields are painful. `List(ctx, WithType(t), WithLimit(10))` is cleaner.
4. **Factory return types** — `createReadModel` should return `*MemoryReadModel` or `*TursoReadModel`, not `ReadModel` interface.

### Code Quality

5. **Reduce fmt.Errorf repetition** — 68 calls. Many are identical patterns. Consider a helper.
6. **Add context to all errors** — Many errors lack context about which item/operation failed.
7. **Add structured logging** — Log items with consistent fields (source, externalID, operation).
8. **Add metrics** — Track sync duration, conflict rate, provider error rate.

### Testing

9. **Increase `cmd/examples/github-sync` coverage** — 10.5% is the biggest gap.
10. **Add `SyncIncremental` tests** — Only 37.5% coverage.
11. **Add `ConflictAwareSyncer` tests** — Only 68% coverage.
12. **Add integration test** — End-to-end with memory backend.

### Features

13. **JSON output flag** — `-json` for stats and sync results.
14. **Structured logging fields** — Add context fields to all log statements.
15. **Provider registration** — Make it easy to add new providers without modifying core.

---

## f) Top #25 Things To Get Done Next

Sorted by **Impact / Effort** ratio (highest first):

| Rank | Task | Impact | Effort | Package | Depends On |
|------|------|--------|--------|---------|-----------|
| 1 | Integrate `pkg/localsync` into `pkg/sync` | 🔥 High | ~2h | `pkg/sync` | #2 |
| 2 | Delete `pkg/localsync` if unused | 🔥 High | ~30min | `pkg/localsync` | — |
| 3 | Fix log dependency inconsistency | 🔥 High | ~15min | `pkg/cqrs` | — |
| 4 | Replace `ItemFilter` with functional options | High | ~2h | `pkg/provider` | — |
| 5 | Add CLI test coverage | High | ~2h | `cmd/examples/github-sync` | — |
| 6 | Add `SyncIncremental` tests | Medium | ~1h | `pkg/sync` | — |
| 7 | Add `ConflictAwareSyncer` tests | Medium | ~1h | `pkg/sync` | — |
| 8 | Reduce `fmt.Errorf` repetition | Medium | ~1h | All | — |
| 9 | Add context to errors | Medium | ~1h | All | — |
| 10 | Factory return concrete types | Medium | ~30min | `pkg/cqrs` | — |
| 11 | Add JSON output flag | Medium | ~1h | `cmd/examples/github-sync` | — |
| 12 | Add structured logging fields | Low | ~1h | `pkg/sync`, `pkg/providers/github` | — |
| 13 | Add metrics collection | Low | ~2h | `pkg/sync` | — |
| 14 | Add integration test | Low | ~2h | `integration_test/` | — |
| 15 | Add provider registration system | Low | ~3h | `pkg/provider` | — |
| 16 | Add export to JSON/CSV | Low | ~2h | `cmd/examples/github-sync` | — |
| 17 | Add multi-user sync | Low | ~3h | `cmd/examples/github-sync` | — |
| 18 | Add event retention/TTL | Low | ~2h | `pkg/cqrs` | — |
| 19 | Add OpenTelemetry tracing | Low | ~3h | All | — |
| 20 | Add PostgreSQL backend | Low | ~4h | `pkg/cqrs` | — |
| 21 | Add Bubble Tea TUI | Low | ~4h | `cmd/` | — |
| 22 | Add WebSocket/SSE | Low | ~3h | New | — |
| 23 | Add real PAT smoke test | Low | ~2h | CI | Secrets |
| 24 | Add schema evolution | Low | ~4h | `pkg/cqrs` | — |
| 25 | Add conflict resolution config | Low | ~2h | `pkg/sync` | #1 |

---

## g) Top #1 Question I Cannot Figure Out Myself

**Should `pkg/localsync` be integrated into `pkg/sync` or deleted?**

`pkg/localsync` contains genuinely useful generic sync primitives:
- `VectorClock` — causal ordering across distributed nodes
- `LWWResolver[T]` — Last-Write-Wins conflict resolution with vector clock comparison
- `Operation[T]` — generic sync operation with payload
- `Conflict[T]` — structured conflict representation

But `pkg/sync` currently hard-codes:
- Simple timestamp comparison (`HasChanged`) instead of vector clocks
- Remote-wins strategy instead of pluggable `ConflictResolver[T]`
- No `Operation[T]` abstraction

**Options:**
1. **Integrate**: Replace hard-coded conflict logic with `LWWResolver[T]` and add vector clock support
2. **Delete**: Remove `pkg/localsync` — it's dead code that confuses the architecture
3. **Keep as-is**: Leave it as a "future use" module (current state)

The decision affects ~584 LOC and the architectural direction of conflict resolution.

**Recommendation needed from project owner.**

---

## Metrics

| Metric | Value |
|--------|-------|
| Total packages | 7 |
| Total test functions | 197+ |
| Overall coverage | 73.7% |
| Lint issues | 0 |
| Build status | ✅ Pass |
| Test status | ✅ Pass |
| Go version | 1.26.2 |
| Production LOC | ~2,947 |
| Test LOC | ~3,997 |
| External dependencies | 17 direct |
| go-cqrs-lite modules used | 5 of 12 |
| nolint directives | 9 (6 justified, 3 factories) |

---

## Dependency Health

| Dependency | Status | Notes |
|------------|--------|-------|
| `go-cqrs-lite/*` | ✅ Healthy | Pseudo-version for storage (local replace) |
| `go-branded-id` | ✅ Healthy | v0.1.0 |
| `go-error-family` | ✅ Healthy | v0.1.1 |
| `charm.land/log/v2` | ⚠️ Concern | Heavy dependency, consider `log/slog` |
| `go-github/v69` | ✅ Healthy | v69.2.0 |
| `turso.tech/database/tursogo` | ✅ Healthy | v0.6.0 |
| `golang.org/x/oauth2` | ✅ Healthy | v0.36.0 |

---

## Build Commands

```bash
# Test
go test ./... -count=1

# Test with race
go test ./... -race -count=1

# Lint
golangci-lint run ./...

# Build
go build ./...

# Coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```
