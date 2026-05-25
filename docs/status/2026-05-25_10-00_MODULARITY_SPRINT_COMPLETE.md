# Go-LocalSync — Full Status Report

**Date:** 2026-05-25 10:00  
**Branch:** master (up to date with origin/master)  
**Working tree:** Clean  
**Last commit:** `f14ef32` — docs: update AGENTS.md with SyncStore interface and ItemFilter location

---

## A. Fully Done ✅

### Architecture

- **SyncStore interface extracted** (`pkg/sync/`) — decouples sync logic from concrete `*cqrs.CQRSStack`. Dependency flows one way: `cqrs → sync → provider/types/errors`.
- **ItemFilter deduplicated** into `pkg/provider/item_filter.go` — was identical in both `pkg/sync/` and `pkg/cqrs/readmodel.go`. Eliminated the `toItemFilter()` adapter bridge.
- **SyncStore interface cleaned** — removed unused `SyncItem()` and `Count()` methods (5 methods remain: `SyncItems`, `ListItems`, `CountItems`, `GetItemTypes`, `Close`).
- **`pkg/testhelpers/` deleted** — all helpers moved into `pkg/providers/github/testhelpers_test.go` as unexported test helpers.
- **`event.IsRetryable` moved** to `pkg/errors.IsRetryable` (delegates to `errorfamily.IsRetryable`).
- **`pkg/localsync/` sub-module** — CRDT/sync primitives (VectorClock, Operation[T], ConflictResolver[T], LWWResolver[T]) with own go.mod, zero coupling.

### go-cqrs-lite API Drift Fixed

| Old API | New API |
|---------|---------|
| `command.Core` | `command.BasicCommand` |
| `query.Core` | `query.BasicQuery` |
| `*event.Core` | `*event.ImmutableEvent` |
| `NewCheckpointStore()` | `NewMemoryCheckpointStore()` |
| Custom logger interface | `middleware.EventLogging(*slog.Logger)` |
| `charmLogAdapter` | `newSlogLogger()` using `slog.New(log.Default())` |

### Build & Test

| Metric | Value |
|--------|-------|
| Total test functions | 203 (198 in `pkg/`, 5 in `cmd/`) |
| Overall coverage | 73.8% |
| `go build ./...` | ✅ Clean |
| `go vet ./...` | ✅ Clean |
| `go test ./... -count=1` | ✅ All pass (0 failures) |
| `golangci-lint` | ✅ 0 issues |

### Per-Package Test Coverage

| Package | Tests | Coverage | Status |
|---------|-------|----------|--------|
| `pkg/cqrs` | 80 | 80.7% | ✅ |
| `pkg/errors` | 10 | 94.4% | ✅ |
| `pkg/localsync` | 52 | N/A | ✅ |
| `pkg/provider` | 1 | 100.0% | ✅ |
| `pkg/providers/github` | 32 | 84.6% | ✅ |
| `pkg/sync` | 14 | 77.8% | ✅ |
| `pkg/types` | 10 | 87.5% | ✅ |
| `cmd/examples/github-sync` | 5 | 10.5% | ⬜ Example only |

### Code Size

| Category | Lines |
|----------|-------|
| Production Go | 3,728 |
| Test Go | 5,428 |
| Total | 9,156 |

---

## B. Partially Done 🔶

### Documentation Drift

FEATURES.md and CHANGELOG.md have stale references from pre-refactor sessions:

| File | Issue |
|------|-------|
| `FEATURES.md:73` | Item Filtering package listed as `pkg/cqrs` → should be `pkg/provider` |
| `FEATURES.md:90` | References deleted `charmLogAdapter` |
| `FEATURES.md:128` | Says "197 test functions" — actually 203 now |
| `FEATURES.md:129` | Lists deleted `pkg/testhelpers` as FULLY_FUNCTIONAL |
| `AGENTS.md:227-228` | References `charmLogAdapter` and `event.IsRetryable` |
| `AGENTS.md:38` | References `sync.ItemFilter → cqrs.ItemFilter` conversion (no longer exists) |
| `CHANGELOG.md:19` | References deleted `pkg/testhelpers/` |

### Test Coverage in `pkg/errors`

- `errors_test.go` tests `event.IsRetryable` and `event.Classify` directly instead of the local `errors.IsRetryable` / `errorfamily.IsRetryable`. The local wrapper exists but isn't being tested through the right API.

---

## C. Not Started ⬜

1. **No second provider** — only GitHub exists. Provider architecture is ready for new implementations (GitLab, Jira, etc.) but none exist.
2. **No flake.nix** — build automation still relies on `go build`/`go test` directly. `go-structure-linter` flags this as HIGH.
3. **No `internal/` directory** — all packages are public. `go-structure-linter` flags this as MEDIUM.
4. **`coverage.out` in root** — should be in `/coverage/` directory.
5. **Replace directive in `go.mod`** — `go-cqrs-lite/storage` has a local replace. Works with `go.work` but `go-structure-linter` flags as supply chain risk.
6. **`main.go` stats section** calls `stack.Count()` and `stack.GetTypes()` directly instead of through `baseSyncer.GetStats()`. Works but bypasses the architectural boundary.
7. **No CLI subcommands** — single binary with flags. No `cobra`/`ff` structure.
8. **No OpenTelemetry/metrics** — library-policy lint flags `prometheus/client_golang` should be replaced with OTEL.
9. **Turso legacy client** — library-policy lint flags `turso.tech/database/tursogo` deprecation path.
10. **No schema migration story** — single schema version (v1). `UpcasterRegistry` in go-cqrs-lite exists but unused.

---

## D. Totally Fucked Up 💀

Nothing is truly broken. The codebase compiles, all 203 tests pass, lint is clean.

