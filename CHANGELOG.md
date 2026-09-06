# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Release dates are reconciled against the actual git tags (`v0.1.0`, `v0.1.1`, `v0.2.0`, `v0.3.0`, `v0.4.0`, `v0.4.1`, `v0.5.0`; the retroactive `v0.4.2` proxy-sweep tag has no section — see the note under 0.5.0).

## [Unreleased]

Nothing yet — the next release is staged as [v0.6.0] below.

## [v0.6.0] - Unreleased

**Breaking release ([ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md) vocabulary alignment).** All sections are relative to [v0.5.0]. The persisted event payloads (`json:"sourceId"`) are UNCHANGED — this release renames the Go surface and the HTTP DTO only; no schema V4, no upcast, no data migration.

### Migration (v0.5.0 → v0.6.0)

| v0.5.0 | v0.6.0 | Notes |
| --- | --- | --- |
| `id.ExternalID` / `id.NewExternalID` | `id.SourceID` / `id.NewSourceID` | Old names remain as deprecated type-alias / function-value shims; `SourceID` is canonical. |
| `cqrs.AggregateID(...)` (panics on error) | `cqrs.StreamID(...)` (error-returning) / `cqrs.MustStreamID(...)` | `AggregateID` remains as a deprecated panicking shim for the migration window. |
| `sync.SyncSummary` | `sync.BatchOutcome` via `SyncResult.Batch` | One user-facing result type replaces the summary/result duality. |
| `syncer.GetStats()` | `syncer.Stats()` | `GetStats` remains as a deprecated alias. |
| HTTP DTO field `externalId` | `sourceId` | Read-path JSON only (`GET /items`); event payloads unchanged. |
| `POST /sync` declares 408 | 499 (client closed) / 504 (deadline) | Matches `pkgerrors.HTTPStatus`; the OpenAPI document declares 499/504 and not 408. |
| `TombstoneItem(ctx, source, sourceID, reason)` | `TombstoneItem(..., opts ...event.Option)` | Variadic tail added for direct-dispatch parity (correlation/causation options). |
| `SyncOptions.MaxPages < 0` silently ignored | rejected by `Validate()` | Classified `ErrInvalidInput` with `field=maxPages` attached. |
| `var ErrX = errorfamily.New*` | `var ErrX error = errorfamily.New*` | Interface-typed sentinels; `errors.Is`/`As`/`AsType` behavior unchanged. |

### Added

- **Tombstone visibility on `/items` (implemented, completing the staged claim)** — `ListItemsInput.IncludeTombstoned` (query param `includeTombstoned`) surfaces tombstoned rows carrying `ItemResponse.Tombstone *TombstoneInfo` (`reason` + `tombstonedAt`, unknown reasons degrade via `model.ParseTombstoneReason`); live items never carry the field. Pinned by a real-SQLite integration test (default view excludes, including view carries the typed reason) and by an OpenAPI schema test asserting the rendered spec declares the `TombstoneInfo` component, the `tombstone` property, and the boolean query parameter. (The v0.6.0 entry previously claimed this API; the code had never landed — it now matches the claim.)
- **`/sync` response now reports the batch outcome** — `synced`, `conflicts`, and `tombstoned` join `fetched`/`skipped`/`errors`, sourced from `SyncResult` and `SyncResult.Batch` (zero when no batch ran). Per-item action detail deliberately stays off the wire.
- **`provider.FetchResult.CacheHits`** — optional, provider-agnostic count of responses served from a conditional cache (ETag 304 revalidations); zero when the provider has no cache. The `provider/github` wiring lands with the post-v0.6.0 re-pin (the nested module pins released core versions).
- **`api.APIKeyClient`** — the canonical per-client rate-limit key extractor (X-Api-Key first, `Authorization: Bearer` fallback, mirroring `WithAPIKey` acceptance rules), documented as the recipe for `WithRateLimiter(perMinute, api.APIKeyClient)` and pinned by bucket-isolation + extractor-contract tests.
- **SARIF output for `localsync-lint`** (`--format=sarif`) — a single SARIF 2.1.0 document with the full rule catalog in `tool.driver.rules`, one result per finding (level mapping, 1-based regions, unpositioned findings omit the region), and `inSource` suppression entries for directive-silenced findings shown via `--show-suppressed`. A process-level help-vs-acceptance test now pins that every format the `-format` help advertises is actually accepted — the advertised-but-unimplemented lie this flag carried is structurally prevented. The CLI flow moved into `run(args, stdout, stderr) int` so tests drive it in-process: coverage 64.8% → 95.5% (`main()` stays process-tested by design).
- **`docs/localsync-lint.md`** — the directives/rules reference: flag table, output formats incl. SARIF, the full suppression-directive grammar (line/block/range/file), and the 15-rule catalog with rationale pointers. `check-doc-counts.sh` derives the catalog size from the `Rules()` declaration table and fails when this page (or AGENTS/README) drifts from it.
- **`check-doc-counts.sh --fix`** — rewrites drifted count claims in place locally (test table cells with dprint-stable padding, totals, dependency versions, coverage columns); CI stays check-only so drift is always reviewed. Live-proven on real drift.
- **Provider CI lint leg** — the `provider` job now runs pinned `golangci-lint v2.13.2` via `go run` (mirroring the actionlint pattern) against a new self-contained `provider/github/.golangci.yml` (which also stops config discovery from walking up to the root config). Two real findings fixed: canonical `X-Ratelimit-*` header literals, and wrapped external errors on the rate-limit/encode paths.
- **CI lint-job summary** — a gate-badge table (golangci-lint, localsync-lint, library cqrs-lint incl. its skip state) is posted to `$GITHUB_STEP_SUMMARY` on every run, so a red gate is visible without opening step logs.
- **Race-flake root cause + wait hardening (pkg/cqrs tests)** — the unexplained 2026-09-06 `-race` flake class is closed: the goroutine-leak test sampled process-wide `NumGoroutine` once after a fixed sleep while running parallel with siblings that start their own stacks (spurious "leak", no DATA RACE — exactly the observed signature); it now polls to baseline. The `subscribeAll` wait's silently-expiring 5s deadline (which failed callers' content assertions instead) is now 30s with a loud failure, and the export test's fixed 50ms sleep became a deterministic poll. Verified: 20x targeted `-race` stress + 3x full-suite `-race` under 4-core CPU load, all clean with full logs captured.
- **Outcome attributes on `localsync.sync` spans** — sync/incremental spans now carry `localsync.fetched/skipped/tombstoned/errors` plus `localsync.synced/conflicts` from the batch (parity with the CQRS batch span), pinned by recorder assertions.
- **Projection scenario specs** — the scenario-DSL convention now covers the read side: `scenario.GivenProjection` drives the real projector against a real memory read model for upsert, hide-but-keep, stale-replay-cannot-resurrect, newer-sync-resurrects, conflict-no-op, and idempotent-unknown-tombstone.
- **Retry/backoff edge tests** — jitter bounds (plus/minus 25% band, never above `MaxBackoff`, overflow-capped shifts, degenerate configs) and the Retry-After override path (advice beats backoff, advice clamped by `MaxBackoff`, zero advice falls back), proven by elapsed time against real waits.
- **EventLogger wiring test** — an end-to-end spec (external test package, so the sync-to-cqrs seam is exercised the way consumers build it) proving a consumer-provided `CQRSConfig.EventLogger` receives the per-event middleware lines.
- **Docs**: DLQ ops runbook (list, replay, delete, purge) at the top of `pkg/cqrs/dlq.go` with the replay-does-NOT-auto-delete contract corrected to match the pinned test; `docs/nix-systems-triage.md` (which systems the flake declares, why `--all-systems` is not the gate); StreamID cache bounded-growth note; CONTRIBUTING log-level configuration snippets; provider README ETag section (usage snippet, config row, `ETagStats` example, and the recorded decision that a page-1 304 probe would be incorrect for shifting event feeds).
- **Local security parity** — full-history gitleaks (781 commits, no leaks) and govulncheck (no vulnerabilities) now run locally in addition to CI.

