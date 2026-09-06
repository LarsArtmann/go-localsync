# localsync-lint — ADR-0004 architectural invariants

`cmd/localsync-lint` statically verifies that `pkg/cqrs` keeps the shape
ADR-0004 fixed: one aggregate, three events, one projection, and the seam
rules that keep `pkg/sync` and `pkg/cqrs` decoupled. It parses Go source
with the standard library `go/parser` — no type resolution, no third-party
dependencies — and it exists so a well-meaning refactor cannot silently widen
the SDK's scope (the exact failure mode the ADR defers).

CI runs it on every push:

```
go run ./cmd/localsync-lint --strict --verbose
```

## Exit codes

| Code | Meaning                                             |
| ---- | --------------------------------------------------- |
| 0    | clean (warnings allowed unless `--strict`)          |
| 1    | findings present (or warnings present + `--strict`) |
| 2    | usage or internal error                             |

## Flags

| Flag               | Effect                                                                      |
| ------------------ | --------------------------------------------------------------------------- |
| `-pkg <path>`      | package to lint (default `pkg/cqrs`)                                        |
| `-strict`          | fail on warnings too (alias: `-fail-on-warning`)                            |
| `-format <name>`   | `text` (default), `json` (NDJSON), `github` (workflow annotations), `sarif` |
| `-json`            | alias for `-format=json`                                                    |
| `-quiet`           | no output; the exit code is the only channel                                |
| `-verbose`         | package info, per-rule status, per-rule suppressed counts, timing           |
| `-show-suppressed` | also print findings silenced by directives                                  |
| `-rules`           | comma-separated rule IDs to run (default: all)                              |
| `-exclude-rules`   | comma-separated rule IDs to skip                                            |
| `-no-suppress`     | ignore every directive; all violations count (CI hardening)                 |
| `-explain <rule>`  | print one rule's full description and exit                                  |
| `-list`            | print the rule catalog and exit                                             |
| `-version`         | print the CLI version and exit                                              |

Unknown rule IDs in `-rules`/`-exclude-rules` and unknown `-format` values
are usage errors (exit 2), never silent no-ops.

## Output formats

- `text` — one finding per line, human-readable.
- `json` — newline-delimited JSON objects (`rule`, `severity`, `file`,
  `line`, `message`, `suppressed`), one per finding.
- `github` — GitHub Actions annotations (`::error file=...,line=...,title=C0002::...`)
  that render inline in the PR diff view.
- `sarif` — a single [SARIF 2.1.0](https://json.sarifspec.com/) document with
  the full rule catalog in `tool.driver.rules` and one result per finding;
  upload it to a code-scanning consumer as-is. Suppressed findings shown via
  `-show-suppressed` carry an `inSource` suppression entry.

## Suppression directives

The directive vocabulary is `//cqrs-lint:` on purpose: one inline-comment
protocol shared with go-cqrs-lite's consumer-side linter, whose rule-ID
schemes this tool tolerates in directives.

| Directive                         | Scope                                                     |
| --------------------------------- | --------------------------------------------------------- |
| `//cqrs-lint:ignore C0005`        | next line (or same line)                                  |
| `//cqrs-lint:ignore C0005 reason` | same, with a human-readable reason (required by review)   |
| `//cqrs-lint:ignore C0001,C0002`  | comma-separated rules                                     |
| `//cqrs-lint:ignore all`          | every rule at this position                               |
| `//cqrs-lint:ignore-file C0005`   | entire file                                               |
| `//cqrs-lint:ignore-start C0005`  | begin a suppressed interval                               |
| `//cqrs-lint:ignore-end C0005`    | end the interval (bare `ignore-end` closes all open ones) |
| `/* cqrs-lint:ignore C0005 */`    | block-comment form of any directive                       |

Rules of the road:

- Blanket ignores without a reason are rejected in review; the annotated
  suppression sites in this repo always carry `//cqrs-lint:ignore <rule> <reason>`.
- Ranges may not nest for the same rule (the inner `ignore-start` is ignored
  and reported); an `ignore-end` with no open range and an unclosed
  `ignore-start` each produce a warning.
- `-no-suppress` disables the whole mechanism — use it when auditing whether
  the surviving suppressions are still earned (`--show-suppressed` lists what
  each directive silences; `--verbose` adds per-rule suppressed counts).

## Rule catalog

15 architectural checks (C0001-C0015), all severity `error`. Every rationale
cites the design decision it protects; run `-list` or `-explain <rule>` for
the authoritative, always-current version.

| Rule  | Title                           | Protects                                                   |
| ----- | ------------------------------- | ---------------------------------------------------------- |
| C0001 | single aggregate type           | exactly one `event.StreamType` const valued `"sync_item"`  |
| C0002 | three fixed event types         | exactly three `event.Type` consts                          |
| C0003 | fold covers all events          | every declared event handled by the fold switch            |
| C0004 | projector subscribes to all     | `Projector.EventTypes` returns every event const           |
| C0005 | hasChanged is provider-agnostic | only ContentHash/UpdatedAt/Type (ADR-0007)                 |
| C0006 | no query dispatcher             | reads call the ReadModel directly                          |
| C0007 | SyncAction stays in pkg/sync    | the seam types live in `pkg/sync`, never `pkg/cqrs`        |
| C0008 | projection version-gate locked  | `Projector.Handle` holds the mutex before the version-gate |
| C0009 | payload fields have json tags   | event payloads are a wire contract                         |
| C0010 | NewEvents uses aggregateType    | events tagged with the const, never a literal              |
| C0011 | single projection               | a second `EventTypes` method means a second projection     |
| C0012 | fold is deterministic           | no `time.Now`/`time.Since` inside a fold                   |
| C0013 | projector never writes events   | the projector is read-side only                            |
| C0014 | wire values stay in const file  | no event/aggregate literals outside the declaring file     |
| C0015 | NewEvents types use consts      | no inline event-type strings in `NewEvents`                |

`scripts/check-doc-counts.sh` derives the catalog size from the `Rules()`
declaration table and fails when this page (or AGENTS/README) drifts from it,
so the table above cannot silently go stale the way the "10 → 15 rules" text
once did.
