# TODO_LIST.md

**Project:** go-localsync
**Last Updated:** 2026-09-05
**Tests:** 437 test functions across 11 packages, all passing (race-clean) | **Latest release:** v0.5.0 + `provider/github/v0.1.0`

## Overview

Actionable short- and mid-term tasks. Completed work is recorded in [CHANGELOG.md](CHANGELOG.md); the feature inventory lives in [FEATURES.md](FEATURES.md); long-term ideas in [ROADMAP.md](ROADMAP.md).

> **Scope note:** go-localsync is deliberately a **single-aggregate Item sync SDK**. Generalising it into a multi-aggregate event-sourcing framework was considered and **deferred** — see [ADR-0004](docs/adr/0004-multi-aggregate-generalisation-deferred.md) and the [DiscordSync adoption feedback](docs/feedback/2026-06-23_discordsync-adoption-feedback.html). The tasks below improve the SDK _within its current scope_; do not add tasks that widen it without revisiting that decision.

---

## 🔴 HIGH PRIORITY

### v0.6 vocabulary window (decided, awaiting the breaking release)

- [ ] **Rename public `AggregateID()` → `StreamID()` (v0.6)**
      **Source:** `pkg/cqrs/aggregate_id.go` — returns `cqrsid.StreamID` since v0.4.1 but kept the old name for API stability
      **Description:** Breaking rename for v0.6, decided and recorded in [ADR-0009](docs/adr/0009-v06-vocabulary-alignment.md) — together with the `SyncResult`/`SyncSummary` consolidation, converting the `AggregateID` panic fallback to an error return, and the deliberate `DeriveStreamID` encoding divergence (keep ours; documented at the definition site).

## 🟡 MEDIUM PRIORITY

### Tooling

- [ ] **Add the `SSH_PRIVATE_KEY` repo secret so the library cqrs-lint CI leg runs** — the error-gated `go-cqrs-lite/cmd/cqrs-lint/v4@v4.8.1` step is restored in the workflow and auto-enables once the secret exists (a deploy key with read access to the private `larsartmann/go-finding` module); until then it skips with a notice and the gate runs locally from the devShell (documented in the workflow + AGENTS.md).

## 🟢 LOWER PRIORITY

- [ ] ~~**Add `govalid` struct tags**~~ — pivoted 2026-09-05: govalid is a buildflow-internal generator, not a proxy-resolvable module; real `Validate()` methods were implemented instead (`SyncOptions.Validate`, `CQRSConfig.Validate`, `ItemFilter.Validate`). Reopen only if govalid is ever published with a stable tag format.