- **Structured log level control** — `CQRSConfig.LogLevel` (string, validated at construction via `CQRSConfig.Validate`, applies to the stack-owned event logger only — consumer-provided `EventLogger`s keep their own control) and `api.WithLogLevel(log.Level)` (typed option applying to the server logger incl. its `log.Default()` fallback, documented). Kills the per-event INFO noise in production without replacing loggers. 8 new tests across both surfaces.
- **v0.6 vocabulary renames enacted** — the [ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md) window closed with the owner's sign-off recorded in TODO_LIST.md: `id.SourceID` (+ `SourceBrand`/`NewSourceID`) replaces `ExternalID` as the canonical name, `cqrs.StreamID`/`MustStreamID` replace the panicking `AggregateID` (error-returning core, 61-file mechanical sweep), `BatchOutcome` replaces `SyncSummary`, and `Stats` replaces `GetStats`. Deprecated shims keep every old name compiling through the migration window.
- **DLQ SDK surface** (`pkg/cqrs/dlq.go`) — `DeadLetters`, `DeadLetterCount`, `DeleteDeadLetter`, `PurgeDeadLetters`, and `ReplayDeadLetters` on `CQRSStack`, backed by the persistent SQLite dead-letter store; replay reports `Replayed`/`StillFailing` and intentionally does NOT auto-delete — callers delete via `DeleteDeadLetter` (pinned by test). Nil-host paths return a classified `ErrDatabase` chain.
- **Per-client API rate limiting** — `api.WithRateLimiter(perMinute, keyExtractor)` adds client-keyed buckets on top of the global limiter; 429 responses now carry canonical `X-Ratelimit-Limit`/`X-Ratelimit-Remaining` headers.
- **OTel spans for the sync engine** (`pkg/sync/otel.go`) — `sync.WithTracer(trace.Tracer)` wraps `Sync`/`SyncIncremental` in `localsync.sync`/`localsync.sync_incremental` spans (error status + `RecordError`); `CQRSConfig.OTel` propagates the tracer end-to-end. Real-meter and real-tracer recorder tests now assert values land in `cqrs.operation.*` counters, projection attributes, and span attributes (noop providers only proved wiring before).
- **Tombstone on the read path + typed attribute write-helpers** — `ItemResponse.Tombstone *TombstoneInfo` exposes the tombstone with the typed reason (`model.ParseTombstoneReason`, unknown reasons degrade to a fallback, never panic); `model.Attributes` gains `WithActorLogin`/`WithActorAvatarURL`/`WithRepoName`/`WithRepoURL` (copy-on-write) mirroring the typed readers.
- **`SyncOptions.MaxPages` validation** — negative values are rejected via `pkgerrors.InvalidField("maxPages", ...)` instead of being silently ignored.
- `localsync-lint` (formerly `cqrs-lint`) CLI phase 2: `--rules`/`--exclude-rules` subset selection with catalog validation (unknown IDs exit 2), `--no-suppress` CI-hardening mode that ignores every directive, and `--explain <rule>` for full rule rationale.
- Block-comment suppression directives (`/* cqrs-lint:ignore ... */`) and range directives (`ignore-start`/`ignore-end`) with a nesting guard; unmatched ends and unclosed ranges now warn.
- `cqrslint.RunOptions`/`RunWithOptions` (rule selection + suppression toggle), `ValidateRuleSelection`, `RuleByID`, and the `ErrUnknownRule` sentinel.

