# Go-LocalSync Agent Configuration

## Project Overview

Go-LocalSync is a generic synchronization SDK that was recently refactored from a GitHub-specific CLI application into a pluggable provider-based architecture.

## Architecture

- **Core SDK**: Located in `pkg/provider/` with generic interfaces
- **Providers**: Implemented in `pkg/providers/` (currently only GitHub)
- **Storage**: Abstraction layer in `pkg/storage/` with SQLite implementation
- **Sync Engine**: Core synchronization logic in `pkg/sync/`

## Development Workflow

1. Code should follow Go best practices and use the configured golangci-lint rules
2. All changes should include appropriate tests
3. Use `go test ./...` to run tests
4. Use `golangci-lint run` to check code quality
5. Build with `go build ./cmd/...`

## Testing

- Unit tests exist for all major components
- Run tests with `go test ./...`
- For coverage, use `go test -cover ./...`

## Provider Development

When adding new providers:

1. Implement the `provider.Provider` interface
2. Add provider-specific tests
3. Update documentation with provider configuration
4. Add example in `cmd/examples/`

## Database Schema

Current schema uses GitHub-specific column names (`github_id`) but should be generalized in future iterations to support multiple providers.

## SQLC Code Generation

The `internal/db/` directory contains sqlc-generated code:

- `events.sql.go` - Query functions and parameter structs (AUTO-GENERATED)
- `models.go` - Data model structs (AUTO-GENERATED)
- `mixins.go` - **Manual** mixin types for code reuse

### ⚠️ Regeneration Warning

The `events.sql.go` file has **manual mixin embedding** applied. After running `sqlc generate`, these changes will be **lost**. To reapply:

1. Edit `internal/db/events.sql.go` and replace the struct definitions with embedded mixins
2. Update callers in `pkg/storage/sqlite.go` to use `PaginationMixin: db.PaginationMixin{...}` syntax

### Mixin Types

| Mixin             | Purpose                    | Used By                                                                                       |
| ----------------- | -------------------------- | --------------------------------------------------------------------------------------------- |
| `PaginationMixin` | Shared Limit/Offset fields | `GetEventsParams`, `GetEventsByActorParams`, `GetEventsByRepoParams`, `GetEventsByTypeParams` |
| `EventCoreMixin`  | Shared event fields        | (Available for future use with `UpsertEventParams` ↔ `Events`)                                |
