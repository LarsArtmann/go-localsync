# Changelog

All notable changes to the `provider/github` nested module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/). This module versions independently of the go-localsync core: it pins a released core version (see `go.mod`), so core breaking changes land here only when this module explicitly adopts them.

## [Unreleased]

### Added

- **Self-contained lint gate** — `provider/github/.golangci.yml` (standard set + canonicalheader/godot/misspell/wrapcheck; exhaustruct deliberately omitted: every struct literal here is an external SDK DTO or test fixture, partial by design). Stops golangci config discovery from walking up to the root config; CI's provider job now runs pinned golangci-lint v2.13.2 alongside build + race.
- **ETag documentation** — README section with the `WithETagCache` usage snippet, a config-table row, an `ETagStats()` example, and the recorded decision that a page-1 304 probe shortcut would be INCORRECT for shifting event feeds (a cached page 1 proves nothing about pages 2..N; per-page 304s via the kernel are the right granularity).
- **ETag conditional cache wiring** — `Client.WithETagCache(githubkit.ETagOptions)` enables the kernel's conditional GET cache (unchanged re-fetches become free 304 revalidations; `Client.ETagStats()` reports hits/stored/entries). Off by default. 4 tests cover hit/refetch/default-off/derive paths.

### Changed

- Test-server header literals canonicalized (`X-RateLimit-*` → `X-Ratelimit-*`, matching Go's `textproto.CanonicalMIMEHeaderKey` and the canonicalheader linter).
- External errors now wrapped with context on the rate-limit fetch and event-encode paths (`github: fetch rate limits: %w`, `github: encode event %s: %w`).

### Verified (claims, no code change)

- The README's kit-behavior claims are now source-annotated: empty PAT → no `WithAuthToken` call (unauthenticated, GitHub-documented 60 req/h core budget); retry = 429 any method + idempotent 5xx, `Retry-After` overrides backoff (go-github-kit v0.3.0 `client.go` / `transport.go`).

## [0.1.0] - 2026-09-05

First release of the GitHub events provider as an optional nested module.

### Added

- `provider.Provider` implementation over [go-github-kit](https://github.com/larsartmann/go-github-kit) v0.3.0: token auth, rate-limit gating fed from `X-RateLimit-*` response headers, retry with backoff (3 attempts, 1s→30s), sequential page-1 probe + concurrent `FetchAll` via `githubkit.FetchPages`.
- Error-family mapping (`wrapGitHubError`) including the transient `ErrProviderUnavailable` sentinel, preserving the original cause in the chain.
- Env-gated live PAT smoke test (`GITHUB_PAT`) proving the released kit wiring end-to-end.
- Parent pin on the released `go-localsync v0.5.0` (replacing the master pseudo-version); standalone CI leg builds + race-tests the module in isolation (`GOWORK=off`).
