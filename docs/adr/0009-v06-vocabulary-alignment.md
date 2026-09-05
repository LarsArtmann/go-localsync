# ADR-0009: v0.6 Vocabulary Alignment (decision note, not yet enacted)

**Status:** Accepted (as a plan for the next breaking release)
**Date:** 2026-09-05
**Supersedes:** nothing — records decisions deferred from the 2026-09-05 SUPERB REFERENCE CONSUMER plan

## Context

Two vocabulary misalignments between go-localsync's public surface and
go-cqrs-lite's were flagged by the naming/data-model reviews and the
go-cqrs-lite utilization audit:

1. **`AggregateID()` returns a `StreamID`.** `pkg/cqrs.AggregateID(source, externalID)`
   has returned `cqrsid.StreamID` since v0.4.1 but kept the pre-rename name
   for API stability. The library renamed `AggregateID → StreamID` (and
   deprecated the old accessor names on events); our function name now lies
   about its return type.

2. **Two near-synonymous result types in `pkg/sync`.** `SyncResult` (the
   Syncer's outcome) and `SyncSummary` (the SyncStore's outcome) overlap in
   meaning ("what happened in this run") but differ in shape and audience.

A third item documents an intentional divergence:

3. **`DeriveStreamID` divergence.** `cqrsid.DeriveStreamID(namespace, keys...)`
   (NUL-separated SHA256, full digest) and our `AggregateID` (length-prefixed
   SHA256, truncated to 16 bytes/hex-32) solve the same problem —
   deterministic composite-key → stream-ID — with incompatible encodings.
   Switching would orphan every existing stored stream: aggregate IDs are
   persisted in event-store primary keys and snapshots. **We keep ours.**

## Decision

For **v0.6** (the next breaking window):

1. Rename `AggregateID()` → `StreamID()`, keeping a deprecated alias for one
   minor cycle. Callers migrate mechanically.
2. Fold `SyncSummary` INTO `SyncResult` as a typed field (or rename
   `SyncSummary` → `BatchOutcome`) so the two types stop competing for the
   same concept. Chosen shape: `SyncResult` stays the public per-run result;
   the batch/store-level summary becomes a field with its own honest name
   decided at enactment time — the decision recorded here is that THERE WILL
   BE exactly one user-facing result type.
3. Keep our `AggregateID` encoding forever (data compatibility trumps
   vocabulary purity); document the divergence at the definition site and
   link here. Do NOT adopt `cqrsid.DeriveStreamID` for existing streams.
   New multi-tenant consumers MAY use `DeriveStreamID` for their own
   namespaces — both functions can coexist.

## Why not now

The 2026-09-05 execution plan's guardrail: no public API breaks inside the
plan. Renames are cheap at release boundaries and expensive mid-stream.

## Consequences

- v0.6 release notes must carry a migration section for both renames.
- `AggregateID`'s panic fallback (unreachable: SHA256 hex of 16 bytes always
  parses; documented in-code) should become an error return in the same
  release if the signature is touched anyway.
- Until v0.6, this ADR is the canonical answer to "why do the names not
  match upstream?".

## Addendum (2026-09-06): ExternalID ↔ SourceID payload duality

**Finding.** One concept — the provider's key for an item — carries two
names: `ExternalID` (branded type `id.ExternalID`; fields on
`provider.Item`, `model.Item`, `model.Key`) and `SourceID` (field name in
all three event payloads, `json:"sourceId"`). Flagged by the session-26/27
self-reviews; never dispositioned.

**Decision for v0.6: align the Go surface to `SourceID`; do NOT touch the
wire.** Concretely:

- `id.ExternalID` (type) → `id.SourceID`, with `NewSourceID`/`ParseSourceID`
  and a deprecated `ExternalID` alias for one minor cycle (same pattern as
  decision 1).
- `provider.Item.ExternalID`, `model.Item.ExternalID`, `model.Key.ExternalID`
  → `SourceID`, mechanically migrated (60 non-test occurrences, 18 files).
- Event payloads stay EXACTLY as they are: `SourceID string` +
  `json:"sourceId"` is already the target vocabulary, and payload field
  names are a persisted wire contract — renaming them would force a schema
  V4 + upcast for zero functional gain.

**Why this beats the two options originally on the table** (align-with-upcast
vs document-dual-names): the original framing assumed "align" meant renaming
the wire (expensive: schema bump, upcast, replay risk) and "document" meant
living with the mismatch forever. Aligning the GO SURFACE instead gets one
name everywhere users touch (`Source` / `SourceID` / `StreamID`) while the
persisted events are already consistent. `AggregateID(source, externalID)`
→ `StreamID(source, sourceID)` (decision 1) completes the picture.

**Scope addition (Stats):** `Syncer.GetStats` → `Syncer.Stats` in the same
window (Go idiom: no `Get` prefix; mirrors `GetRateLimit` → `RateLimit`
upstream moves). Deprecated alias, one minor cycle. The unexported API
handler (`getStats`) needs no change.

**Gate:** v0.6 cannot enact without the owner's sign-off recorded in
TODO_LIST.md — these are public-API breaks under the ADR-0009 guardrail.
