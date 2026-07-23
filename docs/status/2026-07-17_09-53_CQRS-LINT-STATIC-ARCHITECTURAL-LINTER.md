# Status Report: cqrs-lint — Static CQRS Architectural-Invariant Linter

**Date:** 2026-07-17 09:53  
**Session scope:** Build a `cqrs-lint` static analyzer from scratch (the command didn't exist).  
**Verdict:** Shipped and green, but with real gaps (see below).

---

## a) FULLY DONE

### Core analyzer (`internal/cqrslint/`)

| File                | Status | Purpose                                                                                                                                                |
| ------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `finding.go`        | ✅     | `Finding` struct, `Severity` type, `SortFindings` stable sort                                                                                          |
| `load.go`           | ✅     | `LoadPackage` — stdlib `go/parser` recursive loader; skips `_test.go`, `vendor/`, hidden dirs; exported sentinels `ErrNotADirectory`, `ErrNoGoSources` |
| `helpers.go`        | ✅     | `Check` type, `allChecks` registry, `visitGenDecls`, `findFunc`, `findStructType`, `literalStringValue`                                                |
| `analyzer.go`       | ✅     | `Run` orchestrator, `Rules()` catalog, rule ID constants, `estimateFindings` prealloc                                                                  |
| `checks_events.go`  | ✅     | C0001 (aggregate type), C0002 (event consts), C0009 (payload json tags), C0010 (NewEvents uses const)                                                  |
| `checks_runtime.go` | ✅     | C0003 (fold coverage), C0004 (projector subscriptions), C0005 (hasChanged provider-agnostic), C0008 (projection mutex guard)                           |
| `checks_scope.go`   | ✅     | C0006 (no query dispatcher), C0007 (SyncAction stays in pkg/sync)                                                                                      |

### CLI (`cmd/cqrs-lint/main.go`)

- ✅ `-pkg` flag (default `pkg/cqrs`)
- ✅ `-list` flag (prints rule catalog)
- ✅ `-json` flag (newline-delimited JSON)
- ✅ `-fail-on-warning` flag
- ✅ Exit codes: 0 clean, 1 findings, 2 usage error
- ✅ `-help` / `--help` usage output

### Tests (`internal/cqrslint/`)

- ✅ 23 test functions, all passing
- ✅ Mutation-based: compliant base fixture + one `strings.Replace` per test
- ✅ Positive control (compliant base yields 0 findings)
- ✅ Loader edge cases (missing dir, non-Go sources, test-only dir)
- ✅ `SortFindings` stability test
- ✅ `Finding.String()` formatting tests
- ✅ `Rules()` count + order test

### Wiring

- ✅ `.golangci.yml` — path exclusions for `internal/cqrslint/` and `cmd/cqrs-lint/` (style linters relaxed; correctness linters stay on); `cqrslint.Finding` added to exhaustruct exclusion
- ✅ `flake.nix` — `packages.cqrs-lint` (buildGoModule subPackage), `apps.cqrs-lint`, `checks.cqrs-lint` (CI gate via runCommand)
- ✅ `AGENTS.md` — package table row, dev workflow step 6, testing table row + count updated (194→217, 9→10 packages)

### Verification

- ✅ `GOEXPERIMENT=jsonv2 go build ./...` — clean
- ✅ `GOEXPERIMENT=jsonv2 go test ./... -count=1` — all 217 tests pass
- ✅ `golangci-lint run ./internal/cqrslint/... ./cmd/cqrs-lint/...` — 0 issues
- ✅ `golangci-lint fmt` — clean
- ✅ `go run ./cmd/cqrs-lint -pkg pkg/cqrs` — exit 0 (zero findings against the compliant package)
- ✅ `go run ./cmd/cqrs-lint -list` — all 10 rules listed correctly
- ✅ `go run ./cmd/cqrs-lint -pkg pkg/cqrs -json` — clean JSON output

---

## b) PARTIALLY DONE

### flake.nix `checks.cqrs-lint`

- ✅ Added the `runCommand` derivation
- ❌ **Not verified** — couldn't run `nix flake check` because nixpkgs packages Go 1.26.3 while `go.mod` requires 1.26.4 (documented in AGENTS.md). The check may work (linter uses `go/parser` only, no type resolution, no `go-cqrs-lite` import at runtime) but this is unproven.

### CI workflow integration

- ❌ **Not started** — `.github/workflows/ci.yml` was already modified at session start (by prior work). I did not add a `cqrs-lint` step or job. The flake.nix check only runs under `nix flake check`, which the native CI may not invoke.

---

## c) NOT STARTED

1. **GitHub Actions CI step for cqrs-lint** — no `go run ./cmd/cqrs-lint -pkg pkg/cqrs` step in `.github/workflows/ci.yml`
2. **CLI tests** — `cmd/cqrs-lint/main.go` has zero test coverage (flag parsing, exit codes, output format)
3. **Coverage measurement** — AGENTS.md testing table says `—` for `internal/cqrslint`
4. **`--version` flag** — no version output
5. **`--quiet` flag** — no CI-friendly suppress-output mode
6. **`--format=github` option** — no GitHub Actions `::error` annotation output
7. **`testdata/` directory pattern** — tests use inline string fixtures instead of the standard Go `testdata/` approach used by `go/analysis`-based linters
8. **README for `cmd/cqrs-lint/`** — no standalone documentation
9. **Benchmark** — no `BenchmarkRun` to measure analyzer performance
10. **Nix verification** — `nix flake check` not run (blocked by nixpkgs Go version lag)

---

## d) TOTALLY FUCKED UP

Nothing is totally broken. The linter builds, tests pass, lint is clean, and the gate passes against `pkg/cqrs`. But here are the things I'm not proud of:

### 1. `-json` output is hand-rolled and fragile

```go
fmt.Printf(`{"rule":%q,"severity":%q,"file":%q,"line":%d,"message":%q}`+"\n", ...)
```

If a `message` contains a backslash or control character, `%q` produces Go-quoted output (not valid JSON). A message with `\n` would produce `\\n` in Go-quoted but that's actually valid JSON. However, `%q` uses Go quoting rules, not JSON escaping. Should use `encoding/json.Marshal` instead.

### 2. Test fixtures are string-literal hell

The `compliantSource()` function returns a ~70-line Go program as a raw string literal with embedded backtick-escaped struct tags. This is:

- Hard to read
- Easy to break (one wrong backtick = syntax error)
- Not how the Go ecosystem tests analyzers (should be `testdata/` files)

### 3. Function-name-based detection is not type-safe

`findFunc(pkg, "fold")`, `findFunc(pkg, "Handle")`, `findFunc(pkg, "EventTypes")` match by name only. If someone adds an unrelated type with a `Handle` method, C0008 could produce a false positive. Without type resolution (go/types), there's no way to verify the method is on `*Projector`. This is a deliberate tradeoff (zero deps, fast) but it's a correctness gap.

### 4. The `hasChanged` check (C0005) can be bypassed

```go
l := local
return l.Title != remote.Title  // C0005 won't catch l.Title
```

The check only looks for `local.X` and `remote.X` selector patterns. Indirect access via a local variable evades it. Again, AST-only limitation.

### 5. The `NewEvents` check (C0010) can be bypassed

```go
at := aggregateType
event.NewEvents(aggID, at, 0, nil, nil)  // C0010 won't catch this
```

The check only verifies the 2nd argument is an `*ast.Ident` named `aggregateType`. An alias defeats it.

### 6. The `query.Dispatcher` check (C0006) can be bypassed

```go
import query "github.com/larsartmann/go-cqrs-lite/query/v4"
import alias "github.com/larsartmann/go-cqrs-lite/query/v4"
var _ alias.Dispatcher  // C0006 checks for "query" selector only
```

An aliased import of the `query` package would slip through.

---

## e) WHAT WE SHOULD IMPROVE

### High priority

1. **Fix `-json` to use `encoding/json.Marshal`** — current hand-rolled format is not safe
2. **Add GitHub Actions CI step** — `go run ./cmd/cqrs-lint -pkg pkg/cqrs` in `.github/workflows/ci.yml`
3. **Add CLI tests** — at minimum test exit codes (0 clean, 1 findings, 2 usage)
4. **Migrate test fixtures to `testdata/`** — standard Go pattern, more maintainable
5. **Measure and report coverage** — fill in the `—` in AGENTS.md

### Medium priority

6. **Add `--version` flag** using `debug.ReadBuildInfo()` for version info
7. **Add `--quiet` flag** for CI (suppress output, exit code only)
8. **Add `--format=github` option** for GitHub Actions `::error` annotations
9. **Add `cmd/cqrs-lint/README.md`** or at least improve `-help` output
10. **Add a benchmark** to track analyzer performance
11. **Document the AST-only limitations** in `internal/cqrslint/doc.go` — C0005/C0006/C0010 can be bypassed; this is a deliberate tradeoff
12. **Consider `pkg/cqrslint/` instead of `internal/cqrslint/`** — if external consumers should integrate the analyzer into their own tooling

### Low priority

13. **Refactor `LoadPackage` further** to stay under funlen without the exclusion (currently 66 lines, limit 60)
14. **Add check for `decider.NewRepository` using `aggregateType`** — ensure the repository is also tagged with the const
15. **Add check for `command.New` using the aggregate type** — ensure commands reference the right aggregate
16. **Add check for snapshot store configuration** — ensure snapshots are wired (AGENTS.md says they are)
17. **Add check for `projection.Projection` interface implementation** — ensure `*Projector` satisfies the interface
18. **Add check for `event.WithCorrelationID` usage** — AGENTS.md says `SyncItems` generates a unique CorrelationID
19. **Add check for `middleware.EventLogging`** — AGENTS.md says it's wired
20. **Add check for `snapshot.EveryNEvents`** — AGENTS.md says it's configured

### False-positive prevention

21. **Make `findFunc` type-aware** — accept a receiver type name so `Handle` on a non-Projector type doesn't trigger C0008
22. **Make C0006 check all import aliases** — parse the import block and check for any import of `query/v4`
23. **Make C0010 resolve the identifier** — at minimum check it's not a local variable (look for `var <ident>` or `:=` assignments in scope)

### Polish

24. **Improve error messages** — C0003 should say "add `case EventItemTombstoned:` to the fold switch" not just "does not case event"
25. **Add `--max-errors=N` flag** — stop after N findings (useful for large packages)
26. **Add `--exclude-rule=C000X` flag** — let consumers suppress specific rules
27. **Add color output** when stderr is a TTY (like `golangci-lint`)
28. **Add `--watch` flag** for development (re-run on file change)

---

## f) Up to 50 Things to Get Done Next

1. Fix `-json` output to use `encoding/json.Marshal` instead of hand-rolled `%q` formatting
2. Add `cqrs-lint` step to `.github/workflows/ci.yml` (native Go CI, not nix)
3. Write CLI tests (`cmd/cqrs-lint/main_test.go`) — exit codes, flag parsing, output
4. Measure `internal/cqrslint` coverage and update AGENTS.md testing table
5. Migrate test fixtures from inline strings to `testdata/` directory with `.go` files
6. Add `--version` flag using `debug.ReadBuildInfo()`
7. Add `--quiet` flag for CI
8. Add `--format=github` for GitHub Actions `::error` annotations
9. Write `cmd/cqrs-lint/README.md` with usage examples
10. Add `BenchmarkRun` benchmark
11. Write `internal/cqrslint/doc.go` documenting AST-only limitations
12. Consider moving `internal/cqrslint` to `pkg/cqrslint` for external extensibility
13. Make `findFunc` accept a receiver type parameter to prevent false positives
14. Make C0006 parse import aliases to catch `import alias "query/v4"`
15. Make C0010 check that the identifier is the package-level const, not a local variable
16. Make C0005 detect indirect field access (`l := local; l.Title`)
17. Add check C0011: `decider.NewRepository` is called with `aggregateType`
18. Add check C0012: `command.New` uses `commandTypeSyncItem`/`commandTypeTombstone` constants
19. Add check C0013: `event.WithCorrelationID` is used in `SyncItems`
20. Add check C0014: `middleware.EventLogging` is wired in `NewCQRSStack`
21. Add check C0015: `snapshot.EveryNEvents` is configured
22. Add check C0016: `*Projector` implements `projection.Projection` (has `Name`, `EventTypes`, `Handle`)
23. Add check C0017: no `var _ = query.NewDispatcher()` patterns
24. Add `--exclude-rule=C000X` flag to suppress specific rules
25. Add `--max-errors=N` flag to cap output
26. Add color output when stderr is a TTY
27. Improve C0003 message: "add `case EventItemTombstoned:` to the fold switch"
28. Improve C0005 message: "field `Title` is not provider-agnostic; use ContentHash/UpdatedAt/Type only (ADR-0007)"
29. Add integration test: run the binary as a subprocess and check stdout/stderr/exit
30. Verify `nix flake check` passes (once nixpkgs bumps Go to 1.26.4)
31. Verify `nix run .#cqrs-lint` works
32. Add `cqrs-lint` to the `flake.nix` devShell so it's available in `nix develop`
33. Add `cqrs-lint` to the pre-commit hook (with vendor/ excluded)
34. Refactor `LoadPackage` to stay under funlen limit without the exclusion
35. Add a `--rules=C0001,C0003` flag to run only specific rules
36. Add a `--timeout` flag for large packages
37. Add `--debug` flag to print the parsed AST
38. Add `--diff` flag to show only findings that changed since last run
39. Add `SARIF` output format for security tools integration
40. Add check that `SyncItemCommand` carries `*model.Item` (not a different type)
41. Add check that `TombstoneItemCommand` carries `model.TombstoneReason`
42. Add check that `decider.Repository[SyncItemState]` is the only repository type
43. Add check that no `event.Store` is used directly (only through `CQRSStack`)
44. Add check that `ReadModel` interface is not widened (no new methods beyond the documented set)
45. Add check that `SyncStore` interface is not widened (no new methods beyond List/Count/CountByType)
46. Add check that `EventItemSynced` always means "live" (the fold function resurrects)
47. Add check that `EventItemTombstoned` keeps the item (doesn't nil it)
48. Add check that `ItemConflictFound` is a metadata-only event (no state mutation in fold)
49. Add check that `hasChanged` doesn't compare `ActorLogin`/`RepoName` (legacy V1/V2 fields)
50. Add check that no new `event.AggregateType` literals appear in string form (e.g., `"sync_item"` as a raw string, not the const)

---

## g) Questions I Cannot Answer Myself

### Q1: Should `cqrslint` be `internal/` or `pkg/`?

`internal/cqrslint/` means external consumers of `go-localsync` cannot import the analyzer as a library or add their own checks. If the linter is purely a project-internal gate, `internal/` is correct. If consumers building providers on top of `go-localsync` should be able to run the same architectural checks against their own code, it should be `pkg/cqrslint/`. This is a product decision about who the audience is — I can't determine it from the code alone.

### Q2: Should the linter grow beyond the current 10 rules, or stay deliberately minimal?

ADR-0004 says "one aggregate, three events, one projection." The 10 rules enforce that exact scope. But AGENTS.md also documents many other invariants (correlation IDs, snapshot strategy, middleware wiring, error taxonomy, per-source mutex, etc.) that could be statically checked. Adding more rules increases protection but also maintenance burden and false-positive risk. This is a judgment call about how much static enforcement the project wants vs. trusting the developer + code review.

### Q3: Should the CI gate be `go run ./cmd/cqrs-lint` (native) or `nix run .#cqrs-lint` (nix)?

The project has both a native Go CI (`.github/workflows/ci.yml` with `go build`/`go test`/`golangci-lint`) and nix checks (`nix flake check`). The native CI is faster and doesn't need the nixpkgs Go version to match. The nix check is more hermetic but currently blocked by the Go 1.26.4 vs 1.26.3 lag. Which should be the canonical gate? This depends on whether the project plans to fix the nixpkgs lag or accept the native CI as the primary gate.

---

## Resolution (2026-07-22)

`cqrs-lint` shipped in **v0.4.0** (2026-07-18). Current state:

- **23 tests, 88.5% coverage** — all passing.
- **nixpkgs Go lag resolved** — `go_1_26` is now at 1.26.4, so `nix flake check` passes in-sandbox.
- **golangci-lint integration** — cqrs-lint is wired into `.golangci.yml` and `flake.nix checks.cqrs-lint`.
- **CLI tests** remain at **zero coverage** — the `cmd/cqrs-lint/main.go` binary itself is untested. This is the main remaining gap.
- The `-json` output was refactored (Finding helper constructors extracted across all checks in post-v0.4.0 refactoring).
- **Still open:** `--version`/`--quiet`/`--format=github` flags, `testdata/` directory pattern, CI workflow step for cqrs-lint.
