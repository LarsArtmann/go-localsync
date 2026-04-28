# Comprehensive Analysis & Execution Plan

**Date:** 2026-04-27 | **Session:** Post-Turso-Migration Deep Dive

---

## 1. What I Forgot / Could Have Done Better

### During the Turso Migration

1. **Did not rename `LibSQLStorage` → `TursoStorage`** — The struct, function names, and backend constant still say "LibSQL" but the implementation now uses Turso. This is confusing for future maintainers.
2. **Did not update error messages** — All errors still say "failed to open libsql at..." instead of "turso".
3. **Did not add `isRemoteURL` tests** — The helper has zero test coverage.
4. **Did not fix adjacent lint warnings** — The `exhaustruct` in config.go, `noinlineerr` in libsql.go, and `dupl` between sqlite.go/libsql.go were all known but left unfixed.
5. **Did not add an embedded replica file path option** — Remote URLs use `:memory:`, losing data on restart. Should accept a local cache path for production use.

### Architectural Blind Spots

6. **`ItemID` vs `SourceItemID` split brain** — Both are `string` branded types representing the same semantic concept (the provider's unique item ID). Converting between them (`types.NewSourceItemID(id.Get())`) is pure friction. We should have one type.
7. **`Storage` interface has 17 methods** — This is classic interface bloat. The `nolint:interfacebloat` comment acknowledges the problem but doesn't solve it. Should be composed of `Reader`, `Writer`, `Admin` interfaces.
8. **`ConflictAwareSyncer` embeds `Syncer`** — This is inheritance, not composition. The Go way is to hold a `*Syncer` field and delegate, not embed.
9. **`Provider.Item` is too GitHub-specific** — 11 fields with names like `ActorLogin`, `ActorAvatarURL`, `RepoName`. For a generic sync SDK, different providers have different metadata shapes. A `map[string]any` or generic `Metadata` field would be more flexible.
10. **No streaming / channel-based fetch** — `FetchAll` returns `[]*Item`, loading everything into memory. For large datasets, this is a memory bomb. A channel-based iterator pattern would be better.

---

## 2. Type Model Improvements

### Current Types (Problems)

```
EventID       = id.ID[EventBrand, ulid.ULID]      // Internal DB ID — good
SourceItemID  = id.ID[SourceItemBrand, string]    // Provider's item ID — redundant
ItemID        = id.ID[ItemBrand, string]          // Same thing, different brand
ProviderID    = id.ID[ProviderBrand, string]      // Good
ActorID       = id.ID[ActorBrand, string]         // Too GitHub-specific
RepoID        = id.ID[RepoBrand, string]          # Too GitHub-specific
EventTypeID   = id.ID[EventTypeBrand, string]     # Good
```

**Problem:** `SourceItemID` and `ItemID` are identical in shape and purpose. The only reason both exist is because sqlc's `Events` model uses `SourceItemID` for the `source_id` column while the domain `Item` struct uses `ItemID`. This is a DB schema leak into the type system.

**Solution:** Consolidate to a single `ItemID` type. The DB model can use `ItemID` directly — sqlc supports custom types.

**Problem:** `ActorID` and `RepoID` are GitHub-event-specific. If we add a GitLab provider, GitLab has different concepts (project paths, user namespaces). These fields don't belong in a generic `Item`.

**Solution:** Replace fixed fields with a generic `Metadata` map or typed metadata struct per provider.

### Proposed Type Evolution

```go
// Core identifiers (universal)
type ItemID = id.ID[ItemBrand, string]
type ProviderID = id.ID[ProviderBrand, string]

// Provider-specific metadata (extensible)
type Item struct {
    ID        ItemID
    Source    ProviderID
    Type      string        // "PushEvent", "MergeRequest", etc.
    CreatedAt time.Time
    UpdatedAt time.Time
    RawJSON   json.RawMessage
    Metadata  Metadata      // Provider-specific data
}

// GitHub-specific metadata
type GitHubMetadata struct {
    ActorLogin     string
    ActorAvatarURL string
    RepoName       string
    RepoURL        string
}

// GitLab-specific metadata
type GitLabMetadata struct {
    ProjectPath string
    AuthorName  string
    AuthorEmail string
}
```

This makes the SDK truly provider-agnostic. Each provider defines its own metadata type, and the sync engine doesn't care about the shape.

---

## 3. Existing Code That Fits Requirements

Before implementing anything from scratch, check what we already have:

| Requirement | Existing Code | Reuse Strategy |
|---|---|---|
| In-memory read model | `pkg/storage/memory_storage.go` | Extract interface, use as base for CQRS read model |
| Filter/pagination pattern | `MemoryStorage.filterItems()` | Extract to generic `pkg/filter` package |
| Test server factories | `pkg/testhelpers/helpers.go` | Already reusable, just add more variants |
| Retry logic | `pkg/providers/github/client.go` | Extract to `pkg/retry` — already generic enough |
| Rate limit handling | `pkg/providers/github/client.go` | Extract to `pkg/ratelimit` |
| BDD test patterns | `pkg/storage/storage_bdd_suite_test.go` | Reuse pattern for sync BDD tests |
| Config options pattern | `pkg/storage/config.go` | Same functional options pattern works everywhere |
| Null string helpers | `pkg/storage/sqlite.go` | `toNullString`/`fromNullString` — move to `pkg/dbutil` |
| Item conversion | `pkg/storage/sqlite.go` | `toItem`, `toDBParams`, `convertItems` — move to `pkg/convert` |
| Event conversion | `pkg/providers/github/client.go` | `convertEvent` — stays in github package |

---

## 4. Well-Established Libraries That Make Life Easier

### Already in go.mod (transitive or direct)

| Library | Use Case | Already Have? |
|---|---|---|
| `golang.org/x/sync/errgroup` | Concurrent provider fetches (multi-user) | ✅ Transitive |
| `golang.org/x/sync/singleflight` | Deduplicate concurrent identical fetches | ✅ Transitive |
| `charmbracelet/log` | Structured logging | ✅ Direct |
| `charmbracelet/lipgloss` | TUI styling | ✅ Transitive (from log) |
| `cockroachdb/errors` | Rich error wrapping | ✅ Direct |

### Should Add

| Library | Use Case | Effort |
|---|---|---|
| `github.com/caarlos0/env/v11` | 12-factor config from env vars | Low |
| `github.com/go-chi/chi/v5` | HTTP API routing | Low |
| `github.com/robfig/cron/v3` | Daemon mode scheduling | Low |
| `github.com/knadh/koanf` | Unified config (env + flags + files) | Medium |

### Should NOT Add (overkill)

| Library | Why Not |
|---|---|
| `uber-go/zap` | We already have `charmbracelet/log` which is excellent |
| `spf13/cobra` | Our CLI is simple; `flag` is fine. Only add if we get subcommands |
| `gorm.io/gorm` | We're moving away from SQL, not deeper into it |
| `sqlx` | Same reason — sqlc already generates typed queries |

---

## 5. Comprehensive Multi-Step Execution Plan

Sorted by **Impact / Effort ratio** (highest first).

### Tier S: Critical Fixes (Do First — Minutes Each)

| # | Task | Effort | Impact | Files |
|---|---|---|---|---|
| S1 | Fix `exhaustruct` in `config.go` (add `AuthToken: ""`) | 1m | Fixes lint | `config.go` |
| S2 | Fix `noinlineerr` in `libsql.go` (lines 48, 68) | 3m | Fixes lint | `libsql.go` |
| S3 | Fix `noinlineerr` in `sqlite.go` (lines 78, 151) | 3m | Fixes lint | `sqlite.go` |
| S4 | Add `t.Helper()` to compliance test factories | 2m | Fixes lint | `compliance_test.go` |
| S5 | Fix `errcheck` for `rows.Close()` | 5m | Fixes lint | `libsql.go`, `sqlite.go` |
| S6 | Extract shared `batchGetByIDs` from sqlite.go + libsql.go | 15m | Fixes `dupl`, -60 LOC | New `pkg/storage/helpers.go` |
| S7 | Extract `queryWithFilter` to also cover `GetItemsByType` and `GetItemsBySource` | 10m | Consistency | `sqlite.go` |

### Tier A: High Impact / Low Effort (Do Next — Hours)

| # | Task | Effort | Impact | Rationale |
|---|---|---|---|---|
| A1 | Consolidate `ItemID` + `SourceItemID` into single type | 30m | Removes conversion friction everywhere | Eliminates `NewSourceItemID(id.Get())` pattern |
| A2 | Rename `LibSQLStorage` → `TursoStorage`, `BackendLibSQL` → `BackendTurso` | 20m | Naming accuracy | Already committed to Turso |
| A3 | Add `isRemoteURL` tests | 10m | Test coverage | Zero coverage on remote path |
| A4 | Extract `toNullString`/`fromNullString` to `pkg/dbutil` | 10m | Reusability | Used by any SQL backend |
| A5 | Add `golang.org/x/sync/errgroup` for concurrent syncs | 30m | Performance | Multi-user sync foundation |
| A6 | Add `github.com/caarlos0/env/v11` for config | 20m | 12-factor compliance | Clean env-based config |
| A7 | Extract retry logic to `pkg/retry` package | 30m | Reusability | All providers need retry |
| A8 | Add `sync.RWMutex` benchmark for `MemoryStorage` | 15m | Performance insight | Determine if `xsync.Map` is worth it |

### Tier B: Medium Impact / Medium Effort (Do After)

| # | Task | Effort | Impact | Rationale |
|---|---|---|---|---|
| B1 | Decompose `Storage` interface into `Reader` + `Writer` + `Admin` | 45m | Architecture | 17 methods → 3 small interfaces |
| B2 | Make `ConflictAwareSyncer` use composition, not embedding | 20m | Idiomatic Go | `struct { syncer *Syncer }` instead of embedding |
| B3 | Add streaming fetch (`FetchStream` returning `<-chan *Item`) | 1h | Memory efficiency | Large datasets won't OOM |
| B4 | Add `github.com/go-chi/chi/v5` HTTP API | 2h | Usability | REST API for queries |
| B5 | Add `github.com/robfig/cron/v3` daemon mode | 1h | Automation | Periodic sync without cron job |
| B6 | Add GitHub Actions CI | 2h | Quality gate | Automated test/lint on PRs |
| B7 | Add `Metadata` field to `Item`, migrate GitHub fields | 2h | Provider agnosticism | Foundation for GitLab, etc. |

### Tier C: High Impact / High Effort (Strategic)

| # | Task | Effort | Impact | Rationale |
|---|---|---|---|---|
| C1 | CQRS migration (go-cqrs-lite + Pebble) | 8h+ | Architecture | Eliminates ~2000 LOC of SQL infra |
| C2 | Build TUI with Bubble Tea | 2h | UX | Interactive event browser |
| C3 | Add second provider (GitLab) | 4h | Architecture validation | Proves provider abstraction |
| C4 | Add event retention/TTL | 1h | Operations | Prevent unbounded growth |

---

## 6. What We Should Improve That I Didn't Catch Before

### Code Smells Found During Deep Dive

1. **Magic numbers in `FetchAll`** — `maxPages = 10` default, `PerPage = 100` hardcoded. Should be constants.
2. **`github.NewClient` takes a raw token string** — Should accept an interface (`TokenSource`) for testability.
3. **`Syncer` holds `*log.Logger` directly** — Should accept a `log.Logger` interface for testability (mock logger).
4. **No cancellation during `UpsertBatch`** — The batch loop doesn't check `ctx.Done()` between items.
5. **`BatchGetByIDs` builds a raw SQL query** — This bypasses sqlc entirely. The `dupl` warning is actually telling us this method doesn't belong in SQL backends at all — it should be a generic helper that uses `GetByID` in a loop for small N, or the raw SQL for large N.
6. **Test coverage gaps:**
   - `pkg/errors` — 0 tests (the file exists but coverage is from other packages)
   - `cmd/examples/github-sync` — 0 tests
   - `pkg/provider` — interface only, no tests needed
   - Remote Turso path — 0 tests
   - `isRemoteURL` — 0 tests
7. **The `synced_at` column uses `CURRENT_TIMESTAMP`** — But the migration plan says "updated_at passed from provider (not CURRENT_TIMESTAMP) for proper LWW". The `synced_at` is fine (it's when we synced), but this inconsistency between comments and code is confusing.

---

## 7. Top #1 Question I Cannot Figure Out

**Is the `Item` struct's current shape (11 fields, GitHub-specific) an acceptable temporary state, or should we migrate to a generic `Metadata` approach BEFORE adding any new providers?**

The current `Item` works perfectly for GitHub events. But adding GitLab would require:
- Either adding GitLab-specific fields (bloat)
- Or cramming GitLab data into existing fields (wrong semantics)
- Or changing `Item` to use `Metadata` (breaking change)

If we commit to CQRS migration (which rewrites the entire storage layer anyway), it makes sense to defer the `Metadata` refactor until then. But if we stabilize SQL-first, we should do the `Metadata` refactor now.

This is the same strategic tension as the CQRS vs SQL question, but focused on the data model.

---

## 8. Immediate Action Plan (Next 30 Minutes)

If proceeding without waiting for answers, here's the order:

1. **S1-S5** (lint fixes) — 15 minutes, pure cleanup
2. **S6** (extract `batchGetByIDs`) — 15 minutes, eliminates duplication
3. **A1** (consolidate ItemID/SourceItemID) — 30 minutes, removes conversion friction
4. **A2** (rename LibSQL → Turso) — 20 minutes, naming accuracy
5. **A3** (add `isRemoteURL` tests) — 10 minutes, coverage

Total: ~90 minutes of focused, self-contained improvements.

---

_Generated by Crush (GLM-5.1) — 2026-04-27_