- **cqrs-lint architectural rules C0011–C0015** — five new invariants with compliant/violating fixtures, growing the catalog 10 → 15: C0011 single projection (exactly one `EventTypes` method), C0012 fold purity (no `time.Now`/`time.Since` inside `fold*`), C0013 projector read-only (no `Append`/`Save` from `Projector`), C0014 wire values pinned (canonical event/aggregate literals only in their declaring file), C0015 no inline `NewEvents` type literals (event-type slices must use consts). The real `pkg/cqrs` tree runs clean under all 15.
- **cqrs-lint CLI phase 1** — `--version`, `--quiet` (exit-code-only operation for scripts), `--format=github` (GitHub Actions `::error`/`::warning` annotations with file/line so findings surface inline in PRs), per-rule suppressed counts in `--verbose` (stale-directive detection), and `--json` findings now emitted through `encoding/json` against an explicitly tagged schema struct (keys unchanged).

- **`.buildflow.yml`** — buildflow full mode is green again (61 success / 0 failed, ending a three-session deferral): the nix steps that auto-repair hashes by evaluating every flake-check system are skipped locally (aarch64-darwin is unbuildable without a darwin builder); `nix flake check` remains the gate.

- **Single source of truth for attribute keys** — the cqrs adapter's duplicated `legacy*` key constants are deleted; `upcastLegacyAttributes` now writes `model.Attr*` keys directly, and a round-trip test proves adapter-written attributes read back through the model's typed accessors.

- **`CQRSConfig.Validate()` wired into `NewCQRSStack`** — invalid configs (unknown backend) now fail at the constructor boundary before any factory dispatch or resource setup, with the classified `ErrUnknownBackend` chain.

- **Benchmark protocol + new benchmarks** — `docs/benchmarks.md` fixes the measurement protocol (fixed `-benchtime 20x`, `-count 5`, benchstat comparison, environment caveats) and `scripts/run-benchmarks.sh` runs it. `BenchmarkPipeline_Replay10kEvents` now measures TRUE from-zero replay (persisted projection checkpoints are wiped per iteration — the old version measured stack open/close, since the checkpoint resumed at head and replayed nothing). New: `BenchmarkConflict_SyncExisting` (per-item resolver path) and `BenchmarkUpcastedLegacyRead` (V1 upcast-on-read vs native V3 pass-through; measured ~3.3× upcast tax on a protocol run).

- **`pkg/id` ContentHash unit tests + file split (100% package coverage)** — constructor/round-trip, zero-value, and the deliberate named-string literal-compat contract pinned; the type moved from `ids.go` to `content_hash.go`. `pkg/id` coverage: 75.0% → 100.0%.

- **cqrs-lint process-level test harness** — 5 end-to-end tests build the CLI binary and run it against fixtures, pinning the exit-code contract (0 clean / 1 findings / 2 usage error), `--strict` failing on the unknown-rule warning, and the NDJSON output shape (position required for code-level findings, absent for package-level checks). Subprocess runs are invisible to Go coverage by design, so `cmd/cqrs-lint` coverage stays 56.4% while the process contract is now test-enforced.

- **ADR-0009 addendum: ExternalID ↔ SourceID duality decided** — v0.6 will align the Go surface to `SourceID` (`id.ExternalID` type + fields → `SourceID`, deprecated aliases) while the persisted event payloads (`json:"sourceId"`) stay untouched, avoiding a schema V4/upcast; `Syncer.GetStats` → `Stats` joins the same window. Enactment gated on the owner's recorded sign-off.

