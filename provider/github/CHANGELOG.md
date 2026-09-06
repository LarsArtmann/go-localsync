# Changelog

All notable changes to the `provider/github` nested module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/). This module versions independently of the go-localsync core: it pins a released core version (see `go.mod`), so core breaking changes land here only when this module explicitly adopts them.

## [Unreleased]

### Added

- **ETag conditional cache wiring** — `Client.WithETagCache(githubkit.ETagOptions)` enables the kernel's conditional GET cache (unchanged re-fetches become free 304 revalidations; `Client.ETagStats()` reports hits/stored/entries). Off by default. 4 tests cover hit/refetch/default-off/derive paths.

### Verified (claims, no code change)

- The README's kit-behavior claims are now source-annotated: empty PAT → no `WithAuthToken` call (unauthenticated, GitHub-documented 60 req/h core budget); retry = 429 any method + idempotent 5xx, `Retry-After` overrides backoff (go-github-kit v0.3.0 `client.go` / `transport.go`).

## [0.1.0] - 2026-09-05

First release of the GitHub events provider as an optional nested module.

### Added

- `provider.Provider` implementation over [go-github-kit](https://github.com/larsartmann/go-github-kit) v0.3.0: token auth, rate-limit gating fed from `X-RateLimit-*` response headers, retry with backoff (3 attempts, 1s→30s), sequential page-1 probe + concurrent `FetchAll` via `githubkit.FetchPages`.
- Error-family mapping (`wrapGitHubError`) including the transient `ErrProviderUnavailable` sentinel, preserving the original cause in the chain.
- Env-gated live PAT smoke test (`GITHUB_PAT`) proving the released kit wiring end-to-end.
- Parent pin on the released `go-localsync v0.5.0` (replacing the master pseudo-version); standalone CI leg builds + race-tests the module in isolation (`GOWORK=off`).
