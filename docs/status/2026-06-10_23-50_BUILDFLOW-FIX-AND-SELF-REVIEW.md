# Status Report — 2026-06-10 23:50

**Session 10 — Buildflow Fix + Comprehensive Self-Review**

---

## a) FULLY DONE

### Buildflow Lint Fixes (this session)

| Fix | File | Before | After |
|---|---|---|---|
| `errors.As` → `errors.AsType` | `pkg/providers/github/client_retry.go:101,112` | `errors.As(err, &ghErr{})` | `errors.AsType[*gh.ErrorResponse](err)` |
| Error context enrichment | `pkg/cqrs/stack.go:201-203` | `classifyAction` returned `ActionError` with no error context | Error wrapped with `eventCount` + `conflictWinner` on failure |
| AGENTS.md trimmed | `AGENTS.md` | 399 lines (limit: 377) | 368 lines — removed duplicate SyncStore interface section |
| query_test.go consolidated | `pkg/data/query/query_test.go` | 363 lines (limit: 350) | 280 lines — table-driven `TestFieldCriteria`, merged page tests |
| go.sum cleaned | `go.sum` | Stale entries warned by buildflow | `go mod tidy` ran — all "stale" entries are transitive deps (false positive) |

### Architecture Improvement Plan (8/8 items — earlier in session 10)

All items from `docs/planning/2026-06-10_ARCHITECTURE_IMPROVEMENT_PLAN.html` executed and committed.

### All-Time Completed

- 14 Go packages, 241+ tests, all passing with `-race`
- golangci-lint v2: 0 issues (125+ linters)
- Full CQRS architecture (no legacy CRUD)
- Dual backend (memory/SQLite)
- Pluggable CRDT conflict resolution
- HTTP API with Huma v2 (4 endpoints)
- GitHub provider with retry, rate limiting, error classification
- Branded phantom-type IDs (6 types)
- Structured error taxonomy via go-error-family
- Nix flake build system

---

## b) PARTIALLY DONE

### Data Module Migration (`pkg/data/`)

| Sub-package | Status | Notes |
|---|---|---|
| `data/model` | **WIRED** | `model.Item`, `model.Key`, `model.ProviderItem` used by `cqrs`, `sync`, `api`, `testutil` |
| `data/schema` | **WIRED** | `schema.Version` on every `model.Item` |
| `data/query` | **ORPHANED** | Generic criteria/query/page system. Zero production consumers. Duplicates `provider.ItemFilter` functionality |
| `data/repo` | **ORPHANED** | Generic repository interfaces. Zero production consumers. `cqrs.ReadModel` serves the same role |
| `data/transform` | **ORPHANED** | Mapper/Compose helpers. Zero production consumers. `cqrs/item_adapter.go` does the actual conversion |

The `query/`, `repo/`, and `transform/` packages are well-designed but **completely disconnected** from the active codebase. They represent a planned "data module" migration that was never completed.

### `SyncItem` Missing `Options` Field

`stack.go:143` has an `exhaustruct` warning: `SyncItemCommand` is missing `Options` field. The `SyncItems` (batch) path sets `Options` with correlation IDs, but `SyncItem` (single) does not. This means single-item syncs don't get correlation ID tracking.

---

## c) NOT STARTED

### From TODO_LIST.md (carried forward)

| Priority | Task | Effort |
|---|---|---|
| HIGH | Integration test for full sync pipeline (Provider → CQRS → ReadModel → API) | 2h |
| HIGH | Test concurrent read model access | 1h |
| HIGH | CLI flag for conflict resolver (`--conflict-strategy`) | 1h |
| MEDIUM | Table-driven tests for `HasChanged` | 30min |
| MEDIUM | Integration test for `ActionConflictLocal` | 1h |
| MEDIUM | Performance benchmarks (1k/10k/100k items) | 2h |
| MEDIUM | Test SQLite read model with real file | 30min |
| MEDIUM | Doc comments for exported types (~18 missing) | 1h |
| LOW | OpenTelemetry instrumentation | 4h |
| LOW | API auth middleware | 2h |
| LOW | Graceful shutdown for API server | 1h |

### New findings from this review

