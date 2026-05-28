# Domain Language — go-localsync

**Project:** go-localsync  
**Last Updated:** 2026-05-25

A Unified Language for the local-sync domain — shared across users, developers, and AI.

---

## Glossary

| Term             | Definition                                                                                            | Context             |
| ---------------- | ----------------------------------------------------------------------------------------------------- | ------------------- |
| **Local Sync**   | The process of fetching items from a remote provider and storing them locally with conflict detection | Core domain         |
| **Provider**     | A data source that implements the `Provider` interface (e.g., GitHub, GitLab)                         | Architecture        |
| **Item**         | A single syncable entity: a GitHub event, a Jira issue, etc.                                          | Domain model        |
| **External ID**  | The provider-native identifier for an item (e.g., GitHub event ID "1234567890")                       | Identity            |
| **Source**       | The provider identifier (e.g., "github", "gitlab")                                                    | Identity            |
| **Aggregate ID** | A deterministic SHA256-derived ID used within the CQRS event store                                    | Event sourcing      |
| **Conflict**     | When a locally-stored item differs from the provider's version at sync time                           | Sync logic          |
| **Remote Wins**  | The default conflict resolution strategy: provider version overwrites local                           | Conflict resolution |
| **Sync Action**  | Classification of what happened to an item during sync: Created, Updated, Conflict, Unchanged, Error  | Metrics             |
| **Read Model**   | The query-side projection of synced items, filterable and paginated                                   | CQRS                |
| **Event Store**  | The append-only log of all domain events (ItemSynced, ItemConflictFound, ItemDeleted)                 | Event sourcing      |
| **Outbox**       | The transactional outbox pattern for crash-safe event publishing                                      | Infrastructure      |
| **Backend**      | The storage implementation: "memory" (testing) or "turso" (SQLite/Turso)                              | Configuration       |
| **Push/Pull**    | Turso-specific remote sync operations for multi-device data replication                               | Infrastructure      |

---

## Entities

Objects with identity and lifecycle.

| Entity           | Definition                             | Identity                             |
| ---------------- | -------------------------------------- | ------------------------------------ |
| **Item**         | A syncable entity from a provider      | `(Source, ExternalID)` composite key |
| **Sync Session** | A single execution of the sync process | Correlation ID (ULID)                |

---

## Value Objects

Immutable objects defined by attributes.

| Value Object      | Definition                                                    |
| ----------------- | ------------------------------------------------------------- |
| **ItemID**        | Internal ULID-based identifier (generated on first sync)      |
| **ExternalID**    | Provider-native string identifier                             |
| **ProviderID**    | Source identifier (e.g., "github")                            |
| **EventTypeID**   | Classification of item type (e.g., "PushEvent")               |
| **ActorID**       | Username of the entity that triggered the item                |
| **RepoID**        | Repository name (e.g., "owner/repo")                          |
| **RateLimitInfo** | Rate limiting metadata from provider APIs                     |
| **ItemFilter**    | Query filter: Type, Actor, Repo, Source, Since, Limit, Offset |

---

## Events

Domain events emitted by the event store.

| Event                 | Definition                                           | Trigger                                    |
| --------------------- | ---------------------------------------------------- | ------------------------------------------ |
| **ItemSynced**        | An item was successfully synced (created or updated) | `DecideSync` when item is new or changed   |
| **ItemConflictFound** | A conflict was detected and resolved (remote wins)   | `DecideSync` when local and remote differ  |
| **ItemDeleted**       | An item was marked as deleted                        | `DecideDelete` when deleting a synced item |

---

## Commands

Actions the system can perform.

| Command        | Definition                         | Handler                             |
| -------------- | ---------------------------------- | ----------------------------------- |
| **SyncItem**   | Sync a single item from a provider | `handleSyncItem` → `DecideSync`     |
| **DeleteItem** | Mark a synced item as deleted      | `handleDeleteItem` → `DecideDelete` |

---

## Queries

Read-side operations against the read model.

| Query          | Definition                                        |
| -------------- | ------------------------------------------------- |
| **ListItems**  | List items with optional filtering and pagination |
| **GetItem**    | Retrieve a single item by (Source, ExternalID)    |
| **CountItems** | Count items matching a filter                     |
| **GetTypes**   | Get all distinct event types in the read model    |

---

## Bounded Contexts

| Context         | Description                                                                     |
| --------------- | ------------------------------------------------------------------------------- |
| **Sync Engine** | Orchestrates fetching from providers and dispatching to CQRS stack (`pkg/sync`) |
| **CQRS Core**   | Event sourcing, command/query dispatch, projections (`pkg/cqrs`)                |
| **Provider**    | External API integration, rate limiting, retry logic (`pkg/providers/github`)   |
| **Types**       | Branded identifier types for compile-time safety (`pkg/id`)                  |

---

> **How to use this file:**
>
> - Keep terms concise — one clear sentence per definition
> - Update when new domain concepts emerge
> - Use these terms consistently in code, docs, and conversations
> - When in doubt about a word's meaning, check here first
