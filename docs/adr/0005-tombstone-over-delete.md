# ADR-0005: Soft-Tombstone over Hard-Delete

**Status:** Accepted
**Date:** 2026-06-28
**Deciders:** Lars Artmann

## Context

Until this decision, the SDK modeled a gone item with a **hard delete**: `DecideDelete`
nilled out the `Item` pointer on `SyncItemState`, the read model row was removed, and an
`ItemDeleted` event was emitted. Re-syncing the same item resurrected it as a "live"
aggregate again.

This had four problems, all rooted in the same lie — that "deleted" means "gone forever":

1. **History was destroyed.** A nil `Item` discards the actor, repo, timestamps, and raw
   JSON the moment the item disappears upstream. The event journal still exists, but the
   aggregate's *current* state — the thing the read model and queries serve — became empty.
2. **"Gone upstream" was indistinguishable from "never fetched".** A paginated pull mirror's
   central question is *"the provider stopped returning this item — should I drop it?"* With
   a hard delete there was no way to record *why* an item vanished, nor to show it quietly
   hidden rather than fully erased.
3. **No reason, no audit.** An item can disappear for different reasons (deleted upstream,
   hidden by the user, redacted). A hard delete has no field for that.
4. **Resurrect lost continuity.** Re-syncing a deleted item re-created state from scratch;
   there was no record it had ever been tombstoned.

## Decision

Replace hard-delete with a **soft tombstone**. The aggregate keeps the `*model.Item` pointer
and records a `Tombstone{Reason, At}` on `Item.Tombstone` instead of niling it out.

The rename surface:

```
DeleteItemCommand   → TombstoneItemCommand   (carries model.TombstoneReason)
EventItemDeleted    → EventItemTombstoned    ("sync_item.tombstoned")
ItemDeletedPayload  → ItemTombstonedPayload  (gains Reason + TombstonedAt)
DecideDelete        → DecideTombstone
SyncItemState       → { Item *model.Item }   (Deleted bool flag removed)
```

Three design rules follow from the data model:

1. **The tombstone lives on `Item.Tombstone`** — a `Tombstone` struct whose zero value means
   "live". `Reason` is the discriminant, so "a reason without being tombstoned" is
   unrepresentable. `IsTombstoned()` delegates to the reason being non-empty.
2. **A tombstone is a separate event, not a field on `ItemSyncedPayload`.** A sync event
   always means "live", so re-syncing a tombstoned item resurrects it for free: the
   projection upsert resets the tombstone columns. No special-case in `DecideSync`, no schema
   bump (V2 stays V2).
3. **Reconciliation is opt-in.** Tombstoning items the provider stopped returning is
   destructive-ish and unsafe during partial/paginated fetches (a not-yet-fetched item would
   be wrongly declared gone). The caller must explicitly set `SyncOptions.Reconcile = true`,
   signalling "this fetch is complete". `SyncStore.Reconcile(ctx, source, seenKeys)` then
   tombstones live items for `source` absent from `seen` with `ReasonUpstreamGone`.

### Read model

- `ItemFilter.IncludeTombstoned` (default `false`) — tombstoned items are excluded from
  `List`/`Count`/`GetTypes` unless explicitly requested, but `Get` still returns them.
- SQLite adds `tombstoned`, `tombstone_reason`, `tombstoned_at` columns via an idempotent
  `migrateSyncItems`; `Upsert` resets them on resurrect.

## Consequences

### Positive

- **History survives deletion.** The full `*model.Item` (actor, repo, raw JSON) is retained
  while hidden; the read model and the event journal stay consistent.
- **Reversible by design.** A tombstone is undone the next time the provider returns the item
  — automatic, no special-case code path.
- **Reasons are explicit and queryable.** `upstream_gone`, `user_hidden`, `redacted` (with
  `ParseTombstoneReason` defaulting unknowns to `upstream_gone`, the safe "hidden" default).
- **Upstream-gone detection is honest.** The reconciliation pass records *why* an item
  disappeared instead of silently dropping it.
- **No schema upcasting needed.** Tombstone is a read-model column + a new event type, not a
  field on `ItemSyncedPayload`, so V2 payloads are unchanged.

### Negative

- Tombstoned rows accumulate; a future purge/archive policy is needed for unbounded sources.
- Reconciliation requires the caller to know whether a fetch was complete — a partial fetch
  with `Reconcile = true` would wrongly tombstone still-present items. This is documented on
  `SyncOptions.Reconcile` and defaults to off.
- The read model carries three extra columns and a filter flag.

### Future

A TTL / explicit-purge job (delete tombstoned rows older than N) is the natural follow-up;
deliberately deferred until the reference consumer (`github-local-sync`) shows a real storage
cost.

## References

- [ADR-0001](0001-cqrs-adoption.md) (CQRS adoption)
- [ADR-0003](0003-crdt-integration.md) (pluggable conflict resolution — the decider that
  emits `ItemConflictFound` and now `ItemTombstoned`)
- Implementation: `pkg/data/model/tombstone.go`, `pkg/cqrs/decider.go` (`decideTombstone`),
  `pkg/cqrs/stack.go` (`TombstoneItem`, `Reconcile`), `pkg/sync/sync.go` (opt-in reconcile)
