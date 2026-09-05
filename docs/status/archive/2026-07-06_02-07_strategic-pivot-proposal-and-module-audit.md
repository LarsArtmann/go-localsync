# Status Report: Strategic Pivot Proposal + Module Audit

**Date:** 2026-07-06 02:07
**Session focus:** Answering "how to build on go-cqrs-lite for github-local-sync, bank-sync, DiscordSync"

---

## a) FULLY DONE

### 1. Three-consumer adoption study (complete, verified)

Researched all three target consumers by reading actual source code, not just docs:

- **github-local-sync:** Uses go-localsync `pkg/provider` + `pkg/errors` + `pkg/id` + `pkg/testutil` ONLY. Already imports `go-cqrs-lite/stack/sqlite` + `stack/v3` — it bypassed go-localsync's `pkg/cqrs/` entirely. Has its own Branch aggregate, events, projections, sync loop (~700 lines of wiring).
- **bank-sync:** Does NOT use go-localsync at all. Full CQRS via go-cqrs-lite (decider + command + event + storage + watermill). Own `bank.Bank` provider interface. ~1,050 lines of wiring.
- **DiscordSync:** Does NOT use go-localsync at all. Push-based (WebSocket). Event-capture-only CQRS (no decider, no commands). Uses `projectionhost` + custom SQLite DLQ (~200 lines). ~1,520 lines of wiring.

**Verification:** All claims cross-checked against actual `go.mod` files and `.go` import statements via sub-agents. One false claim corrected (initially said github-local-sync uses 3 packages; actually uses 4 — `testutil` too).

### 2. go-cqrs-lite `stack/` layer analysis (complete)

Confirmed that go-cqrs-lite v3.5+ ships a batteries-included stack layer that replaces go-localsync's entire `pkg/cqrs/` (1,773 lines):

- `stack/sqlite.New(dsn)` — one-call stack assembly
- `stack.Materialize[V, K]` — tombstone-aware read-model builder
- `bundle.RunProjections()` — projection runner
- 7 presets (memory, sqlite, pebble, postgres, turso, bench)

### 3. Strategic proposal written and revised (complete)

- `docs/strategy/2026-07-05_localsync-v2-sync-toolkit-proposal.md` — **written twice**. First version proposed a "thin toolkit" (loose primitives). User feedback ("want it as easy as possible") triggered full rewrite to **opinionated Host framework** vision.
- `docs/adr/0008-pivot-to-sync-toolkit.md` — formal decision record matching the revised proposal.

Key claim: the three consumers hand-write **~3,270 lines of structurally identical integration boilerplate**, and 2/3 get it wrong (no checkpoint, no DLQ, no graceful shutdown).

### 4. Five go-cqrs-lite module audit (complete)

Investigated idempotency, signing, encryption, projection, projectionhost — line counts, test counts, consumer adoption:

| Module         | Lines | Tests | Consumer adoption                     |
| -------------- | ----- | ----- | ------------------------------------- |
| projection     | 186   | 5     | All 3 (direct)                        |
| projectionhost | 2,320 | 23    | DiscordSync only                      |
| idempotency    | 960   | 25    | Indirect only (nobody calls directly) |
| signing        | 2,553 | 26    | Zero                                  |
| encryption     | 4,764 | 78    | Zero                                  |

---

## b) PARTIALLY DONE

### 1. Host API design — drafted but unvalidated

The Host API in the proposal (`NewHost`, `WithAggregate`, `WithPullSync[T]`, etc.) is **designed on paper but not validated against actual go-cqrs-lite types**. I didn't verify:

- Whether `stack.Bundle` actually exposes the fields the Host needs (Subscriber, Journal, CheckpointStore, etc.)
- Whether `decider.Decider` is the correct type signature for `WithAggregate`
- Whether `stack.Materialize` can be composed with the Host's projection registration
- Whether the generic `PullSyncer[T]` type parameter approach compiles in context of the Host builder pattern

### 2. De-GitHubify follow-up — from previous session, uncommitted

The previous session completed the de-GitHubify refactor (ADR-0007, 15 production files + 19 test files, 194 tests passing). That work IS committed (`66abf0c`). But the follow-up items are NOT started:

- Test function renames in `sqlite_readmodel_filter_test.go`
- Migration test for pre-V3 databases
- Event upcast test
- `DOMAIN_LANGUAGE.md` update
- `README.md` update

### 3. Consumer boilerplate measurement — estimated, not precisely counted

The "~3,270 lines" figure is an estimate based on file line counts, not a precise line-by-line audit of what's boilerplate vs domain logic. The real number could be ±30%.

---

## c) NOT STARTED

### From this session's proposal (Phase 1–4)

Nothing from the execution plan is started. All 4 phases are proposal-only:

- Phase 1: Build the Host (0/7 steps)
- Phase 2: Validate against github-local-sync (0/3 steps)
- Phase 3: Deprecate + remove old packages (0/4 steps)
- Phase 4: Consumer adoption (0/2 steps)

### From previous session's de-GitHubify follow-up

