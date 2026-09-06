#!/usr/bin/env bash
# run-benchmarks.sh — run Go benchmarks per docs/benchmarks.md protocol.
#
#   scripts/run-benchmarks.sh [bench-filter] [old-results-file]
#
# Uses fixed iterations (-benchtime 20x) and 5 samples so benchstat can do
# meaningful statistics. With an old results file, prints a benchstat
# comparison instead of raw output.
set -euo pipefail

cd "$(dirname "$0")/.."

FILTER="${1:-.}"
OLD="${2:-}"

mkdir -p bench-results

STAMP="$(date +%Y%m%d-%H%M%S)"
OUT="bench-results/${STAMP}.txt"

# The goexperiment.jsonv2 tag must match production builds (see AGENTS.md);
# inside the nix devShell GOFLAGS already carries it.
if [[ -z "${GOFLAGS:-}" ]]; then
	export GOFLAGS="-tags=goexperiment.jsonv2"
fi

echo "running benchmarks matching '$FILTER' (20 iterations, 5 samples) -> $OUT"

go test ./pkg/cqrs/ ./pkg/sync/ -run XXX -bench "$FILTER" -benchtime 20x -count 5 | tee "$OUT"

if [[ -n "$OLD" ]]; then
	if ! command -v benchstat >/dev/null 2>&1; then
		echo "benchstat not found; install with: go install golang.org/x/perf/cmd/benchstat@latest" >&2
		exit 2
	fi

	echo
	benchstat "$OLD" "$OUT"
fi
