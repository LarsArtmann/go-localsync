# ADR-0006: Adopt projectionhost/v3 for managed projection catch-up

**Status:** Accepted
**Date:** 2026-06-29
**Deciders:** Lars Artmann
**Supersedes:** The "no checkpoint store" stance in the runner.go / AGENTS.md projection description (informed by go-cqrs-lite ADR-0037, which moved the `Projection` interface to its own module but did not mandate a host).

## Context

The projection layer has two jobs:

1. **Live delivery** — when a command commits an event, the read model must update synchronously so the caller gets read-your-writes semantics. This is handled by `bus.SubscribeAll(proj.Handle)` (watermill `EventBus` with `BlockPublishUntilSubscriberAck`).
2. **Catch-up** — when a process restarts against a persistent store (SQLite), it must replay historical events into the read model. Until now this was handled by `runner.go`'s `replayJournal`: a background goroutine that calls `Journal.ReadAll` (loading **every** event into memory), filters by `EventTypes()`, and forwards each to `proj.Handle`.

The catch-up implementation has three production-grade gaps:

1. **No checkpoint.** Every restart re-reads the **entire** journal. As the event log grows, restart time grows linearly. A 100k-event store pays the full 100k read + decode + project on every boot.
2. **No poison-message tolerance.** A single event whose payload fails to decode (schema drift, corruption) makes `proj.Handle` return an error. `replayJournal` logs it and **continues**, but the same event will fail again on the next restart — there is no way to skip it permanently.
3. **Silent crash death.** `replayJournal` is a one-shot goroutine. A panic inside `proj.Handle` kills the goroutine; the projection silently stops catching up with no restart, no alert, no `Status()` to inspect.

go-cqrs-lite v3.4 ships `projectionhost/v3`, a module purpose-built to close exactly these gaps. Its doc opens: _"This is the 'last loop every consumer rewrites'"_.

## Decision

Replace `runner.go`'s hand-rolled `replayJournal` with `projectionhost.Host` for the **catch-up** half of projection. **Keep `bus.SubscribeAll` for live delivery** — `projectionhost` is explicitly a batch-drainer, not a live stream consumer (its doc says to pair it with a live subscriber for continuous tailing).

### What projectionhost adds

| Capability                                                                | projectionhost                       | Old replayJournal                    |
| ------------------------------------------------------------------------- | ------------------------------------ | ------------------------------------ |
| Per-projection workers (goroutine each)                                   | ✅                                   | ❌ (single global)                   |
| **Checkpoint persistence** via `event.CheckpointStore`                    | ✅ bounded replay from last position | ❌ reads all events every restart    |
| **Crash auto-restart** with exponential backoff + jitter                  | ✅                                   | ❌ silent goroutine death            |
| **Dead-letter queue** for poison messages (checkpoint advances past them) | ✅                                   | ❌ bad event blocks catch-up forever |
| DLQ replay (`host.ReplayDeadLetters`)                                     | ✅                                   | ❌                                   |
| Health/liveness (`host.Status()` → `[]WorkerState`)                       | ✅                                   | ❌                                   |
| Graceful drain (`host.Stop()`, 30s timeout)                               | ✅                                   | ❌ abrupt `cancel()`                 |

### The architectural reversal

This decision **re-introduces a checkpoint store**, reversing the earlier "the projection is idempotent, so no checkpoint store is needed" stance. The tradeoff:

- **Before:** simpler, no checkpoint schema, but replays the entire journal on every restart and dies silently on poison events.
- **After:** replays only after the last checkpoint (bounded), survives poison events, survives worker crashes — at the cost of one extra table (`projection_checkpoints`) and a `CheckpointStore` dependency per backend.

The idempotent projection is still idempotent — overlap between live delivery and catch-up is still harmless. The checkpoint is an **optimization and resilience boundary**, not a correctness requirement. It bounds work and survives failure; it does not change the projection contract.

### Per-backend checkpoint stores

- **SQLite:** `storage.NewSQLiteCheckpointStore(db)` (creates `projection_checkpoints` table).
- **Memory:** `memory.NewMemoryCheckpointStore()` (in-memory map; checkpoints lost on restart, which is correct — the memory store itself is ephemeral).

### Wiring

`startProjectionRunner` now:

1. Subscribes the projection to the bus for live delivery (unchanged).
2. Constructs a `projectionhost.Host` with the backend's `SeekableJournal` + `CheckpointStore`.
3. Registers the projection and calls `host.Start(ctx)` in a background goroutine.
4. Returns a `cancelRunner` that calls `host.Stop()` for graceful drain.

The `CQRSStack.Close()` path now stops the host (graceful drain) before closing the read model.

## Consequences

### Positive

- **Bounded restart cost.** A store with N events and a checkpoint at position K replays only `N - K` events on restart, not all N.
- **Poison-message survival.** A malformed event is captured to the DLQ after the retry threshold; the checkpoint advances; catch-up continues. `ReplayDeadLetters` re-feeds it after a fix ships.
- **Crash resilience.** A panicking projection handler triggers a worker restart with backoff, not silent death.
- **Observability hook.** `host.Status()` exposes per-projection worker state for future health endpoints.
- **Less hand-rolled infra.** Deletes ~40 lines of `replayJournal`; replaces with a battle-tested library component that the go-cqrs-lite ecosystem maintains.

### Negative

- **One extra table** (`projection_checkpoints`) in the SQLite schema, created by `NewSQLiteCheckpointStore`.
- **One extra dependency** (`projectionhost/v3`) + its transitive deps.
- **The "no checkpoint store" elegance is gone.** A future reader who saw the old comment would need this ADR to understand why a checkpoint store now exists despite the projection being idempotent.

### Neutral

- Live delivery semantics are unchanged (`bus.SubscribeAll` stays).
- The projection implementation (`Projector.Handle`) is unchanged — it still must be idempotent.

## Alternatives considered

1. **Keep replayJournal, add a checkpoint inline.** Rejected — reimplements what `projectionhost` already provides, worse (no DLQ, no restart, no Status).
2. **Use `watermill.CatchUpSubscriber` for both live + catch-up.** Rejected — it replaces the live bus subscription, which would change the read-your-writes contract and require re-encoding the custom ReadModel. Keeping `SubscribeAll` for live is lower-risk.
3. **Poll periodically by re-calling `host.Start` on a fresh Host.** Rejected for now — `projectionhost` workers exit when caught up, so polling is needed for continuous operation. The live bus already covers new events; the host only needs to run once at boot for catch-up. If the live bus is ever dropped, this becomes the fallback.

## References

- go-cqrs-lite `projectionhost/v3` `doc.go` ("the last loop every consumer rewrites")
- go-cqrs-lite `event.SeekableJournal`, `event.CheckpointStore` interfaces
- [ADR-0001](0001-cqrs-adoption.md) (CQRS adoption)
- Implementation: `pkg/cqrs/runner.go` (rewired), `pkg/cqrs/store_factory.go` (checkpoint store creation), `pkg/cqrs/stack.go` (Close drains host)
