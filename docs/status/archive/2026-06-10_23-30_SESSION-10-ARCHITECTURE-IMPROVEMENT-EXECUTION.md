# Status Report — 2026-06-10 23:30

**Session 10 — Architecture Improvement Plan Execution + Comprehensive Review**

---

## a) FULLY DONE

### Architecture Improvement Plan (8/8 items from `docs/planning/2026-06-10_ARCHITECTURE_IMPROVEMENT_PLAN.html`)

| ID   | Item                                                                                                                                                                                                                                                                   | Impact          |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------- |
| LS-1 | **SyncItems through command pipeline** — `SyncItems` now dispatches per-item via `CommandDispatcher` instead of calling `Repo.Execute` directly. New `SyncOutcome` type captures domain semantics. Logging, validation, retry middleware all apply to batch syncs now. | Correctness     |
| LS-2 | **Compile-time SyncStore assertion** — `var _ synclib.SyncStore = (*CQRSStack)(nil)`                                                                                                                                                                                   | Type safety     |
| LS-3 | **CRDT doc.go updated** — Active types vs Future types clearly documented                                                                                                                                                                                              | Clarity         |
| LS-4 | **Consistent not-found semantics** — `MemoryReadModel.Get()` returns `ErrNotFound` (was `(nil, nil)`)                                                                                                                                                                  | Dev/prod parity |
| LS-5 | **NewServer simplified** — `api.NewServer(syncer, logger)`, `Syncer.Store()` getter added                                                                                                                                                                              | API surface     |
| LS-6 | **Duplicate GetTypes removed** — Single `GetItemTypes()` method                                                                                                                                                                                                        | Dead code       |
| LS-7 | **Dead raw_json removed** — Column, scannedItem field, unused `ToItemView` all gone                                                                                                                                                                                    | Schema clarity  |
| LS-8 | **Runner errors logged** — `slog.Error` instead of `_ = runner.Run(ctx)`                                                                                                                                                                                               | Observability   |

### Upstream API Compatibility Fixes (go-cqrs-lite)

- `cqrsid.MustParseAggregateID` → `cqrsid.ParseAggregateID`
- `command.MustNew` → `command.New` with `mustNewCommand` helper
- `query.MustNew` → `query.New` with `mustNewQuery` helper

### Additional Fixes Applied

- Removed unused `fmt` import from `stack_adapters.go`
- Enhanced error wrapping in `SyncItems` (eventCount + conflictWinner in error message)
- `errors.AsType` adoption in GitHub client (cleaner than `errors.As` + empty struct)

### Test Status

- **14 packages, 250+ tests, ALL GREEN**
- `go build ./...` clean
- `go vet ./...` clean
- `go test ./... -count=1` — 0 failures

---

## b) PARTIALLY DONE

### pkg/data/ Layer — Designed but Not Wired

The `pkg/data/` package is a well-architected layer system that is **disconnected from production code**:

| Package          | Purpose                                                        | Production Usage                                                       |
| ---------------- | -------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `data/model`     | Canonical domain types (`Item`, `Key`, `View`, `ProviderItem`) | `model.Item` used by CQRS. `model.Key`, `model.ProviderItem` unused.   |
| `data/query`     | Generic `Criterion[T]`, `Query[T]`, `Page[T]` query system     | Never imported by CQRS or API. They use `provider.ItemFilter` instead. |
| `data/transform` | `Mapper[From,To]` with compose, provider→domain→view mappers   | Never imported by production code. CQRS uses `ToDataItem` directly.    |
| `data/repo`      | `Reader[T]`, `Writer[T]`, `Repository[T]`, `Observable[T]`     | Zero production implementations. Only used in its own tests.           |
| `data/schema`    | `Version` type with V1/V2 constants                            | Used by CQRS `DataItemFromPayload`.                                    |

**Assessment:** `data/query`, `data/transform`, and `data/repo` are well-designed aspirational code with tests. They need to either be wired into the production pipeline or deleted. Currently they add cognitive overhead without providing value.

### Documentation Staleness

| File                           | Issue                                                                        |
| ------------------------------ | ---------------------------------------------------------------------------- |
| `README.md:127`                | Says "Turso-backed" — should be "SQLite-backed"                              |
| `TODO_LIST.md:73-74`           | References `turso_readmodel_test.go` (renamed to `sqlite_readmodel_test.go`) |
| `TODO_LIST.md`                 | Test counts outdated (says "241" — actually 250+)                            |
| `pkg/cqrs/store_factory.go:71` | `ConfigureTursoPool` — misleading name (does generic SQLite pool config)     |

---

## c) NOT STARTED

