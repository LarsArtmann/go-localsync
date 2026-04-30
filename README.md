# go-localsync

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**A pluggable SDK for syncing data from any provider to local SQLite storage.** Build local-first applications that work offline with full data fidelity and incremental sync.

_go-localsync is an SDK, not a CLI application._ Use it as a library to add data synchronization to your Go applications.

## Overview

| Component               | Description                                                                                      |
| ----------------------- | ------------------------------------------------------------------------------------------------ |
| **Provider Interface**  | Implement `provider.Provider` to sync from any data source (GitHub, GitLab, Jira, etc.)          |
| **Storage Abstraction** | SQLite storage with full JSON fidelity — store the complete original payload                     |
| **Sync Engine**         | Full and incremental sync with pagination, configurable rate limiting and retry                  |
| **Conflict-Aware Sync** | CRDT-backed conflict detection via [go-localfirst](https://github.com/larsartmann/go-localfirst) |
| **Branded IDs**         | Type-safe IDs from [go-branded-id](https://github.com/larsartmann/go-branded-id)                 |
| **Schema Migrations**   | Version-tracked database migrations with automatic application on startup                        |

## Who is this for?

This SDK is for Go developers building applications that need:

- **Offline-first** functionality with local data access
- **Offline dashboards** that aggregate data from multiple services
- **Custom sync logic** tailored to specific use cases
- **Data portability** with SQLite as a simple, embedded database
- **Conflict detection** for multi-device or multi-source synchronization

## Installation

```bash
go get github.com/larsartmann/go-localsync
```

> **Note:** This module depends on two private repositories (`go-localfirst` and `go-branded-id`). Set environment variables for private module access:
>
> ```bash
> export GONOSUMCHECK=github.com/larsartmann/*
> export GONOSUMDB=github.com/larsartmann/*
> ```
>
> For local development, create a `go.work` file — see [AGENTS.md](AGENTS.md) for setup instructions.

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/larsartmann/go-localsync/pkg/providers/github"
    "github.com/larsartmann/go-localsync/pkg/storage"
    "github.com/larsartmann/go-localsync/pkg/sync"
)

func main() {
    ctx := context.Background()

    // Create a provider (GitHub built-in)
    ghProvider := github.NewClient("your-github-token")

    // Open storage (creates DB + runs migrations automatically)
    store, err := storage.Open("events.db")
    if err != nil {
        log.Fatal(err)
    }
    defer store.Close()

    // Sync
    syncer := sync.NewSyncer(ghProvider, store, nil)
    result, err := syncer.SyncIncremental(ctx, &sync.SyncOptions{
        Source:   "username",
        MaxPages: 10,
    })
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Fetched: %d, Skipped: %d", result.Fetched, result.Skipped)
}
```

## Core Interfaces

### Provider

Implement `provider.Provider` to add support for any data source:

```go
type Provider interface {
    Name() string
    Fetch(ctx context.Context, opts *FetchOptions) (*FetchResult, error)
    FetchAll(ctx context.Context, source string, maxPages int) (*FetchResult, error)
    GetRateLimit(ctx context.Context) (*RateLimitInfo, error)
}
```

### Item

All providers return `provider.Item` objects with branded ID types:

```go
type Item struct {
    ID             types.ItemID      // Unique ID from source
    Source         types.ProviderID  // Provider name (e.g., "github")
    Type           types.EventTypeID // Item type (e.g., "PushEvent")
    ActorLogin     types.ActorID     // Who triggered it
    ActorAvatarURL string
    RepoName       types.RepoID      // Repository (e.g., "owner/repo")
    RepoURL        string
    CreatedAt      time.Time
    UpdatedAt      time.Time
    RawJSON        []byte            // Full original payload
}
```

### Storage

The `storage.Storage` interface for custom backends:

```go
type Storage interface {
    Upsert(ctx context.Context, item *provider.Item) error
    UpsertBatch(ctx context.Context, items []*provider.Item) error
    GetByID(ctx context.Context, id string) (*provider.Item, error)
    GetLatest(ctx context.Context) (*provider.Item, error)
    GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error)
    GetItemsByType(ctx context.Context, itemType string, limit, offset int) ([]*provider.Item, error)
    GetItemsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*provider.Item, error)
    GetItemsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*provider.Item, error)
    GetItemsBySource(ctx context.Context, source string, limit, offset int) ([]*provider.Item, error)
    GetItemsSince(ctx context.Context, since time.Time) ([]*provider.Item, error)
    Delete(ctx context.Context, id string) error
    DeleteAll(ctx context.Context) error
    Count(ctx context.Context) (int64, error)
    CountByType(ctx context.Context, itemType string) (int64, error)
    GetTypes(ctx context.Context) ([]string, error)
    Close() error
}
```

## Conflict-Aware Sync

For multi-device or multi-source scenarios, use `ConflictAwareSyncer` which leverages [go-localfirst](https://github.com/larsartmann/go-localfirst) CRDT primitives (vector clocks, LWW resolution):

```go
baseSyncer := sync.NewSyncer(ghProvider, store, nil)
conflictSyncer := sync.NewConflictAwareSyncer(baseSyncer,
    sync.WithNodeID("device-1"),
)
result, err := conflictSyncer.SyncWithConflictDetection(ctx, &sync.SyncOptions{
    Source: "username",
})
// result.Conflicts contains detected conflicts
// result.Upserted / result.Skipped / result.Errors for totals
```

## Schema Migrations

The storage layer automatically applies database migrations on `storage.Open()`. Migrations are tracked in a `schema_migrations` table and applied idempotently in order. No manual migration steps required.

## Branded IDs

All entity identifiers use branded types from [go-branded-id](https://github.com/larsartmann/go-branded-id), providing compile-time type safety:

```go
ItemID        // id.ID[ItemBrand, string]
ProviderID    // id.ID[ProviderBrand, string]
EventTypeID   // id.ID[EventTypeBrand, string]
ActorID       // id.ID[ActorBrand, string]
RepoID        // id.ID[RepoBrand, string]
EventID       // id.ID[EventBrand, ulid.ULID]
GithubEventID // id.ID[GithubEventBrand, string]
```

## Built-in Providers

| Provider | Package                | Description             |
| -------- | ---------------------- | ----------------------- |
| GitHub   | `pkg/providers/github` | Sync GitHub user events |

## Example CLI

See `cmd/examples/github-sync/` for a complete CLI implementation:

```bash
# Build the example
go build -o gh-sync ./cmd/examples/github-sync

