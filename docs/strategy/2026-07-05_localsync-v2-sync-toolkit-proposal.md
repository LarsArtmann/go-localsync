# go-localsync v2.0: The Sync Application Framework

**Date:** 2026-07-05
**Status:** Proposal (revised) — awaiting decision
**Author:** Analysis driven by three-consumer adoption study

---

## Executive Summary

go-localsync today is a 4,333-line single-aggregate engine used by zero consumers for its core. The de-GitHubify refactor (ADR-0007) fixed the vocabulary; the structural problem remains.

go-cqrs-lite v3.5 shipped a `stack/` layer that makes go-localsync's `pkg/cqrs/` (1,773 lines) redundant — github-local-sync already imports `stack/sqlite` directly.

But **replacing go-localsync with loose go-cqrs-lite primitives doesn't solve the real problem.** The three consumers hand-write **3,270 lines of integration boilerplate** across their codebases — stack wiring, bus subscription, projection registration, DLQ implementation, lifecycle management, sync loops, reconciliation. The integration is the hard part. Each consumer independently reinvents it, gets it slightly wrong, and maintains it alone.

**The revised vision:** go-localsync becomes the **opinionated sync application framework** — the layer that makes assembling go-cqrs-lite + sync + reconciliation genuinely easy. Not loose primitives. A batteries-included `Host` that pre-wires the CQRS stack, projection host, sync loop, reconciliation, and lifecycle into one coherent, correct whole. Generic over the domain; opinionated about infrastructure.

---

## 1. The Real Problem: Integration Boilerplate

### Measured Across Three Consumers

| Boilerplate Category                              | github-local-sync        | bank-sync                       | DiscordSync                |
| ------------------------------------------------- | ------------------------ | ------------------------------- | -------------------------- |
| Stack wiring (store+bus+codec+repo)               | 158 lines (`service.go`) | 276 lines (`infrastructure.go`) | 321 lines (`init.go`)      |
| Bus subscription (per-event-type `bus.Subscribe`) | 2 calls                  | **7 calls**                     | —                          |
| Command handler registration                      | 17 lines                 | 125 lines (`handlers.go`)       | —                          |
| Custom middleware (logging/recovery/causality)    | via stack                | 118 lines (`middleware.go`)     | via OTel                   |
| Projection wiring (checkpoint, DLQ, host)         | —                        | —                               | 160 lines (`lifecycle.go`) |
| Custom DLQ implementation                         | —                        | —                               | 200+ lines (`dlq_sql.go`)  |
| DI container                                      | —                        | —                               | 343 lines (`container.go`) |
| Sync glue (event→command routing)                 | 400 lines                | 200 lines                       | 100 lines (`storage.go`)   |
| Reconciliation loop                               | —                        | —                               | 200 lines (`reconcile.go`) |
| Upcasting pipeline                                | —                        | 56 lines                        | —                          |
| **Total infrastructure boilerplate**              | **~700 lines**           | **~1,050 lines**                | **~1,520 lines**           |

**Combined: ~3,270 lines of integration code that is structurally identical across all three projects** — same patterns, same wiring sequence, same edge cases, independently reimplemented three times.

### What Each Consumer Gets Wrong (That a Framework Would Get Right)

| Problem                               | How Consumers Handle It Today                                                               | What Goes Wrong                      |
| ------------------------------------- | ------------------------------------------------------------------------------------------- | ------------------------------------ |
| **Projection catch-up after restart** | github-local-sync + bank-sync: none (synchronous bus only — events before startup are lost) | Data loss on restart                 |
| **Poison message handling**           | github-local-sync + bank-sync: none (a bad event crashes the projection)                    | Unrecoverable state                  |
| **DLQ**                               | DiscordSync: 200-line custom SQLite implementation; others: none                            | Only DiscordSync survives bad events |
| **Graceful shutdown**                 | DiscordSync: 160-line lifecycle manager; others: none                                       | In-flight events lost on SIGINT      |
| **Retry classification**              | github-local-sync: via go-localsync; bank-sync + DiscordSync: ad-hoc                        | Inconsistent retry behavior          |
| **Per-source serialization**          | Only go-localsync has it                                                                    | TOCTOU races in bank-sync            |
| **Partial sync semantics**            | Only go-localsync has it                                                                    | Silent data loss in consumers        |

**These are correctness bugs, not cosmetic differences.** The framework doesn't just save lines — it prevents bugs.

---

## 2. The Target: An Opinionated Host

### Vision

go-localsync provides a `Host` — the opinionated assembly point that takes a go-cqrs-lite `stack.Bundle` and consumer-provided domain pieces, and returns a running, correctly-configured sync application.

