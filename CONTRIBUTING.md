# Contributing

Thanks for your interest in contributing to go-localsync!

## Quick Start

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.26+
- golangci-lint v2

### Commands

```bash
go build ./...                          # Build all packages
go test -race -count=1 ./...            # Run all tests with race detector
go vet ./...                            # Static analysis
golangci-lint run ./... --timeout=5m    # Lint (125+ linters, strict config)
```

### Nix Flake (Alternative)

If you use Nix, the project provides a flake with a complete devShell:

```bash
nix develop                    # Enter dev shell (Go + golangci-lint)
nix run .#test                 # Run tests
nix run .#lint                 # Run linter
nix build                      # Build the package
```

## Architecture Overview

```
pkg/api/                      HTTP API (Huma v2, split into server.go + dto.go + handlers.go)
pkg/cqrs/                     CQRS stack (go-cqrs-lite v3, split into focused files)
pkg/crdt/                     CRDT primitives (VectorClock, ConflictResolver, LWWResolver)
pkg/data/model/               Read-side domain types (Item, Key, ItemFilter)
pkg/data/schema/              Schema versioning
pkg/errors/                   Structured error taxonomy
pkg/id/                       Branded phantom-type IDs
pkg/provider/                 Provider interface + Item DTO (contract only — no provider impls shipped)
pkg/sync/                     Sync engine (Syncer, ConflictAwareSyncer, SyncStore interface)
pkg/testutil/                 Shared test helpers
```

> The SDK is a **pure contract library** — it ships no provider implementation or CLI. The reference GitHub provider + CLI lives in the consumer app [`github-local-sync`](https://github.com/larsartmann/github-local-sync).

### Key Design Decisions

- **Event-sourced CQRS**: All state changes are events. No legacy CRUD. See `docs/adr/0001-cqrs-adoption.md`.
- **Branded IDs**: Compile-time type safety via phantom types. See `docs/adr/0002-branded-ids.md`.
- **Pluggable conflict resolution**: `crdt.ConflictResolver[T]` injected into CQRS stack. See `docs/adr/0003-crdt-integration.md`.
- **One-way dependencies**: `cqrs → sync → provider/types/errors`. No circular imports.
- **File splitting convention**: Types in one file (`types.go`, `dto.go`), implementation in another (`sync.go`, `handlers.go`).

## Code Conventions

### File Size

Keep files under **250 lines**. If a file grows beyond this, split by responsibility:

- Types/constants → `types.go`
- Implementation → main file
- Helpers → separate file

### Testing

- All tests use the standard `testing` package
- Table-driven tests for error paths and edge cases
- Use `pkg/testutil/` for shared helpers (`MustNoError`, `AssertEqual`, `AssertStatus`, etc.)
- Run with `-race` flag always
- Target >85% coverage for new code

### Naming

- Package names: lowercase, single word, no underscores
- Types: PascalCase, descriptive (`SyncItemState`, `ItemSyncedPayload`)
- Functions: verb+noun (`DecideSync`, `BuildFilterQuery`)
- No `Manager`, `Handler`, `Helper` suffixes

### Error Handling

- Use `pkg/errors/` sentinels (`ErrNotFound`, `ErrDatabase`, `ErrRateLimited`)
- Wrap with context: `pkgerrors.Wrap(pkgerrors.ErrDatabase, fmt.Sprintf("upsert item: %v", err))`
- Never expose internal errors to API consumers

## Pull Request Checklist

- [ ] `go build ./...` passes
- [ ] `go test -race -count=1 ./...` passes
- [ ] `go vet ./...` passes
- [ ] No lint issues (`golangci-lint run ./...`)
- [ ] New code has tests
- [ ] AGENTS.md updated if architecture changed

## Reporting Issues

Please use GitHub Issues to report bugs or request features.
