# Nix systems triage — what `nix flake check` covers and why `--all-systems` is not the local gate

## The short version

| Command                       | Systems touched                              | Where it is the gate          |
| ----------------------------- | -------------------------------------------- | ----------------------------- |
| `nix flake check`             | current system only (`x86_64-linux` here)    | locally + CI `nix` job        |
| `nix flake check --all-systems` | all four declared systems (evaluation + builds every `checks.*` per system) | nowhere, deliberately        |
| CI `build` matrix             | linux/darwin × amd64/arm64 + windows/amd64 via the Go toolchain | CI only                |

## Which systems the flake declares

The flake builds on flake-parts defaults (via the `go-standard` module from
go-nix-helpers), which evaluate four systems:

- `x86_64-linux`
- `aarch64-linux`
- `x86_64-darwin`
- `aarch64-darwin`

Windows is intentionally absent — Nix does not target it; cross-platform
compile proof for windows/amd64 lives in the CI `build` matrix
(`CGO_ENABLED=0`, pure-Go sqlite).

## Why `--all-systems` is not part of the local or CI gate

1. **Local machines cannot build darwin derivations.** Evaluating
   `aarch64-darwin` checks requires a darwin builder; a Linux-only machine
   fails those legs regardless of repository state. Running
   `nix flake check --all-systems` locally produces guaranteed-red noise, not
   signal. (This is also why buildflow's nix steps are skipped locally —
   see `.buildflow.yml`.)
2. **CI's nix job runs on `ubuntu-latest`** — same constraint. Its value is
   catching vendorHash drift and evaluation errors fast on the system every
   contributor has, not proving cross-compilation.
3. **Cross-platform truth is already covered more cheaply** by the CI `build`
   matrix, which compiles the library for all four Go OS/arch combinations
   plus windows with the plain Go toolchain.

## What each system is FOR

- `x86_64-linux` — the primary development and CI platform; the only system
  `nix flake check` evaluates locally and in CI.
- `aarch64-linux` — arm64 Linux (e.g. ARM CI runners, Graviton). Builds in
  nix terms only where an arm64 builder exists; compile coverage via CI matrix.
- `x86_64-darwin` / `aarch64-darwin` — macOS contributors. `nix develop`
  works there with a local nix installation; the hermetic checks for those
  systems are expected to be run on such machines (or skipped in favor of the
  Go-level gates, which are platform-agnostic).

## When to actually run `--all-systems`

Only when deliberately changing system-dependent flake surface (e.g. adding a
new package with platform variations), and then from a machine with remote
builders configured for the darwin systems — or accept that the darwin legs
fail on builder availability and read only the evaluation results.
