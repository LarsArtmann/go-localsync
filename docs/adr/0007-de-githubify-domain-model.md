# ADR-0007: De-GitHubify the Domain Model

**Status:** Accepted
**Date:** 2026-07-05
**Deciders:** Lars Artmann

## Context

The adoption feedback ([`docs/feedback/2026-06-23_discordsync-adoption-feedback.html`](../feedback/2026-06-23_discordsync-adoption-feedback.html), Finding 5) and [ADR-0004](0004-multi-aggregate-generalisation-deferred.md) both identify the same surface-level problem: `model.Item` carries GitHub-specific fields (`ActorLogin`, `ActorAvatarURL`, `RepoName`, `RepoURL`) and the SDK defines GitHub-specific branded types (`id.ActorLogin`, `id.RepoID`). An SDK named "localsync" that claims provider-agnosticism but ships repo-name columns signals the abstraction was extracted from one consumer, not designed from the domain.

ADR-0004 deferred this as a known negative consequence of staying single-aggregate. However, de-GitHubifying the vocabulary is **orthogonal to the multi-aggregate question** — it is a surface-depth change that broadens the SDK's applicability without touching the core architecture. It is the one finding from the feedback that is both low-risk and unambiguously positive.

Separately, a strategic question arose: what makes go-localsync worth existing when go-cqrs-lite v3 is already the shared foundation? The answer is the **opinionated pull-mirror layer** — resilient retry/backoff, rate-limit honoring, partial-sync semantics, per-source mutex serialization, tombstone-with-resurrection, upstream reconciliation, and the CRDT conflict seam. None of that lives in go-cqrs-lite. De-GitHubifying the domain model makes that value proposition honest.

## Decision

Replace the four GitHub-specific fields on `model.Item` with a single opaque `Attributes map[string]string`. The sync machinery operates on identity + lifecycle + change-detection only; provider-specific content is carried as an opaque map that round-trips through events and projections without the decider ever inspecting it.

### What changes

| Layer               | Before                                                        | After                                                                              |
| ------------------- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `model.Item`        | `ActorLogin`, `ActorAvatarURL`, `RepoName`, `RepoURL` (typed) | `Attributes map[string]string` (opaque)                                            |
| `provider.Item`     | Same 4 fields                                                 | `Attributes map[string]string`                                                     |
| `ItemSyncedPayload` | 4 separate JSON fields                                        | `Attributes` + legacy fields kept with `omitempty` for backward-compatible replay  |
| `hasChanged`        | Checks ContentHash + UpdatedAt + Type + 4 GitHub fields       | Checks ContentHash + UpdatedAt + Type only (ContentHash already covers content)    |
| `ItemFilter`        | `ActorLogin *id.ActorLogin`, `RepoName *id.RepoID`            | `Attributes map[string]string` (key-value match)                                   |
| SQLite DDL          | 4 typed columns + 2 indexes                                   | 1 `attributes TEXT NOT NULL DEFAULT '{}'` JSON column                              |
| API DTO             | `actorLogin`, `repoName`, `repoUrl`, `actorAvatarUrl` fields  | `attributes` map in response; filter via generic attribute matching                |
| `pkg/id/`           | Defines `ActorLogin`, `RepoID` branded types                  | Removed — consumers define their own branded types via `brandid.ID[Brand, string]` |
| Schema version      | V2                                                            | V3 (marks the Attributes transition)                                               |

### What does not change

- The single `sync_item` aggregate type (ADR-0004 scope decision stands)
- The three-event vocabulary (`synced` / `conflict_found` / `tombstoned`)
- The pull-only ingestion model (`Provider.Fetch` + `Syncer`)
- The single flat projection
- The `ContentHash`-based change detection mechanism (already provider-agnostic)

### Event backward compatibility

Old events (schema V1/V2) carry `actorLogin`, `actorAvatarUrl`, `repoName`, `repoUrl` as separate JSON fields. The `ItemSyncedPayload` struct keeps these fields with `omitempty` so old events decode correctly. `dataItemFromPayload` upcasts: if `Attributes` is nil but legacy fields are populated, it reconstructs the Attributes map from them. New events (V3) populate `Attributes` and leave the legacy fields empty.

## Rationale

### Why `map[string]string` and not generics (`Item[P]`)?

Generics would infect the entire CQRS stack (`SyncItemState[P]`, `ReadModel[P]`, `CQRSStack[P]`, `Projector[P]`) — effectively the multi-aggregate generalisation that ADR-0004 deferred. The opaque map keeps the stack concrete while still allowing providers to layer typed accessors on top. This is the pragmatic middle ground: provider-agnostic at the SDK level, type-safe at the provider level.

### Why drop the GitHub-field comparisons from `hasChanged`?

`ContentHash` (a SHA-256 of the provider's raw JSON payload) already detects any content change, including attribute changes. The GitHub-field comparisons were redundant fallbacks from the pre-ContentHash era. `UpdatedAt` and `Type` remain as secondary change signals for providers that don't set `ContentHash`.

## Consequences

### Positive

- The SDK is genuinely provider-agnostic — no GitHub vocabulary in the core
- New pull-mirror consumers (GitLab, Jira, etc.) don't see alien field names
- `hasChanged` is simpler and fully provider-agnostic
- The `pkg/id/` package defines only sync-infrastructure IDs, not consumer-domain IDs
- The reference consumer (github-local-sync) defines its own branded types and attribute keys

### Negative

- Breaking change: `model.Item`, `provider.Item`, `ItemFilter`, event payload, SQLite schema, and API response all change shape
- Existing SQLite databases need a migration (adds `attributes` column; old columns are ignored)
- `map[string]string` is less type-safe than branded IDs — consumers must define their own typed accessors
- The API filter changes: `?actor=` and `?repo=` are replaced by generic attribute filtering

## References

- Adoption feedback (Finding 5): [`docs/feedback/2026-06-23_discordsync-adoption-feedback.html`](../feedback/2026-06-23_discordsync-adoption-feedback.html)
- [ADR-0004](0004-multi-aggregate-generalisation-deferred.md) (multi-aggregate deferral — this ADR is complementary, not a reversal)
- [ADR-0002](0002-branded-ids.md) (original branded ID adoption)
