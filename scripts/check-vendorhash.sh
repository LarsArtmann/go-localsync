#!/usr/bin/env bash
# check-vendorhash.sh — fail when go.mod/go.sum changed without a flake re-pin.
#
# The 2026-09-05 dependency refresh changed go.mod/go.sum after flake.nix's
# vendorHash was recorded; nix build / nix flake check then failed with a
# cryptic hash mismatch and nobody noticed for over an hour (no CI job owned
# the flake until the nix job landed). This guard turns that state into an
# immediate, self-explanatory failure naming the fix.
#
# Usage:
#   scripts/check-vendorhash.sh [BASE]
#     BASE defaults to HEAD~1 locally. In CI pass the event's diff base
#     (PR base sha or the push's before-sha). An unresolvable BASE (new
#     branch, force-push) is skipped with a notice — the nix flake check
#     job remains the authoritative gate.
set -euo pipefail

cd "$(dirname "$0")/.."

BASE="${1:-HEAD~1}"

if ! git rev-parse --verify --quiet "$BASE" >/dev/null 2>&1; then
	echo "::notice::check-vendorhash: base '$BASE' not resolvable — skipping (nix flake check remains the gate)"
	exit 0
fi

deps_changed=0
flake_changed=0
git diff --name-only "$BASE" -- go.mod go.sum | grep -q . && deps_changed=1 || true
git diff --name-only "$BASE" -- flake.nix | grep -q . && flake_changed=1 || true

if [[ "$deps_changed" == 1 && "$flake_changed" == 0 ]]; then
	{
		echo "::error::go.mod/go.sum changed without a flake.nix re-pin — vendorHash is stale."
		echo "nix build / nix flake check will fail (or, worse, have already failed silently)."
		echo "Fix:"
		echo "  1. run: nix build 2>&1 | grep -E 'got:|specified:'"
		echo "  2. paste the 'got:' sha256 into flake.nix -> go-standard.vendorHash"
		echo "  3. commit go.mod/go.sum + flake.nix together, then re-run nix flake check"
	} >&2
	exit 1
fi

echo "vendorHash guard ok (base=$BASE deps_changed=$deps_changed flake_changed=$flake_changed)"