# Use it
export GITHUB_TOKEN=your_token
gh-sync -user octocat
```

## Features

| Feature             | Status  | Description                                              |
| ------------------- | ------- | -------------------------------------------------------- |
| Incremental Sync    | ✅ Done | Only fetch new items since last sync — no duplicate data |
| Full Fidelity       | ✅ Done | Raw JSON stored for 100% data preservation               |
| Branded IDs         | ✅ Done | Compile-time type-safe identifiers                       |
| Schema Migrations   | ✅ Done | Version-tracked, idempotent, auto-applied                |
| Conflict-Aware Sync | ✅ Done | CRDT-backed conflict detection with vector clocks        |
| No CGO              | ✅ Done | Pure Go SQLite driver (modernc.org/sqlite)               |
| Rate Limiting       | ✅ Done | Configurable rate limiting wired into sync flow          |
| Retry Logic         | ✅ Done | Exponential backoff retry with configurable limits       |

## Development

```bash
just build    # Build
just test     # Run tests (48 tests across 8 suites)
just lint     # Run linter (requires golangci-lint v2)
just sqlc     # Generate sqlc code
just verify   # Build + test + lint
just ci       # Full CI gate
```

## Architecture

```
pkg/
├── provider/         # Core interfaces (Provider, Item, FetchResult, configs)
├── providers/
│   └── github/       # GitHub provider implementation
├── storage/          # Storage interface and SQLite implementation
├── sync/
│   ├── sync.go            # Syncer — basic full/incremental sync
│   └── conflict_aware.go  # ConflictAwareSyncer — CRDT-backed sync
├── types/            # Branded ID types (go-branded-id)
├── errors/           # Sentinel errors (cockroachdb/errors)
└── testhelpers/      # Shared test mocks and factories

internal/
├── database/
│   ├── connection.go      # Database connection management
│   └── migration.go       # Schema migration runner
└── db/                    # sqlc-generated code (models, queries)

sql/migrations/             # Reference SQL migration files
cmd/examples/github-sync/   # Example CLI application
```

## Testing

48 tests across 8 test suites (121 including subtests):

| Package                | Tests | Coverage                        |
| ---------------------- | ----- | ------------------------------- |
| `internal/database`    | 6     | Migrations, idempotency, schema |
| `pkg/errors`           | 4     | Sentinel error matching         |
| `pkg/provider`         | 1     | Interface validation            |
| `pkg/providers/github` | 21    | Client, fetch, retry, errors    |
| `pkg/storage`          | Suite | SQLite CRUD operations          |
| `pkg/sync`             | 11    | Syncer + ConflictAwareSyncer    |
| `pkg/types`            | 5     | Branded ID construction         |

```bash
just test              # Run all tests
go test -cover ./...   # Coverage (requires Go 1.26.1 toolchain)
```

## Related Projects

### `go-localfirst` — Local-First Application Framework

[go-localfirst](https://github.com/larsartmann/go-localfirst) is a full local-first application framework with HTTP server, WebSocket sync, event sourcing, and Pebble storage. Its `pkg/sync` package provides the CRDT primitives (vector clocks, LWW resolution) that `go-localsync` uses for conflict-aware sync.

### `go-localfirst/pkg/sync` — Shared CRDT Primitives

Both projects share the same CRDT primitives from `go-localfirst/pkg/sync`:

```go
import pkgsync "github.com/larsartmann/go-localfirst/pkg/sync"

vc := pkgsync.NewVectorClock()
vc.Increment("node-1")
resolver := pkgsync.NewLWWResolver[*MyType](func(t *MyType) time.Time { return t.UpdatedAt })
```

### When to Use Which

| Need                                                                     | Use                              |
| ------------------------------------------------------------------------ | -------------------------------- |
| Build a local-first application (server, WebSocket sync, event sourcing) | **go-localfirst**                |
| Sync primitives (vector clocks, conflict resolution) in any Go project   | **go-localfirst/pkg/sync**       |
| Sync external API data (GitHub, Jira, etc.) to local SQLite              | **go-localsync**                 |
| Both: local-first app + external data aggregation                        | Use both — they share `pkg/sync` |

## License

MIT
