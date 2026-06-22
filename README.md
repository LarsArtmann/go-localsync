# go-localsync

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)
[![License: Proprietary](https://img.shields.io/badge/License-Proprietary-red.svg)](LICENSE)

**A pluggable SDK for syncing data from any provider to local event-sourced storage.** Build local-first applications that work offline with full data fidelity, incremental sync, and CQRS-based state management.

_go-localsync is a **pure contract library** — not a CLI and not a provider implementation._ It defines the `Provider` interface and the sync/CQRS engine. The reference consumer (GitHub provider + CLI) is [`github-local-sync`](https://github.com/larsartmann/github-local-sync).

## Overview

| Component               | Description                                                                                                                    |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| **Provider Interface**  | Implement `provider.Provider` to sync from any data source (GitHub, GitLab, Jira, etc.)                                        |
| **CQRS Stack**          | Event-sourced architecture via [go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite) v3 — Decider, ReadModel, Projection |
| **Sync Engine**         | Full and incremental sync with pagination, configurable rate limiting and retry                                                |
| **Conflict-Aware Sync** | Timestamp-based conflict detection with remote-wins strategy, emitted as domain events                                         |
| **Branded IDs**         | Type-safe IDs from [go-branded-id](https://github.com/larsartmann/go-branded-id)                                               |
| **SQLite Backend**      | SQLite event store with snapshots and pure-Go driver (no CGo)                                                                  |

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

> **Note:** This module depends on a **private** repository (`go-cqrs-lite`). Set environment variables for private module access:
>
> ```bash
> export GONOSUMCHECK=github.com/larsartmann/*
> export GONOSUMDB=github.com/larsartmann/*
> ```
>
> For local development with sibling checkouts, create a `go.work` file — see [AGENTS.md](AGENTS.md) for setup instructions.

## Quick Start

The SDK ships no provider, so the quickest way to see it work is a tiny in-process provider. Implement `provider.Provider`, then wire it into a CQRS stack and sync:

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/sync"
)

// stubProvider is a minimal provider that returns one hardcoded item.
type stubProvider struct{}

func (stubProvider) Name() string { return "stub" }
func (s stubProvider) Fetch(context.Context, *provider.FetchOptions) (*provider.FetchResult, error) {
	return s.fetchResult(), nil
}
func (s stubProvider) FetchAll(context.Context, string, int) (*provider.FetchResult, error) {
	return s.fetchResult(), nil
}
func (stubProvider) GetRateLimit(context.Context) (*provider.RateLimitInfo, error) { return nil, nil }
func (stubProvider) fetchResult() *provider.FetchResult {
	now := time.Now()
	return &provider.FetchResult{
		Items: []*provider.Item{{
			ID:         id.NewItemID(),
			ExternalID: id.NewExternalID("event-1"),
			Source:     id.NewProviderID("stub"),
			Type:       id.NewEventTypeID("PushEvent"),
			ActorLogin: id.NewActorLogin("octocat"),
			RepoName:   id.NewRepoID("octocat/Hello-World"),
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
	}
}

func main() {
	ctx := context.Background()

	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	if err != nil {
		log.Fatal(err)
	}
	defer stack.Close()

	syncer := sync.NewSyncer(stubProvider{}, stack, nil)
	result, err := syncer.Sync(ctx, &sync.SyncOptions{Source: "octocat", MaxPages: 1})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Fetched: %d, Skipped: %d, Errors: %d", result.Fetched, result.Skipped, result.Errors)
}
```

For a real-world example — GitHub API client, CLI flags, SQLite persistence, HTTP server — see the reference consumer app [`github-local-sync`](https://github.com/larsartmann/github-local-sync).

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

All providers return `provider.Item` objects (the DTO at the SDK boundary) with branded ID types from `pkg/id/`:

```go
type Item struct {
	ID             id.ItemID      // Internal ULID-based identifier
	ExternalID     id.ExternalID  // Original ID from source system
	Source         id.ProviderID  // Provider name (e.g., "github")
	Type           id.EventTypeID // Item type (e.g., "PushEvent")
	ActorLogin     id.ActorLogin    // Who triggered it
	ActorAvatarURL string
	RepoName       id.RepoID      // Repository (e.g., "owner/repo")
	RepoURL        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RawJSON        json.RawMessage // Full original payload
}
```

At the CQRS boundary, `provider.Item` is converted to the domain entity `model.Item` (which adds a `SchemaVersion` for event upcasting). The decider, read model, events, and conflict resolver all operate on `*model.Item`.

## CQRS Architecture

The entire storage layer is event-sourced via [go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite) v3. There is no legacy CRUD path.

| Component        | Description                                                                                                                 |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------- |
| **Decider**      | Pure Apply/DecideSync/DecideDelete — single authority for state transitions (`SyncItemState{Item *model.Item}`)             |
| **Events**       | `ItemSynced`, `ItemConflictFound`, `ItemDeleted`                                                                            |
| **Projection**   | Live events via synchronous `bus.SubscribeAll`; SQLite catch-up via background `runner.replayJournal` (no checkpoint store) |
| **Read Model**   | In-memory or SQLite with filter/pagination, stores `*model.Item`                                                            |
| **Aggregate ID** | Deterministic SHA256→hex from (source, sourceID) for idempotency                                                            |

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic IDs**: SHA256→hex from (source, sourceID) with sync.Map cache
- **Delete + resurrect**: deleted items reappear with updated state when synced again
- **Conflict detection**: `DecideSync` compares fields and emits `ItemConflictFound` events
- **Remote wins**: on conflict, incoming item overwrites (remote-wins LWW)
- **Pluggable conflict resolution**: inject any `crdt.ConflictResolver[*model.Item]` for custom merge logic
- **Projection**: synchronous live subscription + background journal replay (idempotent, no checkpoint store needed)
- **Snapshots**: caps replay cost by persisting aggregate state every N events
- **Correlation IDs**: unique per sync run for cross-event tracing and debugging

## Conflict-Aware Sync

For multi-device or multi-source scenarios, use `ConflictAwareSyncer`:

```go
baseSyncer := sync.NewSyncer(myProvider, stack, nil)
conflictSyncer := sync.NewConflictAwareSyncer(baseSyncer)
result, err := conflictSyncer.SyncWithConflictDetection(ctx, &sync.SyncOptions{
	Source: "username",
})
// result.Conflicts contains detected conflicts
// result.Upserted / result.Skipped / result.Errors for totals
```

## Pluggable Conflict Resolution

Inject a custom `crdt.ConflictResolver[T]` for domain-specific merge logic:

```go
stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{
	Backend:          "sqlite",
	DBPath:           "/path/to/local.db",
	ConflictResolver: crdt.NewLWWResolver[*model.Item](),
})
```

## Branded IDs

All entity identifiers use branded phantom types from [go-branded-id](https://github.com/larsartmann/go-branded-id), providing compile-time type safety:

```go
ItemID        // id.ID[ItemBrand, ulid.ULID]      — internal ULID-based identifier
ExternalID    // id.ID[ExternalBrand, string]      — provider-specific item identifier
ProviderID    // id.ID[ProviderBrand, string]       — source provider (e.g., "github")
EventTypeID   // id.ID[EventTypeBrand, string]      — item type (e.g., "PushEvent")
ActorLogin      // id.ID[ActorLoginBrand, string]      — external user who triggered the event
RepoID        // id.ID[RepoBrand, string]           — repository (e.g., "owner/repo")
```

## Features

| Feature            | Status  | Description                                                  |
| ------------------ | ------- | ------------------------------------------------------------ |
| CQRS Stack         | ✅ Done | Event store, bus, decider repository, read model, projection |
| Decider Pattern    | ✅ Done | Pure Apply/DecideSync/DecideDelete with SyncItemState        |
| Incremental Sync   | ✅ Done | Only fetch new items since last sync — no duplicate data     |
| Full Fidelity      | ✅ Done | Raw JSON stored for 100% data preservation                   |
| Conflict Detection | ✅ Done | Timestamp-based comparison, remote-wins resolution           |
| Branded IDs        | ✅ Done | Compile-time type-safe identifiers                           |
| SQLite Backend     | ✅ Done | SQLite event store + read model + snapshots (no CGo)         |
| No CGO             | ✅ Done | Pure Go SQLite driver (modernc.org/sqlite)                   |
| Rate Limiting      | ✅ Done | Configurable rate limiting wired into sync flow              |
| Retry Logic        | ✅ Done | Exponential backoff retry with configurable limits           |
| Snapshots          | ✅ Done | Aggregate state persistence caps replay cost across restarts |
| Correlation IDs    | ✅ Done | Unique per sync run for cross-event tracing and debugging    |
| Event Logging      | ✅ Done | Structured logging of all domain events via middleware       |
| Delete + Resurrect | ✅ Done | Deleted items reappear with updated state when synced again  |

## Development

```bash
go build ./...                        # Build
go test ./... -count=1                # Run tests (225 tests across 9 packages)
golangci-lint run ./... --timeout=5m  # Lint (golangci-lint v2)
golangci-lint fmt ./...               # Format
```

Full pipeline: `buildflow --build-mode full` (ensure `go.work` is removed first — see AGENTS.md).

## Architecture

```
pkg/
├── provider/         # Core interfaces (Provider, Item, FetchResult, configs, RateLimitCache)
├── cqrs/             # CQRS integration (Decider, ReadModel, Projection, Stack, runner)
├── sync/             # Syncer + ConflictAwareSyncer + SyncStore interface + SyncAction
├── data/
│   ├── model/        # Domain entity: Item, Key, ItemFilter, ItemReader
│   └── schema/       # Schema Version (V1/V2) for event upcasting
├── id/               # Branded ID types (go-branded-id)
├── crdt/             # CRDT primitives: VectorClock, LWWResolver, ConflictResolver
├── errors/           # Structured errors via go-error-family constructors
├── api/              # HTTP API server (Huma v2)
└── testutil/         # Shared test helpers (MockProvider, SyncStore double)
```

## Testing

225 test functions across 9 packages:

| Package           | Tests | Coverage | Description                                                     |
| ----------------- | ----- | -------- | --------------------------------------------------------------- |
| `pkg/cqrs`        | 89    | 81.4%    | Decider, ReadModel, Projection, Stack, SQLite RM, CRDT Resolver |
| `pkg/sync`        | 24    | 85.5%    | Syncer + ConflictAwareSyncer + reportProgress                   |
| `pkg/crdt`        | 53    | 96.2%    | VectorClock, Operation, LWWResolver, Conflict, SyncMessage      |
| `pkg/id`          | 12    | 100.0%   | ID construction, roundtrip, zero, equal                         |
| `pkg/errors`      | 9     | 100.0%   | Sentinel errors, wrapping, classification, IsRetryable          |
| `pkg/provider`    | 10    | 96.7%    | Item validation, RateLimitCache                                 |
| `pkg/api`         | 14    | 94.0%    | Server, routes, handlers, health/stats/items/sync endpoints     |
| `pkg/data/model`  | 10    | 100.0%   | Item, Key, Validate, ItemFilter builder                         |
| `pkg/data/schema` | 4     | 100.0%   | Schema Version (V1/V2), CurrentVersion, Valid                   |

## Related Projects

| Need                                                         | Use                                                                       |
| ------------------------------------------------------------ | ------------------------------------------------------------------------- |
| Sync GitHub events to local storage (reference consumer)     | **[github-local-sync](https://github.com/larsartmann/github-local-sync)** |
| Build a local-first application with event sourcing and CQRS | **[go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite)**           |

## License

Proprietary — see [LICENSE](LICENSE) for details.
