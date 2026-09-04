# ADR-0004: Multi-Aggregate Generalisation Deferred — go-cqrs-lite Remains the Sharing Boundary

**Status:** Accepted
**Date:** 2026-06-27
**Deciders:** Lars Artmann

> **Update (2026-06-28):** This review was performed before the delete→tombstone pivot
> ([ADR-0005](0005-tombstone-over-delete.md)). The findings below are left as a historical
> record of the state _at review time_: the event vocabulary is now `synced` /
> `conflict_found` / `tombstoned`, and `SyncItemState` carries the tombstone on `Item` (no
> `Deleted bool`). The scope decision itself is unchanged — the SDK remains a single-aggregate,
> pull-only, flat-Item sync engine.

## Context

An adoption review ([`docs/feedback/2026-06-23_discordsync-adoption-feedback.html`](../feedback/2026-06-23_discordsync-adoption-feedback.html)) evaluated whether [DiscordSync](https://github.com/larsartmann/discordsync) — a push-driven, multi-aggregate event-sourcing project — could adopt go-localsync as its consumer SDK. Both projects already share `go-cqrs-lite v3` as their CQRS foundation.

The review found go-localsync is a **well-engineered single-aggregate sync SDK** for its actual domain (paginated pull of flat items from one REST source into one event-sourced read model), but is **not viable for DiscordSync as-is**. Six structural gaps were identified, all verified against current source:

| # | Gap                                                                                                                                   | Severity            | Verified at                                       |
| - | ------------------------------------------------------------------------------------------------------------------------------------- | ------------------- | ------------------------------------------------- |
| 1 | Single hard-coded aggregate type (`aggregateType = "sync_item"`, `SyncItemState{Item, Deleted}`)                                      | Blocking · Core     | `pkg/cqrs/events.go:10`, `pkg/cqrs/decider.go:17` |
| 2 | Fixed three-event vocabulary (`sync_item.synced/conflict_found/deleted`)                                                              | Blocking · Core     | `pkg/cqrs/events.go:12-19`                        |
| 3 | Single flat projection (`"sync_item_projection"`, hard-coded 3-event switch)                                                          | Blocking · Core     | `pkg/cqrs/projection.go:24-44`                    |
| 4 | Pull-only ingestion (`Provider.Fetch`/`FetchAll` pagination, no push path)                                                            | Blocking · Core     | `pkg/provider/provider.go`, `pkg/sync/sync.go`    |
| 5 | GitHub-shaped vocabulary baked into the core (`ActorLogin`, `RepoName`, `RepoURL` in `model.Item`; `ActorLogin`/`RepoID` branded IDs) | Important · Surface | `pkg/data/model/item.go:17-29`, `pkg/id/ids.go`   |
| 6 | Narrow read query surface (`ItemReader` = 4 methods: `List`/`Count`/`CountByType`/`GetTypes`) vs DiscordSync's 28-method `Database`   | Important · Surface | `pkg/data/model/item.go:47-52`                    |

Findings 1–4 are individually blocking and collectively reinforcing: fixing any one without the others still leaves a single-aggregate or single-projection straitjacket. The review's own conclusion is that closing the gap is a **fundamental rewrite** of the SDK's core, and that **`go-cqrs-lite v3` is the correct sharing boundary today** — both consumers already use it directly and successfully.

## Decision

We **defer** multi-aggregate generalisation. go-localsync remains a focused **single-aggregate Item sync SDK**, and **`go-cqrs-lite v3` remains the sharing boundary** between go-localsync and DiscordSync (and any other multi-aggregate consumer). DiscordSync will continue to use go-cqrs-lite directly rather than adopting go-localsync.

Concretely:

- The single `sync_item` aggregate, three-event vocabulary, single projection, and pull-only `Provider`+`Syncer` model are **accepted as the SDK's intended scope**, not defects to fix now.
- The SDK's documented value proposition is narrowed and made honest: it is the best library for _paginated pull of flat items from a REST source into an event-sourced, idempotent, conflict-resolving read model_ (its reference consumer being [`github-local-sync`](https://github.com/larsartmann/github-local-sync)).
- No code changes result from this ADR. This decision **records** the strategic position so the feedback is not lost and the scope is not silently drifted by a future contributor trying to "fix" findings 1–4.

## Consequences

### Positive

- go-localsync keeps doing one thing well; no risky rewrite of a working, 225-test-green SDK
- `github-local-sync` (the real consumer) is not destabilised by a core re-architecture
- The shared foundation (`go-cqrs-lite v3`) is exactly the right granularity for cross-project reuse
- Engineering effort is freed for SDK-internal improvements that benefit the real consumer (observability, schema upcasters, CI)

### Negative

- DiscordSync cannot adopt go-localsync; it stays on direct go-cqrs-lite
- The SDK's framework-level ambition (a reusable multi-aggregate event-sourcing layer) is explicitly set aside
- Findings 5 (GitHub-shaped vocabulary) and 6 (narrow query surface) remain — the abstraction is still extracted from one consumer rather than designed from a generic domain

### What would change this decision

This is a **reversible, deferrable** decision. It should be revisited if **any** of:

1. A **third or more consumer** emerges needing multi-aggregate generalisation — then a shared framework earns its weight.
2. `go-cqrs-lite` itself proves unable to evolve the multi-aggregate/projection-registry ergonomics both consumers want — then go-localsync becomes the natural home for that layer.
3. Evidence accumulates that the single-aggregate scope is actively costing the reference consumer (`github-local-sync`).

If revisited, the rewrite must address findings 1–4 **together** (aggregate registry, consumer-defined events, N projections, push+pull ingestion); partial fixes do not cross the viability line. Finding 5 (vocabulary) and 6 (query surface) should then be addressed by making the domain model **consumer-defined** — the SDK ships only the write-side machinery (event store, bus, projection wiring, idempotency, CRDT) and the optional `Provider`+`Syncer` pull adapter.

## References

- Adoption feedback (source): [`docs/feedback/2026-06-23_discordsync-adoption-feedback.html`](../feedback/2026-06-23_discordsync-adoption-feedback.html)
- Prior data-module audit (superseded — describes a `pkg/data/{query,transform,repo}` layer that was never built; its only surviving actionable items — observability, schema upcasters — are tracked in `TODO_LIST.md`): [`docs/brainstorming/data-module.html`](../brainstorming/data-module.html)
- Related: [ADR-0001](0001-cqrs-adoption.md) (CQRS adoption), [ADR-0003](0003-crdt-integration.md) (pluggable conflict resolution)
