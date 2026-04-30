# CQRS Migration Plan: go-localsync on go-cqrs-lite

## Why

go-localsync currently has a hand-rolled CRUD storage layer (16-method `Storage` interface, SQLite + LibSQL + memory backends, sqlc-generated queries, migration system). This is ~2000 lines of infrastructure code that reimplements what go-cqrs-lite + go-localfirst already provide.

go-localfirst already builds on go-cqrs-lite with Pebble-backed event sourcing. We should too.

**What we gain:**

- Shared `event.Store` (Pebble) instead of 3 duplicated SQL backends
- Event sourcing — full audit trail of every sync operation
- Command/Query separation — clean write/read paths
- Optimistic concurrency via go-cqrs-lite's version tracking
- Reuse go-localfirst's `CQRSAdapter` (Pebble) directly
- Eliminate sqlc, migration system, and `internal/database/` entirely
- Natural conflict detection through aggregate versioning

**What we lose:**

- SQL query flexibility (offset/limit pagination, type/actor/repo filters)
- `internal/db/` sqlc-generated code
- Migration system (`internal/database/`)

These are replaced by Pebble prefix scans and in-memory read model projections.

---

## Target Architecture

```
Provider (GitHub, ...)
    │
    │  FetchAll() → []*provider.Item
    ▼
SyncItem Command Dispatcher
    │
    │  For each fetched item:
    │    1. Load SyncItem aggregate from event store
    │    2. Compare with incoming item data
    │    3. Apply business logic (new / update / conflict)
    │    4. Produce events
    ▼
SyncItem Aggregate (per source+sourceID)
    │
    │  Events: ItemSynced, ItemConflictDetected, ItemDeleted
    │  Embeds aggregate.Core from go-cqrs-lite
    ▼
event.Store (Pebble or Memory)
    │
    │  Persisted events — source of truth
    ▼
event.Bus (MemoryBus)
    │
    │  Publishes events to subscribers
    ▼
Read Model Projection
    │
    │  Subscribes to events, maintains current item state
    │  Backed by Pebble (key: "item:{source}:{sourceID}", value: JSON)
    ▼
Query Dispatcher
    │
    │  GetItem, ListItems, CountItems, GetItemTypes
    │  All read from the projected read model
    ▼
Caller
```

---

## New Package Structure

```
pkg/
  provider/              # UNCHANGED — Item, Provider interface
  providers/github/      # UNCHANGED — GitHub provider
  types/                 # UNCHANGED — branded IDs
  errors/                # UNCHANGED — sentinel errors
  testhelpers/           # UNCHANGED — shared test utilities

  cqrs/                  # NEW — CQRS integration layer
    aggregate/
      sync_item.go       # SyncItem aggregate (embeds aggregate.Core)
    commands/
      sync_item.go       # SyncItemCommand + handler
      delete_item.go     # DeleteItemCommand + handler
    queries/
      get_item.go        # GetItem query + handler
      list_items.go      # ListItems query + handler
      count_items.go     # CountItems query + handler
    events/
      types.go           # Event type constants + payload structs
    projection/
      read_model.go      # Read model projection (subscribes to events)
      pebble_store.go    # Pebble-backed read model storage
      memory_store.go    # In-memory read model storage

  sync/                  # REFACTORED — uses CQRS internally
    syncer.go            # Syncer dispatches SyncItem commands
    conflict_aware.go    # ConflictAwareSyncer — same, but through CQRS

internal/                # DELETED entirely
  database/              # ← gone (no SQL migrations)
  db/                    # ← gone (no sqlc)
sql/                     # ← gone (no SQL queries or migrations)
```

---

## Key Design Decisions

### 1. One Aggregate Per Sync Item

Each `(source, sourceID)` pair is its own aggregate with its own event stream.

```
AggregateType: "sync_item"
AggregateID:   derived from source + sourceID (e.g. "github:12345")
```

This gives us:

- Per-item version tracking (optimistic concurrency for free)
- Per-item event history (full audit trail)
- Natural conflict detection (version mismatch = concurrent edit)

### 2. Event Types

```go
const (
    EventItemSynced          event.Type = "sync_item.synced"          // new or updated item
    EventItemConflictFound   event.Type = "sync_item.conflict_found"  // conflict detected + resolved
    EventItemDeleted         event.Type = "sync_item.deleted"         // item removed
)
```

