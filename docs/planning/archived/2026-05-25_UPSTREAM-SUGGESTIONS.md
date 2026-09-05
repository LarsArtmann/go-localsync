# Upstream Suggestions for go-cqrs-lite

**Date:** 2026-05-25\
**From:** go-localsync team (primary consumer)\
**To:** go-cqrs-lite maintainers

---

## Context

go-localsync is the deepest consumer of go-cqrs-lite, using 5 of 12 modules at ~85% API surface coverage. These suggestions are derived from real consumer friction.

---

## 1. Add `StorageStack` Factory

**Problem:** Every consumer writes the same boilerplate to wire store + bus + outbox + snapshot + checkpoint.

**Current consumer code (~200 LOC in go-localsync):**

```go
func createStoreAndBus(cfg Config) (storeResult, error) {
    switch cfg.Backend {
    case "memory":
        return storeResult{
            store:  memory.NewMemoryStore(),
            bus:    memory.NewMemoryBus(),
            outbox: nil,
            db:     nil,
            loader: nil,
        }, nil
    case "turso":
        // 50+ lines of Turso-specific wiring
    }
}
```

**Suggestion:** Provide a `storage.NewStack(cfg)` or `storage.Builder` that returns a pre-wired struct:

```go
type Stack struct {
    Store     event.Store
    Bus       event.Bus
    Outbox    event.Outbox
    DB        *sql.DB
    Loader    event.GlobalLoader
}
```

**Priority:** HIGH — eliminates the #1 consumer boilerplate

---

## 2. Add Runner Wiring Helpers

**Problem:** Starting projection runners, outbox publishers, and in-memory runners requires copy-pasted goroutine management.

**Current consumer code (~80 LOC in go-localsync):**

```go
func startProjectionRunner(loader, bus, checkpointStore, proj) (cancelFunc, error) {
    runner, err := projection.NewRunner(loader, bus, checkpointStore)
    // ... register, run in goroutine, return cancel
}

func startOutboxPublisher(outbox, bus) (*event.OutboxPublisher, error) {
    publisher, err := event.NewOutboxPublisher(outbox, bus)
    // ... start, return
}
```

**Suggestion:** Add package-level helpers:

```go
// projection.StartRunner(ctx, loader, bus, cp, proj) → cancel, error
// event.StartOutboxPublisher(outbox, bus) → *OutboxPublisher, error
// event.StartInMemoryRunner(bus, cp, proj) → error
```

**Priority:** HIGH

---

## 3. Export `TimestampNano` / `FromTimestampNano` Helpers

**Problem:** Unix-nanosecond serialization of `time.Time` in event payloads is universal. Every consumer reinvents it.

**Current consumer code (go-localsync):**

```go
func unixNano(t time.Time) int64 {
    if t.IsZero() { return 0 }
    return t.UnixNano()
}
func fromUnixNano(n int64) time.Time {
    if n == 0 { return time.Time{} }
    return time.Unix(0, n)
}
```

**Suggestion:** Add to `core/event/`:

```go
func TimestampNano(t time.Time) int64
func FromTimestampNano(n int64) time.Time
```

**Priority:** LOW — small but universally needed

---

## 4. Add `CharmLogAdapter` to `middleware/`

**Problem:** Consumers using `charm.land/log/v2` need an adapter for `middleware.Logger`.

**Current consumer code:**

```go
type charmLogAdapter struct{ logger *log.Logger }
func (a *charmLogAdapter) Info(msg string, keyvals ...any) { ... }
func (a *charmLogAdapter) Error(msg string, keyvals ...any) { ... }
```

**Suggestion:** Either add `middleware/charm/` sub-module or document the adapter pattern.

**Priority:** LOW — only affects charm.land/log users

---

## 5. Improve `ireturn` Linter Guidance for Factories

**Problem:** `ireturn` linter flags factory functions that legitimately return interfaces. Consumers must add `//nolint:ireturn`.

**Current consumer code:**

```go
//nolint:ireturn
func createReadModel(cfg Config, sr storeResult) (ReadModel, error) {
```

**Suggestion:** Document that factory functions (returning different concrete types based on config) are valid exceptions. Consider adding `factory` or `builder` to the `ireturn` allow-list in project templates.

**Priority:** LOW — documentation fix

---

## Summary

| # | Suggestion                 | Priority | Effort        |
| - | -------------------------- | -------- | ------------- |
| 1 | `StorageStack` factory     | HIGH     | Medium        |
| 2 | Runner wiring helpers      | HIGH     | Low           |
| 3 | `TimestampNano` helpers    | LOW      | Trivial       |
| 4 | `CharmLogAdapter`          | LOW      | Low           |
| 5 | `ireturn` factory guidance | LOW      | Documentation |

---

## Resolution (2026-09-05 docs-health sweep)

Suggestion #1 (consumer boilerplate) was answered by go-cqrs-lite's own stack/ layer; the rest remain upstream correspondence, superseded by the v4 module split. Verified against the 2026-09-05 tree (`9625b1b`: v0.5.0, 309 core tests, CI green). Report fully resolved → archived.