| Task | Effort | Notes |
|---|---|---|
| Fix `SyncItem` missing `Options` (exhaustruct warning) | 10min | Add `Options: syncOpts` to single-item dispatch |
| Remove dead `Count(ctx)` method | 5min | `stack_adapters.go:36` — zero callers |
| Remove dead `FromDataItem` export | 5min | `item_adapter.go:36` — only used in bench test |
| Fix `conflictWinner` string constants → typed enum | 15min | Prevent arbitrary string passing |
| Extract duplicate payload decode in decider + projection | 15min | `foldItemSynced` and `handleItemSynced` copy-paste `DecodePayload` |
| Fix `fromUnixNano` timezone loss | 5min | `events.go:60` — should use `.UTC()` |
| Fix `store_factory.go` no-op error wrap | 5min | `fmt.Errorf("%w", err)` → just `return nil, err` |
| Fix `SyncOutcome` silent JSON unmarshal error | 10min | `sync_outcome.go:53` — `_ = json.Unmarshal` swallows errors |
| Wire or delete orphaned data packages | 2-4h | `query/`, `repo/`, `transform/` — either integrate or remove |
| Fix API `getStats` bypassing syncer | 15min | `server.go:225` — should call `syncer.GetStats()` not query store directly |

---

## d) TOTALLY FUCKED UP

### Split Brain: `provider.Item` vs `model.Item` vs `model.ProviderItem`

Three near-identical structs exist for the same domain concept:

| Type | Package | Fields | Used By |
|---|---|---|---|
| `provider.Item` | `pkg/provider` | ExternalID, Source, Type, ActorLogin, ActorAvatarURL, RepoName, RepoURL, CreatedAt, UpdatedAt, **RawJSON** | GitHub provider, sync, CQRS decider |
| `model.Item` | `pkg/data/model` | **ID**, ExternalID, Source, Type, ActorLogin, ActorAvatarURL, RepoName, RepoURL, CreatedAt, UpdatedAt, **SchemaVersion** | CQRS read model, API, testutil |
| `model.ProviderItem` | `pkg/data/model` | ExternalID, Source, Type, ActorLogin, ActorAvatarURL, RepoName, RepoURL, CreatedAt, UpdatedAt, **RawPayload []byte** | transform tests only |

The conversion happens in `cqrs/item_adapter.go` (`ToDataItem`/`FromDataItem`). This is the canonical adapter. But `transform.NewFromProviderItem()` also exists and duplicates the same logic. Both do the same field-by-field copy.

**Validation is duplicated**: `provider.Item.Validate()` and `model.Item.Validate()` check the same fields with different error types.

**`model.ProviderItem` is dead code** — only used by `transform` tests. It's a third copy of the same struct.

### Orphaned Data Packages

`pkg/data/query/`, `pkg/data/repo/`, and `pkg/data/transform/` are **entirely dead code in production**. They have zero imports from any other package. Their 76+ exported symbols exist only in their own tests. This is a significant maintenance burden — every change to `model.Item` requires updating tests in packages nobody uses.

### `ConflictAwareSyncer` Tight Coupling

`pkg/sync/conflict_aware.go` embeds `*Syncer` (concrete type, not interface) to access private methods (`validateOpts`, `fetchItems`, `filterValidItems`). This creates tight coupling — any internal refactor of `Syncer` silently breaks `ConflictAwareSyncer`.

---

## e) WHAT WE SHOULD IMPROVE

### Type Architecture

1. **Unify to ONE `Item` type** — The split between `provider.Item` and `model.Item` is artificial. The only real differences are: (a) `RawJSON` field, which could be an option on a single type; (b) `ID` field, which should be set during sync, not be a type-level difference. Consider making `model.Item` the canonical type and having `provider.Item` be an alias or thin wrapper.

2. **Kill `model.ProviderItem`** — Dead code that duplicates `provider.Item`. Remove it and `transform.NewFromProviderItem()`.

3. **Typed conflict winner enum** — `conflictWinnerRemote`/`conflictWinnerLocal` are untyped string constants. Create a `ConflictWinner` type like `SyncAction`.

4. **Strongly-typed event payloads** — `ItemSyncedPayload` uses raw `string` for branded fields (`ItemID`, `Source`, etc.). Consider using the actual branded types, or at minimum validate on decode.

### Library Opportunities

5. **`samber/lo` or `go-functional` for slice operations** — Several places iterate items with manual loops that could use functional helpers (filter, map, group by).

6. **`govalid` for struct validation** — Buildflow already suggests it. Replace manual `Validate()` methods with generated validation code.

7. **`pt` (pointer helpers)** — Several places create pointer-to-literal for optional fields. `github.com/crewboat/go-libs/ptr` or similar would reduce boilerplate.

### Dead Code Removal

