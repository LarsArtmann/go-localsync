# go-cqrs-lite Integration Gap Analysis

**Date:** 2026-05-21
**Scope:** Why go-localsync isn't using go-cqrs-lite for "proper" CQRS event-sourcing
**Context:** Follow-up to sprint 3 status report (`docs/status/2026-05-21_21-29_GO-CQRS-LITE_BEST_USE_SPRINT_COMPLETE.md`)

---

## TL;DR

The answer is **both**: there are library API design friction points that make adoption unnatural, and there are integration gaps where go-localsync reinvents things the library already provides — one of which is a **production correctness bug** (silent event loss in the hand-rolled outbox poller).

---

## 1. Library Design Friction Points

These make adoption harder or unnatural for the sync domain.

### 1.1 `command.Handler` Requires Upfront Aggregate ID

The library's `Command` interface requires:

```go
type Command interface {
    Type() Type
    AggregateID() id.AggregateID    // must be known at command construction
    IdempotencyKey() string
}
```

In go-localsync, the aggregate ID is **computed deterministically** inside the decide function via `AggregateID(source, sourceID)` — it's a SHA256 hash derived from the item's source and source ID. The caller doesn't know it at command construction time.

**Impact:** `command.Dispatcher` can't be used without restructuring the sync flow to pre-compute aggregate IDs. This is why commands are currently implicit closures passed to `Repo.Execute`.

**Possible fix:** The library could offer a `command.DecideFunc` variant where the aggregate ID is returned by the decide function, not required upfront. Or go-localsync could pre-compute the aggregate ID before dispatch.

### 1.2 `DecideFunc` Returns Only Error

```go
type DecideFunc[State any] func(state State, currentVersion event.Version) ([]event.Event, error)
```

The caller has no way to know **what happened** — was an item created, updated, or had a conflict? go-localsync works around this with a `countingDecide` wrapper that reverse-engineers intent by counting emitted events and checking `state.IsNew()`:

```go
countingDecide := func(state SyncItemState, v event.Version) ([]event.Event, error) {
    events, err := DecideSync(item, opts...)(state, v)
    eventCount = len(events)
    return events, err
}
action := classifyAction(err, eventCount, wasNew)
```

This is a code smell. The decider should communicate domain semantics directly.

**Possible fix:** `DecideFunc` could return a richer type, e.g.:

```go
type Decision struct {
    Events  []event.Event
    Action  SyncAction    // Created, Updated, Conflict, Noop
}
type DecideFunc[State any] func(state State, version event.Version) (Decision, error)
```

Or: the decider attaches metadata to events that the caller reads (but this is implicit and fragile).

### 1.3 `NewTypedProjection[T]` Assumes Single Payload Type

```go
func NewTypedProjection[T any](...) (event.Projection, error)
```

Real projections handle **multiple event types with different payload types**. go-localsync's `Projector` handles 3 event types (`ItemSynced`, `ItemConflictFound`, `ItemDeleted`) with 3 different payload structs. The library has no clean composition for this.

**Current workaround:** Manual `Handle()` with a switch on event type — correct but verbose.

**Possible fix:** The library could offer a `ProjectionBuilder` that registers multiple typed handlers:

```go
event.NewProjection("name").
    On[ItemSyncedPayload](EventItemSynced, handleSynced).
    On[ItemDeletedPayload](EventItemDeleted, handleDeleted).
    Build()
```

---

## 2. Integration Gaps (go-localsync's Fault)

These are things the library provides correctly that go-localsync reinvents poorly.

### 2.1 Reinvented `OutboxPublisher` — PRODUCTION BUG

**Severity: HIGH — silent event loss possible**

The library provides `event.OutboxPublisher`:

```go
publisher, _ := event.NewOutboxPublisher(outbox, bus,
    event.WithPollInterval(1*time.Second),
    event.WithBatchSize(100),
)
publisher.Start()
// ...
publisher.Close() // blocks until goroutine exits
```

go-localsync hand-rolls `startOutboxPoller` at `pkg/cqrs/stack.go:328`:

```go
func startOutboxPoller(outbox event.Outbox, bus event.Bus) context.CancelFunc {
    // ...
    go func() {
        ticker := time.NewTicker(time.Millisecond)  // 1ms = busy loop
        for {
            select {
            case <-ticker.C:
                entries, _ := outbox.PollPending(ctx, 100)
                for _, entry := range entries {
                    for _, evt := range entry.Events {
                        _ = bus.Publish(ctx, evt)    // error ignored
                    }
                    _ = outbox.Ack(ctx, []event.OutboxID{entry.ID})  // acks even on publish failure
                }
            }
        }
    }()
    return cancel  // no graceful shutdown wait
}
```

**What's broken:**

