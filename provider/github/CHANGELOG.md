# Changelog

All notable changes to the `provider/github` nested module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/). This module versions independently of the go-localsync core: it pins a released core version (see `go.mod`), so core breaking changes land here only when this module explicitly adopts them.

## [Unreleased]

Nothing yet. Planned (post core-v0.6.0): adopt the v0.6 `SourceID`/`StreamID` vocabulary (tracked in the core [TODO_LIST.md](../TODO_LIST.md)).

## [0.1.0] - 2026-09-05

First release of the GitHub events provider as an optional nested module.

### Added

- `provider.Provider` implementation over [go-github-kit](https://github.com/larsartmann/go-github-kit) v0.3.0: token auth, rate-limit gating fed from `X-RateLimit-*` response headers, retry with backoff (3 attempts, 1s→30s), sequential page-1 probe + concurrent `FetchAll` via `githubkit.FetchPages`.
- Error-family mapping (`wrapGitHubError`) including the transient `ErrProviderUnavailable` sentinel, preserving the original cause in the chain.
- Env-gated live PAT smoke test (`GITHUB_PAT`) proving the released kit wiring end-to-end.
- Parent pin on the released `go-localsync v0.5.0` (replacing the master pseudo-version); standalone CI leg builds + race-tests the module in isolation (`GOWORK=off`).
