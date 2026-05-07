# FEATURES.md — go-localsync

**Updated:** 2026-05-07

| Feature               | Status      | Description                                                  |
| --------------------- | ----------- | ------------------------------------------------------------ |
| CQRS Stack            | ✅ Complete | Event store, bus, decider repository, read model             |
| Decider Pattern       | ✅ Complete | Pure Fold/DecideSync/DecideDelete with SyncItemState         |
| Conflict Detection    | ✅ Complete | HasChanged comparison with timestamp, type, actor, repo      |
| Memory Backend        | ✅ Complete | In-memory event store and read model for testing             |
| Turso Backend         | ✅ Complete | SQLite/Turso event store with remote sync support            |
| GitHub Provider       | ✅ Complete | Fetch user events with pagination, rate limiting, retry      |
| Provider Interface    | ✅ Complete | Generic provider.Interface for any data source               |
| Branded IDs           | ✅ Complete | ItemID, ExternalID, ProviderID, ActorID, RepoID, EventTypeID |
| Sync Engine           | ✅ Complete | Full and incremental sync with configurable pagination       |
| Conflict-Aware Syncer | ✅ Complete | CRDT-backed conflict detection with vector clocks            |
| Schema Migrations     | ✅ Complete | Version-tracked database migrations, auto-applied            |
| Error Handling        | ✅ Complete | Sentinel errors with cockroachdb/errors wrapping             |
| Example CLI           | ✅ Complete | cmd/examples/github-sync/ CLI tool                           |
| Read Model Queries    | ✅ Complete | Count, GetTypes, GetItems with filtering                     |
| No CGO                | ✅ Complete | Pure Go SQLite driver (modernc.org/sqlite)                   |
