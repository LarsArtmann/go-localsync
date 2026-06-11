# ADR-003: Pluggable CRDT Conflict Resolution

**Status:** Accepted
**Date:** 2026-05-29
**Deciders:** Lars Artmann

## Context

When the same item is synced multiple times and the data differs, the system must decide which version wins. The default strategy (remote-wins / last-write-wins) is not always appropriate — some use cases need merge strategies or custom logic.

## Decision

We made conflict resolution **pluggable** via the `crdt.ConflictResolver[T]` interface, injected into `CQRSConfig.ConflictResolver`:

```go
type ConflictResolver[T any] interface {
    Resolve(conflict *Conflict[T]) (*ConflictResult[T], error)
}
```

When `nil` (default), the system uses **remote-wins LWW** — the incoming item always overwrites local state. When configured, any resolver implementing the interface can be injected.

The `LWWResolver[T]` is the provided implementation, comparing timestamps:

```go
resolver, _ := crdt.NewLWWResolver[*model.Item](func(i *model.Item) time.Time {
    return i.UpdatedAt
})
```

## Consequences

### Positive

- Default behavior (remote-wins) is preserved — zero-config backward compatibility
- Any custom merge strategy can be injected without changing the decider
- `ActionConflictLocal` and `ActionConflictRemote` distinguish which side won
- The CRDT package is a real dependency of the CQRS layer (not just a standalone library)
- CLI flag `--conflict-strategy lww` enables LWW resolution at runtime

### Negative

- Vector clocks are currently empty in conflict payloads (not yet tracking causal history)
- Custom resolvers can't see the full event history — only the two conflicting items
- Error in resolver falls back to remote-wins silently

### Future

When multi-node sync is implemented, vector clocks will be populated and the CRDT package's `VectorClock`, `Operation[T]`, and `SyncMessage` types will be used for causal tracking and protocol serialization.
