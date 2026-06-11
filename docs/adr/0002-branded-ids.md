# ADR-002: Branded Phantom-Type IDs

**Status:** Accepted
**Date:** 2026-05-23
**Deciders:** Lars Artmann

## Context

The project uses multiple ID types: `ItemID`, `ExternalID`, `ProviderID`, `ActorID`, `RepoID`, `EventTypeID`. Without type safety, it's easy to pass an `ExternalID` where a `ProviderID` is expected — both are strings at runtime.

## Decision

We adopted **branded phantom-type IDs** via the `go-branded-id` library. Each ID type is a distinct compile-time type:

```go
type ItemID = id.ID[ItemIDBrand, ulid.ULID]
type ExternalID = id.ID[ExternalIDBrand, string]
type ProviderID = id.ID[ProviderIDBrand, string]
```

The phantom type parameter (`ItemIDBrand`) makes each ID type incompatible with all others at compile time, even though they share the same memory layout.

## Consequences

### Positive

- Compile-time prevention of ID confusion — passing `ExternalID` where `ItemID` is expected is a type error
- Zero runtime overhead — phantom types are erased at compile time
- `.Get()` accessor is explicit about unwrapping (no accidental stringification)
- `id.NewItemID()` generates ULIDs (sortable, unique) for internal use
- `id.ParseItemID()` returns an error instead of panicking

### Negative

- Slightly more verbose: `id.NewExternalID("123")` vs `"123"`
- Branded types can't be used as map keys directly (must use `.Get()` or the struct itself)
- `go-branded-id` is a custom dependency (but we own it)

### Tradeoff

The verbosity is intentional — it makes ID conversions explicit and auditable.
