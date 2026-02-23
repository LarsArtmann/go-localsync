# Project Split Analysis: go-localsync

## Executive Summary

go-localsync is a focused Go library and CLI for syncing GitHub user events to a SQLite database. **Splitting is NOT recommended.** The project is well-structured with clear package boundaries, has a single coherent purpose, and is appropriately sized as a single repository (~500 LOC).

## Current Architecture

```
pkg/
├── github/     # GitHub API client (rate limiting, retry, event fetching)
├── storage/    # Storage interface + SQLite implementation
├── sync/       # Sync orchestration (fetch → transform → store)
├── event/      # Event domain model
└── errors/     # Typed errors

internal/
├── database/   # Database connection management
└── db/         # sqlc generated code

cmd/
└── gh-sync/    # CLI entrypoint

sql/
├── schema/     # SQLite schema
└── queries/    # SQL queries for sqlc
```

## Split Recommendation: NOT RECOMMENDED

### Reasons Against Splitting

| Factor               | Assessment                                                  |
| -------------------- | ----------------------------------------------------------- |
| Project size         | Small (~500 LOC across 12 source files)                     |
| Purpose coherence    | Single, clear purpose (GitHub event sync)                   |
| Package coupling     | Tightly integrated; splitting creates artificial boundaries |
| Reuse potential      | Low - storage/event models are domain-specific              |
| Maintenance overhead | Splitting adds complexity without benefit                   |
| Consumer count       | Single CLI tool, no external consumers                      |

### Why No Sub-Projects Make Sense

1. **GitHub client** - Domain-specific to events, not a general GitHub SDK
2. **Storage layer** - Tied to event schema, not a generic SQLite helper
3. **Sync logic** - Core business logic that binds the other packages together
4. **CLI** - Single entrypoint with no variant needs

### Benefits of Keeping Monolithic

- Simple dependency management (single go.mod)
- Easier versioning and releases
- Simpler CI/CD (single workflow)
- Atomic refactoring across packages
- Lower cognitive overhead for contributors

### Risks of Splitting

- Circular dependency risk between packages
- Version compatibility matrix complexity
- Increased repository maintenance overhead
- Fragmented documentation and examples
- Slower development iteration (multi-repo PRs)

## Implementation Path

N/A - No split recommended.

## Conclusion

**Confidence: HIGH (95%)**

go-localsync exemplifies a well-structured, appropriately-sized Go project. The current architecture follows Go best practices with clean package boundaries and clear separation of concerns. The project is too small and too coherent to benefit from splitting. Any future expansion (e.g., supporting other data sources or storage backends) would be better served by internal refactoring rather than repository splitting. Keep as a single repository.