- Test function renames (cosmetic)
- Migration test for pre-V3 databases
- Event upcast test for V2-format payloads
- API attribute filtering (`?attr[key]=value`)
- `pkg/provider/github` reference package
- `DOMAIN_LANGUAGE.md` + `README.md` updates

### Project health items

- AGENTS.md dependency table stale (says v3.4.0/v3.3.0 — actual is v3.5.0, go-cqrs-lite is now v3.6.0)
- AGENTS.md coverage table stale
- `golangci-lint` + `gofumpt` check on de-GitHubify changes
- Commit the new docs (`docs/adr/0008`, `docs/strategy/`)

---

## d) TOTALLY FUCKED UP / MISTAKES MADE

### 1. Didn't notice go-localsync ALREADY has projectionhost integration

I discovered this only when writing the status report. `pkg/cqrs/runner.go` already uses `projectionhost.Host` with checkpoint, DLQ, crash-restart, and graceful drain — exactly what I proposed as "new." **My proposal to "bake projectionhost in as a default" described work that already exists** (at least within `pkg/cqrs/`, though not in a reusable Host abstraction). I should have read `runner.go` before proposing.

The nuance: go-localsync HAS the integration but it's **buried inside the single-aggregate `CQRSStack`** — not exposed as a reusable Host. The proposal's value is making it reusable, not inventing it. But my framing was misleading.

### 2. Used `MemoryDeadLetterStore` in production path

`runner.go:42` uses `projectionhost.NewMemoryDeadLetterStore()` — the in-memory DLQ. This means **poison messages are lost on restart**. DiscordSync wrote a 200-line `SQLiteDeadLetterStore` to fix this. go-localsync should have promoted that to the library. I noticed this during the module audit but didn't flag it as a bug in the report — just mentioned it in passing.

### 3. First proposal was wrong direction

The first version of the proposal described a "thin toolkit" with loose primitives. The user immediately redirected: they want it easy, not loose. I should have read the user's intent better — "build on top of go-cqrs-lite and make it better and easier" clearly implied opinionated assembly, not loose primitives.

### 4. Didn't run a single test or build this session

Pure analysis/proposal session. No code was written, no tests run, no builds verified. All claims about go-localsync code are from reading, not from running.

### 5. Didn't check go-cqrs-lite version lag

go-localsync's `go.mod` pins go-cqrs-lite at v3.5.0, but go-cqrs-lite is now at v3.6.0. The consumers (bank-sync, DiscordSync) are already on v3.6.0. I noticed this only at the end of the session while writing the status report.

### 6. Overcounted idempotency as "indirect"

Said all three consumers use idempotency as indirect. That's technically true (it's a transitive dependency via stack), but github-local-sync references "idempotency" in code comments only — it doesn't wire the middleware. The claim was misleading by omission.

---

## e) WHAT WE SHOULD IMPROVE

### Process improvements

1. **Read existing integration code before proposing new abstractions.** The `runner.go` miss was embarrassing. Before proposing "bake projectionhost in," I should have searched for existing projectionhost usage.

2. **Validate API designs against actual types.** The Host API is designed against my mental model of go-cqrs-lite, not against the actual `stack.Bundle` struct definition. Before writing more proposal code, read `stack/bundle.go` to see what fields are actually exposed.

3. **Don't split proposal and ADR into separate documents until the vision is agreed.** Writing ADR-0008 before user buy-in was premature — the ADR now records a proposal that hasn't been accepted.

4. **Run builds/tests even in analysis sessions.** The go-cqrs-lite version lag and the MemoryDLQ-in-production issue would have surfaced sooner.

### Technical improvements

5. **The proposal should address the go-cqrs-lite v3.5→v3.6 version lag.** It's an upgrade that needs to happen before or during the pivot.

6. **The Host design needs to decide: does it own the `stack.Bundle` or receive it?** The proposal says "consumer provides bundle, Host wraps it" but doesn't address lifecycle ownership (who calls `bundle.Close()`?).

7. **The `PullSyncer[T]` design needs to address how it interacts with `decider.Repository.Execute`.** The sync loop calls `repo.Execute(ctx, aggID, func(state) []event)` — but that's a go-cqrs-lite type. The `SyncSink[T]` interface needs to bridge to that.

8. **The reconciliation framework needs more design.** I proposed `Healer` interface + `CompositeHealer` but didn't think through: does reconciliation need access to the event store? The read model? Both? DiscordSync's reconciliation touches 5 different subsystems (attachments, avatars, GCS URLs, hashes, cleanup).

9. **The push provider interface is underdesigned.** DiscordSync's WebSocket Gateway has backpressure, rate limits, resume sessions, heartbeat — none of which fit cleanly into a `StreamProvider` interface.

---

## f) UP TO 25 THINGS WE SHOULD GET DONE NEXT

### Immediate (this proposal, before any code)

1. **Commit the new docs** (`docs/adr/0008-pivot-to-sync-toolkit.md`, `docs/strategy/`) — they're untracked
2. **Read `stack/bundle.go`** and verify the Host API design against actual Bundle fields
3. **Read `stack/materialize.go` fully** (only read 80/270 lines) — understand how Materialize composes with projection registration
4. **Decide: does the Host own the Bundle lifecycle or not?** (ownership question)
5. **Upgrade go-cqrs-lite from v3.5.0 to v3.6.0** in go.mod (consumers are already ahead)