**Payload structs** (JSON-encoded `[]byte`):

```go
type ItemSyncedPayload struct {
    Source         string `json:"source"`
    SourceID       string `json:"source_id"`
    Type           string `json:"type"`
    ActorLogin     string `json:"actor_login,omitempty"`
    ActorAvatarURL string `json:"actor_avatar_url,omitempty"`
    RepoName       string `json:"repo_name,omitempty"`
    RepoURL        string `json:"repo_url,omitempty"`
    CreatedAt      int64  `json:"created_at"`      // unix nano
    UpdatedAt      int64  `json:"updated_at"`      // unix nano
    RawJSON        []byte `json:"raw_json"`
}
```

### 3. SyncItem Aggregate

```go
type SyncItem struct {
    *aggregate.Core

    // Current state (rebuilt from events)
    source         string
    sourceID       string
    itemType       string
    actorLogin     string
    actorAvatarURL string
    repoName       string
    repoURL        string
    createdAt      time.Time
    updatedAt      time.Time
    rawJSON        []byte
    deleted        bool
}

// Sync processes an incoming item against current state.
// Produces ItemSynced or ItemConflictFound events.
func (s *SyncItem) Sync(ctx context.Context, item *provider.Item) error { ... }

// Delete marks the item as deleted.
func (s *SyncItem) Delete(ctx context.Context) error { ... }

// Apply rebuilds state from events.
func (s *SyncItem) Apply(evt event.Event) error { ... }
```

### 4. Read Model Projection

The projection subscribes to `event.Bus` and maintains current item state in a KV store.

**Pebble-backed read model** (production):

```
Key:   "item:{source}:{sourceID}"
Value: JSON(provider.Item)

Key:   "item_type:{itemType}"
Value: ""  (exists = item type present)

Key:   "item_source:{source}"
Value: ""  (exists = source present)
```

Queries use Pebble prefix scans:

- `GetItem(source, sourceID)` → point lookup
- `ListItems(limit, offset)` → prefix scan `item:` with skip/take
- `ListItemsByType(type, limit, offset)` → prefix scan `item:` filtered
- `CountItems()` → count keys with prefix `item:`
- `GetTypes()` → scan `item_type:` prefix

**In-memory read model** (testing):

- `map[string]*provider.Item` with `sync.RWMutex`
- Same query methods, just backed by a map

### 5. Batch Sync Optimization

The current `Syncer` fetches hundreds of items and does `UpsertBatch`. With CQRS, we dispatch one `SyncItemCommand` per item. To handle bulk efficiently:

1. **Command handler** loads aggregate, applies business logic, saves events — all per-item.
2. **Projection** subscribes to bus and batches writes to read model.
3. **No transaction boundary needed** across items — each item is its own aggregate.

For the initial bulk sync (first run), we use `event.Store.AppendBatch` to skip optimistic concurrency checks (all items are new, version 0).

### 6. Reuse go-localfirst's CQRSAdapter

The Pebble `event.Store` adapter at `go-localfirst/internal/cqrs/store/pebble_adapter.go` is not imported — it lives in `internal/`. Two options:

**Option A (recommended): Extract to shared package**

- Move `CQRSAdapter` to `go-cqrs-lite/event/pebble_store.go` (or a new `go-cqrs-lite/store/pebble/` package)
- Both go-localfirst and go-localsync import it
- go-cqrs-lite already has `BackendMemory`; add `BackendPebble`

**Option B: Copy into go-localsync**

- Copy the adapter into `pkg/cqrs/store/pebble_adapter.go`
- Simpler, no cross-project changes
- But duplicates code with go-localfirst

---

## Migration Steps

### Phase 1: Foundation (no breaking changes)

1. **Add go-cqrs-lite dependency**
   - `go.mod`: add `github.com/larsartmann/go-cqrs-lite`
   - Update `go.work` to include `../go-cqrs-lite`

2. **Create `pkg/cqrs/events/types.go`**
   - Event type constants
   - Payload structs (`ItemSyncedPayload`, `ItemConflictFoundPayload`, `ItemDeletedPayload`)