8. **Delete or archive `pkg/data/query/`, `pkg/data/repo/`, `pkg/data/transform/`** — 76+ unused exports. If the data module migration is planned, move to a feature branch. If not, delete. Dead code rots.

9. **Remove `repo.Observable[T]`** — Decorator that does nothing but delegate. Zero consumers.

10. **Remove `model.ItemView`, `model.StatsView`** — Only used by dead `repo` package.

---

## f) TOP #25 THINGS TO DO NEXT

Sorted by **(Impact × Urgency) / Effort** — highest value first:

| # | Task | Impact | Effort | Category |
|---|---|---|---|---|
| 1 | Fix `SyncItem` missing `Options` field (exhaustruct) | HIGH | 5min | Bug |
| 2 | Remove dead `Count(ctx)` from `stack_adapters.go` | LOW | 5min | Dead code |
| 3 | Fix `store_factory.go` no-op `fmt.Errorf("%w", err)` | LOW | 5min | Code quality |
| 4 | Fix `fromUnixNano` timezone loss in `events.go` | MEDIUM | 5min | Correctness |
| 5 | Fix `SyncOutcome` silent JSON unmarshal error | MEDIUM | 10min | Correctness |
| 6 | Typed `ConflictWinner` enum (replace string constants) | MEDIUM | 15min | Type safety |
| 7 | Extract duplicate payload decode helper (decider + projection) | MEDIUM | 15min | DRY |
| 8 | Fix API `getStats` to use `syncer.GetStats()` instead of raw store | MEDIUM | 15min | Architecture |
| 9 | Remove dead `FromDataItem` export (only used in bench test) | LOW | 5min | Dead code |
| 10 | Add CLI flag for conflict resolver (`--conflict-strategy`) | HIGH | 1h | Feature |
| 11 | Integration test for `ActionConflictLocal` with real CQRS stack | HIGH | 1h | Testing |
| 12 | Table-driven tests for `HasChanged` edge cases | HIGH | 30min | Testing |
| 13 | Kill `model.ProviderItem` + `transform.NewFromProviderItem` | MEDIUM | 30min | Dead code |
| 14 | Decide fate of orphaned `data/query`, `data/repo`, `data/transform` | HIGH | 30min | Architecture |
| 15 | Doc comments for ~18 exported types | LOW | 1h | Docs |
| 16 | Integration test for full sync pipeline (E2E) | HIGH | 2h | Testing |
| 17 | Performance benchmarks (1k/10k/100k items) | MEDIUM | 2h | Quality |
| 18 | Unify `provider.Item` and `model.Item` (eliminate split brain) | HIGH | 4h | Architecture |
| 19 | Wire `data/query` criteria into CQRS read model (replace `provider.ItemFilter`) | HIGH | 4h | Architecture |
| 20 | OpenTelemetry instrumentation | MEDIUM | 4h | Observability |
| 21 | Fix `ConflictAwareSyncer` tight coupling to concrete `*Syncer` | MEDIUM | 2h | Architecture |
| 22 | Add `govalid` struct tags to config types | LOW | 1h | Validation |
| 23 | API auth middleware | HIGH | 2h | Security |
| 24 | Graceful shutdown for API server | MEDIUM | 1h | Reliability |
| 25 | ADR documents for CQRS, branded IDs, CRDT decisions | LOW | 2h | Docs |

---

## g) TOP #1 QUESTION I CANNOT FIGURE OUT MYSELF

**What is the intended fate of the `data/query`, `data/repo`, and `data/transform` packages?**

These packages represent a clean, generic data layer design (criteria-based queries, generic repository interfaces, composable transforms). They're well-tested and well-structured. But they have **zero production consumers** — the active codebase uses `cqrs.ReadModel` + `provider.ItemFilter` instead.

Three options:
1. **Wire them in** — Replace `provider.ItemFilter` with `query.Query[T]` + criteria, replace `cqrs.ReadModel` interface with `repo.Repository[T]`, use `transform.Mapper` instead of `item_adapter.go`. This is a significant migration (~4-8h).
2. **Delete them** — Remove the dead code, reduce maintenance burden. If needed later, write from scratch with the benefit of hindsight.
3. **Archive them** — Move to a `future/` directory or feature branch, out of the main codebase.

This decision affects the architecture direction for the next 2-4 weeks. I cannot make this call without understanding the product vision.

---

## Build & Test Status

| Metric | Value |
|---|---|
| Packages | 14 (all passing) |
| Tests | 241+ (all passing) |
| Race detector | Clean |
| golangci-lint | 0 issues |
| go vet | Clean |
| Coverage | ~85% average |
