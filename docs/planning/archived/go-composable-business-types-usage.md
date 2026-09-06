# go-composable-business-types Integration Plan

> **Classification (2026-09-06 docs-policy sweep):** undated integration plan, superseded. The goal (branded, compile-time-safe IDs) shipped via **go-branded-id** v0.5.1 instead (ADR-0002, `pkg/id/`); `go-composable-business-types` was never adopted. Kept as decision context only.

## Executive Summary

This document outlines how to integrate `github.com/larsartmann/go-composable-business-types/id` into the go-localsync project to replace primitive string identifiers with strongly-typed, branded IDs that provide compile-time safety.

## Current State Analysis

### Primitive String Usage

The codebase currently uses raw `string` types for all identifiers:

```go
// pkg/provider/provider.go
type Item struct {
    ID             string `json:"id"`          // Generic ID from source
    Source         string `json:"source"`      // Provider identifier (e.g., "github")
    Type           string `json:"type"`        // Item type (e.g., "PushEvent")
    ActorLogin     string `json:"actorLogin"`  // Actor username
    RepoName       string `json:"repoName"`    // Repository identifier
    // ...
}
```

### Problems with Current Approach

1. **No compile-time safety**: Can accidentally pass `RepoName` where `ActorLogin` is expected
2. **Silent failures**: Mixing up ID types compiles but fails at runtime
3. **Poor documentation**: String types don't convey semantic meaning
4. **Refactoring hazard**: Changing ID semantics requires global search/replace

## Proposed Integration

### 1. New ID Types Package

Create `pkg/types/ids.go` to define domain-specific branded IDs:

```go
package types

import (
    "github.com/larsartmann/go-composable-business-types/id"
)

// Brand types (phantom types for type safety)
type (
    ItemBrand    struct{}
    ProviderBrand struct{}
    ActorBrand   struct{}
    RepoBrand    struct{}
    EventTypeBrand struct{}
)

// ID type aliases for convenience
type (
    ItemID       = id.ID[ItemBrand, string]
    ProviderID   = id.ID[ProviderBrand, string]
    ActorID      = id.ID[ActorBrand, string]
    RepoID       = id.ID[RepoBrand, string]
    EventTypeID  = id.ID[EventTypeBrand, string]
)

// Constructor functions
type (
    NewItemID     = id.NewID[ItemBrand, string]
    NewProviderID = id.ID[ProviderBrand, string]
    NewActorID    = id.NewID[ActorBrand, string]
    NewRepoID     = id.NewID[RepoBrand, string]
    NewEventTypeID = id.NewID[EventTypeBrand, string]
)
```

### 2. Updated Provider.Item

```go
// pkg/provider/provider.go
type Item struct {
    ID             types.ItemID      `json:"id"`
    Source         types.ProviderID  `json:"source"`
    Type           types.EventTypeID `json:"type"`
    ActorLogin     types.ActorID     `json:"actorLogin,omitempty"`
    ActorAvatarURL string            `json:"actorAvatarUrl,omitempty"`
    RepoName       types.RepoID      `json:"repoName,omitempty"`
    RepoURL        string            `json:"repoUrl,omitempty"`
    CreatedAt      time.Time         `json:"createdAt"`
    RawJSON        []byte            `json:"rawJson"`
}
```

### 3. Storage Layer Adaptation

The storage layer needs to handle SQL serialization. The `id.ID` type already implements:

- `sql.Scanner` - for reading from database
- `driver.Valuer` - for writing to database

Current database schema uses `github_id` (string), which is compatible.

```go
// pkg/storage/interface.go - minimal changes needed
// The toItem/toDBParams converters remain largely unchanged
// because id.ID[string] marshals/unmarshals seamlessly
```

### 4. Provider Implementation Updates

GitHub provider creates items from API responses:

```go
// pkg/providers/github/client.go
func convertEvent(e *gh.Event) (*provider.Item, error) {
    // ...
    return &provider.Item{
        ID:             types.NewItemID(strconv.FormatInt(e.GetID(), 10)),
        Source:         types.NewProviderID("github"),
        Type:           types.NewEventTypeID(e.GetType()),
        ActorLogin:     types.NewActorID(actorLogin),
        RepoName:       types.NewRepoID(repoName),
        // ...
    }, nil
}
```

## Implementation Phases

### Phase 1: Setup and Foundation