### Known Gaps (from ROADMAP.md and codebase analysis)

1. **TUI with Bubble Tea** — Interactive terminal UI for sync management
2. **Multi-user sync** — Multiple `-user` flags for parallel sync
3. **Daemon/background mode** — cron/systemd support
4. **JSON/CSV export** — Data export endpoints
5. **OpenTelemetry instrumentation** — Tracing/metrics
6. **Real-time sync protocol** — CRDT `SyncRequest`/`SyncResponse` wire protocol
7. **Second provider** — Only GitHub exists; GitLab or others would validate the Provider interface
8. **Migration tooling** — Schema evolution for SQLite read model
9. ~~**CI pipeline** — No GitHub Actions or equivalent configured~~ → false — CI has been green across test/lint/security/build/provider jobs
10. **Benchmarking/Profiling** — No performance baselines or load tests

---

## d) TOTALLY FUCKED UP!

### Nothing Is Broken — But Several Things Need Attention

1. **LSP Diagnostics Are Lying** — The `golangci_lint_ls` reports `typecheck` errors for `main.go:140` and `server_test.go:104` that **do not exist in the actual code**. These are stale cache artifacts. `go build ./...` and `go test ./...` both pass clean. The LSP needs a restart.

2. **`ConfigureTursoPool` Function Name** — In `store_factory.go:71`, this function does generic SQLite pool configuration but still carries the "Turso" name. This was missed during the Session 8 turso→sqlite rename. Low priority but confusing.

3. **`HasChanged` Doesn't Check `ActorAvatarURL`** — `decider.go:223-228` compares `UpdatedAt`, `Type`, `ActorLogin`, `RepoName`, `RepoURL` but NOT `ActorAvatarURL`. An avatar URL change would be silently dropped. This may be intentional (avatars are cosmetic) but should be documented.

4. **`SyncIncremental` Timestamp Edge Case** — Uses `CreatedAt` for cutoff with `Before()` (strict less-than). Items created at exactly the same timestamp as the latest item are re-synced. This is correct but combined with the fact that GitHub events can share timestamps, it could cause unnecessary syncs.

5. **`go.work` References `go 1.26.3`** — This version doesn't exist in standard Go releases. It's either a custom build or forward-looking. Not a bug but worth noting.

---

## e) WHAT WE SHOULD IMPROVE!

### Critical Architecture Issues

1. **`provider.Item` vs `model.Item` vs `model.ProviderItem` — Three Item Types**
   - The sync pipeline converts `provider.Item → model.Item` via `cqrs.ToDataItem()`
   - `model.ProviderItem` was designed for the `data/transform` layer but never wired
   - `provider.Item` carries `RawJSON` while `model.Item` carries `SchemaVersion`
   - **Fix:** Decide on ONE canonical item type. Either make `model.Item` the universal type (with optional RawJSON) or eliminate `model.ProviderItem` and keep the current two-type system with clear documentation.

2. **`data/` Package Is an Island**
   - `query/`, `transform/`, `repo/` are well-designed generics-based utilities with tests but zero production usage
   - The CQRS layer has its own filter/pagination implementation (`provider.ItemFilter` + `matchesFilter()`)
   - **Fix:** Either wire `data/query` into the read model (replacing `provider.ItemFilter`), or delete the unused packages. Having both is confusing.

3. **`sync_outcome.go` Has No Direct Tests**
   - The `SyncOutcome`, `contextWithSyncOutcome`, `decideWithOutcome` helpers are only tested indirectly through stack integration tests
   - The `nlreturn` lint warning is real (missing blank line before return in `syncOutcomeFromContext`)
   - **Fix:** Add focused unit tests for the outcome capture mechanism.

### Type Model Improvements

4. **Branded IDs Could Carry More Type Safety**
   - `Source` is `id.ProviderID` (a `StringID[ProviderTag]`) but could have a `Source()` method that returns the provider name — currently `item.Source.Get()` is the pattern
   - `ExternalID` is used as a generic string but could be `SourceSpecificID` with source validation
   - The `id` package is clean and well-branded — minor improvements only

5. **Error Types Are Strong But Under-Utilized**
   - `go-error-family` provides `Rejection`, `Transient`, `Infrastructure` classification
   - The API layer maps errors to HTTP status codes via `mapSyncError` but doesn't use structured error responses
   - **Fix:** Return structured error bodies from the API using Huma's error types

### Library Opportunities

6. **`samber/lo` or `samber/slice` for Collection Operations**
   - The codebase has hand-rolled filter/map/pagination in `memory_readmodel.go`
   - `lo.Filter`, `lo.Map`, `lo.Slice` would reduce ~50 lines of boilerplate
   - Already in indirect dependencies (via `golang.org/x/exp`)

