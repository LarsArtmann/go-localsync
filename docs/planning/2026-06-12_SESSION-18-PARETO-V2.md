# Session 18 — Self-Review + Pareto Improvement Sprint v2

**Date:** 2026-06-12
**Branch:** master

---

## Self-Review: What Was Forgotten / Could Be Better

### Critical Mistakes from Prior Attempt

1. **Left build broken between sessions** — The #1 rule is "never leave the build red." `FetchOptions.Source` was branded but the 4 downstream sites weren't updated. A simple `go build ./...` before committing would have caught this.

2. **Plan was too ambitious for type-branding** — The old plan tried to brand every `string` as a branded ID in one shot. The correct approach: brand at the **boundary** (provider interface, sync options) first, then propagate inward. Event payloads should stay `string` — they're serialization DTOs, not domain types.

3. **Event payloads as branded types is WRONG** — `ItemSyncedPayload` fields MUST stay `string` because they're JSON serialization DTOs. `json.Marshal`/`json.Unmarshal` doesn't know about branded types. The adapter layer (`item_adapter.go`) already handles encode/decode correctly. Branding event payloads would add complexity with zero safety benefit — the brand is lost at the JSON boundary anyway.

4. **`Provider.FetchAll(source string)` signature** — This takes `string` because it's a convenience method where callers pass raw usernames. The internal `Fetch()` already takes branded `FetchOptions`. This is fine — the boundary is at `FetchAll`, which wraps to branded internally.

5. **`SyncOptions.Source` as `string`** — Similar to `FetchAll`. The sync layer receives raw config (env vars, CLI flags) and converts to branded internally. The validation `if o.Source == ""` is idiomatic for raw strings. Branded here would force every caller to wrap.

6. **`Provider.Name()` returning `string`** — This is fine. It's a static identifier, not a flow variable. Branded here adds ceremony with no safety gain.

7. **Duplicate `provider.Item` / `model.Item`** — This is NOT duplication. `provider.Item` is the write-side DTO (has `RawJSON`). `model.Item` is the canonical domain entity (has `SchemaVersion`). They serve different roles. The `item_adapter.go` converts between them at the CQRS boundary. This is correct architecture.

### What Was Actually Good

- `errors.Join` in `model.Item.Validate()` — collects all errors at once
- `RegisterTyped` adoption — eliminates boilerplate
- `SyncSummary.String()` / `SyncAction.String()` — structured logging
- Branded `FetchOptions.Source` — correct boundary, just incomplete execution
- `stack_adapters.go` doc comment — prevents future confusion

### What Could Still Improve

| # | Issue                                                                                          | Why It Matters                                                   |
| - | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| 1 | `provider.Item.Validate()` uses individual returns, `model.Item.Validate()` uses `errors.Join` | Inconsistent DX — `provider.Item` should also collect all errors |
| 2 | `Stats.ItemTypes` is `[]string` not `[]id.EventTypeID`                                         | Loses type info at read boundary                                 |
| 3 | `GetTypes()` returns `[]string`                                                                | Same issue — forces re-wrapping                                  |
| 4 | API DTO layer is all raw strings                                                               | Acceptable for HTTP boundaries but validation could be typed     |
| 5 | No `Backend` type enum for `CQRSConfig.Backend`                                                | Magic string "memory"/"sqlite"                                   |
| 6 | Linter warnings in `item.go` (wsl_v5, nlreturn)                                                | Cosmetic but fixable                                             |

---

## Revised Pareto Plan

### Phase 0: Fix Build (BLOCKER — must do first)

| #   | Task                                              | Effort | Files                            |
| --- | ------------------------------------------------- | ------ | -------------------------------- |
| 0.1 | Fix `client.go:124,136` — `opts.Source.Get()`     | XS     | `pkg/providers/github/client.go` |
| 0.2 | Fix test files — `id.NewProviderID(...)` wrapping | XS     | 4 test files                     |
| 0.3 | Build + test → green                              | XS     | —                                |

### Phase 1: 1% / 51% — Consistency Fixes

| #   | Task                                                             | Effort | Files                      |
| --- | ---------------------------------------------------------------- | ------ | -------------------------- |
| 1.1 | `provider.Item.Validate()` → use `errors.Join` like `model.Item` | S      | `pkg/provider/provider.go` |
| 1.2 | Add `Backend` type enum for `CQRSConfig.Backend`                 | S      | `pkg/cqrs/stack.go`        |
| 1.3 | Fix linter warnings in `model/item.go`                           | XS     | `pkg/data/model/item.go`   |
| 1.4 | Commit + verify                                                  | XS     | —                          |

### Phase 2: 4% / 64% — Type Model Deepening

| #   | Task                                                                              | Effort | Files                                                               |
| --- | --------------------------------------------------------------------------------- | ------ | ------------------------------------------------------------------- |
| 2.1 | `GetTypes()` → return `[]id.EventTypeID`                                          | S      | `pkg/data/model/item.go`, `pkg/cqrs/stack_adapters.go`, read models |
| 2.2 | `Stats.ItemTypes` → `[]id.EventTypeID`, `TypeCounts` → `map[id.EventTypeID]int64` | S      | `pkg/sync/types.go`, callers                                        |
| 2.3 | `SyncOptions.Source` validation uses branded check                                | XS     | `pkg/sync/sync.go`                                                  |
| 2.4 | Commit + verify                                                                   | XS     | —                                                                   |

### Phase 3: 20% / 80% — Product Features (from prior plan, still valid)

| #   | Task                                            | Effort | Files                              |
| --- | ----------------------------------------------- | ------ | ---------------------------------- |
| 3.1 | API auth middleware (API-key)                   | M      | `pkg/api/middleware.go` (new)      |
| 3.2 | API pagination headers                          | M      | `pkg/api/handlers.go`              |
| 3.3 | CLI `--conflict-strategy` flag                  | S      | `cmd/examples/github-sync/main.go` |
| 3.4 | CLI `--watch` flag (ticker + graceful shutdown) | M      | `cmd/examples/github-sync/main.go` |

---

## Execution Order

```
Phase 0 (BLOCKER):
  0.1 → 0.2 → 0.3 → [COMMIT]

Phase 1 (1% → 51%):
  1.1 → 1.2 → 1.3 → [COMMIT + PUSH]

Phase 2 (4% → 64%):
  2.1 → 2.2 → 2.3 → [COMMIT + PUSH]

Phase 3 (20% → 80%):
  3.1 → 3.2 → 3.3 → 3.4 → [COMMIT + PUSH]
```
