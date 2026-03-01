# PARTS.md - Component Analysis & Extraction Opportunities

> **Last Updated:** 2026-02-28
> **Project:** go-localsync
> **Purpose:** Identify components that could be split into reusable libraries/SDKs

---

## Executive Summary

| Component          | Location        | Extract? | Confidence      | Primary Value                                      |
| ------------------ | --------------- | -------- | --------------- | -------------------------------------------------- |
| Provider Interface | `pkg/provider/` | **YES**  | High (85%)      | Generic data source abstraction with rate limiting |
| Sync Engine        | `pkg/sync/`     | NO       | High (90%)      | Core business logic, not useful standalone         |
| Storage Layer      | `pkg/storage/`  | NO       | Medium (70%)    | Too coupled to Item model                          |
| Errors Package     | `pkg/errors/`   | NO       | Very High (98%) | No unique value over cockroachdb/errors            |

**Recommendation:** Extract `pkg/provider/` as a standalone library `go-provider` or `localsync-provider`.

---

## Component 1: Provider Interface

### Location

```
pkg/provider/
├── provider.go    # Core interfaces, Item model, configs
```

### What It Does

- Defines `Provider` interface for any data source
- Universal `Item` model with full JSON fidelity
- Rate limiting configuration and handling
- Retry with exponential backoff configuration
- Pagination support via `FetchOptions` / `FetchResult`

### Key Types

```go
type Provider interface {
    Name() string
    Fetch(ctx context.Context, opts *FetchOptions) (*FetchResult, error)
    FetchAll(ctx context.Context, source string, maxPages int) (*FetchResult, error)
    GetRateLimit(ctx context.Context) (*RateLimitInfo, error)
}

type Item struct {
    ID             string    // Unique ID from source
    Source         string    // Provider name (e.g., "github")
    Type           string    // Item type (e.g., "PushEvent")
    ActorLogin     string    // Who triggered it
    ActorAvatarURL string
    RepoName       string    // Repository (e.g., "owner/repo")
    RepoURL        string
    CreatedAt      time.Time
    RawJSON        []byte    // Full original payload - KEY FEATURE
}

type RateLimitConfig struct {
    Enabled      bool
    MinRemaining int
    MaxWait      time.Duration
}

type RetryConfig struct {
    Enabled        bool
    MaxRetries     int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
}
```

### Alternatives

| Project            | Stars | Description               | Gap                                     |
| ------------------ | ----- | ------------------------- | --------------------------------------- |
| `google/go-github` | 10k+  | Official GitHub SDK       | GitHub-specific, no generic abstraction |
| `xanzy/go-gitlab`  | 1.8k  | GitLab SDK                | GitLab-specific                         |
| `OmniSerp`         | ~100  | Search engine abstraction | Different domain, no Item model         |
| `pocketbase/auth`  | 40k+  | Auth provider abstraction | Auth-focused, not data sync             |

**No direct competitor exists for a generic, rate-limit-aware data source abstraction.**

### Unique Value Proposition

1. **Universal Item Model**
   - Single struct for any data source
   - `RawJSON` preserves 100% of original data
   - Common fields (ID, Type, Actor, Repo, CreatedAt) cover 80% of use cases

2. **Built-in Resilience**
   - Rate limiting with configurable wait/abort
   - Retry with exponential backoff
   - Both configurable per-provider

3. **Provider-Agnostic Pagination**
   - `FetchOptions` / `FetchResult` pattern
   - `HasMore` for pagination detection
   - Works with any API pagination style

4. **Minimal Dependencies**
   - Only standard library + time
   - No transport coupling (HTTP, gRPC, etc.)

### Extraction Path

**New Repository:** `github.com/larsartmann/go-provider`

```
go-provider/
├── provider.go      # Core interfaces
├── item.go          # Item model
├── config.go        # RateLimitConfig, RetryConfig
├── options.go       # FetchOptions, FetchResult
├── ratelimit.go     # Rate limit utilities
├── retry.go         # Retry utilities
└── errors.go        # Provider-specific errors
```

### Use Cases for Standalone Library

1. **Multi-source data aggregation** - Fetch from GitHub, GitLab, Jira, Linear, etc.
2. **Activity feed builders** - Universal event model
3. **Audit log collectors** - Standardized item format with full fidelity
4. **CLI tools** - Provider-agnostic data fetching
5. **Data pipelines** - ETL from multiple SaaS providers

### Risks

| Risk                     | Mitigation                                              |
| ------------------------ | ------------------------------------------------------- |
| Generic model too simple | Allow provider-specific extensions via RawJSON          |
| Rate limit patterns vary | Configurable per-provider, interface allows custom impl |
| Low adoption             | Dogfood in go-localsync, document well                  |

---

## Component 2: Sync Engine

### Location

```
pkg/sync/
├── sync.go       # Syncer struct, Sync/GetStats methods
├── sync_test.go
```

### What It Does

- Orchestrates fetch → store operations
- Incremental sync with cutoff detection
- Statistics aggregation

### Key Types

```go
type Syncer struct {
    provider provider.Provider
    storage  storage.Storage
    logger   *log.Logger
}

type SyncOptions struct {
    Source   string
    MaxPages int
}

type SyncResult struct {
    Fetched int
    Skipped int
    Errors  int
}
```

### Alternatives

| Project                | Description        | Gap                        |
| ---------------------- | ------------------ | -------------------------- |
| Kubernetes reconciler  | Desired state sync | Much heavier, K8s-specific |
| Airbyte                | Data integration   | Heavy infrastructure       |
| Custom implementations | Per-project        | Reinventing the wheel      |

### Why NOT Extract

