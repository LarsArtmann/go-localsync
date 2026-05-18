# FEATURES.md — go-localsync

**Updated:** 2026-05-18

| Feature               | Status      | Description                                                                          |
| --------------------- | ----------- | ------------------------------------------------------------------------------------ |
| CQRS Stack            | ✅ Complete | Event store, bus, decider repository, read model, projection                         |
| Decider Pattern       | ✅ Complete | Pure Fold/DecideSync/DecideDelete with SyncItemState                                 |
| Conflict Detection    | ✅ Complete | HasChanged comparison with timestamp, type, actor, repo; decider is single authority |
| Memory Backend        | ✅ Complete | In-memory event store and read model for testing                                     |
| Turso Backend         | ✅ Complete | SQLite/Turso event store with remote Push/Pull sync                                  |
| GitHub Provider       | ✅ Complete | Fetch user events with pagination, rate limiting, retry (35 tests)                   |
| Provider Interface    | ✅ Complete | Generic provider.Interface for any data source                                       |
| Branded IDs           | ✅ Complete | ItemID, ExternalID, ProviderID, ActorID, RepoID, EventTypeID                         |
| Sync Engine           | ✅ Complete | Full and incremental sync with configurable pagination                               |
| Conflict-Aware Syncer | ✅ Complete | Delegates to CQRS decider, maps SyncAction to ConflictResult metrics                 |
| Deterministic IDs     | ✅ Complete | SHA256→ULID from (source, sourceID) for idempotent aggregates                        |
| Turso Remote Sync     | ✅ Complete | Push/Pull to remote Turso database for multi-device sync                             |
| Error Handling        | ✅ Complete | Sentinel errors with stdlib fmt.Errorf + %w wrapping                                 |
| Example CLI           | ✅ Complete | cmd/examples/github-sync/ with flag parsing, signal handling, exit codes             |
| Read Model Queries    | ✅ Complete | Count, GetTypes, GetItems with filter/pagination                                     |
| No CGO                | ✅ Complete | Pure Go SQLite driver (modernc.org/sqlite)                                           |
