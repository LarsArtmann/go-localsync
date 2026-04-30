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
| `pkg/types/`                | Branded phantom-type IDs (`ItemID`, `ProviderID`, `EventTypeID`, `ActorID`, `RepoID`, `GithubEventID`) |
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
| `pkg/types`                | 0     | ⬜ No tests                                                 |
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

1. Implement the `storage.Storage` interface (17 methods)
2. Add case to `NewStorage` switch in `config.go`
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

- Events table with `github_id` as unique key (to be generalized to `source_id` in future migration)
- `source` column for multi-provider tracking (default: `'github'`)
- `updated_at` passed from provider (not CURRENT_TIMESTAMP) for proper LWW conflict resolution
- 7 indexes: `created_at`, `type`, `github_id`, `actor_login`, `repo_name`, `source`, `(source, github_id)`

## SQLC Code Generation

The `internal/db/` directory contains sqlc-generated code:

- `events.sql.go` — Query functions and parameter structs (AUTO-GENERATED)
- `models.go` — Data model structs (AUTO-GENERATED)
- `db.go` — DB connection and query helpers (AUTO-GENERATED)
- `querier.go` — Querier interface (AUTO-GENERATED)

After running `sqlc generate`, all files in `internal/db/` are overwritten.

## Migration System

- `internal/database/migration.go` — Migration runner with `RunMigrations()`
- Migrations tracked in `schema_migrations` table (version, name, applied_at)
- SQL reference files in `sql/migrations/` (embedded as constants in Go)
- Each migration runs in a transaction; `CREATE IF NOT EXISTS` for idempotency
- Current migrations: 001 (initial schema + indexes), 002 (source indexes)

## Dependencies

| Dependency                    | Purpose                                                                               |
| ----------------------------- | ------------------------------------------------------------------------------------- |
| `go-localfirst`               | CRDT primitives (`VectorClock`, `LWWResolver[T]`) for conflict-aware sync             |
| `go-branded-id`               | Branded phantom-type IDs for compile-time safety                                      |
| `modernc.org/sqlite`          | Pure Go SQLite driver (no CGO)                                                        |
| `cockroachdb/errors`          | Sentinel errors with detail wrapping                                                  |
| `go-github/v69`               | GitHub API client                                                                     |
| `turso.tech/database/tursogo` | Turso Go client — embedded local + remote sync (replaces deprecated libsql-client-go) |
| `charmbracelet/log`           | Structured logging                                                                    |
