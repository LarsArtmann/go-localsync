# Go-LocalSync Agent Configuration

## Project Overview

Go-LocalSync is a generic synchronization SDK with a pluggable provider-based architecture. It supports conflict-aware sync via go-localfirst CRDT primitives and uses branded IDs from go-branded-id for compile-time type safety.

## Architecture

| Package                     | Purpose                                                                                                |
| --------------------------- | ------------------------------------------------------------------------------------------------------ |
| `pkg/provider/`             | Core interfaces (`Provider`, `Item`, `FetchResult`, `RateLimitConfig`, `RetryConfig`)                  |
| `pkg/providers/github/`     | GitHub provider implementation (only provider currently)                                               |
| `pkg/storage/`              | Storage abstraction (pluggable: SQLite, Turso, in-memory)                                              |
| `pkg/sync/`                 | `Syncer` (basic), `ConflictAwareSyncer` (CRDT-aware via go-localfirst)                                 |
| `pkg/types/`                | Branded phantom-type IDs (`ItemID` ULID, `ExternalID` string, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`) |
| `pkg/errors/`               | Sentinel errors using cockroachdb/errors (`ErrNotFound`, `ErrStorage`, `ErrRateLimited`, etc.)         |
| `pkg/testhelpers/`          | Shared test mocks and factories                                                                        |
| `internal/database/`        | Connection management (`Open()`) + migration system (`RunMigrations()`)                                |
| `internal/db/`              | sqlc-generated query code from `sql/queries/events.sql`                                                |
| `sql/queries/`              | SQL query definitions for sqlc                                                                         |
| `sql/migrations/`           | Reference copies of migration SQL (embedded as Go constants)                                           |
| `cmd/examples/github-sync/` | Example CLI entry point                                                                                |

## Development Workflow

### Local Development

1. Create `go.work` in project root (already in `.gitignore`):
   ```
   go 1.26.1
   use .
   use (
       ../go-localfirst
       ../go-branded-id
   )
   ```
2. Build: `go build ./...`
3. Test: `go test ./... -count=1`
4. Lint: `golangci-lint run ./... --timeout=5m`
5. Format: `golangci-lint fmt ./...`
6. CI gate: `just verify`

### CI (No go.work)

CI uses pseudo-versions from GitHub (no replace directives in `go.mod`):

```bash
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go build ./...
GONOSUMCHECK=github.com/larsartmann/* GONOSUMDB=github.com/larsartmann/* go test ./... -count=1
```

### Known Blockers

- **golangci-lint v1/v2 mismatch**: Config is v2 format, installed binary is v1.64.8. Fix: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`
- **Go toolchain**: `go.mod` says 1.26.1, installed is 1.26.0. Blocks `go test -cover`. Regular build/test works.
- **Pre-commit hooks**: Use `--no-verify`. Hooks ban testify; entire test suite uses it.

## Testing

| Package                    | Tests | Status                                                      |
| -------------------------- | ----- | ----------------------------------------------------------- |
| `internal/database`        | 6     | ✅ Migration tests (idempotency, ordering, schema, indexes) |
| `pkg/providers/github`     | 21    | ✅ Client, fetch, retry, error handling                     |
| `pkg/storage`              | 70+   | ✅ SQLite + Memory + Turso compliance (22+ tests each)      |
| `pkg/sync`                 | 11    | ✅ Syncer + ConflictAwareSyncer                             |
| `cmd/examples/github-sync` | 0     | ⬜ No tests                                                 |
| `pkg/errors`               | 0     | ⬜ No tests                                                 |
| `pkg/provider`             | 0     | ⬜ Interface only                                           |
| `pkg/types`                | 13    | ⚠️ ID construction, roundtrip, zero, equal — no edge case tests               |
| `pkg/testhelpers`          | 0     | ⬜ Helper package                                           |

Run: `go test ./... -count=1`

## Pluggable Storage Architecture

Storage backends are selected via `storage.NewStorage(Config)`. Switch-based factory, no global registries. External backends implement the `Storage` interface directly.

| Backend  | Flag/Config                  | Use Case                                  |
| -------- | ---------------------------- | ----------------------------------------- |
| `sqlite` | `--backend sqlite` (default) | Persistent production storage             |
| `turso`  | `--backend turso`            | Local Turso file or remote Turso database |
| `memory` | `--backend memory`           | Testing, development                      |

### Adding a New Backend

1. Implement the `storage.Storage` interface (16 methods)
2. Create constructor that embeds `sqlStorage` (for SQL backends) or implement directly
3. Add case to `NewStorage` switch in `config.go`
3. Run compliance tests: `go test ./pkg/storage/ -run TestStorageCompliance`
4. SQLite, Turso, and Memory must all pass the same compliance suite

### CLI Usage

```bash
go run ./cmd/examples/github-sync --backend memory
```

## Provider Development

When adding new providers:

1. Implement the `provider.Provider` interface (`Name`, `Fetch`, `FetchAll`, `GetRateLimit`)
2. Convert provider-specific data to `provider.Item` using branded types from `pkg/types/`
3. Add provider-specific tests
4. Update documentation with provider configuration
5. Add example in `cmd/examples/`

## Database Schema

- Events table with `source_id` as unique key for upsert conflict detection
- `source` column for multi-provider tracking (default: `'github'`)
- `id` column is ULID PK (`ItemID`), `source_id` stores external provider ID (`ExternalID`)
- `updated_at` passed from provider (not CURRENT_TIMESTAMP) for proper LWW conflict resolution
- 7 indexes: `created_at`, `type`, `source_id`, `actor_login`, `repo_name`, `source`, `(source, source_id)`

## SQLC Code Generation

The `internal/db/` directory contains sqlc-generated code:

- `events.sql.go` — Query functions and parameter structs (AUTO-GENERATED)
- `models.go` — Data model structs (AUTO-GENERATED)
- `db.go` — DB connection and query helpers (AUTO-GENERATED)
- `querier.go` — Querier interface (AUTO-GENERATED)

After running `sqlc generate`, all files in `internal/db/` are overwritten.

## Migration System

- `internal/database/migration.go` — Migration runner with `RunMigrations()`, uses `sync.Once` for lazy loading
- Migrations tracked in `schema_migrations` table (version, name, applied_at)
- SQL reference files in `sql/migrations/` (embedded as Go constants)
- Each migration runs in a transaction; `CREATE IF NOT EXISTS` for idempotency
- Current migrations: 001 (initial schema + indexes), 002 (source indexes), 003 (rename github_id), 004 (ULID PK)

## Dependencies

| Dependency                    | Purpose                                                                               |
| ----------------------------- | ------------------------------------------------------------------------------------- |
| `go-cqrs-lite`               | CQRS library with event sourcing, branded IDs, catalog — **NOT yet imported** (planned) |
| `go-localfirst`               | CRDT primitives (`VectorClock`, `LWWResolver[T]`) for conflict-aware sync             |
| `go-branded-id`               | Branded phantom-type IDs for compile-time safety                                      |
| `modernc.org/sqlite`          | Pure Go SQLite driver (no CGO)                                                        |
| `cockroachdb/errors`          | Sentinel errors with detail wrapping                                                  |
| `go-github/v69`               | GitHub API client                                                                     |
| `turso.tech/database/tursogo` | Turso Go client — embedded local + remote sync (replaces deprecated libsql-client-go) |
| `charmbracelet/log`           | Structured logging                                                                    |

## go-cqrs-lite Integration Status

go-localsync **does not import** go-cqrs-lite despite sharing go-branded-id and having a detailed CQRS migration plan (`CQRS_MIGRATION_PLAN.md`).

| Area | go-localsync (current) | go-cqrs-lite (available) |
|------|------------------------|-------------------------|
| IDs | `id.ID[T, ULID]` (ItemID) + `id.ID[T, string]` (ExternalID) via go-branded-id | `id.Of[T]` (ULID-only) via go-branded-id |
| Storage | 16-method `Storage` interface, 3 SQL backends | `event.Store` (4 methods) + projections |
| Conflict | Inline LWW in `ConflictAwareSyncer` | `LWWResolver[T]` in go-localfirst |
| Retry | Hand-rolled in `github/client.go` | `middleware.CommandRetry` with jitter |

**Option A completed (2026-05-01)**: `ItemID` migrated from `id.ID[ItemBrand, string]` to `id.ID[ItemBrand, ulid.ULID]`. External provider IDs now stored as `ExternalID` (string-backed `id.ID[ExternalBrand, string]`). This aligns with go-cqrs-lite's `id.Of[T]` which uses ULID-only. Both systems share the same value type (ULID) — conversion is trivial (`.Get()` the ULID, wrap in the other brand). The ID type incompatibility blocker is **resolved**.

**Deduplication done**: `sqlite.go` (27 lines) and `turso.go` (77 lines) now embed shared `sqlStorage` (356 lines). ~247 lines of duplication eliminated.

See `docs/planning/2026-04-30_23-08-CQRS_LITE_INTEGRATION.md` for full audit and execution plan.

## Lint Status

golangci-lint v2.11.4 reports **0 issues**. Config is strict with 125+ linters enabled.