- **Pre-release pipeline** — `scripts/verify-release.sh <core-tag> [provider-tag]` verifies publication end-to-end (tags local+origin+ancestry, GitHub Release, proxy.golang.org `@v/list`/`@latest` for both modules, pkg.go.dev indexing warn-only); `nix flake check` is now the one-command full suite (hermetic `checks.test` + `checks.lint` joined build/format/cqrs-lint); CONTRIBUTING.md gained a release checklist. Dry-run against the live release: `v0.5.0 v0.1.0` → all green.
- **vendorHash drift guard (`scripts/check-vendorhash.sh`)** — the CI `nix` job fails fast with re-pin instructions when `go.mod`/`go.sum` change without a `flake.nix` re-pin (base: PR base sha / push before-sha / `HEAD~1` locally); proven red→green with a dummy dep touch.
- **Doc-count truth in CI (`scripts/check-doc-counts.sh`)** — the lint job now fails when hand-copied numbers drift from code: per-package + total test counts (AGENTS table, AGENTS/README/FEATURES totals) and the AGENTS dependency table vs `go.mod`; `--coverage` adds the coverage-column check (±1.0 pt) for local runs. Its first run immediately caught the drift from the upcaster session (+4 tests: 309→313, pkg/cqrs 144→148) — fixed in the same commit.
- **CI truth cluster** — a `nix` job now runs `nix flake check` on every push (the 2026-09-05 vendorHash drift left `nix build` silently red for over an hour because no CI job owned it; it overrides the flake's SSH `go-nix-helpers` input to anonymous HTTPS), `actionlint` validates all workflow files (pinned `@v1.7.12`, matching the devShell copy), and golangci-lint is pinned to `v2.13.2` — the exact devShell version — instead of `latest`. The build/release jobs gate on the new `nix` job.
- **Resurrection disposition pinned (ADR-0005 addendum)** — documented as by-design that `DecideSync` never consults the `ConflictResolver` when resurrecting a tombstoned item: the tombstoned local is a deleted marker, not live content, and a sync event is the only path back to "live" (a local-wins veto would hide an upstream-restored item forever). Pinned by `TestDecideSync_ResurrectTombstonedItem_BypassesResolver`; annotated at the branch in `decider.go`.
- **Persistent SQLite projection DLQ (C017 fix)** — the SQLite backend now wires `projectionhost.NewSQLiteDeadLetterStore` sharing the same database file, so captured poison events survive restarts instead of vanishing with the old in-memory store; the memory backend keeps the in-memory DLQ by design.
- **Correlation + causation metadata on all write paths** — `SyncItem` and `TombstoneItem` now default a fresh correlation ID (previously only the batch path had one), `TombstoneItemCommand` grew an `Options` field, and every stored event names its causing command (type + ID) via `decider.WithEnricher(event.CommandCausalityEnricher)`. Note: bus-delivered messages keep the `command.type`/`command.id` custom fallbacks; the typed `Metadata.Causation` pointer lives on the durable stream (watermill event-wire gap, source-verified and filed upstream as [go-cqrs-lite#21](https://github.com/LarsArtmann/go-cqrs-lite/issues/21)).
- **OTel opt-in surface** — `CQRSConfig.OTel *middleware.OTelBundle`: when set, command/event middleware chains (spans + `cqrs.operation.*` metrics) attach to dispatcher and bus, `SyncItems` opens a `localsync.sync_items` span, and the projection host reports event/dead-letter/worker/checkpoint health through the same instruments via a `projectionhost.MetricsRecorder` adapter (`pkg/cqrs/otel.go`). `stack.OTel()` exposes the bundle for reuse (e.g. HTTP middleware). Nil = zero behavior change.
- **`/metrics` endpoint hook** — `api.WithMetricsHandler(h)` mounts any `http.Handler` under `GET /metrics` (e.g. promhttp); the SDK stays exporter-agnostic.
- **Structured log fields** — every sync-completion and item-warning log line in `pkg/sync` now carries `source` (plus attempt/wait on retries), making logs filterable per source without message parsing.
- **Shutdown hygiene** — all 8 swallowed `Close()` errors on construction-cleanup paths are now logged (`closeLogged`); the `AggregateID` panic fallback is kept unreachable-by-construction and documented (error return deferred to v0.6, ADR-0009).
- **Deterministic tombstones** — `TombstoneItemCommand.At time.Time`: zero = now, explicit value makes the decider testable/backdatable (clock injection replaces the hidden `time.Now()`).
- **`cmd/cqrs-lint` CLI tests** — exit-code decision table, `countFindings`/`emitSummary`, `--json` schema, `--verbose` rule status, and an end-to-end violating-fixture + suppression round trip (the ADR-0004 gate is no longer the least-tested code in the repo).
- **SQLite file-backed integration tests** — WAL concurrency smoke (4 sources × 10 items in parallel against one file), file-DB roundtrip harness (`newFileDBStack`), plus the DLQ persistence test.
- **API authentication** — `api.WithAPIKey(key)`: constant-time key check on every route except `/health` and the OpenAPI docs, 401 with `WWW-Authenticate` + JSON body, and the `apiKey` security scheme declared in the OpenAPI document. Off by default.
- **API rate limiting** — `api.WithRateLimit(perMinute)`: token bucket guarding `POST /sync` only (reads stay unlimited), 429 with `Retry-After` and a JSON body. Off by default.
- **API pagination headers** — `GET /items` now returns `X-Total-Count` and `X-Next-Cursor` (opaque base64 cursor mapped onto `ItemFilter.Offset`; bad cursors are 400). Offset paging still works; cursors are the stable contract.
- **Validation middleware swap** — the hand-rolled `commandValidationMiddleware` is replaced by `middleware.CommandValidation` + `WithLogger`: same checks, same `ErrInvalidInput` chain, plus failure logging and library classification.
- **`scenario` DSL adoption** — flagship decider behaviors (resurrection, conflict-winner, first-sync) specified with `go-cqrs-lite/scenario/v4` Given/When/Then (`pkg/cqrs/scenario_test.go`); new decider tests follow this convention. (`eventtest` has no released version — not adopted.)
- **`pkg/cqrs` coverage 87.7%** — CountByType across all three surfaces, store-factory error branches, legacy upcast matrix, nil guards, cleanup path.
- **Suppression audit trail** — cqrslint findings carry `SuppressedBy`/`SuppressedReason` (which directive silenced them and why), `--json` output includes both, and directives naming a nonexistent internal-scheme rule now WARN (fails `--strict`) while library-scheme rule IDs (C017/E005-style) are respected as cross-linter directives. The directive parser accepts both `ignore C0005 reason` and `ignore(C0005) reason` forms so one comment serves both linters.
- **Upcaster registry adoption** — V1/V2 `ItemSynced` events are upcast to V3 at the store read boundary (`event.DecorateStore` + `schema.UpcastSourceTransform` in both backends): legacy actor/repo fields fold into `Attributes`, every downstream consumer (fold, projection, replay, export) sees current-schema payloads. New events are stamped `WithSchemaVersion(3)` at creation — closing a real data race where the registry's in-place version stamping mutated events shared with concurrent bus readers.
- **Event export** — `stack.ExportEvents` (NDJSON) and `stack.ExportEventsCSV` write the full journal with identity, positioning, base64 payload, correlation + causation metadata.
- **`provider/github` live PAT smoke test** — env-gated (`GITHUB_PAT`) one-round-trip proof of the released kit wiring; skips without the env var.
- **OpenAPI error schemas** — per-endpoint error responses are declared (400/401/429/500/408 as applicable, option-aware), so generated clients get real error contracts.
- **Type-safety cluster** — branded `id.ContentHash` (named string type: literal-compatible, signature-safe), typed `Attributes` accessors (`ActorLogin()`, `RepoName()`, …) with canonical `Attr*` key constants, and `ItemFilter.Validate()` rejecting negative limit/offset.
- **Per-sync conflict resolver** — `SyncOptions.ConflictResolver` overrides the stack-configured strategy for one run through the new optional `ResolverAwareStore` seam (`CQRSStack.SyncItemsWithResolver`); precedence: command > per-sync option > stack config.
- **`CQRSConfig.Validate()`** — rejects unknown backends at construction sites (the `govalid`-tags TODO was pivoted: govalid is a buildflow-internal generator, not a proxy-resolvable module — real validation beats unverifiable tags).
- **ADR-0009** — v0.6 vocabulary decisions recorded (AggregateID→StreamID rename, SyncResult/SyncSummary consolidation, deliberate DeriveStreamID encoding divergence); the divergence is pinned at the definition site.
- **Full-pipeline benchmarks** — `BenchmarkPipeline_Sync10kItems` (~62µs/item memory), `BenchmarkPipeline_Replay10kEvents` (~2.8ms checkpoint-bounded reopen), `BenchmarkPipeline_SQLiteGrowth` (~250µs/item on a growing file DB).
- **CONTRIBUTING.md** — real contributor guide: architecture map, dependency rules, file-split conventions, testing requirements (scenario DSL, file-backed SQLite, ≥87% cqrs coverage), linter suppression policy.
- **Standalone CI leg for `provider/github`** — a dedicated workflow job builds and race-tests the nested module in isolation (`GOWORK=off`), so the module graph it ships is the graph CI proves. CI also dropped all private-repo auth (GOPRIVATE/SSH-agent): the repository and its dependencies are public now.
- **dprint in the devShell** — `flake.nix` now installs `dprint` (v0.56) alongside the Go tooling, so `dprint check` / `dprint fmt` (JSON/YAML/Markdown/Dockerfile per `dprint.json`) run locally with format parity.

### Changed

- **`errors.AsType` modernization driven to zero** — `erraudit lint ./... --type-aware` reports 0 violations; the one `exec.ExitError` case in the CLI test harness migrated to `errors.AsType`. Remaining buildflow `hierarchical-errors` findings (17) are deliberate patterns (cleanup-log-only, ignored writes, defer-Close) dispositioned to formal-track; the `.buildflow.yml` `suppress:` key was tested and proven a silent no-op.
- **Sentinel errors interface-typed** — `pkg/errors` + `pkg/crdt` sentinels are now declared `var X error = errorfamily.New*` so no package exports a concrete error type as an API surface.
- **Renamed the internal linter command `cqrs-lint` → `localsync-lint`** to remove the name collision with go-cqrs-lite's library `cqrs-lint` (the pinned 203-rule CI gate). The `//cqrs-lint:` directive vocabulary is unchanged on purpose: one inline comment can target both linters.
- **CI/tooling hardening** — dprint format check added to the lint job (pinned `0.56.1`), `windows/amd64` added to the build matrix (`CGO_ENABLED=0`; modernc.org/sqlite is pure-Go), three stale `.golangci.yml` exclusion blocks deleted (paths from pre-restructure layouts), inert pre-commit hooks formally disabled (documented decision), and `verify-release.sh` gained a docs-consistency section running `check-doc-counts.sh` — which immediately proved itself by failing the v0.5.0 smoke run on the 358→378 test-count drift.
- **Release integrity verified (post-public-flip)** — `v0.5.0` and `provider/github/v0.1.0` tags are pushed (annotated), both GitHub Releases exist, and proxy.golang.org serves both versions with the correct `@latest` for the core and provider modules; no bump release needed.
- **Stale `vendorHash` re-pinned** — the 2026-09-05 evening dependency refresh changed `go.mod`/`go.sum` after the flake's `vendorHash` was recorded, silently breaking `nix build` / `nix flake check` with a hash mismatch; the hash now matches again and both pass.
- **`exhaustruct` → `exhaustruct_v5`** — golangci-lint v2.13 deprecates the old linter; the config migrates to `settings.exhaustruct_v5.ignore-patterns` (same stdlib patterns) and renames the linter in every exclusion rule. Full lint is clean with no deprecation warning.
- **Library cqrs-lint gate is CI-ready** — the error-gated `go-cqrs-lite` linter step is back in the `lint` job, auto-enabling once the `SSH_PRIVATE_KEY` secret (deploy key with read access to the private `go-finding` module) is added and skipping with a notice until then; the step no longer masks a linter failure with its git-config cleanup (exit code is captured), and the local devShell invocation stays documented.
- **`provider/github` README verified against the `FetchPages` rebuild** — every prose claim re-checked against code (concurrency 3, `MinRemaining` 10, `MaxWait` 15m, retries 3 with 1s→30s backoff, short-page stop, error family, PAT smoke test); the pagination bullet now names the sequential page-1 probe and the `WithBaseURL` row states its non-chainable `(client, error)` return.
- **`provider/github` v0.1.0 tagged** — the module's parent pin moved from a master pseudo-version to the released `go-localsync v0.5.0`, and its `go-github-kit` dependency bumped to v0.3.0. `FetchAll` now delegates pagination to `githubkit.FetchPages` (concurrency + short-page early stop come from the kernel), and `wrapGitHubError` drops its native-rate-limit shims in favor of the kit's classification while preserving the original cause in the error chain.

### Fixed

- **`POST /sync` timeout mapping** — a canceled request now maps to 499 (client closed request) and an exceeded deadline to 504, matching `pkgerrors.HTTPStatus`; the previous 408 mapping is gone and the OpenAPI document declares 499/504 (pinned by `pkg/api/timeout_test.go`).
- **Upcaster pass-through could mutate stored events (residual race)** — a legacy-versioned `ItemSynced` event (schema stamp 1/2) whose payload already carried `Attributes` was handed back to the upcaster registry as the STORED pointer; the registry's in-place schema-version stamp then raced concurrent readers (the memory backend serves shared event pointers). Such events now always rebuild a private copy, making "the registry stamp never lands on a stored event" structural rather than data-dependent. Pinned by a pointer-identity test and a 100-stream barrier-start concurrent replay regression (verified: 3 DATA RACEs against the old logic, clean after; 5× race-clean).

## [0.5.0] - 2026-09-05

The provider-extraction release: GitHub integration moves into an optional
nested module, and the module registry gains the `ErrProviderUnavailable`
sentinel. Also carries the accumulated cqrslint tooling features and the
go-standard flake migration.

> Tag note: the `v0.4.2` tag is a retroactive proxy-sweep tag pointing at an
> April 2026 snapshot (it predates v0.4.1's content); it has no changelog
> section of its own. This release re-establishes `@latest` on current code.

### Added

- **`provider/github` optional nested module** — a GitHub events `provider.Provider` built on `go-github-kit` v0.2.0 (token auth, rate-limit gating fed from response headers, retry with backoff). Extracted from github-local-sync's proven `internal/github` package. The core module stays free of GitHub dependencies; consumers opt in by requiring `github.com/larsartmann/go-localsync/provider/github`. Development runs through the new root `go.work`; the module's parent pin is a master pseudo-version until this release.
- **`ErrProviderUnavailable` sentinel** (`pkg/errors`, transient) — fills the vocabulary gap for "the provider API could not be reached or kept failing", mapped from the kit's `ErrAPIUnavailable` and unclassified GitHub failures. User-facing message template registered.
- **cqrslint suppression directives** — `//cqrs-lint:ignore` comments suppress individual findings, with verbose output and improved report formatting (`emitSummary` simplification, bundled emit parameters).

### Changed

- **go-cqrs-lite v4.9 stack** — dependency refresh across all go-cqrs-lite modules, pinned CI actions, dprint doc formatting.
- **Command dispatch on `ExecuteRef`** — both command handlers migrated off the deprecated `Repository.Execute` pair form to `ExecuteRef` with `id.NewStreamRef`, clearing the staticcheck SA1019 findings and unblocking the future go-cqrs-lite v5 upgrade; the `NewCQRSStack` failure-path cleanup was extracted into a named helper at the same time.
- **Flake migrated to the go-standard module** — flake.nix reduced from 237 to 86 lines.

### Fixed

- `pkg/api/map_error_test.go` did not compile: `errors.AsType` requires a type parameter that satisfies `error`, but the test passed a bare `interface{GetStatus() int}`. Now asserts against `huma.StatusError` directly. Pre-existing at v0.4.2-era master; surfaced by the first full-suite run during the provider extraction.
- `nix build .#default` failed on master: `mkPreparedSource`'s private-dep validation flagged five LarsArtmann modules that are all public now (`go-codec`, `go-cqrs-lite/codec`, `go-flightrecorder`, `go-idempotency`, `go-retry`), and the pinned private `go-cqrs-lite` master snapshot no longer contains the extracted `codec` module at all. Removed the obsolete `git+ssh` inputs and the `deps` map — everything resolves from the module proxy, pinned by `go.sum`, matching what local `go build`/`go test` already used — and rotated `vendorHash` accordingly. `nix build` and `nix flake check` (treefmt, cqrs-lint) pass.

## [0.4.1] - 2026-07-23

A maintenance release: go-cqrs-lite v4.1 dependency bump with full deprecation cleanup, build-system migration from committed `vendor/` to Nix `mkPreparedSource`, and internal refactoring.

### Changed

- **go-cqrs-lite v4.1.0** — all modules bumped from v4.0.x to v4.1.0 (`codec`, `command`, `decider`, `dispatcher`, `event`, `id`, `middleware`, `projection`, `projectionhost`, `snapshot`, `storage`, `storage/memory`, `watermill`). Adopted the upstream `AggregateID`→`StreamID` and `AggregateType`→`StreamType` vocabulary: migrated every deprecated type reference (`cqrsid.AggregateID`→`cqrsid.StreamID`, `ParseAggregateID`→`ParseStreamID`, `NewAggregateID`→`NewStreamID`, `event.AggregateType`→`event.StreamType`). Source-compatible — the old names are retained as type aliases.
- **go-error-family v0.8.0** — bumped from v0.7.0.
- **Build system: `vendor/` → `mkPreparedSource`** — replaced the committed `vendor/` directory with Nix `mkPreparedSource` for hermetic builds. Eliminates the force-add workaround for private deps and shrinks the repository. CI configured with SSH agent for private repo access.
- **cqrslint refactoring** — extracted `Finding` helper constructors applied consistently across all 10 checks; extracted `queryRows` helper; consolidated `AssertInt`→`AssertEqual`; shared per-source lock across sync entry points; simplified `isSelectorType` with `slices.Contains`.
- **cqrs-lint C0001** — updated to recognize `event.StreamType` (the new canonical type name) while still accepting the legacy `event.AggregateType` alias.
- **Dependency refresh** — `golang.org/x/exp` refreshed; nix inputs updated to latest nixpkgs; `huma/v2` bumped to v2.39.0.

### Fixed

- **devShell `GOFLAGS` propagation** — `GOFLAGS=-tags=goexperiment.jsonv2` was not inherited by buildflow's native go subcommands (`test-race`, `go-fix`, `go-auto-upgrade`, `govalid-generate`), causing misleading partial-green results. Now documented and wired consistently.
- **`slices.Contains` migration bug** — masked iterator-semantics regression from the Go 1.26 `slices` package migration.

### Removed

- **Unused `warningAt` helper** — dead code in `internal/cqrslint/finding.go`.

## [0.4.0] - 2026-07-18

A major release: tombstone soft-deletes, resilient managed projection (`projectionhost.Host`), a static architectural linter (`cqrs-lint`), a full error-handling overhaul, de-githubification of the domain model, and the go-cqrs-lite v4 + JSON v2 migration.

### Added

- **Tombstone soft-delete (ADR-0005)** — tombstones replace hard-deletes; tombstoned items keep full history on `Item.Tombstone`; re-syncing a tombstoned item resurrects it via projection upsert; opt-in `SyncOptions.Reconcile` tombstones upstream-gone items (`ReasonUpstreamGone`). New `DecideTombstone` + `TombstoneItemCommand`.
- **`projectionhost.Host` catch-up (ADR-0006)** — managed batch-drainer for resilient SQLite projection: checkpoint persistence, crash auto-restart with backoff, dead-letter queue for poison messages. Replaces the prior bare `replayJournal`. Mutex-guarded version-gate prevents stale replay events from resurrecting tombstoned rows.
- **`cqrs-lint` architectural linter** — `internal/cqrslint` enforces 10 invariants (C0001–C0010) for `pkg/cqrs` (ADR-0004 scope guard). CLI: `cmd/cqrs-lint`. Zero third-party deps (stdlib `go/parser` only).
- **Error-handling overhaul** — `go-error-family` constructors with intrinsic classification (Rejection, Transient, Infrastructure); central `pkgerrors.HTTPStatus` translator (per-sentinel overrides + family defaults; `context.Canceled`→499, `context.DeadlineExceeded`→504); `WithCtx`/`WithCtxf`/`InvalidField` structured context; partial-sync surfacing (Transient `ErrPartialSync` → HTTP 200-with-result rather than discarding synced items).
- **Schema V3 (de-githubify)** — `pkg/data/schema` `Version` extended to V3 (removing GitHub-specific fields from the model). Carried on every `model.Item` for forward event migration (upcasting). V1/V2 were introduced in v0.2.0.
- **govulncheck + gitleaks** in CI — reachability-based dependency CVE scanning and full-history secret scanning (vendor/ excluded).

### Changed

- **Breaking: provider-agnostic domain model (ADR-0007)** — removed `ActorLogin`, `ActorAvatarURL`, `RepoName`, `RepoURL` from `provider.Item` and `model.Item`. Provider-specific content flows through `Attributes map[string]string`. `hasChanged` is ContentHash-first (with `UpdatedAt`/`Type` fallbacks). SQLite `actor_login` index dropped; indexes now on `type`, `created_at`, `(type, created_at)`. `GET /items` no longer accepts `actor`/`repo` query params.
- **Breaking: go-cqrs-lite v4 migration** — all modules moved to v4 paths (`event/v4`, `command/v4`, `decider/v4`, `id/v4`, `codec/v4`, `projection/v4`, `projectionhost/v4`, `snapshot/v4`, `storage/v4`, `storage/memory/v4`, `middleware/v4`, `watermill/v4`). Adopted `encoding/json/v2` (gated behind `GOEXPERIMENT=jsonv2` build tag in Go 1.26).
- **Strategic pivot (ADR-0008)** — re-centred the SDK as a single-aggregate, pull-only sync toolkit; explicit scope boundary against multi-aggregate generalisation (ADR-0004). The broader `Host` framework pivot proposed in ADR-0008 was **not** executed — the project stayed within ADR-0004 scope.
- **Per-source serialization** split into lock-free `runSync`/`runSyncIncremental` to avoid re-entrant deadlock when incremental falls back to full.
- **CI hardening** — `cancel-in-progress`, `paths-ignore`, per-job `timeout-minutes`; cross-platform build matrix verifies library compilation (linux/darwin × amd64/arm64).
- **Dependencies** — `go-error-family` v0.7.0, `huma/v2` v2.39.0, `go-branded-id` v0.3.2, `modernc.org/sqlite` v1.54.0, `charm.land/log/v2` v2.0.0.
- `flake.nix` now derives `version` from git revision; `CONTRIBUTING.md` streamlined.

### Removed

- **CRDT distributed-sync types** — `VectorClock`, `Operation[T]`, `SyncMessage`, and the multi-writer protocol types deleted. A single-writer pull mirror has no second writer and no causal ordering to track (see ADR-0004).
- **QueryDispatcher** — removed by design. Reads call the `ReadModel` directly (see note in `stack_adapters.go`); the command side stays dispatched for logging/retry/validation middleware.

### Fixed

- **Projection version-gate TOCTOU race** — concurrent live + replay delivery for the same aggregate could let a stale event resurrect a tombstoned row. Now mutex-guarded so deliveries serialize per aggregate.
- **`NewCQRSStack` resource leak** — error paths after store creation could leak store/bus/db/goroutine resources. Now uses named returns + cleanup defer.
- **Aggregate-ID collision and `hasChanged` data loss** — content-hash comparison was silently dropping items; aggregate-ID parsing was swallowing errors and caching zero values.
- **Partial-sync error dropping** — `ConflictAwareSyncer` silently dropped item-level errors when some items failed validation/persistence but the run completed. Now surfaces `ErrPartialSync` (Transient).
- **SQLite read model dropping columns** — `ContentHash` and `SchemaVersion` were silently dropped on upsert. SQLite error chains now preserved via multi-`%w` wrapping so `errors.Is` and `errors.As` both work.
- **`Since` filter boundary** — SQLite exclusive `>` corrected to inclusive `>=` to match the memory read model.

## [0.3.0] - 2026-06-23

### Changed

- **Breaking: `ActorID` renamed to `ActorLogin`** (`pkg/id`). The type previously called `ActorID` actually represents an external provider actor login (e.g. a GitHub username like `"octocat"`), and every field using it was already named `ActorLogin` — the type name didn't match its purpose. The rename also resolves a P0 seam violation where three incompatible types across sibling repos were all named `ActorID`. Affected public API:
  - `ActorBrand` → `ActorLoginBrand` (phantom type)
  - `ActorID` → `ActorLogin` (type alias)
  - `NewActorID(v string)` → `NewActorLogin(v string)` (constructor)
  - Field types updated across `pkg/data/model` (`Item.ActorLogin`, `ItemFilter.ActorLogin`, `WithActorLogin`), `pkg/provider` (`Item.ActorLogin`), `pkg/api`, and `pkg/cqrs`. Consumers referencing `id.ActorID` or `id.NewActorID` must update to `id.ActorLogin` / `id.NewActorLogin`.
- **go-error-family upgraded from v0.4.0 to v0.5.0.** Vendored dependencies re-synced. No code changes needed — `RegisterTemplate`, `NewRejection`, `Wrap`, and `IsRetryable` APIs are backward-compatible (global functions now delegate to `DefaultRegistry`).

## [0.2.0] - 2026-06-22

### Added

- **go-cqrs-lite v3 migration** — all modules moved to v3.0.0 paths (`event/v3`, `command/v3`, `query/v3`, `decider/v3`, `id/v3`, `codec/v3`, `snapshot/v3`, `storage/v3`, `storage/memory/v3`, `middleware/v3`, `watermill/v3`).
- **`watermill/v3` EventBus** — replaces the deleted `memory.NewMemoryBus` for in-process event delivery. `BlockPublishUntilSubscriberAck` preserves read-your-writes on synchronous projection.
- **Exported `ConflictWinner` constants** — `ConflictWinnerRemote` / `ConflictWinnerLocal` plus `ParseConflictWinner` for safe payload→enum decoding (unknown values default to remote-wins).
- **`runner.go` projection wiring** — direct `bus.SubscribeAll` for synchronous live event delivery, plus a background `replayJournal` (reads all persisted events via `Journal.ReadAll`) for SQLite catch-up. The idempotent projection tolerates replay/live overlap, so no checkpoint store is needed.
- **DTO/domain boundary** — `item_adapter.go` converts `provider.Item` (DTO) → `model.Item` (domain entity with `SchemaVersion`). Decider, read model, events, and conflict resolver now all operate on `*model.Item`.
- **`pkg/data/schema`** — `Version` (V1/V2) with `CurrentVersion()`, `Valid()`, carried on every item for forward event migration (upcasting).

### Changed

- **Logging middleware** — replaced the hand-rolled logging adapter with `middleware.EventLogging` from go-cqrs-lite v3.
- **`event.Version`** — migrated from `int` to `uint64` (`Increment()`, `Add()`); no `int()` casts needed.
- **`uint64` conflict winner** — winner decoded via `ParseConflictWinner` rather than raw string compares.
- **go directive** bumped to `1.26.4`.
- **Dependencies** — `go-branded-id` v0.3.1, `go-error-family` v0.4.0, `modernc.org/sqlite` v1.52.0, `huma/v2` v2.38.0.

### Removed

- **All provider implementations and the example CLI** — the SDK is now a **pure contract library**. `pkg/providers/`, `cmd/examples/github-sync`, the `go-github` dependency, and `caarlos0/env` were removed. The reference GitHub provider + CLI moved to the consumer app [`github-local-sync`](https://github.com/larsartmann/github-local-sync).
- **Checkpoint stores** — `SQLiteCheckpointStore` / `MemoryCheckpointStore` removed; the v3 projection is idempotent and needs no checkpoint.
- **`projection.Runner`** — go-cqrs-lite v3 dropped `projection/`; replaced by `runner.go` (see Added).
- **Dead config** — `RemoteURL`, `AuthToken`, Push/Pull flags, and the Turso backend removed in favor of local SQLite.

### Performance

- **Rate limit cache** — `RateLimitCache` caches rate-limit info between calls to avoid redundant API requests; concurrency-safe.
- **Concurrent `FetchAll`** — pages fetched in parallel (`MaxConcurrentFetches`, default 3).
- **SQLite optimizations** — WAL mode, aggregate-ID `sync.Map` cache, scan-path query optimization.
- **`CountByType`** — fixes the `GetStats` N+1 query; `TypeCounts` added to the API `StatsOutput`.

### Fixed

- **`flake.nix` release coherence** — bumped package `version` to 0.2.0 (was stuck at 0.1.0), removed stale `mainProgram = "go-localsync"` and the broken `apps.default` (it used `getExe` on a library, so `nix run` failed). The SDK is a pure contract library with no binaries as of this release.

## [0.1.1] - 2026-06-14

### Changed

- Upgraded go-cqrs-lite sub-modules to v2.3.0 (`schema/v2` v2.2.0 → v2.3.0; `event/v2` → v2.3.1).
- Performance sprint: rate limit cache, concurrent `FetchAll`, SQLite WAL mode, aggregate-ID cache, `CountByType`, `TypeCounts` in API stats.
- Vendored private dependencies for offline nix builds (`vendor/` + `vendorHash = null`).

## [0.1.0] - 2026-05-28

### Added

- **CQRS architecture** — full event-sourced storage via go-cqrs-lite (Decider, ReadModel, Projection, Stack). No legacy CRUD path.
- **Deterministic aggregate IDs** — SHA256→hex from (source, sourceID) for idempotency.
- **Conflict-aware sync** — `DecideSync` detects conflicts and emits `ItemConflictFound` events; pluggable `crdt.ConflictResolver[T]` with `LWWResolver` default.
- **CRDT primitives** — `VectorClock`, `Operation[T]`, `Conflict[T]`, `SyncMessage` in `pkg/crdt`.
- **Dual backends** — in-memory and SQLite (`modernc.org/sqlite`, no CGo) with snapshots.
- **Branded IDs** — 6 phantom-type IDs via go-branded-id.
- **HTTP API** — Huma v2 with `GET /items`, `GET /stats`, `POST /sync`, `GET /health`; OpenAPI 3 spec.
- **Error taxonomy** — `go-error-family` constructors with intrinsic classification (Rejection, Transient, Infrastructure).
- **Nix flake** — devShell + `buildGoModule`.
- **GitHub Actions CI** — test, lint, build, release jobs.
