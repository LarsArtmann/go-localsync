# Benchmark protocol

Benchmark numbers in this repo are only comparable when produced the same
way. Ad-hoc `go test -bench` output with default settings is NOT comparable
to protocol numbers (default `-benchtime=1s` auto-scales iterations, which
hides the SQLite write-amplification effects these benchmarks exist to
expose).

## Protocol

```bash
./scripts/run-benchmarks.sh                # all benchmarks
./scripts/run-benchmarks.sh Replay         # filter (passed to -bench)
./scripts/run-benchmarks.sh Replay old.txt # benchstat compare vs old run
```

- Fixed iterations: `-benchtime 20x` — every iteration does real work
  (batch syncs, replays), so a fixed count keeps work per sample constant
  and keeps runs fast enough to repeat.
- Repeated samples: `-count 5` — benchstat needs ≥5 samples for meaningful
  statistics (median, p-values).
- Comparison: `benchstat old.txt new.txt` — only deltas with low variance
  and a small p-value are regressions/improvements; single-run numbers are
  noise until proven otherwise.
- Results land in `bench-results/<date>-<label>.txt`; commit the file when
  recording a number in docs.

## Environment caveats (record honestly next to any published number)

- The benchmarks hit real SQLite files (modernc, CGO-free) with WAL — disk
  speed matters; a dev machine's NVMe will beat CI runners by a wide margin.
- `waitForCountTB` polling granularity (1ms) adds sub-millisecond noise to
  pipeline benchmarks.
- GOEXPERIMENT=jsonv2 build tag changes encoding performance; always run
  inside the devShell so the tag matches production builds.
- CPU frequency scaling and background load dominate short benchmarks —
  that is why the protocol mandates 5 samples and benchstat.

## What each benchmark measures

| Benchmark                                  | Question it answers                                                        |
| ------------------------------------------ | -------------------------------------------------------------------------- |
| `BenchmarkPipeline_Sync10kItems`           | End-to-end batch ingest cost (validation → decide → events → projection).  |
| `BenchmarkPipeline_Replay10kEvents`        | True from-zero replay: checkpoints are wiped per iteration so the drain    |
|                                            | re-consumes the whole journal (measures replay, not reopen).               |
| `BenchmarkPipeline_SQLiteGrowth`           | Write amplification as one database file grows across batches.             |
| `BenchmarkConflict_SyncExisting`           | Per-item conflict path with a resolver (detect + resolve + 2 events).      |
| `BenchmarkUpcastedLegacyRead`              | V1→V3 upcast-on-read vs native pass-through: the price of legacy support.  |
| `BenchmarkMemoryReadModel_List` etc.       | Read-model query cost with filters/pagination.                             |