### Phase 1: Build the Host (execution)

6. **Generalize `pkg/provider/`** — add push/streaming support alongside pull
7. **Extract `pkg/sync/retry.go`** as standalone zero-dependency module
8. **Build generic `PullSyncer[T]`** with `ChangeDetector[T]` + `SyncSink[T]` — validate compiles
9. **Build `pkg/reconcile/`** — `Loop` + `Healer` interface + `CompositeHealer`
10. **Build `pkg/projection/`** — promote DiscordSync's `SQLiteDeadLetterStore` (fixes the MemoryDLQ-in-production bug)
11. **Build `pkg/host/`** — `Host`, `NewHost`, `Option` pattern, `Run`, lifecycle, health checks
12. **Write integration test** — wire a test aggregate + projection + mock provider through the Host

### Phase 2: Validate (before deleting anything)

13. **Port github-local-sync to the Host** — simplest consumer, already uses stack/sqlite
14. **Extract learnings and refine Host API** based on real usage
15. **Create example app** in go-localsync repo (minimal sync demo)

### Phase 3: Deprecate + remove

16. **Mark `pkg/cqrs/`, `pkg/data/`, `pkg/api/` as deprecated** with migration guides
17. **Delete deprecated packages** (1,773 + 400 + 50 + 320 = 2,543 lines)
18. **Update AGENTS.md** — dependency table, coverage table, package descriptions
19. **Update README.md** — new value proposition, getting-started guide
20. **Tag v2.0.0**

### De-GitHubify follow-up (from previous session)

21. **Rename test functions** in `sqlite_readmodel_filter_test.go` (cosmetic)
22. **Write migration test** for pre-V3 SQLite databases
23. **Write event upcast test** for V2-format payloads

### Project health

24. **Run `golangci-lint run ./...` + `gofumpt -l pkg/`** on the de-GitHubify changes
25. **Fix the `MemoryDeadLetterStore` in production** — either promote SQLite DLQ now or flag as known debt in AGENTS.md

---

## g) TOP #1 QUESTION I CANNOT FIGURE OUT MYSELF

**Should the Host own the `stack.Bundle` lifecycle, or should the consumer retain ownership?**

This is a fundamental architectural decision I cannot resolve without your input:

- **Option A: Host owns the Bundle** (`host.NewHost(opts...)` creates the Bundle internally). Simpler for consumers — one call, one lifecycle. But it couples go-localsync to go-cqrs-lite's stack presets (which DB? which options? WAL? multi-DB?). It also means go-localsync re-exports stack configuration surface.

- **Option B: Consumer owns the Bundle** (`host.NewHost(bundle, opts...)` — what the proposal currently says). Consumer calls `sqlite.New()` themselves, passes it in. More flexible — consumer controls DB layout, pragma tuning, multi-DB splits. But the lifecycle question gets messy: who calls `bundle.Close()`? If the Host calls it on `Run` exit, the consumer can't use the Bundle after Host stops. If the consumer calls it, they must wait for Host drain to finish first.

- **Option C: Host wraps but doesn't own** (Host takes the Bundle, uses it, but never closes it — consumer closes after `Run` returns). This is what go-localsync's current `CQRSStack` effectively does. Clean separation but requires the consumer to understand drain ordering.

This decision cascades into the entire Host API surface. I can't design the Option pattern, the lifecycle methods, or the error handling until it's resolved. Every consumer does it differently today (github-local-sync: consumer owns; bank-sync: consumer owns; DiscordSync: DI container owns).

---

## Resolution (2026-07-22)

**ADR-0008 was never executed — the Host framework was never built.** The project stayed within ADR-0004 scope (single-aggregate, pull-only). Since this report:

- **`MemoryDeadLetterStore` in production** — **fixed**. The `projectionhost.Host` now uses a persistent DLQ (wired in session 26, shipped in v0.4.0).
- **projectionhost already existed** — the report's blind spot ("didn't notice go-localsync ALREADY has projectionhost integration") was correct; the integration was further hardened (v3 → v4, DLQ wired).
- **All 4 proposal phases remain 0%** — no `pkg/host/`, `pkg/reconcile/`, or `pkg/projection/` framework packages exist. `pkg/cqrs/` actually grew (~2,089 LOC).
- **go-cqrs-lite** upgraded from v3.5 to **v4** (JSON v2). The `stack/` layer the proposal referenced has evolved further.
- **Trigger to revisit:** a 3rd consumer hitting the boilerplate wall. No such consumer has appeared.

Full resolution in `docs/strategy/2026-07-05_localsync-v2-sync-toolkit-proposal.md` Resolution section.
---

## Resolution (2026-09-05 docs-health sweep)

ADR-0008 remains dormant by recorded decision; the DLQ gap was fixed (v0.4.0, SQLite-durable 2026-09-05); the module audit's ask/shipped deltas are superseded by the v4.9 stack + M-plan execution. Verified against the 2026-09-05 tree (`9625b1b`: v0.5.0, 309 core tests, CI green). Report fully resolved → archived.