```
Consumer provides:              go-localsync Host provides:
┌─────────────────┐             ┌──────────────────────────────┐
│ Domain types    │             │ Stack assembly (via bundle)  │
│ Decider[S]      │──────┐      │ Projection host lifecycle    │
│ Projections     │      ├─────▶│ Sync loop orchestration      │
│ Provider[T]     │      │      │ Retry/backoff/rate-limit     │
│ ChangeDetector  │──────┘      │ Reconciliation scheduling    │
│ Optional:       │             │ Graceful shutdown            │
│   Reconciler    │             │ Health checks                │
│   ConflictRes   │             │ SQLite DLQ (built-in)        │
└─────────────────┘             └──────────────────────────────┘
```

### What Consumer Code Looks Like (github-local-sync, ~30 lines)

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    // 1. One-call stack assembly (go-cqrs-lite)
    bundle, err := sqlite.New("branches.db",
        sqlite.WithOptimizations(),
        sqlite.WithEventDB("branches-events.db"),
    )
    if err != nil { log.Fatal(err) }
    defer bundle.Close()

    // 2. Opinionated host wiring (go-localsync)
    host, err := localsync.NewHost(bundle,
        // Domain
        localsync.WithAggregate(branch.Decider),
        localsync.WithProjections(branch.NewProjection, branch.NewFlowProjection),
        localsync.WithCommands(branch.RegisterCommands),

        // Sync
        localsync.WithPullSync[branch.Data](
            github.NewProvider(token),
            localsync.ContentHashDetector[branch.Data](),
            branch.NewSink,         // routes detected items → commands
        ),

        // Resilience
        localsync.WithRetry(localsync.DefaultRetry()),
        localsync.WithReconciler(1*time.Hour, branch.NewHealer()),
    )
    if err != nil { log.Fatal(err) }

    // 3. Run — blocks, manages all goroutines, graceful shutdown on ctx
    if err := host.Run(ctx); err != nil { log.Fatal(err) }
}
```

Compare to today: github-local-sync's `service.go` (158 lines) + `syncer.go` (263 lines) + `branch_flow_context.go` (200 lines) + `event_processor.go` (200 lines) = **~820 lines** of manual wiring and sync glue.

### What Consumer Code Looks Like (DiscordSync push, ~25 lines)

```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    bundle, err := turso.New("discord.db", turso.WithSync())
    if err != nil { log.Fatal(err) }
    defer bundle.Close()

    host, err := localsync.NewHost(bundle,
        localsync.WithProjections(
            discord.NewMessageProjection,
            discord.NewReactionProjection,
            discord.NewMemberProjection,
            discord.NewThreadProjection,
        ),
        localsync.WithPushSource(discord.NewGateway(botToken)),

        localsync.WithReconciler(1*time.Hour, discord.NewHealer(
            discord.HealAttachments(),
            discord.HealAvatars(),
            discord.HealGCSUrls(),
        )),
    )
    if err != nil { log.Fatal(err) }

    if err := host.Run(ctx); err != nil { log.Fatal(err) }
}
```

Compare to today: DiscordSync's `init.go` (321) + `container.go` (183) + `lifecycle.go` (160) + `shutdown.go` (74) + `dlq_sql.go` (200) = **~940 lines** of wiring.

---

## 3. The Package Map

### What Stays (existing, reusable)

| Package         | Purpose                         | Status                            |
| --------------- | ------------------------------- | --------------------------------- |
| `pkg/provider/` | Provider contract — pull + push | Exists, needs push generalization |
| `pkg/crdt/`     | Conflict resolution strategies  | Done — already generic            |
| `pkg/errors/`   | Error taxonomy + HTTP mapping   | Done — already standalone         |
| `pkg/id/`       | Sync-infrastructure branded IDs | Done — de-GitHubified             |
| `pkg/testutil/` | Mock provider + test helpers    | Exists                            |

### What's New

| Package           | Purpose                                                                                                                       | Est. Lines |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------- | ---------- |
| **`pkg/host/`**   | The opinionated assembly point: `Host`, `NewHost`, `HostOption`, `Run`, lifecycle, health checks                              | ~400       |
| `pkg/sync/`       | Generic `PullSyncer[T]` with pluggable `ChangeDetector[T]` + `SyncSink[T]`, retry/backoff (extracted from existing `sync.go`) | ~350       |
| `pkg/reconcile/`  | Reconciliation loop framework: interval scheduler + `Healer` interface + backoff                                              | ~150       |
| `pkg/projection/` | Built-in SQLite DLQ store (promoted from DiscordSync), projection registration helpers                                        | ~150       |

**Total: ~1,050 lines of framework code** replacing 4,333 lines of engine code + eliminating ~3,270 lines of consumer boilerplate.

### What's Dropped

| Package            | Lines | Replaced By                         |
| ------------------ | ----- | ----------------------------------- |
| `pkg/cqrs/`        | 1,773 | `go-cqrs-lite/stack/` + `pkg/host/` |
| `pkg/data/model/`  | ~400  | Consumer owns domain model          |
| `pkg/data/schema/` | ~50   | go-cqrs-lite `schema/` module       |
| `pkg/api/`         | ~320  | cqrs-htmx or consumer-owned         |

---

## 4. The Host Design

### Core Types

```go
package host

