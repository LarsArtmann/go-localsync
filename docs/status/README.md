# docs/status/ — Session Status Reports

Point-in-time snapshots of individual work sessions. **They are historical records, not living docs** — never treat an open item here as current work: the living trackers are [TODO_LIST.md](../../TODO_LIST.md) (actionable), [ROADMAP.md](../../ROADMAP.md) (raw ideas), and [CHANGELOG.md](../../CHANGELOG.md) (what shipped).

## Layout

- `*.md`, `*.html` (this directory) — recent reports, plus generated HTML status dashboards.
- `archive/` — fully resolved reports (every forward item closed or routed), moved here via `git mv`.

## Annotation & archive policy

1. **Recent reports** (current work cycle) get **inline-first** annotations: every numbered item / table row carries a verdict — `~~original~~ done at <hash>`, `done (<evidence>)`, `**Won't implement — <reason>.**`, or stays unmarked (absence of a marker _is_ the "open" signal). Resolution appendices answer the report's open questions.
2. **Era-closed historical reports** (superseded toolchains/decisions, e.g. the Turso/`vendor/`/v2-v3 eras) get a **dated bucket appendix** (`## Resolution (<date> docs-health sweep)`) closing all forward items as _shipped / superseded / moot / routed_, plus **inline strikes on the worst now-false claims** (assertions a reader today would be misled by). Per-item strikes are not applied when they would add noise without value ("so-what" restraint).
3. **Archive criterion:** a report moves to `archive/` only when _every_ forward-looking item in it is closed or routed to a living doc, and its stale claims are struck or bucket-closed.
4. **Never rewrite history:** annotations are additive and non-destructive; original wording stays visible under strikethrough.
5. **Generated HTML artifacts (decided 2026-09-06):** HTML reports referenced by living docs (reviews/, brainstorming/, research/, planning/) stay where they are — they are linked deliverables, not clutter. Only **superseded era dashboards** that sit in `status/` root (e.g. the June 2026 vendor/buildflow dashboards) move to `archive/` via `git mv`.
6. **dprint scope (decided 2026-09-06):** `docs/status/**` and `docs/planning/**` are **excluded** from dprint (see `dprint.json`) — point-in-time snapshots are frozen after write, so formatter reflows can never masquerade as history edits. Living docs (AGENTS/README/FEATURES/TODO_LIST/ROADMAP/CONTRIBUTING/docs/*.md/docs/adr) stay formatted.
7. **Archive freshness norm (decided 2026-09-06):** same-day archiving of the freshest report is the norm once every forward item is closed or routed — the root intentionally holds only this README between sessions; the next report starts a fresh cycle.

## For harvesters (AI sessions)

Harvest forward-looking items from the most recent 1–3 reports only; verify against code before adding to TODO_LIST; drop anything already shipped. `archive/` is context, not a backlog.