**Closest to fucked:**

1. **Intermittent test failure in `pkg/cqrs`** — `outbox publish cycle failed: context canceled` race condition in outbox shutdown. Doesn't cause test failures consistently but produces noisy logs. The outbox poller's graceful shutdown races with test cleanup.
2. **LSP perpetually reports `go.work` version mismatch** — `go.work` says `1.26.3` but gopls/golangci-lint cache thinks `1.26.2`. Requires `go work use` or LSP restart to clear. Annoying but not blocking.

---

## E. What We Should Improve

### Architecture

- **Formalize conflict resolution** — `sync.LWWResolver[T]` + `sync.VectorClock` from `pkg/localsync/` are imported but unused. The sync layer uses `DecideSync.HasChanged()` which is effectively LWW. Should adopt the formal CRDT types or remove them.
- **Provider config unification** — `RateLimitConfig` and `RetryConfig` live in `pkg/provider/` but are only used by GitHub. Should they be per-provider or generic?
- **`cmd/examples/` is the only entry point** — no library-level "start here" guide or quickstart.

### Code Quality

- **File sizes** — `pkg/providers/github/client_test.go` at 655 lines (87% over 350-line limit). `pkg/sync/sync_test.go` at 460 lines. Should be split.
- **Test naming** — mix of `TestX_Y` and `TestX_Y_Z` patterns. Could standardize.
- **`cmd/examples/github-sync` coverage** at 10.5% — mostly untested CLI wiring.

### Dependency Hygiene

- **`pkg/errors` imports `go-cqrs-lite/core/event` in tests** — should only test through the local `errorfamily` API.
- **`replace` directive** in `go.mod` for `go-cqrs-lite/storage` — only needed for CI without `go.work`. Should document this clearly.

---

## F. Top 25 Things We Should Get Done Next

### High Impact — Architecture

| # | Task | Effort | Impact |
|---|------|--------|--------|
| 1 | Fix stale docs: FEATURES.md, AGENTS.md, CHANGELOG.md references | S | M |
| 2 | Fix `errors_test.go` to test local `errors.IsRetryable` not `event.IsRetryable` | S | S |
| 3 | Fix intermittent outbox shutdown race (noisy test logs) | M | M |
| 4 | Adopt or remove `sync.LWWResolver`/`sync.VectorClock` from `pkg/localsync/` | M | L |
| 5 | Wire `main.go` stats through `baseSyncer.GetStats()` instead of direct stack calls | S | M |
| 6 | Add a second provider (GitLab or Jira) to validate the provider abstraction | L | L |
| 7 | Extract `pkg/cqrs` into `internal/cqrs` — non-public implementation detail | M | M |
| 8 | Move `coverage.out` to `coverage/` directory | S | S |
| 9 | Create `flake.nix` for reproducible builds | M | M |
| 10 | Add `internal/` package boundary for implementation packages | M | M |

### High Impact — Code Quality

| # | Task | Effort | Impact |
|---|------|--------|--------|
| 11 | Split `client_test.go` (655 lines) into focused test files by concern | S | M |
| 12 | Split `sync_test.go` (460 lines) — separate unit from integration | S | M |
| 13 | Increase `pkg/sync` coverage from 77.8% to 85%+ | S | M |
| 14 | Increase `cmd/examples/github-sync` coverage from 10.5% to 50%+ | M | M |
| 15 | Add integration test for full sync cycle (fetch → sync → read model → stats) | M | L |
| 16 | Standardize test naming conventions across packages | S | S |

### Medium Impact — Dependencies & Ops

| # | Task | Effort | Impact |
|---|------|--------|--------|
| 17 | Replace `prometheus/client_golang` with OpenTelemetry if metrics are needed | M | M |
| 18 | Evaluate Turso client migration (legacy → unified) | M | M |
| 19 | Document the `replace` directive strategy (CI vs local) | S | S |
| 20 | Add `go.work.sum` sync to prevent LSP cache staleness | S | S |

### Lower Impact — Nice to Have

| # | Task | Effort | Impact |
|---|------|--------|--------|
| 21 | Add CLI subcommand structure (cobra/ff) for better UX | M | M |
| 22 | Wire `UpcasterRegistry` for schema evolution readiness | M | S |
| 23 | Wire `middleware.CommandRetry` for automatic provider retry | S | S |
| 24 | Add example in `cmd/examples/` for non-GitHub provider | M | M |
| 25 | Add README quickstart / "how to add a provider" guide | S | M |

---

## G. Top #1 Question I Cannot Figure Out Myself

**Should `pkg/localsync/` (VectorClock, Operation[T], ConflictResolver[T], LWWResolver[T]) be adopted as the formal conflict resolution layer, or is it dead code that should be removed?**

The types exist as an independent sub-module with 52 tests and solid coverage. But the actual sync flow uses `DecideSync.HasChanged()` for conflict detection — a simpler, less formal approach. Using the CRDT types would make the conflict resolution pluggable and formally verifiable, but requires wiring `LWWResolver[T]` into the decider's `Fold`/`Decide` functions. That's a non-trivial refactor that changes the core state machine. I can't tell if this is a planned evolution or an extraction that's no longer needed.

---

## Session 5 Commits (2026-05-25)

| Commit | Description |
|--------|-------------|
| `b394a1e` | Extract SyncStore interface, decouple sync from cqrs, fix API drift |
| `9d67fbe` | Bump `pkg/localsync/go.mod` go version 1.26.2→1.26.3 |
| `1b2f31c` | Deduplicate ItemFilter into `pkg/provider/` |
| `153c34f` | Clean SyncStore interface — remove unused SyncItem and Count |
| `f14ef32` | Update AGENTS.md with SyncStore interface and ItemFilter location |
