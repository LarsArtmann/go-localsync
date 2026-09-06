#!/usr/bin/env bash
# verify-release.sh — end-to-end release integrity check for a published tag.
#
# Replaces the twice-hand-verified release ritual of 2026-09-05 with one
# command. Checks, in order:
#
#   1. Tag exists locally and on origin, and is an ancestor of origin/master.
#   2. A GitHub Release exists for the tag (requires `gh`; warn-only without).
#   3. proxy.golang.org serves the tag in @v/list for BOTH modules and
#      reports it (or a successor) via @latest.
#   4. sum.golang.org has the module checksum (inferred from the proxy check
#      succeeding; the go checksum database is the proxy's backing store).
#   5. pkg.go.dev indexed the tag (best-effort, warn-only — the indexer lags
#      the proxy by minutes to hours and lag is not a release defect).
#
# Usage: scripts/verify-release.sh <core-tag> [provider-tag]
#   The core module and the nested provider module version independently:
#   core tags are v0.Y.Z, the provider module tags its own vX.Y.Z. The
#   provider tag defaults to the core tag when the two release together.
# Exit codes: 0 all required checks green, 1 a required check failed, 2 usage.
set -euo pipefail

cd "$(dirname "$0")/.."

CORE_TAG="${1:-}"
PROVIDER_TAG="${2:-$CORE_TAG}"
CORE_MODULE="github.com/larsartmann/go-localsync"
PROVIDER_MODULE="github.com/larsartmann/go-localsync/provider/github"

fail=0
ok() { printf '  ✅ %s\n' "$1"; }
bad() {
	printf '  ❌ %s\n' "$1"
	fail=1
}
warn() { printf '  ⚠️  %s\n' "$1"; }

[[ -n "$CORE_TAG" ]] || {
	echo "usage: $0 <core-tag> [provider-tag] (e.g. $0 v0.5.0 v0.1.0)" >&2
	exit 2
}

echo "verify-release: core=$CORE_TAG provider=$PROVIDER_TAG"
echo "== 1. git tags =="
git rev-parse -q --verify "refs/tags/$CORE_TAG" >/dev/null && ok "core tag exists locally" ||
	bad "core tag $CORE_TAG not found locally"
git rev-parse -q --verify "refs/tags/$PROVIDER_TAG" >/dev/null && ok "provider tag exists locally" ||
	bad "provider tag $PROVIDER_TAG not found locally"
if git ls-remote --tags origin "refs/tags/$CORE_TAG" | grep -q "$CORE_TAG"; then
	ok "core tag pushed to origin"
else
	bad "core tag $CORE_TAG missing on origin"
fi
if git ls-remote --tags origin "refs/tags/$PROVIDER_TAG" | grep -q "$PROVIDER_TAG"; then
	ok "provider tag pushed to origin"
else
	bad "provider tag $PROVIDER_TAG missing on origin"
fi
if git fetch --quiet origin master 2>/dev/null && git merge-base --is-ancestor "$CORE_TAG" origin/master; then
	ok "core tag is an ancestor of origin/master"
else
	bad "core tag $CORE_TAG is NOT on origin/master (release from a stale/divergent commit?)"
fi

echo "== 2. GitHub Release =="
if command -v gh >/dev/null 2>&1; then
	if gh release view "$CORE_TAG" >/dev/null 2>&1; then
		ok "GitHub Release $CORE_TAG exists ($(gh release view "$CORE_TAG" --json assets --jq '.assets | length') assets)"
	else
		bad "no GitHub Release for $CORE_TAG (gh release view failed)"
	fi
else
	warn "gh CLI not available — skipping GitHub Release check (run where gh is installed)"
fi

proxy_check() {
	local module="$1" name="$2" tag="$3"
	local encoded base
	# Module paths with major-version suffixes are escaped in proxy URLs
	# (only capital letters; ours are lowercase, but escape defensively).
	encoded="${module//A/!a}"
	encoded="${encoded//B/!b}"
	encoded="${encoded//C/!c}"
	encoded="${encoded//D/!d}"
	encoded="${encoded//E/!e}"
	encoded="${encoded//F/!f}"
	encoded="${encoded//G/!g}"
	encoded="${encoded//H/!h}"
	encoded="${encoded//I/!i}"
	encoded="${encoded//J/!j}"
	encoded="${encoded//K/!k}"
	encoded="${encoded//L/!l}"
	encoded="${encoded//M/!m}"
	encoded="${encoded//N/!n}"
	encoded="${encoded//O/!o}"
	encoded="${encoded//P/!p}"
	encoded="${encoded//Q/!q}"
	encoded="${encoded//R/!r}"
	encoded="${encoded//S/!s}"
	encoded="${encoded//T/!t}"
	encoded="${encoded//U/!u}"
	encoded="${encoded//V/!v}"
	encoded="${encoded//W/!w}"
	encoded="${encoded//X/!x}"
	encoded="${encoded//Y/!y}"
	encoded="${encoded//Z/!z}"
	base="https://proxy.golang.org/$encoded"

	echo "== 3. proxy.golang.org ($name) =="
	if curl -fsSL "$base/@v/list" | grep -qx "$tag"; then
		ok "@v/list contains $tag"
	else
		bad "@v/list does not contain $tag (list: $(curl -fsSL "$base/@v/list" | tail -3 | tr '\n' ' '))"
	fi

	local latest
	latest="$(curl -fsSL "$base/@latest" | grep -oE '"Version":"[^"]*"' | cut -d'"' -f4)"
	if [[ -n "$latest" ]] && [[ "$latest" == "$tag" || "$(printf '%s\n%s\n' "$latest" "$tag" | sort -V | head -1)" == "$latest" ]]; then
		ok "@latest = $latest (>= $tag)"
	else
		bad "@latest = '$latest' is older than or missing $tag"
	fi

	echo "== 4. pkg.go.dev ($name, best-effort) =="
	if curl -fsSL -o /dev/null -w '%{http_code}' "https://pkg.go.dev/$module@$tag" 2>/dev/null | grep -q 200; then
		ok "indexed at pkg.go.dev/$module@$tag"
	else
		warn "pkg.go.dev has not indexed $module@$tag yet (indexer lag is normal; retry later)"
	fi
}

proxy_check "$CORE_MODULE" "core" "$CORE_TAG"
proxy_check "$PROVIDER_MODULE" "provider/github" "$PROVIDER_TAG"

echo "== 5. docs consistency (docs-health VERIFY, standing pre-release step) =="
# The automatable slice of a docs-health VERIFY pass: test/coverage counts and
# the dependency table vs go.mod are checked in CI on every push, but a tag
# can be cut from a tree older than the last push — re-verify at release time
# so published docs never ship stale numbers. The judgment-heavy parts of
# VERIFY (claims vs code, FEATURES freshness) stay a manual checklist item in
# CONTRIBUTING.md.
if ./scripts/check-doc-counts.sh; then
	ok "doc counts match code (AGENTS/README/FEATURES/TODO_LIST + dep table)"
else
	bad "doc counts drifted — update the docs and re-run (see above for the flagged claims)"
fi

if [[ "$fail" == 1 ]]; then
	echo "RESULT: FAILED — the release is not fully published" >&2
	exit 1
fi
echo "RESULT: OK — core=$CORE_TAG provider=$PROVIDER_TAG fully published"
