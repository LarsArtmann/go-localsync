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
#   4. rule-catalog size claims ("N architectural checks (C0001-C00NN)")
#   5. optional: per-package coverage column vs a fresh `go test -cover` run
#
# Default mode is compile-only (go test -list) and CI-friendly. Pass
# --coverage to also compare the per-package coverage column (±1.0
# percentage-point tolerance). Pass --fix to rewrite drifted claims in place
# (local convenience only — CI stays check-only so drift is always reviewed).
set -euo pipefail

cd "$(dirname "$0")/.."

MODULE_PREFIX="github.com/larsartmann/go-localsync"
AGENTS="AGENTS.md"
README="README.md"
FEATURES="FEATURES.md"
LINT_DOC="docs/localsync-lint.md"

fix=0
coverage=0
for arg in "$@"; do
	case "$arg" in
	--fix) fix=1 ;;
	--coverage) coverage=1 ;;
	*)
		printf 'usage: %s [--fix] [--coverage]\n' "$0" >&2
		exit 2
		;;
	esac
done

fail=0
warn_prefix="::error::"

die() {
	printf '%s%s\n' "$warn_prefix" "$*"
	fail=1
}

# fixed MSG... — announce an in-place rewrite (plain prefix: it is good news).
fixed() {
	printf 'fixed: %s\n' "$*"
}

# fix_number FILE ERE ACTUAL [first|last] — in every line of FILE matching ERE,
# replace the first (default) or last [0-9]+ span inside the FIRST ERE match.
#
# Limitation (documented, deliberate): check_pair's pick semantics operate on
# grep -oE matches across the whole file; this helper rewrites the first match
# per line. Every claim site in these docs is one match per line, so the two
# views coincide. The regex travels via ENVIRON so awk performs no -v escape
# mangling on patterns like `\*\*[0-9]+ total test functions\*\*`.
fix_number() {
	local file="$1"
	FIX_REGEX="$2" FIX_VALUE="$3" FIX_PICK="${4:-first}" awk '
		function replace_num(line, regex, value, pick,    m, rest, base, cnt, s, e, fs, fe, ls, le, nm, or_start, or_len) {
			if (!match(line, regex)) return line
			or_start = RSTART; or_len = RLENGTH # saved before the loop below clobbers them
			m = substr(line, or_start, or_len)
			rest = m; base = 1; cnt = 0; ls = 0; le = 0
			while (match(rest, /[0-9]+/)) {
				cnt++
				s = base + RSTART - 1
				e = base + RSTART + RLENGTH - 2
				if (cnt == 1) { fs = s; fe = e }
				ls = s; le = e
				base = e + 1
				rest = substr(m, base)
			}
			if (cnt == 0) return line
			if (pick == "last") { fs = ls; fe = le }
			nm = substr(m, 1, fs - 1) value substr(m, fe + 1)
			return substr(line, 1, or_start - 1) nm substr(line, or_start + or_len)
		}
		{ print replace_num($0, ENVIRON["FIX_REGEX"], ENVIRON["FIX_VALUE"], ENVIRON["FIX_PICK"]) }
	' "$file" >"$file.tmp" && mv "$file.tmp" "$file"
}

