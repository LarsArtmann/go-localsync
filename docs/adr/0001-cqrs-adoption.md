# ADR-001: Event-Sourced CQRS Architecture

**Status:** Accepted
**Date:** 2026-05-03
**Deciders:** Lars Artmann

## Context

Go-LocalSync needs to track sync state for items from external providers (GitHub). The system must:

- Detect when items have changed between syncs
- Handle conflicts when the same item is synced multiple times
- Support idempotent operations (same item synced twice = no duplicate)
- Provide a read model for querying synced items
- Persist state across restarts

We evaluated three approaches:

1. **CRUD with upsert**: Simple INSERT OR REPLACE with a last-updated timestamp
2. **Event sourcing with CQRS**: All state changes recorded as events, read model projected from events
3. **CRDT-only**: Conflict-free replicated data types for all state

## Decision

We chose **event-sourced CQRS** via the `go-cqrs-lite` library.

### Rationale

- **Audit trail**: Every sync, conflict, and deletion is recorded as an immutable event. This is essential for debugging sync issues.
- **Idempotency by design**: Deterministic aggregate IDs (SHA256 from source+sourceID) mean the same item always maps to the same aggregate, preventing duplicates naturally.
- **Pluggable conflict resolution**: The decider pattern separates conflict detection from resolution. Any `crdt.ConflictResolver[T]` can be plugged in (LWW, custom merge, etc.).
- **Temporal queries**: Events preserve the full history. We can reconstruct state at any point in time.
- **go-cqrs-lite integration**: We own the library and can evolve it alongside this project.

### Why not CRUD?

- No audit trail — can't reconstruct why a conflict happened
- Conflict detection must be bolted on, not designed in
- No replay capability for read model recovery

### Why not CRDT-only?

- CRDTs require vector clocks for distributed scenarios. We're single-node for now.
- Conflict resolution is already pluggable via the CQRS decider — CRDT resolvers are injected, not the architecture itself.
- The `pkg/crdt/` package exists as a strategy, not the foundation.

## Consequences

### Positive

- All state changes are auditable via events
- Read model can be rebuilt from events at any time
- Conflict detection is in the decider — single source of truth
- Deterministic aggregate IDs prevent duplicates without unique constraints
- Dual backend (memory + SQLite) works identically through the same interfaces

### Negative

- Higher complexity than simple CRUD (~15 files in `pkg/cqrs/` vs ~3 for CRUD)
- Event store grows unbounded (no TTL or compaction yet)
- Learning curve for developers unfamiliar with event sourcing
- Projection lag: read model updates asynchronously via bus subscription

### Mitigations

- Snapshots cap replay cost (`EveryNEvents(10)`)
- `pkg/cqrs/doc.go` explains the architecture
- Memory backend for development, SQLite for production
- Test coverage at 85.9% validates all paths