7. **`ochinchina/gorl` or custom rate limiter**
   - The GitHub client has hand-rolled rate limiting via `time.Sleep` + header parsing
   - A token-bucket rate limiter would be more robust and testable

8. **`matryer/mo` for Option/Result Types**
   - `SyncOutcome` uses `*SyncOutcome` (nil = no outcome) — a proper `Option[SyncOutcome]` would make intent clearer
   - `Result[T]` would make error handling in the sync pipeline more explicit

---

## f) Top #25 Things We Should Get Done Next

Sorted by **Impact × Effort** (highest ROI first):

### Tier 1: Quick Wins (< 30 min each, high impact)

| # | Item                                                                 | Effort | Impact       | Why                                                                   |
| - | -------------------------------------------------------------------- | ------ | ------------ | --------------------------------------------------------------------- |
| 1 | Fix README.md "Turso-backed" → "SQLite-backed"                       | 5 min  | Correctness  | Misleading docs                                                       |
| 2 | Rename `ConfigureTursoPool` → `ConfigureSQLitePool` in store_factory | 5 min  | Clarity      | Last turso remnant in code                                            |
| 3 | Fix `nlreturn` lint in `sync_outcome.go`                             | 2 min  | Lint hygiene | One blank line                                                        |
| 4 | Add `HasChanged` doc comment about fields NOT compared               | 5 min  | Clarity      | Silent data loss potential                                            |
| 5 | Update TODO_LIST.md stale references (test count, file names)        | 10 min | Accuracy     | Outdated metrics                                                      |
| 6 | Add compile-time `ReadModel` assertions for both implementations     | 5 min  | Type safety  | `var _ ReadModel = (*MemoryReadModel)(nil)` already exists for SQLite |
| 7 | Restart LSP to clear stale diagnostics                               | 1 min  | DX           | False errors distracting                                              |

### Tier 2: Medium Effort (1-3 hours each, high impact)

| #  | Item                                                      | Effort | Impact       | Why                           |
| -- | --------------------------------------------------------- | ------ | ------------ | ----------------------------- |
| 8  | Add focused tests for `SyncOutcome` + `decideWithOutcome` | 30 min | Coverage     | New code has no direct tests  |
| 9  | Delete or wire `data/repo/` package                       | 1 hr   | Clarity      | Zero prod implementations     |
| 10 | Delete or wire `data/transform/` package                  | 1 hr   | Clarity      | Never wired into pipeline     |
| 11 | Delete `model.ProviderItem` or wire it                    | 30 min | Clarity      | Dead type with tests          |
| 12 | Add `ActorAvatarURL` to `HasChanged` comparison           | 15 min | Correctness  | Silent data loss              |
| 13 | Extract `provider.ItemFilter` → use `data/query` criteria | 2 hr   | Architecture | Unify query layer             |
| 14 | Add structured API error responses via Huma               | 1 hr   | API quality  | Currently returns bare errors |

### Tier 3: Larger Efforts (3+ hours each, strategic impact)

| #  | Item                                                    | Effort | Impact                  | Why                                   |
| -- | ------------------------------------------------------- | ------ | ----------------------- | ------------------------------------- |
| 15 | Second provider (GitLab) to validate Provider interface | 4 hr   | Architecture validation | Only GitHub exists                    |
| 16 | Wire `data/query` into CQRS read model                  | 3 hr   | Architecture            | Replace hand-rolled filter/pagination |
| 17 | OpenTelemetry tracing for sync pipeline                 | 3 hr   | Observability           | Production readiness                  |
| 18 | CI pipeline (GitHub Actions)                            | 2 hr   | Quality                 | No automated CI exists                |
| 19 | Benchmark suite with baselines                          | 2 hr   | Performance             | No perf regression detection          |
| 20 | Schema migration tooling for SQLite                     | 4 hr   | Operations              | No DDL evolution strategy             |

### Tier 4: Long-term Strategic

| #  | Item                                                         | Effort  | Impact           | Why          |
| -- | ------------------------------------------------------------ | ------- | ---------------- | ------------ |
| 21 | TUI with Bubble Tea                                          | 1 week  | UX               | Roadmap goal |
| 22 | Multi-node sync protocol (CRDT `SyncRequest`/`SyncResponse`) | 2 weeks | Architecture     | Roadmap goal |
| 23 | Daemon/background mode                                       | 1 week  | Operations       | Roadmap goal |
| 24 | JSON/CSV export endpoints                                    | 4 hr    | API completeness | Roadmap goal |
| 25 | Multi-user sync (parallel `-user` flags)                     | 1 week  | Scale            | Roadmap goal |

