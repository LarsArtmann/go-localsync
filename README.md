# go-localsync

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**A pluggable SDK for syncing data from any provider to local event-sourced storage.** Build local-first applications that work offline with full data fidelity, incremental sync, and CQRS-based state management.

_go-localsync is an SDK, not a CLI application._ Use it as a library to add data synchronization to your Go applications.

## Overview

| Component               | Description                                                                                                                 |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| **Provider Interface**  | Implement `provider.Provider` to sync from any data source (GitHub, GitLab, Jira, etc.)                                     |
| **CQRS Stack**          | Event-sourced architecture via [go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite) — Decider, ReadModel, Projection |
| **Sync Engine**         | Full and incremental sync with pagination, configurable rate limiting and retry                                             |
| **Conflict-Aware Sync** | Timestamp-based conflict detection with remote-wins strategy, emitted as domain events                                      |
| **Branded IDs**         | Type-safe IDs from [go-branded-id](https://github.com/larsartmann/go-branded-id)                                            |
| **Turso Backend**       | SQLite/Turso event store with remote Push/Pull sync support                                                                 |

## Who is this for?

This SDK is for Go developers building applications that need:

- **Offline-first** functionality with local data access
- **Offline dashboards** that aggregate data from multiple services
- **Custom sync logic** tailored to specific use cases
- **Event sourcing** with full audit trail and replay capability
- **Conflict detection** for multi-device or multi-source synchronization

## Installation

```bash
go get github.com/larsartmann/go-localsync
```

> **Note:** This module depends on private repositories (`go-cqrs-lite` and `go-branded-id`). Set environment variables for private module access:
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

    "github.com/larsartmann/go-localsync/pkg/cqrs"
    "github.com/larsartmann/go-localsync/pkg/providers/github"
    "github.com/larsartmann/go-localsync/pkg/sync"
)

func main() {
    ctx := context.Background()

    ghProvider := github.NewClient("your-github-token")

    stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
    if err != nil {
        log.Fatal(err)
    }
    defer stack.Close()

    syncer := sync.NewSyncer(ghProvider, stack, nil)
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
    FetchAll(ctx context.Context, source string, maxPages int) (*FetchLimitInfo, error)
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

## CQRS Architecture

The entire storage layer is event-sourced via [go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite). There is no legacy CRUD path.

| Component        | Description                                                                |
| ---------------- | -------------------------------------------------------------------------- |
| **Decider**      | Pure Fold/DecideSync/DecideDelete — single authority for state transitions |
| **Events**       | `ItemSynced`, `ItemConflictFound`, `ItemDeleted`                           |
| **Projection**   | Subscribes to event bus, updates read model                                |
| **Read Model**   | In-memory or Turso-backed with filter/pagination                           |
| **Aggregate ID** | Deterministic SHA256→ULID from (source, sourceID) for idempotency          |

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic IDs**: SHA256→ULID from (source, sourceID)
- **Conflict detection**: `DecideSync` compares fields and emits `ItemConflictFound` events
- **Remote wins**: on conflict, incoming item overwrites

## Conflict-Aware Sync

For multi-device or multi-source scenarios, use `ConflictAwareSyncer`:

```go
baseSyncer := sync.NewSyncer(ghProvider, stack, nil)
conflictSyncer := sync.NewConflictAwareSyncer(baseSyncer)
result, err := conflictSyncer.SyncWithConflictDetection(ctx, &sync.SyncOptions{
    Source: "username",
})
// result.Conflicts contains detected conflicts
// result.Upserted / result.Skipped / result.Errors for totals
```

## Turso Remote Sync

Push and pull changes to/from a remote Turso database:

```go
stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{
    Backend:   "turso",
    DBPath:    "/path/to/local.db",
    RemoteURL: "libsql://your-db.turso.io",
    AuthToken: "your-turso-token",
})

// Pull remote changes before sync
changed, err := stack.Pull(ctx)

// Push local changes after sync
err = stack.Push(ctx)
```

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

See `cmd/examples/github-sync/` for a complete CLI implementation with flag parsing, signal handling, and exit codes:

```bash
go build -o gh-sync ./cmd/examples/github-sync

export GITHUB_TOKEN=your_token
gh-sync -user octocat
gh-sync -user octocat --conflict-aware
gh-sync -user octocat --push --pull
gh-sync -stats
```

## Features

| Feature            | Status  | Description                                                  |
| ------------------ | ------- | ------------------------------------------------------------ |
| CQRS Stack         | ✅ Done | Event store, bus, decider repository, read model, projection |
| Decider Pattern    | ✅ Done | Pure Fold/DecideSync/DecideDelete with SyncItemState         |
| Incremental Sync   | ✅ Done | Only fetch new items since last sync — no duplicate data     |
| Full Fidelity      | ✅ Done | Raw JSON stored for 100% data preservation                   |
| Conflict Detection | ✅ Done | Timestamp-based comparison, remote-wins resolution           |
| Branded IDs        | ✅ Done | Compile-time type-safe identifiers                           |
| Turso Backend      | ✅ Done | SQLite/Turso event store + read model + Push/Pull sync       |
| No CGO             | ✅ Done | Pure Go SQLite driver (modernc.org/sqlite)                   |
| Rate Limiting      | ✅ Done | Configurable rate limiting wired into sync flow              |
| Retry Logic        | ✅ Done | Exponential backoff retry with configurable limits           |
| GitHub Provider    | ✅ Done | Full implementation with pagination, rate limiting, retry    |

## Development

```bash
go build ./...                    # Build
go test ./... -count=1            # Run tests (107 tests across 6 packages)
go vet ./...                      # Vet
golangci-lint run ./... --timeout=5m  # Lint (golangci-lint v2)
golangci-lint fmt ./...           # Format
```

## Architecture

```
pkg/
├── provider/         # Core interfaces (Provider, Item, FetchResult, configs)
├── providers/
│   └── github/       # GitHub provider implementation
├── cqrs/             # CQRS integration (Decider, ReadModel, Projection, Stack)
├── sync/
│   ├── sync.go            # Syncer — basic full/incremental sync
│   └── conflict_aware.go  # ConflictAwareSyncer — conflict-aware sync
├── types/            # Branded ID types (go-branded-id)
├── errors/           # Sentinel errors with stdlib fmt.Errorf wrapping
└── testhelpers/      # Shared test mocks and factories

cmd/examples/github-sync/   # Example CLI application
```

## Testing

107 test cases across 6 test packages:

| Package                | Tests | Description                                     |
| ---------------------- | ----- | ----------------------------------------------- |
| `pkg/cqrs`             | 46    | Decider, ReadModel, Projection, Stack, Turso RM |
| `pkg/providers/github` | 35    | Client, fetch, retry, errors (19 unit + 16 BDD) |
| `pkg/sync`             | 11    | Syncer + ConflictAwareSyncer                    |
| `pkg/types`            | 10    | Branded ID construction, roundtrip              |
| `pkg/errors`           | 4     | Sentinel errors, wrapping                       |
| `pkg/provider`         | 1     | Item validation                                 |

## Related Projects

| Need                                                         | Use              |
| ------------------------------------------------------------ | ---------------- |
| Sync external API data (GitHub, Jira, etc.) to local storage | **go-localsync** |
| Build a local-first application with event sourcing and CQRS | **go-cqrs-lite** |
| HTTP endpoints with HTMX, templ, Casbin auth                 | **cqrs-htmx**    |

## License

MIT