3. **Create `pkg/cqrs/aggregate/sync_item.go`**
   - `SyncItem` struct embedding `*aggregate.Core`
   - `Sync()` method with conflict detection logic (migrated from `ConflictAwareSyncer`)
   - `Apply()` method (switch on event types, rebuild state)
   - `Delete()` method

4. **Create `pkg/cqrs/commands/`**
   - `SyncItemCommand` (embeds `command.Core`, carries `provider.Item`)
   - `DeleteItemCommand` (embeds `command.Core`, carries item ID)
   - Handlers that load aggregate → call business method → save via `aggregate.Repository`

5. **Create `pkg/cqrs/projection/`**
   - `ReadModel` interface (matches current query needs: Get, List, Count, GetTypes)
   - `PebbleReadModel` implementation
   - `MemoryReadModel` implementation
   - `Projector` that subscribes to `event.Bus` and updates read model

6. **Create `pkg/cqrs/queries/`**
   - `GetItemQuery` + handler (reads from `ReadModel`)
   - `ListItemsQuery` + handler (supports filter by type/actor/repo/source/since)
   - `CountItemsQuery` + handler

7. **Create `pkg/cqrs/store/`**
   - Pebble `event.Store` adapter (copy or extract from go-localfirst)
   - Or import from go-cqrs-lite if Option A is chosen

### Phase 2: Wire CQRS into sync

8. **Refactor `pkg/sync/syncer.go`**
   - Accept `command.Dispatcher` + `query.Dispatcher` instead of `storage.Storage`
   - `Sync()`: fetch items → dispatch `SyncItemCommand` for each
   - `SyncIncremental()`: query latest from read model → fetch → dispatch commands
   - `GetStats()`: query from read model via query dispatcher

9. **Refactor `pkg/sync/conflict_aware.go`**
   - Conflict detection moves into `SyncItem.Sync()` aggregate method
   - `SyncWithConflictDetection()`: same flow, but conflicts are now aggregate-level events
   - Result comes from counting events produced (not from comparing items)

### Phase 3: Update consumers

10. **Update `cmd/examples/github-sync/main.go`**
    - Wire: `event.Store` → `event.Bus` → `aggregate.Repository` → `command.Dispatcher` → `query.Dispatcher` → `Syncer`
    - CLI flags: `--backend pebble` (default) or `--backend memory`
    - Remove SQLite/libSQL references

11. **Update tests**
    - `pkg/storage/compliance_test.go` → becomes `pkg/cqrs/projection/compliance_test.go`
    - All tests use `MemoryStore` + `MemoryBus` + `MemoryReadModel`
    - BDD tests updated to use CQRS wiring

### Phase 4: Cleanup

12. **Delete old packages**
    - `internal/database/` — gone
    - `internal/db/` — gone
    - `sql/` — gone
    - `pkg/storage/sqlite.go` — gone
    - `pkg/storage/libsql.go` — gone
    - `pkg/storage/memory_storage.go` — replaced by `MemoryReadModel`
    - `pkg/storage/interface.go` — replaced by `ReadModel` interface
    - `pkg/storage/config.go` — replaced by CQRS wiring
    - Remove `modernc.org/sqlite`, `libsql-client-go`, `sqlc` dependencies

13. **Update `AGENTS.md`** with new architecture

---

## Interface Mapping (Old → New)