---

## g) Top #1 Question I Cannot Figure Out Myself

**Should the `pkg/data/` aspirational packages (`query/`, `transform/`, `repo/`) be wired into production or deleted?**

The `data/query` package provides a clean generic `Criterion[T]` + `Query[T]` + `Page[T]` system that is strictly more capable than the hand-rolled `provider.ItemFilter` + `matchesFilter()` in the CQRS read model. But wiring it in would mean:

- `provider.ItemFilter` becomes `query.Query[*model.Item]` with typed criteria
- The read model's `List`/`Count` methods accept `query.Query[T]` instead of `provider.ItemFilter`
- The API layer constructs queries using the criteria builders
- The CQRS `SyncStore` interface changes (breaking change)

Alternatively, we delete the unused packages and keep the simpler `provider.ItemFilter` approach. The current filter is ~30 lines of straightforward code that works well.

**I need your call on direction:** Invest in wiring the generic query system (higher architecture quality, more effort), or prune the dead code and keep it simple?

---

## Build & Test Status

```
$ go build ./...     ✅ Clean (0 errors)
$ go test ./...      ✅ 14 packages, 250+ tests, ALL GREEN
$ go vet ./...       ✅ Clean (0 warnings)
$ git status         3 modified files (minor: golangci.yml regex, error wrapping, errors.AsType)
```

## File Change Summary (Session 10)

### New Files

- `pkg/cqrs/sync_outcome.go` — `SyncOutcome` type + context helpers + `decideWithOutcome`

### Modified Files (committed)

- `pkg/cqrs/stack.go` — SyncItems through command pipeline, compile-time assertion, error wrapping
- `pkg/cqrs/stack_adapters.go` — Removed duplicate GetTypes, removed unused fmt import
- `pkg/cqrs/stack_test.go` — GetTypes → GetItemTypes
- `pkg/cqrs/commands_queries.go` — Added Options to SyncItemCommand, mustNewCommand/mustNewQuery helpers, handleSyncItem uses outcome
- `pkg/cqrs/memory_readmodel.go` — Get() returns ErrNotFound
- `pkg/cqrs/readmodel_test.go` — Updated for ErrNotFound semantics
- `pkg/cqrs/runner.go` — Log runner.Run() errors
- `pkg/cqrs/sqlite_readmodel.go` — Removed raw_json column/field
- `pkg/cqrs/item_adapter.go` — Removed dead ToItemView
- `pkg/cqrs/aggregate_id.go` — MustParseAggregateID → ParseAggregateID
- `pkg/cqrs/dispatch_test.go` — MustNew → mustNewCommand/mustNewQuery
- `pkg/sync/sync.go` — Added Store() getter
- `pkg/api/server.go` — NewServer takes (syncer, logger) only
- `pkg/api/server_test.go` — Updated NewServer call
- `pkg/api/integration_test.go` — Updated NewServer call
- `cmd/examples/github-sync/helpers.go` — runAPIServer signature simplified
- `cmd/examples/github-sync/main.go` — Updated runAPIServer call
- `pkg/crdt/doc.go` — Active vs Future types documented
- `AGENTS.md` — Session 10 documentation

### Uncommitted (minor)

- `.golangci.yml` — Regex pattern fix for typecheck exclude
- `pkg/cqrs/stack.go` — Enhanced error wrapping in SyncItems
- `pkg/providers/github/client_retry.go` — `errors.AsType` adoption

---

## Resolution (2026-09-05 docs-health sweep)

All forward-looking items in this report are closed as of 2026-09-05 (verified against the tree at `9625b1b`: go-localsync v0.5.0, 309 core tests / 11 packages, CI green, both cqrs-lint gates clean).

- **Shipped since:** The data/ aspirational packages were deleted (session 13); export + OTel shipped 2026-09-05; TUI/daemon/second-provider remain ROADMAP themes per ADR-0004.
- **Superseded/moot:** anything tied to the Turso backend, committed `vendor/`, go-cqrs-lite v2/v3 WIP, or the pre-de-githubify domain model — all removed or reshaped by ADR-0005/0006/0007 and the go-cqrs-lite v4 migration.
- **Routed:** ideas that still matter live in [TODO_LIST.md](../../TODO_LIST.md) or [ROADMAP.md](../../ROADMAP.md); deliberately deferred work is recorded in the ADRs.
- **Policy:** bucket closure per this directory's [README](README.md); the worst now-false claims are struck inline above.

_Report fully resolved → archived 2026-09-05._