1. **Too Thin** - Only ~100 LOC, mostly orchestration
2. **Storage Coupling** - Depends on `storage.Storage` interface
3. **Provider Coupling** - Depends on `provider.Provider` interface
4. **No Standalone Value** - Useless without both provider and storage

**Keep as core business logic within go-localsync.**

---

## Component 3: Storage Layer

### Location

```
pkg/storage/
├── interface.go   # Storage interface
├── sqlite.go      # SQLite implementation
├── sqlite_test.go
```

### What It Does

- SQLite storage with sqlc-generated queries
- Upsert, query by type/actor/repo
- Full-text search potential (not implemented)

### Key Types

```go
type Storage interface {
    Upsert(ctx context.Context, item *provider.Item) error
    GetLatest(ctx context.Context) (*provider.Item, error)
    GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error)
    GetItemsByType(ctx context.Context, itemType string, limit, offset int) ([]*provider.Item, error)
    GetItemsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*provider.Item, error)
    GetItemsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*provider.Item, error)
    Count(ctx context.Context) (int64, error)
    CountByType(ctx context.Context, itemType string) (int64, error)
    GetTypes(ctx context.Context) ([]string, error)
    Close() error
}
```

### Alternatives

| Project                      | Stars | Description                    | Gap                            |
| ---------------------------- | ----- | ------------------------------ | ------------------------------ |
| `thefabric-io/eventsourcing` | 59    | Event sourcing with PostgreSQL | PostgreSQL-only, heavier       |
| `fatherlybre/eventstore`     | ~20   | SQLite event store             | GORM-based, less type-safe     |
| `cr-sqlite`                  | 3.6k  | CRDT-enabled SQLite            | CRDT focus, different use case |
| `sqlc`                       | 13k+  | SQL-to-Go generator            | Tool, not a library            |

### Why NOT Extract

1. **Item Coupling** - Tightly bound to `provider.Item` model
2. **sqlc Generated** - Queries are specific to this schema
3. **SQLite-Specific** - Not a generic storage abstraction
4. **Small Surface** - Interface is simple enough to re-implement

**Better Alternative:** If extraction is needed, create a generic SQLite event store that accepts any JSON payload.

---

## Component 4: Errors Package

### Location

```
pkg/errors/
├── errors.go    # Typed errors
```

### What It Does

- Defines sentinel errors
- Wraps `cockroachdb/errors`

### Key Types

```go
var (
    ErrRateLimited  = errors.New("rate limited")
    ErrInvalidToken = errors.New("invalid token")
    ErrUserNotFound = errors.New("user not found")
    ErrSyncFailed   = errors.New("sync failed")
    ErrStorage      = errors.New("storage error")
)
```

### Why NOT Extract

1. **No Unique Value** - Just wraps `cockroachdb/errors`
2. **Domain-Specific** - Errors are specific to sync use case
3. **Better Alternative** - Use `cockroachdb/errors` directly or `larsartmann/uniflow`

---

## Recommended Action Plan

### Phase 1: Provider Interface Extraction (High Priority)

1. **Create `go-provider` repository**
   - Extract `pkg/provider/` as standalone
   - Add comprehensive documentation
   - Create provider development guide

2. **Update go-localsync**
   - Import `go-provider` as dependency
   - Remove local `pkg/provider/`
   - Update all imports

3. **Build Reference Providers**
   - `go-provider-github` (extract from current)
   - `go-provider-gitlab` (new)
   - `go-provider-jira` (new)

4. **Documentation**
   - Provider development guide
   - Rate limiting best practices
   - Retry configuration guide

### Phase 2: Enhance Provider Interface (Medium Priority)

1. **Add Streaming Support**

   ```go
   type StreamingProvider interface {
       Provider
       FetchStream(ctx context.Context, opts *FetchOptions) <-chan ItemOrError
   }
   ```

2. **Add Batch Support**

   ```go
   type BatchProvider interface {
       Provider
       FetchBatch(ctx context.Context, sources []string) (*FetchResult, error)
   }
   ```

3. **Add Webhook Support**
   ```go
   type WebhookProvider interface {
       Provider
       ParseWebhook(payload []byte) ([]*Item, error)
   }
   ```

### Phase 3: Ecosystem (Low Priority)

1. **Provider Registry** - Community-contributed providers
2. **CLI Generator** - Scaffold new providers
3. **Testing Utilities** - Mock providers, fixtures

---

## Comparison with Similar Projects

| Feature              | go-localsync | cr-sqlite   | thefabric/eventsourcing | OmniSerp           |
| -------------------- | ------------ | ----------- | ----------------------- | ------------------ |
| Language             | Go           | Rust/SQLite | Go                      | Go                 |
| Focus                | Local sync   | CRDT sync   | Event sourcing          | Search abstraction |
| Provider abstraction | Yes          | No          | No                      | Yes                |
| Rate limiting        | Built-in     | No          | No                      | No                 |
| Retry logic          | Built-in     | No          | No                      | No                 |
| Full JSON fidelity   | Yes          | Yes         | Yes                     | No                 |
| SQLite storage       | Yes          | Yes         | PostgreSQL              | No                 |
| Incremental sync     | Yes          | Yes         | Yes                     | No                 |

**Unique Position:** go-localsync is the only Go library combining:

- Generic provider abstraction
- Built-in rate limiting and retry
- Full JSON fidelity
- SQLite storage
- Incremental sync

---

## Conclusion

**Extract:** `pkg/provider/` as `go-provider`

**Keep Integrated:**

- `pkg/sync/` - Core business logic
- `pkg/storage/` - Too coupled to Item model
- `pkg/errors/` - No unique value

**Next Steps:**

1. Create `go-provider` repository
2. Extract provider interface with documentation
3. Build 2-3 reference providers
4. Update go-localsync to use extracted library
