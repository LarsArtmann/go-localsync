# go-localsync

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](https://go.dev)
[![License: Proprietary](https://img.shields.io/badge/License-Proprietary-red.svg)](LICENSE)

**A Go SDK for building a local-first mirror of any paginated REST API.** Pull every item from GitHub, GitLab, Jira, or your own service into an offline, idempotent read model — with full-fidelity JSON storage, incremental sync, conflict detection, soft-deletes (tombstones), and a CQRS event log you can replay from scratch.

_go-localsync is a **single-writer pull mirror**: the provider is the sole source of truth and there is no local mutation or multi-writer merge. The core module is a **pure contract library** — no CLI binary._ It defines the `Provider` interface and the sync/CQRS engine. A ready-made **GitHub events provider** ships as the optional nested module [`provider/github`](provider/github) (see [GitHub events out of the box](#github-events-out-of-the-box)); the reference CLI consumer is [`github-local-sync`](https://github.com/larsartmann/github-local-sync).

## Overview

| Component               | Description                                                                                                                    |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| **Provider Interface**  | Implement `provider.Provider` to sync from any data source (GitHub, GitLab, Jira, etc.)                                        |
| **CQRS Stack**          | Event-sourced architecture via [go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite) v4 — Decider, ReadModel, Projection |
| **Sync Engine**         | Full and incremental sync with pagination, rate limiting, retry with backoff, and per-source serialization                     |
| **Tombstones**          | Soft-delete with reasons (upstream-gone, user-hidden); optional reconciliation tombstones items the provider stopped returning |
| **Conflict Resolution** | Detects when a re-synced item changed since last sync; remote-wins by default, pluggable `crdt.ConflictResolver`               |
| **Branded IDs**         | Type-safe IDs from [go-branded-id](https://github.com/larsartmann/go-branded-id)                                               |
| **SQLite Backend**      | SQLite event store with snapshots and pure-Go driver (no CGo)                                                                  |

## Who is this for?

Go developers building applications that need to keep a local, queryable copy of data living behind a paginated REST API — and know when that data changed upstream:

- **Offline-first apps** that read from a local store instead of hitting the network on every view
- **Dashboards & aggregators** that merge items from several services into one local database
- **Audit-sensitive pipelines** that need a full event log and the ability to replay state from scratch
- **Change-aware mirrors** that detect the moment a re-fetched item differs from what you already stored
- **Custom sync logic** where a generic sync tool won't fit and you need to own the loop

Not for multi-writer or distributed sync: the provider is the sole source of truth per aggregate. If you need push ingestion or multi-node CRDT merge, use [`go-cqrs-lite`](https://github.com/larsartmann/go-cqrs-lite) directly.

## Installation

```bash
go get github.com/larsartmann/go-localsync
```

> For local development with sibling checkouts, create a `go.work` file — see [AGENTS.md](AGENTS.md) for setup instructions.

## Quick Start

The SDK core ships no provider, so the quickest way to see it work is a tiny in-process provider. Implement `provider.Provider`, then wire it into a CQRS stack and sync:

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
			Attributes: map[string]string{
				"actor_login": "octocat",
				"repo_name":   "octocat/Hello-World",
			},
			CreatedAt: now,
			UpdatedAt: now,
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

### GitHub events out of the box

Syncing GitHub user events? The optional [`provider/github`](provider/github) module implements `provider.Provider` over GitHub's `GET /users/{user}/events` endpoint — token auth, rate-limit gating fed from response headers, retry with backoff, and error classification onto the SDK's error family:

```bash
go get github.com/larsartmann/go-localsync@latest
go get github.com/larsartmann/go-localsync/provider/github@latest
```

See the [module README](provider/github/README.md) for configuration and usage.

For a full application — CLI flags, SQLite persistence, HTTP server — see the reference consumer app [`github-local-sync`](https://github.com/larsartmann/github-local-sync).

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
	ID         id.ItemID         // Internal ULID-based identifier
	ExternalID id.ExternalID     // Original ID from source system
	Source     id.ProviderID     // Provider name (e.g., "github")
	Type       id.EventTypeID    // Item type (e.g., "PushEvent")
	Attributes map[string]string // Provider-specific key-values (actor_login, repo_name, ...)
	CreatedAt  time.Time
	UpdatedAt  time.Time
	RawJSON    json.RawMessage   // Full original payload
}
```

At the CQRS boundary, `provider.Item` is converted to the domain entity `model.Item` (which adds a `SchemaVersion` for event upcasting). The decider, read model, events, and conflict resolver all operate on `*model.Item`.

## CQRS Architecture

The entire storage layer is event-sourced via [go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite) v4. There is no legacy CRUD path.

| Component        | Description                                                                                                                                 |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| **Decider**      | Pure Apply/DecideSync/DecideTombstone — single authority for state transitions (`SyncItemState{Item *model.Item}`)                          |
| **Events**       | `ItemSynced`, `ItemConflictFound`, `ItemTombstoned`                                                                                         |
| **Projection**   | Live events via synchronous `bus.SubscribeAll`; SQLite catch-up via `projectionhost.Host` (checkpoint persistence, crash auto-restart, DLQ) |
| **Read Model**   | In-memory or SQLite with filter/pagination, stores `*model.Item`                                                                            |
| **Aggregate ID** | Deterministic SHA256→hex from (source, sourceID) for idempotency                                                                            |

### Key Properties

- **Idempotent**: same item synced twice → 1 aggregate, 1 read model entry
- **Deterministic IDs**: SHA256→hex from (source, sourceID) with sync.Map cache
- **Tombstone + resurrect**: hidden items keep their history; re-syncing a tombstoned item makes it live again
- **Reconciliation**: opt-in `SyncOptions.Reconcile` tombstones items the provider stopped returning (`upstream_gone`)
- **Conflict detection**: `DecideSync` compares fields and emits `ItemConflictFound` events
- **Remote wins**: on conflict, incoming item overwrites (remote-wins LWW)
- **Pluggable conflict resolution**: inject any `crdt.ConflictResolver[*model.Item]` for custom merge logic
- **Resilient fetch**: exponential backoff with jitter, honors `errors.IsRetryable`, optional Retry-After
- **Per-source serialization**: concurrent syncs of the same source are ordered; different sources run in parallel
- **Projection**: synchronous live subscription + managed `projectionhost.Host` catch-up (checkpoint persistence, crash auto-restart, dead-letter queue)
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

Inject a custom `crdt.ConflictResolver[T]` for domain-specific merge logic. `NewLWWResolver` takes a timestamp extractor and returns `(resolver, error)`:

```go
resolver, err := crdt.NewLWWResolver[*model.Item](func(i *model.Item) time.Time {
	return i.UpdatedAt
})
if err != nil {
	log.Fatal(err)
}

stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{
	Backend:          "sqlite",
	DBPath:           "/path/to/local.db",
	ConflictResolver: resolver,
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

| Feature                 | Status  | Description                                                            |
| ----------------------- | ------- | ---------------------------------------------------------------------- |
| CQRS Stack              | ✅ Done | Event store, bus, decider repository, read model, projection           |
| Decider Pattern         | ✅ Done | Pure Apply/DecideSync/DecideTombstone with SyncItemState               |
| Incremental Sync        | ✅ Done | Only fetch new items since last sync — no duplicate data               |
| Full Fidelity           | ✅ Done | Raw JSON stored for 100% data preservation                             |
| Conflict Detection      | ✅ Done | Timestamp-based comparison, remote-wins resolution                     |
| Branded IDs             | ✅ Done | Compile-time type-safe identifiers                                     |
| SQLite Backend          | ✅ Done | SQLite event store + read model + snapshots (no CGo)                   |
| No CGO                  | ✅ Done | Pure Go SQLite driver (modernc.org/sqlite)                             |
| Rate Limiting           | ✅ Done | Configurable rate limiting wired into sync flow                        |
| Retry Logic             | ✅ Done | Exponential backoff + jitter, honors IsRetryable, optional Retry-After |
| Snapshots               | ✅ Done | Aggregate state persistence caps replay cost across restarts           |
| Correlation IDs         | ✅ Done | Unique per sync run for cross-event tracing and debugging              |
| Event Logging           | ✅ Done | Structured logging of all domain events via middleware                 |
| Tombstone + Resurrect   | ✅ Done | Soft-delete keeps history; re-syncing a tombstoned item resurrects it  |
| Upstream Reconciliation | ✅ Done | Tombstone items the provider stopped returning (opt-in per sync)       |
| Per-source Locking      | ✅ Done | Concurrent syncs of one source are ordered; sources run in parallel    |

## Development

```bash
go build ./...                        # Build
go test ./... -count=1                # Run tests (232 tests across 10 packages)
golangci-lint run ./... --timeout=5m  # Lint (golangci-lint v2)
golangci-lint fmt ./...               # Format
```

> **Build tag required (Go 1.26):** the storage layer uses `encoding/json/v2`, which Go 1.26 gates behind `GOEXPERIMENT=jsonv2`. Enter the nix devShell (`nix develop`, or `direnv allow` with the committed `.envrc`) — it exports `GOFLAGS=-tags=goexperiment.jsonv2` for you. Plain-shell `go build`/`go test` fail with `encoding/json/v2: build constraints exclude all Go files` until Go 1.27 graduates the package.

Full pipeline: `buildflow --build-mode full` (inside the devShell; see AGENTS.md for the go.work caveat).

## Architecture

```
pkg/
├── provider/         # Core interfaces (Provider, Item, FetchResult, configs, RateLimitCache)
├── cqrs/             # CQRS integration (Decider, ReadModel, Projection, Stack, runner)
├── sync/             # Syncer + ConflictAwareSyncer + SyncStore interface + SyncAction
├── data/
│   ├── model/        # Domain entity: Item, Key, ItemFilter, ItemReader
│   └── schema/       # Schema Version (V1/V2/V3) for event upcasting
├── id/               # Branded ID types (go-branded-id)
├── crdt/             # Conflict resolution: Conflict, ConflictResolver, LWWResolver
├── errors/           # Structured errors via go-error-family constructors
├── api/              # HTTP API server (Huma v2)
└── testutil/         # Shared test helpers (MockProvider, SyncStore double)

provider/github/      # Optional nested module: GitHub events provider (go-github-kit)
```

## Testing

232 test functions across 10 packages:

| Package             | Tests | Coverage | Description                                                             |
| ------------------- | ----- | -------- | ----------------------------------------------------------------------- |
| `pkg/cqrs`          | 97    | 82.4%    | Decider, ReadModel, Projection, Stack, SQLite RM, tombstone, regression |
| `pkg/sync`          | 32    | 88.4%    | Syncer + ConflictAwareSyncer + retry + reconcile + regression           |
| `pkg/crdt`          | 8     | 100.0%   | Conflict, ConflictResolver, LWWResolver                                 |
| `pkg/id`            | 12    | 100.0%   | ID construction, roundtrip, zero, equal                                 |
| `pkg/errors`        | 16    | 92.9%    | Sentinel errors, wrapping, classification, IsRetryable, HTTPStatus      |
| `pkg/provider`      | 2     | 92.3%    | Item validation                                                         |
| `pkg/api`           | 15    | 93.1%    | Server, routes, handlers, health/stats/items/sync endpoints             |
| `pkg/data/model`    | 10    | 80.5%    | Item, Key, Validate, ItemFilter, Tombstone                              |
| `pkg/data/schema`   | 4     | 100.0%   | Schema Version (V1/V2/V3), CurrentVersion, Valid                        |
| `internal/cqrslint` | 36    | 90.0%    | 10 architectural checks (C0001–C0010), suppression, rules catalog       |

## Related Projects

| Need                                                         | Use                                                                       |
| ------------------------------------------------------------ | ------------------------------------------------------------------------- |
| Sync GitHub events to local storage (ready-made provider)    | **[`provider/github`](provider/github)** (this repo's nested module)      |
| Full application example: CLI, SQLite, HTTP server           | **[github-local-sync](https://github.com/larsartmann/github-local-sync)** |
| Build a local-first application with event sourcing and CQRS | **[go-cqrs-lite](https://github.com/larsartmann/go-cqrs-lite)**           |

## License

Proprietary — see [LICENSE](LICENSE) for details.
