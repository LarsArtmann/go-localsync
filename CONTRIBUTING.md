# Contributing

Thanks for your interest in contributing! This document explains the
architecture you are contributing to, the conventions the codebase follows,
and the bar a change must clear before merging.

## How to Contribute

1. Fork the repository
2. Create a feature branch
3. Make your changes (see conventions below)
4. Submit a pull request

## Development Setup

The project builds with Nix. The devShell provides Go with the required
`GOFLAGS=-tags=goexperiment.jsonv2` (the event-sourcing library adopts
`encoding/json/v2`, gated by Go 1.26 until graduation):

```bash
# one-time
direnv allow        # or: nix develop

# every-change loop
go build ./... && go test ./... -count=1
golangci-lint run ./... --timeout=5m
go run ./cmd/cqrs-lint --strict
```

CI runs the same gates plus a pinned library-lint leg; see AGENTS.md for the
full CI matrix.

## Architecture in one page

go-localsync is a **single-aggregate, pull-only sync SDK** (ADR-0004 scope
boundary — do not widen it):

```
provider (contract)  →  pkg/sync (Syncer orchestration)  →  pkg/cqrs (event-sourced core)
        pkg/data/model (domain)      pkg/crdt (conflict)      pkg/id (branded types)
                                                           pkg/api (HTTP surface)
```

- **One aggregate**: `sync_item`. Three events (`ItemSynced`,
  `ItemConflictFound`, `ItemTombstoned`), one projection. Adding a fourth
  event type or a second aggregate requires revisiting ADR-0004 first —
  the internal `cqrs-lint` gate enforces this mechanically.
- **Dependency direction**: `pkg/cqrs → pkg/sync → provider/types/errors`.
  Never import upward. `pkg/sync.SyncStore` is the seam that keeps sync
  logic decoupled from CQRS infrastructure.
- **Storage**: go-cqrs-lite v4 (decider, repository, projectionhost, storage).
  Prefer library primitives (upcasters, middleware, scenario DSL) over
  hand-rolled equivalents — the library is the cross-project sharing boundary.

## File-split conventions

- `pkg/cqrs` splits by concern: `decider.go` (pure decide/fold),
  `stack.go` (wiring + public commands), `store_factory.go` (backend
  construction), `projection.go`/`runner.go` (read side), `events.go`
  (wire payloads), `upcaster.go` (schema evolution), `otel.go`
  (telemetry adapter). New cross-cutting concerns get their own file.
- Event payloads are a **wire contract**: every field carries an explicit
  `json` tag; new fields are optional (omitempty) and require a schema
  version decision (see `pkg/data/schema`, ADR-0007, and `upcaster.go`).
- Public API changes: additive only within a minor; renames wait for the
  next breaking window (see ADR-0009 for the planned v0.6 vocabulary moves).

## Testing requirements

- Every PR ships tests; behavior tests over implementation tests.
- **New decider behavior tests use the `scenario` DSL**
  (`go-cqrs-lite/scenario/v4`) — see `pkg/cqrs/scenario_test.go` for the
  Given/When/Then pattern and the command adapters.
- SQLite-touching tests use file-backed databases (`t.TempDir()`), not
  `:memory:`, when they assert restart/WAL behavior.
- Error paths are test paths: `go test ./pkg/cqrs -cover` must stay ≥87%.
- The internal linter (`cmd/cqrs-lint`) and the library linter both run in
  CI; suppressions need an inline `//cqrs-lint:ignore <rule> <reason>` —
  never blanket-disable.

## Commit / PR conventions

- Conventional-commit-style prefixes (`feat:`, `fix:`, `docs:`, `ci:`,
  `refactor:`, `test:`, `chore:`).
- User-visible changes update `CHANGELOG.md` (Keep-a-Changelog format) and
  `FEATURES.md` in the same PR.
- `TODO_LIST.md` is the source of truth for open work; move finished items
  out when their PR merges.

## Release checklist

Releases are tag-triggered (the `release` job cuts the GitHub Release after
CI gates pass). Both modules version **independently**: the core module tags
`v0.Y.Z`, `provider/github` tags its own `vX.Y.Z`.

1. **Green tree**: `nix flake check` locally (runs build + format + hermetic
   lint + hermetic test + the cqrs-lint gate) and confirm CI is green on
   master — including the `nix` job.
2. **CHANGELOG**: the release needs a `[X.Y.Z]` section (not
   `[Unreleased]`); release notes are auto-generated.
3. **Tag and push**: `git tag -a vX.Y.Z -m "..." && git push origin vX.Y.Z`
   (repeat for the provider module when it changed).
4. **Verify publication** (run after CI finishes, a few minutes later):

   ```bash
   ./scripts/verify-release.sh vX.Y.Z [provider-vX.Y.Z]
   # e.g.: ./scripts/verify-release.sh v0.5.0 v0.1.0
   ```

   It checks tags (local + origin + ancestry), the GitHub Release,
   proxy.golang.org `@v/list` / `@latest` for both modules, and —
   best-effort, warn-only — pkg.go.dev indexing.

If any required check fails, the release is not out: fix (usually a missing
push or a proxy that has not seen the tag yet) and re-run. Never re-tag a
pushed version — see the go-release discipline notes in AGENTS.md.

## Reporting Issues

Please use GitHub Issues. Include the Go version, backend (`memory`/
`sqlite`), and a minimal reproduction. Security reports: use GitHub's
private vulnerability reporting, not public issues.