# fix_row_value FILE KEY COL VALUE — rewrite column COL of the first markdown
# table row whose second cell (backticks and spaces stripped) equals KEY.
# Used for the AGENTS.md test-count table (KEY=pkg path) and the dependency
# table (KEY=module path); both are keyed uniquely.
fix_row_value() {
	local file="$1" key="$2" col="$3" value="$4"
	FIX_KEY="$key" FIX_COL="$col" FIX_VALUE="$value" awk -F'|' '
		BEGIN { OFS = "|" }
		function strip(cell) { gsub(/[` ]/, "", cell); return cell }
		!done && NF > 2 && strip($2) == ENVIRON["FIX_KEY"] {
			col = ENVIRON["FIX_COL"] + 0
			if (col == 3 || col == 4) {
				new = " " ENVIRON["FIX_VALUE"]
				# preserve the cell width dprint aligned so a shrinking value
			# cannot de-format the table (growing values still need dprint fmt)
				if (length($col) > length(new)) new = new sprintf("%*s", length($col) - length(new), "")
				$(col) = new
			}
			done = 1
		}
		{ print }
	' "$file" >"$file.tmp" && mv "$file.tmp" "$file"
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
		if [[ "$fix" == 1 ]]; then
			fix_row_value "$AGENTS" "$pkg" 3 "$actual"
			fixed "AGENTS.md test table: $pkg says $documented, code has $actual"
		else
			die "AGENTS.md test table: $pkg says $documented, code has $actual"
		fi
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
		if [[ "$fix" == 1 ]]; then
			fix_number "$file" "$pattern" "$actual" "$pick"
			fixed "$file: '$what' says $documented, code has $actual"
		else
			die "$file: '$what' says $documented, code has $actual"
		fi
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
	"## Dependencies")
		in_deps=1
		continue
		;;
	"## "* | "### "*)
		in_deps=0
		continue
		;;
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
			if [[ "$fix" == 1 ]]; then
				fix_row_value "$AGENTS" "$mod" 3 "$gomod_ver"
				fixed "AGENTS.md dependency table: $mod says $ver, go.mod has $gomod_ver"
			else
				die "AGENTS.md dependency table: $mod says $ver, go.mod has $gomod_ver"
			fi
		fi
	fi
done <"$AGENTS"

# --- 5. rule-catalog size claims ---------------------------------------------
# "10 → 15 rules" drifted silently when C0011 landed; the catalog size is now
# derived from the Rules() declaration table and every doc claim must match.
# The range half of the claim (C0001-C00NN) is not validated — only the count.

rules_file="$(grep -l '^func Rules()' internal/cqrslint/*.go | head -1)"
if [[ -z "$rules_file" ]]; then
	die "rule-catalog check: no 'func Rules()' found in internal/cqrslint — extraction pattern drifted"
fi
rule_count="$(awk '/^func Rules\(\)/,/^}/' "$rules_file" | grep -c 'ID: rule' || true)"
if [[ "$rule_count" -eq 0 ]]; then
	die "rule-catalog check: extracted 0 rules from $rules_file — extraction pattern drifted"
fi

RULE_CLAIM='[0-9]+ architectural checks \(C[0-9]+[-–]C[0-9]+\)'

check_rule_count() {
	local file="$1" found=0 documented
	while IFS= read -r match; do
		found=1
		documented="$(grep -oE '[0-9]+ architectural checks' <<<"$match" | grep -oE '[0-9]+' | head -1)"
		if [[ "$documented" != "$rule_count" ]]; then
			if [[ "$fix" == 1 ]]; then
				fix_number "$file" "$RULE_CLAIM" "$rule_count"
				fixed "$file: rule-count claim says $documented, catalog has $rule_count"
			else
				die "$file: rule-count claim says $documented rules, catalog has $rule_count (was the claim 'N architectural checks (C0001-C00NN)' reworded?)"
			fi
			return
		fi
	done < <(grep -hE '[0-9]+ architectural checks' "$file" || true)
	if [[ "$found" == 0 ]]; then
		die "$file: no rule-count claim (expected 'N architectural checks (C0001-C00NN)')"
	fi
}

check_rule_count "$AGENTS"
check_rule_count "$README"
check_rule_count "$LINT_DOC"

# --- 6. optional: per-package coverage column --------------------------------

if [[ "$coverage" == 1 ]]; then
	cover_out="$(go test -cover ./...)"
	while read -r pkg actual; do
		documented="$(grep -E "^\| \`$pkg\`" "$AGENTS" | head -1 | awk -F'|' '{gsub(/[% ]/,"",$4); print $4}')"
		actual_pct="$(grep -E "^ok[[:space:]]+$MODULE_PREFIX/$pkg[[:space:]]" <<<"$cover_out" | grep -oE '[0-9]+\.[0-9]% of statements' | grep -oE '^[0-9]+\.[0-9]' | head -1 || true)"
		if [[ -z "$actual_pct" ]]; then
			die "coverage run produced no number for $pkg"
		fi
		drift="$(awk -v a="$documented" -v b="$actual_pct" 'BEGIN{d=a-b; if(d<0) d=-d; printf "%.1f", d}')"
		if awk -v d="$drift" 'BEGIN{exit !(d > 1.0)}'; then
			if [[ "$fix" == 1 ]]; then
				fix_row_value "$AGENTS" "$pkg" 4 "$actual_pct%"
				fixed "AGENTS.md coverage for $pkg: $documented% -> $actual_pct% (drift $drift)"
			else
				die "AGENTS.md coverage for $pkg says $documented%, fresh run shows $actual_pct% (drift $drift > 1.0)"
			fi
		fi
	done < <(per_pkg "$core_list")
fi

# ----------------------------------------------------------------------------

if [[ "$fail" == 1 ]]; then
	if [[ "$fix" == 1 ]]; then
		printf 'Documentation counts had drift that --fix could not rewrite — fix the flagged claims manually.\n'
	else
		printf '%sDocumentation counts drifted from code — fix the flagged claims (they are the numbers humans copy next), or run: ./scripts/check-doc-counts.sh --fix\n' "$warn_prefix"
	fi
	exit 1
fi

if [[ "$fix" == 1 ]]; then
	echo "doc counts in sync after --fix: $actual_total core tests / $actual_pkgs packages (+$actual_provider provider), deps match go.mod, $rule_count lint rules"
else
	echo "doc counts in sync: $actual_total core tests / $actual_pkgs packages (+$actual_provider provider), deps match go.mod, $rule_count lint rules"
fi