| Old (CRUD)                                  | New (CQRS)                                                          |
| ------------------------------------------- | ------------------------------------------------------------------- |
| `storage.Storage.Upsert(item)`              | `command.Dispatcher.Dispatch(SyncItemCommand{item})`                |
| `storage.Storage.UpsertBatch(items)`        | Loop: dispatch `SyncItemCommand` per item                           |
| `storage.Storage.GetByID(id)`               | `query.Dispatcher.Dispatch(GetItemQuery{id})`                       |
| `storage.Storage.GetLatest()`               | `query.Dispatcher.Dispatch(ListItemsQuery{Limit:1})`                |
| `storage.Storage.GetItems(limit, offset)`   | `query.Dispatcher.Dispatch(ListItemsQuery{Limit,Offset})`           |
| `storage.Storage.GetItemsByType(t, l, o)`   | `query.Dispatcher.Dispatch(ListItemsQuery{Filter:TypeFilter{t}})`   |
| `storage.Storage.GetItemsByActor(a, l, o)`  | `query.Dispatcher.Dispatch(ListItemsQuery{Filter:ActorFilter{a}})`  |
| `storage.Storage.GetItemsByRepo(r, l, o)`   | `query.Dispatcher.Dispatch(ListItemsQuery{Filter:RepoFilter{r}})`   |
| `storage.Storage.GetItemsBySource(s, l, o)` | `query.Dispatcher.Dispatch(ListItemsQuery{Filter:SourceFilter{s}})` |
| `storage.Storage.GetItemsSince(t)`          | `query.Dispatcher.Dispatch(ListItemsQuery{Filter:SinceFilter{t}})`  |
| `storage.Storage.BatchGetByIDs(ids)`        | `query.Dispatcher.Dispatch(BatchGetItemsQuery{ids})`                |
| `storage.Storage.Delete(id)`                | `command.Dispatcher.Dispatch(DeleteItemCommand{id})`                |
| `storage.Storage.DeleteAll()`               | Iterate + delete (or dedicated command)                             |
| `storage.Storage.Count()`                   | `query.Dispatcher.Dispatch(CountItemsQuery{})`                      |
| `storage.Storage.CountByType(t)`            | `query.Dispatcher.Dispatch(CountItemsQuery{Filter:TypeFilter{t}})`  |
| `storage.Storage.GetTypes()`                | `query.Dispatcher.Dispatch(GetItemTypesQuery{})`                    |
| `storage.Storage.Close()`                   | Close event store + read model                                      |

---

## Read Model Interface

```go
type ReadModel interface {
    Get(ctx context.Context, source, sourceID string) (*provider.Item, error)
    List(ctx context.Context, filter ItemFilter) ([]*provider.Item, error)
    Count(ctx context.Context, filter ItemFilter) (int64, error)
    GetTypes(ctx context.Context) ([]string, error)
    Upsert(ctx context.Context, item *provider.Item) error
    Delete(ctx context.Context, source, sourceID string) error
    Close() error
}

type ItemFilter struct {
    Type       *string
    ActorLogin *string
    RepoName   *string
    Source     *string
    Since      *time.Time
    Limit      int
    Offset     int
}
```

This replaces the 16-method `Storage` interface with a cleaner 7-method interface that uses a single `ItemFilter` struct for all queries — exactly the pattern go-localfirst uses with `TodoFilter`.

---

## Dependencies (Old → New)

| Old                                   | New                                                                   |
| ------------------------------------- | --------------------------------------------------------------------- |
| `modernc.org/sqlite`                  | **removed**                                                           |
| `tursodatabase/libsql-client-go`      | **removed**                                                           |
| sqlc (build tool)                     | **removed**                                                           |
| `github.com/larsartmann/go-cqrs-lite` | **added**                                                             |
| `github.com/cockroachdb/pebble`       | **added** (via go-cqrs-lite or direct)                                |
| `go.uber.org/zap`                     | **added** (for Pebble adapter logging, consistent with go-localfirst) |
| `google/go-github/v69`                | unchanged                                                             |
| `cockroachdb/errors`                  | unchanged                                                             |
| `charm.land/log/v2`                   | unchanged                                                             |

---

## Risk Mitigation

1. **Read model rebuild**: If the read model gets out of sync, replay all events from the event store. This is a standard CQRS recovery operation.

2. **Migration from SQLite**: One-time script that reads existing SQLite data and produces initial events via `AppendBatch`. Or just re-sync from scratch (providers are the source of truth anyway).

3. **Performance**: Pebble is an LSM tree optimized for writes. Batch sync writes will be faster than SQLite transactions. Read queries use prefix scans which are O(k) not O(n).

4. **Test continuity**: Compliance tests are rewritten against `ReadModel` interface, covering the same behavioral contracts.

---

## Estimated Effort

| Phase                     | Scope                   | Effort |
| ------------------------- | ----------------------- | ------ |
| Phase 1: Foundation       | 7 new files, ~800 lines | Medium |
| Phase 2: Wire sync        | 2 files refactored      | Medium |
| Phase 3: Update consumers | 1 CLI + test rewrites   | Medium |
| Phase 4: Cleanup          | Delete ~2000 lines      | Easy   |
