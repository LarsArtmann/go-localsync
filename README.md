# go-localsync

A Go SDK for building local sync applications. Fetch data from any provider, store it locally in SQLite.

## Overview

**go-localsync is an SDK, not a CLI application.** It provides:

- **Provider Interface** - Implement `provider.Provider` to sync from any data source (GitHub, GitLab, Jira, etc.)
- **Storage Abstraction** - SQLite storage with full JSON fidelity
- **Sync Logic** - Incremental sync, pagination, rate limiting, retry with backoff

## Installation

```bash
go get github.com/larsartmann/go-localsync
```

## Quick Start

```go
package main

import (
    "context"
    "github.com/larsartmann/go-localsync/pkg/providers/github"
    "github.com/larsartmann/go-localsync/pkg/storage"
    "github.com/larsartmann/go-localsync/pkg/sync"
)

func main() {
    // Create a provider (GitHub built-in)
    ghProvider := github.NewClient("your-github-token")

    // Create storage
    store, _ := storage.NewSQLiteStorage(db)

    // Sync
    syncer := sync.NewSyncer(ghProvider, store, logger)
    result, _ := syncer.SyncIncremental(ctx, &sync.SyncOptions{
        Source:   "username",
        MaxPages: 10,
    })
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

All providers return `provider.Item` objects:

```go
type Item struct {
    ID             string    // Unique ID from source
    Source         string    // Provider name (e.g., "github")
    Type           string    // Item type (e.g., "PushEvent")
    ActorLogin     string    // Who triggered it
    ActorAvatarURL string
    RepoName       string    // Repository (e.g., "owner/repo")
    RepoURL        string
    CreatedAt      time.Time
    RawJSON        []byte    // Full original payload
}
```

### Storage

The `storage.Storage` interface for custom backends:

```go
type Storage interface {
    Upsert(ctx context.Context, item *provider.Item) error
    GetLatest(ctx context.Context) (*provider.Item, error)
    GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error)
    Count(ctx context.Context) (int64, error)
    GetTypes(ctx context.Context) ([]string, error)
    // ... more methods
}
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

- **Incremental Sync** - Only fetch new items since last sync
- **Rate Limiting** - Automatic rate limit handling with configurable wait
- **Retry Logic** - Exponential backoff for transient errors
- **Full Fidelity** - Raw JSON stored for 100% data preservation
- **Type-Safe** - sqlc-generated queries for SQLite
- **No CGO** - Pure Go SQLite driver

## Development

```bash
just build    # Build
just test     # Run tests
just sqlc     # Generate sqlc code
just lint     # Run linter
```

## Architecture

```
pkg/
├── provider/         # Core interfaces (Provider, Item, Storage)
├── providers/        # Built-in providers
│   └── github/       # GitHub provider implementation
├── storage/          # Storage interface and SQLite implementation
├── sync/             # Sync logic (fetch → store)
└── errors/           # Typed errors

internal/
├── database/         # Database connection management
└── db/               # sqlc generated code

cmd/examples/         # Example CLI applications
```

## License

MIT