// Host is the opinionated assembly point. It owns the lifecycle of:
// - the go-cqrs-lite stack bundle (consumer-provided)
// - projection host (checkpoint + DLQ + auto-restart)
// - sync loop (pull or push)
// - reconciliation loop
// All goroutines are managed; ctx cancellation triggers graceful shutdown.
type Host struct {
    bundle     *stack.Bundle
    projHost   *projectionhost.Host
    pullSyncer *sync.PullSyncer   // nil if push-only
    reconciler *reconcile.Loop     // nil if no reconciliation
    health     *HealthChecker
}

// NewHost assembles a running sync application from a go-cqrs-lite bundle
// and consumer-provided domain pieces.
func NewHost(bundle *stack.Bundle, opts ...Option) (*Host, error)

// Run blocks until ctx is cancelled. Manages all goroutines.
// On ctx.Done: drains projections, flushes in-flight sync, closes bundle.
func (h *Host) Run(ctx context.Context) error

// HealthCheck returns nil if all subsystems are healthy.
func (h *Host) HealthCheck(ctx context.Context) error
```

### Options (Consumer's Configuration Surface)

```go
// Domain wiring
func WithAggregate(d decider.Decider) Option                    // register the decider
func WithProjections(projs ...ProjectionFactory) Option          // register N projections
func WithCommands(regs ...CommandRegistration) Option            // register command handlers

// Sync wiring
func WithPullSync[T any](p Provider[T], d ChangeDetector[T], s SyncSink[T]) Option
func WithPushSource(src EventSource) Option                      // for push consumers

// Resilience
func WithRetry(r RetryConfig) Option                            // default: exponential, 5 retries, ±25% jitter
func WithReconciler(interval time.Duration, h Healer) Option    // default: no reconciliation

// Infrastructure overrides
func WithDLQ(store DeadLetterStore) Option                      // default: SQLite DLQ
func WithSnapshot(strategy SnapshotStrategy) Option             // default: EveryNEvents(50)
func WithCodec(c Codec) Option                                  // default: CBOR
```

### The Sync Interfaces (Consumer-Implemented)

```go
// ChangeDetector determines what's new, changed, or gone.
// go-localsync ships ContentHashDetector[T] (default) and
// SeenKeysDetector[T] (for bank-sync-style dedup).
type ChangeDetector[T any] interface {
    Detect(ctx context.Context, fetched []T) (ChangeSet[T], error)
}

// SyncSink routes detected changes to the consumer's command/event path.
// The consumer implements this to dispatch their domain commands.
type SyncSink[T any] interface {
    OnNew(ctx context.Context, items []T) (partial error)
    OnChanged(ctx context.Context, items []T) (partial error)
    OnGone(ctx context.Context, keys []string) (partial error)
}

