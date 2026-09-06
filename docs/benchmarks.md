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

| Benchmark                            | Question it answers                                                       |
| ------------------------------------ | ------------------------------------------------------------------------- |
| `BenchmarkPipeline_Sync10kItems`     | End-to-end batch ingest cost (validation → decide → events → projection). |
| `BenchmarkPipeline_Replay10kEvents`  | True from-zero replay: checkpoints are wiped per iteration so the drain   |
|                                      | re-consumes the whole journal (measures replay, not reopen).              |
| `BenchmarkPipeline_SQLiteGrowth`     | Write amplification as one database file grows across batches.            |
| `BenchmarkConflict_SyncExisting`     | Per-item conflict path with a resolver (detect + resolve + 2 events).     |
| `BenchmarkUpcastedLegacyRead`        | V1→V3 upcast-on-read vs native pass-through: the price of legacy support. |
| `BenchmarkMemoryReadModel_List` etc. | Read-model query cost with filters/pagination.                            |

## 2026-09-06 protocol run (Ryzen AI MAX+ 395, NVMe, devShell jsonv2, 20x × 5)

| Benchmark                               | Result             | Reading                                                          |
| --------------------------------------- | ------------------ | ---------------------------------------------------------------- |
| `BenchmarkConflict_SyncExisting`        | ~26 ms / 200 items | ~130 µs per conflicting item (detect + resolve + 2 events).      |
| `BenchmarkUpcastedLegacyRead/legacy-v1` | ~12.7 ms / 1k evts | Upcast pipeline (decode → fold → CBOR re-encode → rebuild).      |
| `BenchmarkUpcastedLegacyRead/native-v3` | ~3.8 ms / 1k evts  | Pass-through fast path.                                          |
| Upcast tax                              | **~3.3×**          | Real but bounded; only paid on pre-V3 events, never on new ones. |

`BenchmarkPipeline_Replay10kEvents` was fixed the same day to a true from-zero
replay (checkpoints wiped per iteration); any number recorded before that fix
measured stack open/close, not replay — do not compare across the fix.

## 2026-09-06 evening re-run (post log-level / ETag changes)

Question: did the v0.6 log-level control or the provider ETag cache shift any
benchmarked path? **No — by code path and by numbers.** The log-level knob
applies to loggers the benchmarks never construct (`WithLogLevel` /
`CQRSConfig.LogLevel` are configuration-time, not in the measured loop); the
ETag cache lives in `provider/github`, which contributes zero benchmarks to
the protocol. The sync-span attributes added the same day sit behind the
nil-tracer fast path the benchmarks take (unconfigured tracer → zero work).

Protocol re-run on the same machine (evening, load average ~20 — this box's
normal multi-user state; both the morning baseline and this run recorded
under comparable load, so the deltas are apples-to-apples):

- geomean 623µ → 632µ (**+1.4%**, i.e. noise; per-benchmark deltas split in
  both directions: `DataItemFromPayload` −55%, `SQLiteReadModel_List` +44%,
  upcast legacy −18%, everything else `~`).
- The two cache-heavy SQLite read numbers remain the most load-sensitive
  (`SQLiteReadModel_List` swung +44% between two same-day runs under load) —
  consistent with the environment caveat above: disk+CPU contention dominate
  these on a busy box, which is exactly why the protocol mandates benchstat
  over single runs.