1. Add dependency to `go.mod`:

   ```bash
   go get github.com/larsartmann/go-composable-business-types/id
   ```

2. Create `pkg/types/ids.go` with brand types and constructors

3. Verify build: `go build ./...`

### Phase 2: Provider Package Updates

1. Update `provider.Item` to use branded IDs
2. Update `FetchOptions.Source` from `string` to `types.ProviderID`
3. Update all provider interface methods
4. Run tests: `go test ./pkg/provider/...`

### Phase 3: Storage Layer Updates

1. Update storage interface signatures
2. Update SQLiteStorage implementation
3. Verify SQL serialization still works
4. Run tests: `go test ./pkg/storage/...`

### Phase 4: Sync Layer Updates

1. Update `SyncOptions.Source` type
2. Update syncer to work with new ID types
3. Run tests: `go test ./pkg/sync/...`

### Phase 5: GitHub Provider Updates

1. Update `convertEvent` to create branded IDs
2. Update any test fixtures
3. Run tests: `go test ./pkg/providers/github/...`

### Phase 6: Example and Integration

1. Update `cmd/examples/github-sync/main.go`
2. Run full integration test
3. Verify end-to-end functionality

## Benefits

1. **Compile-time safety**: Cannot mix ItemID with ActorID
2. **Self-documenting code**: Types convey intent
3. **Refactoring support**: Rename brands, compiler finds all usages
4. **Zero runtime cost**: Brands are compile-time only
5. **Serialization preserved**: JSON/SQL work unchanged

## Migration Strategy

### Backward Compatibility

- JSON serialization remains compatible (IDs serialize as strings)
- Database schema unchanged
- API responses parse correctly

### Breaking Changes

- Function signatures change (accept `types.ItemID` instead of `string`)
- Callers must use constructors: `types.NewItemID("...")` instead of raw strings

## Testing Strategy

1. Unit tests for new ID types
2. Integration tests for serialization round-trips
3. End-to-end sync test
4. Verify database persistence works

## Risk Assessment

| Risk                          | Likelihood | Impact | Mitigation            |
| ----------------------------- | ---------- | ------ | --------------------- |
| Serialization issues          | Low        | High   | Comprehensive tests   |
| SQL driver incompatibility    | Low        | High   | Test with SQLite      |
| Breaking downstream consumers | Medium     | Medium | Clear migration guide |
| Increased complexity          | Low        | Low    | Good documentation    |

## Dependencies

Required in `go.mod`:

```
require (
    github.com/larsartmann/go-composable-business-types/id v0.1.0
)
```

## Success Criteria

- [ ] All tests pass
- [ ] Code compiles without warnings
- [ ] JSON serialization unchanged
- [ ] Database operations work correctly
- [ ] No primitive string IDs in domain types
- [ ] Documentation updated

## Implementation Summary

Successfully completed integration:

### Changes Made

1. **go.mod**: Added `replace` directive and `require` for go-composable-business-types
2. **pkg/types/ids.go**: New package defining domain-specific branded ID types
3. **pkg/provider/provider.go**: Updated `Item` struct to use branded IDs
4. **pkg/storage/interface.go**: Updated conversion functions to use `.Get()` for DB and `types.New*ID()` for domain
5. **pkg/providers/github/client.go**: Updated `convertEvent` to create branded IDs
6. **pkg/sync/sync_test.go**: Updated tests to use branded ID constructors
7. **pkg/storage/sqlite_test.go**: Updated tests to use branded ID constructors
8. **pkg/providers/github/client_test.go**: Updated assertions to use `.Get()` to compare values

### Testing

All tests pass:

```
ok  github.com/larsartmann/go-localsync/pkg/providers/github
ok  github.com/larsartmann/go-localsync/pkg/storage
ok  github.com/larsartmann/go-localsync/pkg/sync
```

### Usage Example

```go
// Creating IDs
itemID := types.NewItemID("12345")
providerID := types.NewProviderID("github")
actorID := types.NewActorID("larsartmann")

// Accessing values
idString := itemID.Get()

// Type safety prevents mixing
// Cannot pass ItemID where ActorID is expected - compile error!
```

## Timeline Estimate

- Phase 1 (Setup): 15 minutes
- Phase 2-5 (Core changes): 1-2 hours
- Phase 6 (Integration): 30 minutes
- Testing & verification: 30 minutes
- **Total: 2-3 hours**

**Actual time: ~45 minutes**