// Healer fixes drift during reconciliation.
// go-localsync provides CompositeHealer to combine multiple healers.
type Healer interface {
    Heal(ctx context.Context) (HealResult, error)
}
```

---

## 5. What Gets Eliminated Per Consumer

### github-local-sync: ~700 lines → ~30 lines

| Today                                                                              | After v2.0                                             |
| ---------------------------------------------------------------------------------- | ------------------------------------------------------ |
| `service.go` (158 lines — stack wiring, repo, bus subscribe, command registration) | `sqlite.New()` + `host.NewHost()`                      |
| `syncer.go` (263 lines — sync loop, rate-limit, pagination, session tracking)      | `host.WithPullSync()`                                  |
| `branch_flow_context.go` (200 lines — event→command routing)                       | `branch.NewSink` (consumer's SyncSink impl, ~50 lines) |
| `event_processor.go` (200 lines — GitHub event classification)                     | part of `branch.NewSink`                               |
| No checkpoint, no DLQ, no graceful shutdown                                        | **All included in Host**                               |

### bank-sync: ~1,050 lines → ~40 lines

| Today                                                               | After v2.0                             |
| ------------------------------------------------------------------- | -------------------------------------- |
| `infrastructure.go` (276 lines — 15-component manual wiring)        | `sqlite.New()` + `host.NewHost()`      |
| `handlers.go` (125 lines — 5 command registrations)                 | `host.WithCommands()`                  |
| `middleware.go` (118 lines — custom logging/recovery/causality)     | built-in Host middleware               |
| `upcasting.go` (56 lines — field rename upcaster)                   | go-cqrs-lite `schema/` module          |
| `projections.go` (300 lines — 5 projections, 7 bus.Subscribe calls) | `host.WithProjections()` (5 factories) |
| CLI context (183 lines — `SetupInfrastructure` wrapper)             | eliminated                             |
| No checkpoint, no DLQ, no reconciliation                            | **All included in Host**               |

### DiscordSync: ~1,520 lines → ~35 lines

| Today                                                                   | After v2.0                        |
| ----------------------------------------------------------------------- | --------------------------------- |
| `init.go` (321 lines — 6 init functions, projection host setup)         | `turso.New()` + `host.NewHost()`  |
| `container.go` (183 lines — samber/do DI container)                     | eliminated                        |
| `lifecycle.go` (160 lines — projection runtime, health check, shutdown) | built-in Host lifecycle           |
| `shutdown.go` (74 lines — signal handling, reconciliation start)        | built-in `host.Run(ctx)`          |
| `dlq_sql.go` (200 lines — custom SQLite DLQ)                            | built-in Host DLQ                 |
| `storage.go` (100 lines — EventCapture wrapper)                         | `host.WithPushSource()`           |
| `builder.go` (142 lines — projection registry)                          | `host.WithProjections()`          |
| `reconcile.go` (200 lines — ad-hoc reconciliation loop)                 | `host.WithReconciler()` + healers |

---

## 6. Sequenced Execution Plan

### Phase 1: Build the Host (additive, zero breakage)

| Step | What                                                                                                                                   | Effort      | Output                         |
| ---- | -------------------------------------------------------------------------------------------------------------------------------------- | ----------- | ------------------------------ |
| 1.1  | Generalize `pkg/provider/` — add `StreamProvider` for push alongside pull `Provider`                                                   | Low         | Provider supports push         |
| 1.2  | Extract `pkg/sync/retry.go` standalone (zero-dep module)                                                                               | Low         | Standalone retry               |
| 1.3  | Build generic `PullSyncer[T]` with `ChangeDetector[T]` + `SyncSink[T]` interfaces, shipping `ContentHashDetector` + `SeenKeysDetector` | Medium-High | Generic sync loop              |
| 1.4  | Build `pkg/reconcile/` — `Loop` (interval scheduler) + `Healer` interface + `CompositeHealer`                                          | Medium      | Reconciliation framework       |
| 1.5  | Build `pkg/projection/` — promote SQLite DLQ from DiscordSync, add projection registration helpers                                     | Medium      | Built-in DLQ                   |
| 1.6  | Build `pkg/host/` — `Host`, `NewHost`, `Option` pattern, `Run`, lifecycle, health checks, graceful shutdown                            | High        | The opinionated assembly point |
| 1.7  | Integration test: wire a test aggregate + projection + mock provider through the Host, verify end-to-end                               | Medium      | Proof of concept               |

### Phase 2: Validate Against Real Consumers

| Step | What                                                                              | Effort | Risk Mitigation             |
| ---- | --------------------------------------------------------------------------------- | ------ | --------------------------- |
| 2.1  | Port github-local-sync to the Host (simplest consumer, already uses stack/sqlite) | Medium | Validates pull path         |
| 2.2  | Extract learnings, refine Host API                                                | Low    | API stabilization           |
| 2.3  | Create example app in go-localsync repo (minimal sync demo)                       | Low    | Reference for new consumers |

### Phase 3: Deprecate + Remove

| Step | What                                                    | Effort |
| ---- | ------------------------------------------------------- | ------ |
| 3.1  | Mark `pkg/cqrs/`, `pkg/data/`, `pkg/api/` as deprecated | Low    |
| 3.2  | Delete deprecated packages                              | Low    |
| 3.3  | Update AGENTS.md, README, all docs                      | Medium |
| 3.4  | Tag v2.0.0                                              | Low    |

### Phase 4: Consumer Adoption

| Step | What                                                       | Effort |
| ---- | ---------------------------------------------------------- | ------ |
| 4.1  | bank-sync adopts Host (full CQRS + pull)                   | Medium |
| 4.2  | DiscordSync adopts Host (event capture + push + reconcile) | Medium |

---

## 7. Why Framework, Not Library?

go-cqrs-lite is explicitly a **library** — its docs say "Not a framework: no opinionated transport, broker, or SQL driver." Its `stack.Bundle` is "a bag of peer capability fields, not a lifecycle owner."

That's the right design for go-cqrs-lite. But it leaves a gap: **someone has to own the lifecycle, the wiring sequence, the defaults, and the edge cases.** Today, every consumer does it themselves, gets it wrong differently, and maintains it alone.

go-localsync is the **framework** layer — opinionated about how the pieces fit together, defaults that work, lifecycle that's correct. The consumer provides the domain; the framework provides the assembly.

| go-cqrs-lite (library)                     | go-localsync (framework)                       |
| ------------------------------------------ | ---------------------------------------------- |
| `stack.Bundle` — peer fields, no lifecycle | `Host` — owns lifecycle, manages goroutines    |
| `Materialize[V,K]` — read-model builder    | Projection registration + host wiring          |
| `event.Store` interface                    | DLQ, checkpoint, auto-restart defaults         |
| `command.Dispatcher`                       | Command registration helpers                   |
| Tombstone primitives                       | Tombstone-aware sync loop                      |
| No sync, no reconcile, no provider         | Provider contract + sync loop + reconciliation |

---

## 8. Tradeoffs

### What We Gain

- **~3,270 lines of consumer boilerplate eliminated** (measured across 3 consumers)
- **Correctness baked in** — checkpoint, DLQ, graceful shutdown, retry classification, per-source serialization — all correct by default, not opt-in
- **All three consumers served** — pull, push, event-capture-only
- **71% code reduction in go-localsync itself** (4,333 → ~1,050 lines)
- **ADR-0004 dissolved** — generic over domain without generics-infecting the CQRS stack

### What We Risk

| Risk                                                        | Mitigation                                                                 |
| ----------------------------------------------------------- | -------------------------------------------------------------------------- |
| Host API is too constrained for some consumer shape         | Phase 2 validates against real consumers before Phase 3 removes old code   |
| Defaults don't fit all consumers                            | Every default is overridable via Option                                    |
| Framework coupling (consumers depend on Host lifecycle)     | Host is thin — consumer can drop to go-cqrs-lite stack/ directly if needed |
| Push provider interface doesn't fit DiscordSync's WebSocket | Design WITH DiscordSync (Phase 4.2) not before                             |

### What We Lose

- The single-aggregate batteries-included experience (nobody used it)
- The reference domain model (`model.Item` — no consumer used it)

---

## 9. Decision Required

The revised vision: **go-localsync = opinionated sync application framework on go-cqrs-lite.** Not loose primitives — an assembled Host that makes the integration genuinely easy.

1. **Is the Host vision right?** Should go-localsync own the lifecycle/wiring/defaults, or stay as loose primitives?
2. **Phase 1 first?** Build the Host alongside existing code, validate with github-local-sync before removing anything.
3. **Start with the Host or with the primitives?** The Host depends on generic PullSyncer + reconcile + provider. Build bottom-up (1.1→1.6) or prototype the Host API first and fill in the pieces?

---

## Appendix: Consumer Evidence Sources

| Finding                  | Source                                                                                                                                                                                                                                                                                                                                               |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| github-local-sync wiring | `internal/branch/service.go` (158 lines), `internal/sync/syncer.go` (263 lines), `internal/sync/branch_flow_context.go` (200 lines), `internal/sync/event_processor.go` (200 lines)                                                                                                                                                                  |
| bank-sync wiring         | `internal/cqrs/infrastructure.go` (276 lines), `internal/cqrs/handlers.go` (125 lines), `internal/cqrs/middleware.go` (118 lines), `internal/cqrs/upcasting.go` (56 lines), `internal/cqrs/projections.go` (300 lines), `cmd/bank-sync/helpers.go` (183 lines)                                                                                       |
| DiscordSync wiring       | `cmd/discordsync/init.go` (321 lines), `cmd/discordsync/container.go` (183 lines), `cmd/discordsync/lifecycle.go` (160 lines), `cmd/discordsync/shutdown.go` (74 lines), `internal/storage/dlq_sql.go` (200 lines), `internal/storage/storage.go` (100 lines), `internal/projection/builder.go` (142 lines), `internal/bot/reconcile.go` (200 lines) |
| go-cqrs-lite stack layer | `stack/sqlite/preset.go`, `stack/materialize.go`, `stack/run_projections.go`, `docs/ECOSYSTEM_BOUNDARIES.md`                                                                                                                                                                                                                                         |
| Adoption feedback        | `docs/feedback/2026-06-23_discordsync-adoption-feedback.html` (6 structural findings)                                                                                                                                                                                                                                                                |
