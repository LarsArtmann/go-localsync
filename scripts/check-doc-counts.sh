#!/usr/bin/env bash
# check-doc-counts.sh — fail when hand-copied numbers in docs drift from code.
#
# Every documentation-drift incident in this repo's history involved
# hand-copied numbers (test counts went 232 → 437 → 309 across releases
# without anyone noticing). This script makes the drift loud:
#
#   1. per-package test counts (AGENTS.md test table)
#   2. total test-function / package / provider counts (AGENTS, README, FEATURES)
#   3. AGENTS.md dependency-table versions vs go.mod
#
# Default mode is compile-only (go test -list) and CI-friendly. Pass
# --coverage to also compare the per-package coverage column against a
# fresh `go test -cover` run (±1.0 percentage-point tolerance).
set -euo pipefail

cd "$(dirname "$0")/.."

MODULE_PREFIX="github.com/larsartmann/go-localsync"
AGENTS="AGENTS.md"
README="README.md"
FEATURES="FEATURES.md"

fail=0
warn_prefix="::error::"

die() {
	printf '%s%s\n' "$warn_prefix" "$*"
	fail=1
}

# --- 1. actual per-package test counts --------------------------------------

core_list="$(go test -list '.*' ./...)"
provider_list="$(cd provider/github && GOWORK=off go test -list '.*' ./...)"

# name_lines "<list output>" -> count of top-level test identifiers.
name_lines() {
	grep -cE '^(Test|Benchmark|Example|Fuzz)[A-Za-z0-9_]*$' <<<"$1" || true
}

# per_pkg "<list output>" -> "pkg/short count" lines.
# go test -list prints a package's names BEFORE its `ok <pkg>` boundary line,
# so names are accumulated into `pending` and flushed on the boundary.
per_pkg() {
	awk -v prefix="$MODULE_PREFIX/" '
		/^ok / { print substr($2, length(prefix)+1), pending; pending = 0; next }
		/^\?/  { pending = 0; next }
		/^(Test|Benchmark|Example|Fuzz)/ { pending++ }
	' <<<"$1"
}

actual_total="$(name_lines "$core_list")"
actual_provider="$(name_lines "$provider_list")"

# packages that HAVE tests (the docs claim "N test packages")
actual_pkgs="$(per_pkg "$core_list" | wc -l | tr -d ' ')"

# --- 2. AGENTS.md per-package test table ------------------------------------

while read -r pkg actual; do
	documented="$(grep -E "^\| \`$pkg\`" "$AGENTS" | head -1 | awk -F'|' '{gsub(/ /,"",$3); print $3}')"
	if [[ -z "$documented" ]]; then
		die "AGENTS.md test table has no row for $pkg (actual: $actual tests)"
	elif [[ "$documented" != "$actual" ]]; then
		die "AGENTS.md test table: $pkg says $documented, code has $actual"
	fi
done < <(per_pkg "$core_list")

# --- 3. totals across AGENTS / README / FEATURES ----------------------------

check_pair() {
	local file="$1" pattern="$2" what="$3" actual="$4" pick="${5:-first}"
	local documented
	local pick_arg="head -1"
	[[ "$pick" == "last" ]] && pick_arg="tail -1"
	documented="$(grep -oE "$pattern" "$file" | grep -oE '[0-9]+' | eval "$pick_arg" || true)"
	if [[ -z "$documented" ]]; then
		die "$file: no match for '$what' — pattern drifted or claim removed"
	elif [[ "$documented" != "$actual" ]]; then
		die "$file: '$what' says $documented, code has $actual"
	fi
}

check_pair "$AGENTS" '\*\*[0-9]+ total test functions\*\*' \
	"total test functions" "$actual_total"
check_pair "$AGENTS" 'total test functions\*\* across [0-9]+ test packages' \
	"AGENTS test-package count" "$actual_pkgs"
check_pair "$AGENTS" 'plus [0-9]+ in the standalone .provider/github. module' \
	"provider/github test count" "$actual_provider" last

check_pair "$README" '[0-9]+ tests across [0-9]+ packages' \
	"README quickstart count" "$actual_total"
check_pair "$README" '[0-9]+ test functions across [0-9]+ packages \(plus [0-9]+ in the standalone' \
	"README testing-section total" "$actual_total"
check_pair "$README" 'test functions across [0-9]+ packages \(plus [0-9]+ in the standalone' \
	"README provider count" "$actual_provider" last

check_pair "$FEATURES" '[0-9]+ test functions across [0-9]+ packages \(plus [0-9]+ in the standalone' \
	"FEATURES test-suite total" "$actual_total"
check_pair "$FEATURES" 'test functions across [0-9]+ packages \(plus [0-9]+ in the standalone' \
	"FEATURES provider count" "$actual_provider" last

# --- 4. AGENTS.md dependency table vs go.mod --------------------------------

in_deps=0
while IFS= read -r line; do
	case "$line" in
	"## Dependencies") in_deps=1; continue ;;
	"## "*|"### "*) in_deps=0; continue ;;
	esac

	[[ "$in_deps" == 1 ]] || continue
	[[ "$line" == "| \`"* ]] || continue

	mod="$(awk -F'|' '{gsub(/[` ]/,"",$2); print $2}' <<<"$line")"
	ver="$(awk -F'|' '{gsub(/ /,"",$3); print $3}' <<<"$line")"

	[[ "$ver" =~ ^v[0-9] ]] || continue # skip rows without a version column

	full=""
	for candidate in "$mod" "github.com/larsartmann/$mod" "github.com/$mod"; do
		if grep -qE "^	$candidate " go.mod; then
			full="$candidate"
			break
		fi
	done

	if [[ -z "$full" ]]; then
		die "AGENTS.md dependency table: $mod not found in go.mod"
	else
		gomod_ver="$(awk -v m="$full" '$1==m{print $2}' go.mod)"
		if [[ "$gomod_ver" != "$ver" ]]; then
			die "AGENTS.md dependency table: $mod says $ver, go.mod has $gomod_ver"
		fi
	fi
done <"$AGENTS"

# --- 5. optional: per-package coverage column -------------------------------

if [[ "${1:-}" == "--coverage" ]]; then
	cover_out="$(go test -cover ./...)"
	while read -r pkg actual; do
		documented="$(grep -E "^\| \`$pkg\`" "$AGENTS" | head -1 | awk -F'|' '{gsub(/ /,"",$4); print $4}')"
		actual_pct="$(grep -E "^ok +$MODULE_PREFIX/$pkg( |	)" <<<"$cover_out" | grep -oE '[0-9]+\.[0-9]% of statements' | grep -oE '^[0-9]+\.[0-9]' | head -1)"
		if [[ -z "$actual_pct" ]]; then
			die "coverage run produced no number for $pkg"
		fi
		drift="$(awk -v a="$documented" -v b="$actual_pct" 'BEGIN{d=a-b; if(d<0) d=-d; printf "%.1f", d}')"
		if awk -v d="$drift" 'BEGIN{exit !(d > 1.0)}'; then
			die "AGENTS.md coverage for $pkg says $documented%, fresh run shows $actual_pct% (drift $drift > 1.0)"
		fi
	done < <(per_pkg "$core_list")
fi

# ----------------------------------------------------------------------------

if [[ "$fail" == 1 ]]; then
	printf '%sDocumentation counts drifted from code — fix the flagged claims (they are the numbers humans copy next).\n' "$warn_prefix"
	exit 1
fi

echo "doc counts in sync: $actual_total core tests / $actual_pkgs packages (+$actual_provider provider), deps match go.mod"
