# Go-LocalSync Agent Configuration

## Project Overview

Go-LocalSync is a single-writer pull-mirror SDK with a pluggable provider-based architecture. It uses event-sourced CQRS via go-cqrs-lite for state management, pluggable conflict resolution (`pkg/crdt/`), tombstone-based soft-deletes with upstream reconciliation, and branded IDs from go-branded-id for compile-time type safety. There is no multi-writer/distributed CRDT machinery — the provider is the sole writer per aggregate.

> **Scope boundary (ADR-0004):** The SDK is deliberately a **single-aggregate, pull-only, flat-Item sync engine** — one `sync_item` aggregate, three fixed events, one projection. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md). `go-cqrs-lite v4` is the cross-project sharing boundary. Do not widen the scope (multi-aggregate, push ingestion, consumer-defined events) without revisiting that ADR.

> **Vocabulary (ADR-0009, enacted v0.6):** canonical names are `id.SourceID`, `cqrs.StreamID`/`MustStreamID`, `sync.BatchOutcome` (+ `Syncer.Stats()`). The old names (`ExternalID`, `AggregateID`, `SyncSummary`, `GetStats`) exist only as deprecated shims. See the migration table in [README](README.md#upgrading-to-v06-breaking-vocabulary-alignment) / [CHANGELOG](CHANGELOG.md).

## Architecture

| Package              | Purpose                                                                                                                                                                                                                                    |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `pkg/provider/`      | Core contract: `Provider`, `Item`, `FetchResult`, `RetryConfig`, `RateLimitInfo`. Concrete providers live in consumer apps or nested modules.                                                                                              |
| `pkg/sync/`          | `Syncer`, `ConflictAwareSyncer`, `SyncStore` interface, `SyncAction`, `ItemSyncResult`, `BatchOutcome` (single user-facing result), retry/backoff, per-source mutex, opt-in reconciliation, OTel spans (`otel.go`).                        |
| `pkg/cqrs/`          | go-cqrs-lite **v4** integration: Decider, ReadModel (memory+SQLite), Projector, `CQRSStack`, typed commands, DLQ surface (`dlq.go`), OTel wiring (`otel.go`). See [CQRS Architecture](#cqrs-architecture).                                 |
| `pkg/data/`          | Domain model (`model.Item` with `SchemaVersion` + optional `Tombstone`, `Key`, `ItemFilter`) + `schema.Version` V1/V2/V3 event upcasting (V3 = de-githubify, ADR-0007). Everything CQRS operates on `*model.Item`.                         |
| `pkg/id/`            | Branded phantom-type IDs (`ItemID` ULID, `SourceID` string, `ContentHash`, `ProviderID`, `ActorLogin`, `RepoID`).                                                                                                                          |
| `pkg/errors/`        | Structured errors via go-error-family: intrinsic classification, `IsRetryable`, `HTTPStatus`, `WithCtx`/`InvalidField`.                                                                                                                    |
| `pkg/crdt/`          | `Conflict[T]`, `ConflictResolver[T]`, `LWWResolver[T]` — pluggable conflict strategy wired into `DecideSync`. No vector clocks (single writer needs none).                                                                                 |
| `pkg/api/`           | HTTP API (Huma v2): `GET /items`, `GET /stats`, `POST /sync`, `GET /health`; auth, rate limiting, pagination, error mapping. Split server.go + dto.go + handlers.go.                                                                       |
| `internal/cqrslint/` | ADR-0004 architectural-invariant linter: 10 AST checks (C0001-C0010), `//cqrs-lint:` suppression directives (line/block/range). CLI: `cmd/localsync-lint` (`--strict`, `--json`, `--rules`, `--explain`, ...). Details in the package doc. |
| `pkg/testutil/`      | Shared test doubles (`MockProvider`, `SyncStore` fake, `BuildPairs`).                                                                                                                                                                      |

**SyncStore seam:** `pkg/sync` defines `SyncStore`; `*cqrs.CQRSStack` implements it via adapters. Dependency flows one way: `cqrs → sync → provider/types/errors`. No import cycles. `SyncAction`/`ItemSyncResult` live in `pkg/sync/` (the seam), never in `pkg/cqrs/` (enforced by cqrslint C0007).

## CQRS Architecture

The entire storage layer is CQRS via go-cqrs-lite v4. **No legacy CRUD path.** Narrative decisions live in the ADRs; only operational essentials are inlined here.

- **Streams:** `cqrs/aggregate_id.go` — deterministic SHA256→hex from (source, sourceID) with a sync.Map cache; `StreamID()` (error) / `MustStreamID()`. Our encoding deliberately diverges from go-cqrs-lite's `DeriveStreamID` (pinned + documented at the definition site).
- **Decider** (`decider.go`): `SyncItemState{Item *model.Item}` (tombstone lives on `Item.Tombstone`), pure `Apply`, `DecideSync`/`DecideTombstone`, `HasChanged` = ContentHash/UpdatedAt/Type only (ADR-0007). Resurrect-bypasses-resolver is **by design** (ADR-0005 addendum): a tombstoned local is a deleted marker; a sync event is the only path back to live.
- **Events** (`events.go`): exactly three — `ItemSynced`, `ItemConflictFound`, `ItemTombstoned`. A sync event always means "live" → resurrection is automatic via projection upsert.
- **Read models:** `ReadModel` interface (embeds `model.ItemReader` + filter/pagination); memory + SQLite implementations. Errors preserve chains via multi-`%w` (`wrapDBErr`).
- **Projection** (`projection.go` + `runner.go`): live delivery via direct `bus.SubscribeAll` (synchronous, read-your-writes); SQLite catch-up via `projectionhost.Host` (checkpoint, crash auto-restart, DLQ — ADR-0006). The version-gate is **mutex-guarded** so concurrent live+replay delivery serializes per aggregate (stale events can't resurrect tombstoned rows).
- **Stack** (`stack.go`): Store+Bus+Repo+ReadModel+CommandDispatcher, SQL snapshots, event-logging + validation middleware, correlation IDs per sync run, `CQRSConfig.Validate()` at construction, named-return cleanup so error paths release resources. Commands: `SyncItem`, `TombstoneItem(..., opts ...event.Option)` (variadic, parity with direct dispatch), `Reconcile(ctx, source, seenKeys)` (opt-in tombstone of upstream-gone items; only after a COMPLETE fetch). No query dispatcher — reads call the ReadModel directly (see `stack_adapters.go`).
- **DLQ surface** (`dlq.go`): `DeadLetters`, `DeadLetterCount`, `DeleteDeadLetter`, `PurgeDeadLetters`, `ReplayDeadLetters` (replay does NOT auto-delete — callers delete via `DeleteDeadLetter`).
- **OTel** (`otel.go`, opt-in via `CQRSConfig.OTel`): command/event middleware (spans + `cqrs.operation.*` metrics), batch span, projection-host metrics adapter. `pkg/sync/otel.go` wraps `Sync`/`SyncIncremental` in `localsync.sync*` spans via `sync.WithTracer`.

**Key properties (short form):** idempotent sync (deterministic stream IDs); tombstone soft-delete + auto-resurrect; per-source mutex serialization (TOCTOU guard; different sources parallel; lock-free `runSync`/`runSyncIncremental` internals avoid re-entrant deadlock); remote-wins default with pluggable `crdt.ConflictResolver[T]` (resolver error → remote-wins fallback; winner recorded in `ItemConflictFound` and decoded safely via `ParseConflictWinner`); resilient fetch (exponential backoff + jitter, `errors.IsRetryable`, Retry-After hook, unclassified provider errors fail-open as Transient); partial sync returns the result **plus** `ErrPartialSync` (HTTP maps to 200-with-result); boundary validation via `pkgerrors.InvalidField` (classification + `field` context); `pkgerrors.HTTPStatus(err)` is the single error→status translator (`context.Canceled`→499, `DeadlineExceeded`→504; new sentinels inherit family status).

## Development Workflow

**Required first step** — enter the nix devShell (`direnv allow` with the committed `.envrc`, or `nix develop`) so `GOFLAGS=-tags=goexperiment.jsonv2` is set. Plain-shell `go build`/`go test`/buildflow native-go steps fail with `encoding/json/v2: build constraints exclude all Go files` (see Gotchas).

1. `go.work` (root, untracked, never commit): `use ( . ./provider/github )`. **Never add sibling checkouts** — buildflow runs tests in EVERY workspace module.
2. Build: `go build ./...`
3. Test: `go test ./... -count=1`
4. Lint: `golangci-lint run ./... --timeout=5m` · Format: `golangci-lint fmt ./...`
5. CQRS gate: `go run ./cmd/localsync-lint --strict --verbose` (suppressions: `//cqrs-lint:ignore`; `--show-suppressed` to audit)
6. Library cqrs-lint gate: `go run github.com/larsartmann/go-cqrs-lite/cmd/cqrs-lint/v4@v4.8.1 ./pkg --min-severity error` (needs the private `go-finding` module → devShell/SSH only; CI runs it secret-gated)
7. Doc-count truth: `./scripts/check-doc-counts.sh` (CI runs it; `--coverage` adds coverage-column checking locally). When you add/remove tests or bump deps, run it and fix flagged claims — **never hand-carry numbers**.
8. Full pipeline: `buildflow --build-mode full` (devShell active; go.work limited to the two in-repo modules)
9. Release: `./scripts/verify-release.sh <core-tag> [provider-tag]` (tags, GitHub Releases, proxy `@v/list`/`@latest`, pkg.go.dev, docs consistency); checklist in CONTRIBUTING.md

### CI (No go.work)

All dependencies are public; no private-repo auth except the secret-gated library gate. Jobs: `test` (race + coverage), `lint` (vet + actionlint@v1.7.12 + golangci-lint@v2.13.2 + dprint@0.56.1 + both cqrs-lint gates), `security` (govulncheck + gitleaks), `nix` (`nix flake check` — guards vendorHash drift), `build` (linux/darwin/windows compile matrix), `release` (tag-triggered), `provider` (standalone `GOWORK=off` build + race for `provider/github`). Build/release gate on test/lint/security/nix.

**`GOEXPERIMENT: jsonv2` is set at workflow `env:` level** — every job compiles go-cqrs-lite v4 code importing `encoding/json/v2`; without it the whole workflow goes red.

**Two cqrs-lint gates in `lint`:** (1) internal `go run ./cmd/localsync-lint --strict` (ADR-0004 invariants, fail on warnings); (2) library `cqrs-lint/v4@v4.8.1 ./pkg --min-severity error` — auto-enables when the `SSH_PRIVATE_KEY` secret exists (deploy key for **private** `larsartmann/go-finding`), otherwise skips with a `::notice::`. Known annotated false positives (`//cqrs-lint:ignore <rule> <reason>`, never blanket): C017 (memory-DLQ pairing in `store_factory.go`), E005 (closure-registered handlers), E014 (synchronous bus drain heuristic).

### Pre-commit Hooks

Use `buildflow`; not executable, formally disabled (documented decision). Re-enabling needs a formatter scope/budget review.

### Build & Lint Gotchas

- **GOEXPERIMENT=jsonv2 is load-bearing**: go-cqrs-lite v4 imports `encoding/json/v2`, gated in Go 1.26 behind `goexperiment.jsonv2`. Wired in 3 places: devShell `GOFLAGS` (buildflow native-go subcommands inherit it **only inside the devShell** — plain-shell buildflow fails while golangci-lint/`nix build` still pass: misleading partial-green), flake `preBuild` export (`buildGoModule` drops `GOEXPERIMENT` from `env`), `.golangci.yml` `run.build-tags` (golangci-lint ignores `GOFLAGS`). `go mod tidy` works on v4.
- **`vendorHash` drifts on any go.mod/go.sum change** (the auto-commit daemon commits deps without touching the flake) → `nix build`/`nix flake check` fail with a hash mismatch naming the new hash. Re-pin immediately; CI's `nix` job + `scripts/check-vendorhash.sh` fail fast on drift.
- **go.work is a local, untracked file**; buildflow detects it **on disk**. Keep it to `.` + `./provider/github` (both CI-proven); sibling checkouts make buildflow run their tests too (they fail). Never commit.
- **`provider/github` pins released core versions** (`go-localsync v0.5.0`): with `GOWORK=off` in that directory it resolves the PIN, not your local checkout — provider tests stay green on old vocabulary by design. Core vocabulary changes reach the provider only via an explicit post-release re-pin (TODO_LIST has the follow-up item).
- **SQLite driver registration is consumer-owned**: go-cqrs-lite storage registers NO sqlite driver; any SQLite-path consumer (and `pkg/api` integration tests) must blank-import `_ "modernc.org/sqlite"` or `SELECT`s fail with "unknown driver".
- **gopls is not authoritative after mechanical edits**: post-sed/move, LSP shows ghost errors (e.g. `item.ExternalID` in `decider.go`, `""`→`StreamID` conversions in tests) that the CLI disproves. Trust `go build` / `go test` / `golangci-lint run` — never edit based on a stale diagnostic; cross-check with a fresh CLI run first.
- **gopls `stdversion` warnings are known GOEXPERIMENT noise**: `json.Marshal*` "requires go1.27" LSP warnings come from the jsonv2 experiment vs the `go 1.26` directive; builds with `GOEXPERIMENT=jsonv2` are clean. Do not "fix" them, do not lower the go directive.
- **All `larsartmann/*` deps are public** (proxy + pkg.go.dev resolve without credentials; no GOPRIVATE/SSH in CI for fetches). The only private dep is `go-finding` (library cqrs-lint gate only). The old committed `vendor/` workaround is gone — flake uses a real `vendorHash`. `.envrc` must stay force-added (`git add -f .envrc`) since buildflow's gitignore-upserter lists it.
- **Pre-commit hooks are inert by decision**: not executable, skipped. Re-enable only after a formatter scope/budget review.
- **golangci-lint v2.13 `exhaustruct_v5`**: `ignore-patterns` (renamed from `exclude`) does not match local-package types in full runs — suppress optional-field structs (`ItemFilter`, `FetchResult`) via `issues.exclusions.rules` `text:` regexes instead.
- **`SA5012` disabled**: staticcheck v0.7 panics on cross-package even-elements analysis. Disabled in `linters.settings.staticcheck.checks`.
- **`go.mod` requires `go 1.26.7`** (matches the toolchain). Do not lower the directive to dodge toolchain lag; re-raise when deps require newer.
- **dprint freezes snapshot docs**: `docs/status/**` and `docs/planning/**` are excluded in `dprint.json` (point-in-time records stay as written); living docs stay formatted. Run `dprint fmt` after table edits in living docs.
- **Golangci pins + CI versions must match the devShell** (golangci-lint v2.13.2, dprint 0.56.1, actionlint 1.7.12) — never `latest`.

## Testing

**Decider-test convention (since 2026-09-05):** new decider behavior tests use the library-native `scenario` DSL (`go-cqrs-lite/scenario/v4`, see `pkg/cqrs/scenario_test.go`). Keep plain table tests for fold edge cases. The `eventtest` module has no released version — do not depend on it until tagged.

| Package              | Tests | Coverage | Status                                                                                                                                                                                                                                                                            |
| -------------------- | ----- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/cqrs`           | 162   | 85.1%    | ✅ Decider, ReadModel, Projection, Stack, SQLite RM, Replay, Correlation, tombstone, upcasting, scenario specs, regression tests                                                                                                                                                  |
| `pkg/sync`           | 36    | 87.7%    | ✅ Syncer + ConflictAwareSyncer + retry + reconcile + per-source lock + regression                                                                                                                                                                                                |
| `pkg/id`             | 15    | 100.0%   | ✅ ID construction, roundtrip, zero, equal, ContentHash (constructor, literal-compat, sha256 round trip)                                                                                                                                                                          |
| `pkg/errors`         | 16    | 92.9%    | ✅ Sentinels, wrapping, classification, IsRetryable, HTTPStatus, WithCtx/InvalidField, templates, partial-sync                                                                                                                                                                    |
| `pkg/provider`       | 2     | 92.3%    | ✅ Item validation                                                                                                                                                                                                                                                                |
| `pkg/api`            | 38    | 95.2%    | ✅ Server, routes, handlers, health/stats/items/sync endpoints, error mapping, partial-sync→200                                                                                                                                                                                   |
| `pkg/crdt`           | 8     | 100.0%   | ✅ Conflict, ConflictResolver, LWWResolver, example test                                                                                                                                                                                                                          |
| `pkg/data/model`     | 13    | 87.1%    | ✅ Item, Key, Validate, ItemFilter, Tombstone                                                                                                                                                                                                                                     |
| `pkg/data/schema`    | 4     | 100.0%   | ✅ Schema Version (V1/V2/V3), CurrentVersion, Valid                                                                                                                                                                                                                               |
| `internal/cqrslint`  | 69    | 93.2%    | ✅ 10 architectural checks (C0001-C0010), loader, finding sort/format, rules catalog, suppression directives                                                                                                                                                                      |
| `cmd/localsync-lint` | 23    | 35.2% *  | ✅ exit-code contract, summary/JSON output, violating-fixture round trip + **process-level harness** (builds the binary, pins 0/1/2 exits, strict, NDJSON shape). *the phase-1 flags/formatting grew the surface while its tests are process-level (coverage-invisible by design) |

**386 total test functions** across 11 test packages (incl. `cmd/localsync-lint`), plus 35 in the standalone `provider/github` module; the whole suite is race-clean.

Run: `go test ./... -count=1`

## Backend Selection

Selected via `CQRSConfig.Backend` in `cqrs.NewCQRSStack()`; event store + read model + snapshots + DLQ share the backend.

| Backend  | Config value        | Use Case                                 |
| -------- | ------------------- | ---------------------------------------- |
| `memory` | `Backend: "memory"` | Testing, development (default)           |
| `sqlite` | `Backend: "sqlite"` | Local SQLite file via modernc.org/sqlite |

## Provider Development

The core is a pure contract library; concrete providers are separate modules. Reference: **`provider/github`** (released `provider/github/v0.1.0`, go-github-kit v0.3.0, own CHANGELOG). To add one: implement `provider.Provider` (`Name`, `Fetch`, `FetchAll`, `GetRateLimit`); map provider data to `provider.Item` with branded `pkg/id` types + `Attributes` (typed write-helpers: `WithActorLogin` etc.); add tests; document config.

- `provider/github/` is a separate module pinning released parent versions; builds standalone with `GOWORK=off`; the root `go.work` wires it for local dev; CI job `provider` runs it in isolation. Its vocabulary adoption lags core by design (pinned parent) — see the Gotcha above.
- **go-localsync is a PUBLIC repo** (2026-09-05): `go get github.com/larsartmann/go-localsync@vX.Y.Z` works for everyone; no GOPRIVATE/SSH for dependency fetches.

## Database Schema

Two tables managed by the CQRS stack (DDL in `sqlite_readmodel.go` + go-cqrs-lite storage):

- **Events** (go-cqrs-lite/storage): `id`, `event_type`, `aggregate_type`, `aggregate_id`, `version`, `schema_version`, `payload`, `metadata`, `occurred_at`, `created_at`; unique on `(aggregate_type, aggregate_id, version)`.
- **Sync Items** (read model): `item_id`, `source`, `source_id`, `type`, `attributes` (JSON map), `content_hash`, `created_at`, `updated_at`, `schema_version`, `tombstoned`, `tombstone_reason`, `tombstoned_at` (added idempotently by `migrateSyncItems`); PK on `(source, source_id)`. Pre-V3 legacy columns (`actor_login`, …) exist but are never read/written (ADR-0007).

## Dependencies

| Dependency                         | Version | Purpose                                                                |
| ---------------------------------- | ------- | ---------------------------------------------------------------------- |
| `go-cqrs-lite/event/v4`            | v4.9.0  | Event types, Store, Bus, Journal (requires `GOEXPERIMENT=jsonv2`)      |
| `go-cqrs-lite/command/v4`          | v4.8.1  | Command types, Dispatcher, TypedHandler[T], `ExecuteRef`               |
| `go-cqrs-lite/query/v4`            | v4.7.1  | Indirect; no QueryDispatcher — reads call the ReadModel directly       |
| `go-cqrs-lite/decider/v4`          | v4.5.0  | Decider, Repository, snapshot/codec options                            |
| `go-cqrs-lite/id/v4`               | v4.5.0  | StreamID, CorrelationID                                                |
| `go-cqrs-lite/codec/v4`            | v4.4.0  | JSONCodec (uses `encoding/json/v2`)                                    |
| `go-cqrs-lite/projection/v4`       | v4.3.0  | Projection interface                                                   |
| `go-cqrs-lite/projectionhost/v4`   | v4.4.0  | Managed projection host: checkpoint, crash-restart, DLQ (ADR-0006)     |
| `go-cqrs-lite/snapshot/v4`         | v4.4.0  | SnapshotStore, EveryNEvents                                            |
| `go-cqrs-lite/storage/memory/v4`   | v4.4.0  | In-memory event + snapshot store                                       |
| `go-cqrs-lite/middleware/v4`       | v4.5.1  | EventLogging + CommandRetry + CommandValidation + OTel middleware      |
| `go-cqrs-lite/watermill/v4`        | v4.5.1  | In-process `EventBus`                                                  |
| `go-cqrs-lite/storage/v4`          | v4.8.1  | SQLite event store, snapshot, KV, DLQ store                            |
| `go-cqrs-lite/schema/v4`           | v4.3.1  | Schema `Version` + upcaster registry wiring                            |
| `go-cqrs-lite/otel/v4`             | v4.3.0  | OTel bundle: command/event spans + `cqrs.operation.*` metrics (opt-in) |
| `go.opentelemetry.io/otel`         | v1.46.0 | OpenTelemetry API (metric + trace direct; core/sdk indirect)           |
| `go-branded-id`                    | v0.5.1  | Branded phantom-type IDs                                               |
| `go-error-family`                  | v0.10.0 | Error classification + message templates                               |
| `modernc.org/sqlite`               | v1.56.0 | Pure-Go SQLite driver (no CGo; blank-import required)                  |
| `charm.land/log/v2`                | v2.0.1  | Structured logging                                                     |
| `github.com/danielgtaylor/huma/v2` | v2.39.1 | HTTP API + OpenAPI 3 generation                                        |
| `github.com/oklog/ulid/v2`         | v2.1.2  | ULID for `ItemID`                                                      |
| `go-cqrs-lite/scenario/v4`         | v4.2.0  | Test-only: decider Given/When/Then DSL                                 |

### Test Dependencies

| Dependency       | Purpose                                |
| ---------------- | -------------------------------------- |
| `onsi/ginkgo/v2` | Indirect only (via go-cqrs-lite tests) |
| `onsi/gomega`    | Indirect only (via go-cqrs-lite tests) |

Note: the `eventtest` module has no released version — do not depend on it until tagged (tracked in ROADMAP open questions).

### Build System

| File        | Purpose                                                                                                                                                     |
| ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `flake.nix` | Nix flake using the `go-standard` module (go-nix-helpers): devShell, `packages.default` + `packages.localsync-lint`, hermetic `checks.test` + `checks.lint` |

## go-cqrs-lite Integration (quick map)

| Area        | go-localsync                                                            | go-cqrs-lite provides                 |
| ----------- | ----------------------------------------------------------------------- | ------------------------------------- |
| IDs         | `id.ID[B, V]` via go-branded-id                                         | `id.Of[T]` (same memory layout)       |
| Storage     | `CQRSStack` → `decider.Repository[SyncItemState]`                       | `event.Store` + `event.Bus`           |
| Read model  | Memory + SQLite read models w/ filter/pagination                        | projection contract (`projection/v4`) |
| Projection  | Live `bus.SubscribeAll` + `projectionhost.Host` catch-up (ADR-0006)     | `projectionhost/v4`, `watermill/v4`   |
| Errors      | go-error-family constructors + `pkgerrors.HTTPStatus`/`WithCtx`         | 5-family error taxonomy               |
| Correlation | `event.WithCorrelationID` per sync run + causation enricher on commands | typed Metadata (stream-durable)       |

Note: typed `Metadata.Causation` is stream-durable; bus-delivered messages keep only the `command.type`/`command.id` custom fallbacks (watermill event-wire gap — filed upstream as [go-cqrs-lite#21](https://github.com/LarsArtmann/go-cqrs-lite/issues/21), source-verified: `eventToMessage` drops CorrelationID/CausationID/Causation while `buildMetadata` parses them).
