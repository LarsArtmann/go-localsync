# Go-LocalSync Agent Configuration

## Project Overview

Go-LocalSync is a generic synchronization SDK with a pluggable provider-based architecture. It supports conflict-aware sync via go-localfirst CRDT primitives.

## Architecture

- **Core SDK**: `pkg/provider/` — Generic interfaces (`Provider`, `Item`, `FetchResult`)
- **Providers**: `pkg/providers/github/` — GitHub implementation (only provider currently)
- **Storage**: `pkg/storage/` — Storage abstraction with SQLite backend
- **Sync Engine**: `pkg/sync/` — `Syncer` (basic), `ConflictAwareSyncer` (CRDT-aware via go-localfirst)
- **Types**: `pkg/types/` — Branded IDs from go-composable-business-types
- **Errors**: `pkg/errors/` — Sentinel errors using cockroachdb/errors
- **Test Helpers**: `pkg/testhelpers/` — Shared mocks and factories
- **Database**: `internal/database/` — Connection management + migration system
- **SQLC**: `internal/db/` — Auto-generated query code from `sql/queries/events.sql`
- **CLI**: `cmd/examples/github-sync/` — Example CLI entry point

## Development Workflow

1. Use `go.work` for local development (already in `.gitignore`)
2. CI uses pseudo-versions from GitHub (no replace directives in `go.mod`)
3. Build: `go build ./...`
4. Test: `go test ./... -count=1`
5. Lint: `golangci-lint run ./... --timeout=5m` (requires v2 binary)
6. Format: `golangci-lint fmt ./...`

## Testing

- Unit tests for `pkg/providers/github`, `pkg/storage`, `pkg/sync`
- Run: `go test ./... -count=1`
- Coverage: `go test -cover ./...`
- Missing: `internal/database` migration tests, CLI integration tests

## Provider Development

When adding new providers:

1. Implement the `provider.Provider` interface
2. Add provider-specific tests
3. Update documentation with provider configuration
4. Add example in `cmd/examples/`

## Database Schema

- Events table with `github_id` as unique key (to be generalized to `source_id` in future)
- `source` column for multi-provider tracking
- `updated_at` passed from provider (not CURRENT_TIMESTAMP) for proper LWW

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
