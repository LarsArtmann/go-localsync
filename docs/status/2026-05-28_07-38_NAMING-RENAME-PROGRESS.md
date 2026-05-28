# Comprehensive Status Report — go-localsync Naming Rename Progress

**Date:** 2026-05-28 07:38  
**Branch:** master  
**Commit:** 394725b  
**Author:** Lars Artmann <git@lars.software>

---

## What I Forgot / Could Do Better

1. **Should have committed the test additions FIRST** — I modified `cmd/examples/github-sync/main_test.go` adding CLI tests but never committed them separately. They're sitting uncommitted alongside doc formatting changes.
2. **Should have checked `go-cqrs-lite` sync module FIRST** — Before proposing `pkg/localsync` deletion, I should have verified whether go-cqrs-lite actually has the same primitives. Turns out it doesn't have a `sync` module — `pkg/localsync` is genuinely unique to this project.
3. **Import alias conflicts** — Renaming `pkg/types` → `pkg/id` caused conflicts with `go-cqrs-lite/core/pkg/id` imports. I had to add `cqrsid` aliases in 5 files. This was predictable and should have been planned for.
4. **LSP cache chaos** — The LSP still shows stale errors from deleted files (`aggregate_id_test.go`, old `pkg/types` paths). I should have restarted the LSP or ignored these rather than chasing ghosts.
5. **Should have asked about `pkg/localsync` fate BEFORE renaming** — The user corrected me: this is an SDK, not an app. I cannot delete public packages. I should have confirmed the integration approach first.

---

## What Was Done (Since Last Report)

| #   | Task                                             | Status  | Files                                                                                      |
| --- | ------------------------------------------------ | ------- | ------------------------------------------------------------------------------------------ |
| 1   | `pkg/types` → `pkg/id` rename                    | ✅ DONE | 25 files changed, 526 insertions, 194 deletions                                            |
| 2   | Fixed `go-cqrs-lite/core/pkg/id` alias conflicts | ✅ DONE | `pkg/cqrs/aggregate_id.go`, `decider.go`, `stack.go`, `testing_test.go`, `decider_test.go` |
| 3   | Fixed import ordering (gci)                      | ✅ DONE | All affected files via `goimports`                                                         |
| 4   | Verified build + tests + lint                    | ✅ DONE | 0 lint issues, all 7 packages pass                                                         |

---

## What's Still Pending (From Plan)

### Package Renames (In Progress)

| Current         | Target                    | Complexity | Files Affected                         |
| --------------- | ------------------------- | ---------- | -------------------------------------- |
| `pkg/provider`  | `pkg/item` + `pkg/source` | HIGH       | 20 files import `pkg/provider`         |
| `pkg/localsync` | `pkg/crdt`                | LOW        | Not imported by any production code    |
| `pkg/sync`      | `pkg/engine`              | MEDIUM     | 2 files import it                      |
| `pkg/cqrs`      | `pkg/store`               | HIGH       | 1 file imports it + many internal refs |

### Testing (Not Started)

| Task              | Target Coverage | Current |
| ----------------- | --------------- | ------- |
| CLI tests         | ≥ 50%           | 10.5%   |
| `pkg/sync` tests  | ≥ 85%           | 77.8%   |
| `SyncIncremental` | ≥ 90%           | 37.5%   |

### Features (Not Started)

| Task                  | Status      |
| --------------------- | ----------- |
| `-json` flag          | Not started |
| ItemFilter builder    | Not started |
| Error wrapping helper | Not started |

---

## What's Totally Fucked Up

| #   | Issue                                 | Severity                                                                                                                                        |
| --- | ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Uncommitted test additions**        | `main_test.go` has new tests but they're not committed                                                                                          |
| 2   | **Doc formatting-only changes**       | DOMAIN_LANGUAGE.md, UPSTREAM-SUGGESTIONS.md have whitespace changes sitting uncommitted                                                         |
| 3   | **LSP stale cache**                   | Still shows errors for deleted `aggregate_id_test.go` and old `pkg/types` paths                                                                 |
| 4   | **Planning doc bloat**                | `2026-05-28_06-25_COMPREHENSIVE-EXECUTION-PLAN.md` is 221 lines but hasn't been updated since the rename decision changed (pkg/localsync stays) |
| 5   | **go-mod-tidy failure in pre-commit** | Saga module version mismatch — pre-existing, not my fault                                                                                       |

---

## Top 1 Question I Cannot Figure Out Myself

**Should I continue with the remaining 4 package renames in this session, or commit the current state and ask for confirmation on each rename?**

The renames are:

1. `pkg/provider` → `pkg/item` + `pkg/source` (requires splitting `provider.go` into two packages)
2. `pkg/localsync` → `pkg/crdt` (simple directory rename, not imported by production code)
3. `pkg/sync` → `pkg/engine` (2 importers, but shadows stdlib `sync`)
4. `pkg/cqrs` → `pkg/store` (1 external importer, many internal refs)

Each rename is a breaking change for SDK consumers. Doing all 4 at once creates a massive migration burden. But doing them one at a time means 4 separate breaking changes.

**What does the project owner want?**

- All renames in one big v0.2.0 release?
- One rename at a time with deprecation periods?
- Skip some renames and focus on features/tests instead?

---

## Current State

| Metric              | Value                                           |
| ------------------- | ----------------------------------------------- |
| Build               | ✅ Pass                                         |
| Tests               | ✅ 7/7 packages pass                            |
| Lint                | ✅ 0 issues                                     |
| Uncommitted changes | 4 files (docs + tests)                          |
| Last commit         | 394725b `refactor!: rename pkg/types to pkg/id` |
| Go version          | 1.26.2                                          |

---

## Build Commands

```bash
# Test
go test ./... -count=1

# Lint
golangci-lint run ./...

# Build
go build ./...
```
