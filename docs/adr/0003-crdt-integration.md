# ADR-003: Pluggable CRDT Conflict Resolution

**Status:** Accepted (revised 2026-06-28)
**Date:** 2026-05-29
**Deciders:** Lars Artmann

> **Revision (2026-06-28):** The conflict surface was simplified to match what the SDK
> actually is — a single-writer pull mirror with no causal ordering to track. The
> `ConflictResult[T]` wrapper was removed: `Resolve` now returns the winner directly as
> `(T, error)`. The `Conflict[T]` struct dropped its vector-clock fields (`Local`/`Remote`/
> `Timestamp` only). The unused `VectorClock`, `Operation[T]`, `SyncMessage`, `NodeID`, and
> `OperationID` types were deleted as dead code (see [ADR-0005](0005-tombstone-over-delete.md)
> for the related delete→tombstone pivot). The decision below stands; only the code shape
> changed.

## Context

When the same item is synced multiple times and the data differs, the system must decide which version wins. The default strategy (remote-wins / last-write-wins) is not always appropriate — some use cases need merge strategies or custom logic.

## Decision

We made conflict resolution **pluggable** via the `crdt.ConflictResolver[T]` interface, injected into `CQRSConfig.ConflictResolver`:

```go
type ConflictResolver[T any] interface {
    Resolve(conflict *Conflict[T]) (T, error)
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

- Custom resolvers can't see the full event history — only the two conflicting items
- Error in resolver falls back to remote-wins silently
- No causal-history tracking: a single-writer pull mirror has no second writer to order
  against, so vector clocks were removed rather than left as a lie (see revision note above)

### Future

If a genuine multi-writer/multi-node sync mode is ever added, causal metadata
(vector clocks or hybrid logical clocks) would belong on `Conflict[T]` and a real CRDT
package would be reintroduced. For the current single-writer pull mirror this is explicitly
**not** needed.