| Feature                  | Library `OutboxPublisher`                           | Hand-rolled poller                                        |
| ------------------------ | --------------------------------------------------- | --------------------------------------------------------- |
| Poll interval            | 1s default, configurable                            | 1ms hard-coded (busy loop)                                |
| Batch size               | Configurable via `WithBatchSize()`                  | Hard-coded `100`                                          |
| State machine            | `publisherState` enum (idle/running/closed)         | None                                                      |
| Graceful shutdown        | `Close()` blocks until goroutine exits via `<-done` | `cancel()` returns immediately, goroutine may still run   |
| Panic recovery           | `defer recover()` + stack trace logging             | None — silent goroutine death                             |
| Partial-batch safety     | Only acks successfully published entries            | **Acks unconditionally — events lost on publish failure** |
| Error logging            | `slog.Warn` on failures                             | Silently swallows all errors                              |
| `PublishNow()` for tests | Synchronous one-shot poll→publish→ack               | No equivalent                                             |
| `io.Closer` interface    | Composable with `errors.Join`                       | Returns bare `context.CancelFunc`                         |
| Constructor validation   | Returns `ErrNilOutbox`/`ErrNilBus`                  | Returns nil CancelFunc for nil outbox                     |
| Idempotent Close         | Explicit state check                                | Relies on stdlib cancel being safe twice                  |

**The two critical correctness bugs:**

1. **Acks on publish failure** — if `bus.Publish` fails, the entry is still acked, meaning those events are permanently lost
2. **No graceful shutdown** — `Close()` calls `cancel()` but doesn't wait for the goroutine to finish, so events may be in-flight during shutdown

### 2.2 No Command Types — Implicit Closures

go-localsync has no formal command types at all. "Commands" are closures passed directly to `Repo.Execute`:

```go
err := s.Repo.Execute(ctx, aggID, AggregateTypeSyncItem, func(state SyncItemState, v event.Version) ([]event.Event, error) {
    return DecideSync(item, opts...)(state, v)
})
```

The library's `command.Dispatcher` provides:

- Typed command handlers with compile-time safety (`RegisterTyped[T]`)
- Middleware pipeline (logging, retry, validation, metrics, tracing)
- Idempotency keys for deduplication
- Catalog integration for API documentation
- Structured error taxonomy

**Impact:** No command logging, no retry middleware, no validation, no observability.

### 2.3 Read Model Bypasses `query.Dispatcher`

The `ReadModel` interface is entirely hand-rolled:

```go
type ReadModel interface {
    Get(ctx context.Context, source, sourceID string) (*provider.Item, error)
    List(ctx context.Context, filter ItemFilter) ([]*provider.Item, error)
    Count(ctx context.Context, filter ItemFilter) (int, error)
    // ...
}
```

The library provides `query.Dispatcher` with:

- Typed query handlers (`RegisterTyped[T]`)
- `Pagination` + `PaginatedResult[T]` with `HasNext()`/`HasPrev()`
- Middleware pipeline for logging/metrics/tracing
- Catalog integration

**Impact:** No query logging, no standard pagination, no middleware.

---

## 3. What's Working Well

To be fair, these areas have solid library integration:

| Area                                       | Library Usage                        | Quality   |
| ------------------------------------------ | ------------------------------------ | --------- |
| `decider.Decider[State]` + `Fold`          | Pure function event sourcing         | Excellent |
| `decider.Repository` with `Execute`        | Load→Fold→Decide→Save→Publish        | Excellent |
| `event.JSONCodec` + `DecodePayload[T]`     | Typed payload encoding               | Excellent |
| `event.NewEvents`                          | Event construction                   | Excellent |
| `event.Version` with `Increment()`/`Add()` | No int() casts                       | Excellent |
| `event.InMemoryRunner`                     | Projection replay for memory backend | Good      |
| `projection.Runner`                        | Replay + live subscription for Turso | Good      |
| `SQLiteSnapshotStore` + `EveryNEvents(10)` | Caps replay cost                     | Good      |
| `SQLiteCheckpointStore`                    | Persists projection positions        | Good      |
| `middleware.EventLogging`                  | Structured event logging             | Good      |
| `event.WithCorrelationID`                  | Per-sync-run tracing                 | Good      |
| `decider.WithOutbox`                       | Atomic save+publish                  | Good      |
| Deterministic aggregate IDs                | `id.AggregateID` with SHA256 cache   | Good      |

---

## 4. Recommended Fix Priority

| #   | Fix                                                                 | Type     | Severity | Effort | Impact                           | Status   |
| --- | ------------------------------------------------------------------- | -------- | -------- | ------ | -------------------------------- | -------- |
| 1   | Replace `startOutboxPoller` with `event.OutboxPublisher`            | Bug fix  | **HIGH** | 30min  | Fixes silent event loss          | **DONE** |
| 2   | Return domain result from `DecideFunc` (eliminate `countingDecide`) | Refactor | Medium   | 1h     | Proper domain semantics          | Open     |
| 3   | Wire `command.Dispatcher` with `SyncItems`/`DeleteItem` commands    | Feature  | Medium   | 4h     | Enables middleware pipeline      | **DONE** |
| 4   | Wire `query.Dispatcher` for read model queries                      | Feature  | Low      | 2h     | Standard pagination + middleware | **DONE** |

---

## 5. Library Improvement Opportunities

These are go-cqrs-lite API improvements that would help go-localsync (and similar domains):

1. **`DecideFunc` returns result type** — allow deciders to communicate domain semantics (created/updated/conflict/noop) without event-count heuristics
2. **Deferred aggregate ID in commands** — support domains where the aggregate ID is computed inside the decide function
3. **Multi-type projection builder** — fluent API for registering multiple typed event handlers in a single projection
4. **`Repository.Execute` returns metadata** — version, event count, or a result type from the executed decision

---

_Generated by Crush on 2026-05-21_
