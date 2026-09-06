# go-localsync/provider/github

A GitHub events `provider.Provider` for
[go-localsync](https://github.com/larsartmann/go-localsync), built on
[go-github-kit](https://github.com/LarsArtmann/go-github-kit).

This is an **optional nested module**: the core `go-localsync` module has no
GitHub dependencies. Consumers opt in by requiring this module separately.

```bash
go get github.com/larsartmann/go-localsync@latest
go get github.com/larsartmann/go-localsync/provider/github@latest
```

## What it does

Implements the `provider.Provider` interface over GitHub's
`GET /users/{user}/events` endpoint:

- **Token authentication** via `GITHUB_TOKEN` / `GH_TOKEN` or an explicit PAT.
- **Rate-limit gating** (kernel transport): waits for the window reset when
  the remaining budget is at or below the floor, bounded by a max wait, fed
  from the `X-RateLimit-*` response headers of every call.
- **Retry with backoff** on 429 and idempotent 5xx (verified against the kit source, go-github-kit v0.3.0 `transport.go`: 429 is retried for any method because GitHub rejects before processing; 5xx only for idempotent methods; a `Retry-After` header overrides the computed backoff).
- **Optional ETag conditional cache** (`WithETagCache(githubkit.ETagOptions{...})`): unchanged re-fetches replay stored ETags as `If-None-Match` — GitHub answers 304, one request spent and zero budget counted against the data endpoints, with the kernel serving the cached body transparently. Inspect hits via `ETagStats()`. Off by default.
- **Concurrent pagination**: `FetchAll` fetches page 1 sequentially (a
  cheap probe for an exhausted endpoint), then walks pages 2..N on a bounded
  pool (default 3) and stops at the first short page.
- **Error classification**: GitHub failures map onto the go-localsync error
  family — `ErrRateLimited`, `ErrInvalidToken`, `ErrUserNotFound`,
  `ErrProviderUnavailable` — while the original SDK error stays reachable in
  the chain for `errors.AsType` diagnostics.

## Usage

```go
client := github.NewClient(os.Getenv("GITHUB_TOKEN")).
    WithRateLimitConfig(github.DefaultRateLimitConfig).
    WithFetchConfig(github.DefaultFetchConfig)

result, err := client.FetchAll(ctx, "octocat", 10)
if err != nil {
    switch {
    case errors.Is(err, localsyncerrors.ErrRateLimited):
        // back off and retry later
    case errors.Is(err, localsyncerrors.ErrUserNotFound):
        // bad username
    }
}

for _, item := range result.Items { /* ... */ }
if result.RateLimit != nil {
    log.Printf("core budget: %d/%d, resets %s",
        result.RateLimit.Remaining, result.RateLimit.Limit, result.RateLimit.ResetAt)
}
```

## Configuration

| Method                | Effect                                                                                                                                                                                                                                                             |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `NewClient(token)`    | Explicit PAT; an empty token means no auth header is sent (verified: kit skips `WithAuthToken` for empty tokens, go-github-kit v0.3.0 `client.go`) → GitHub's documented unauthenticated core budget of 60 req/h.                                                  |
| `WithRateLimitConfig` | `Enabled`, `MinRemaining` (default 10), `MaxWait` (default 15m).                                                                                                                                                                                                   |
| `WithFetchConfig`     | `MaxConcurrentFetches` (default 3), `OnProgress` callback.                                                                                                                                                                                                         |
| `WithRetryConfig`     | Max retries (default 3), initial/max backoff (default 1s → 30s).                                                                                                                                                                                                   |
| `WithETagCache`       | Conditional-GET cache (off by default): unchanged re-fetches replay stored ETags as `If-None-Match`; GitHub answers 304 and the cached body is served. Zero-value `githubkit.ETagOptions{}` gets kit defaults (256 entries, 8 MiB bodies). Returns a derived copy. |
| `WithBaseURL`         | Point at a different API root (GitHub Enterprise, test servers). Returns `(client, error)` — validate the URL, so not chainable.                                                                                                                                   |

### Conditional requests (ETag cache)

```go
client := github.NewClient(os.Getenv("GITHUB_TOKEN")).
    WithETagCache(githubkit.ETagOptions{}) // zero value = kit defaults

result, err := client.FetchAll(ctx, "octocat", 10)

// Cumulative cache counters; ok=false while the cache is disabled.
if stats, ok := client.ETagStats(); ok {
    log.Printf("etag cache: %d hits (304 revalidations), %d stored, %d entries",
        stats.Hits, stats.Stored, stats.Entries)
}
```

304 revalidations keep the data identical to the uncached path — same items,
same rate-limit header handling — while spending no core rate-limit budget
on the data endpoints. The cache keys are credential-scoped (kit v0.3.0), so
two tokens never share entries.

**Why there is no "page-1 304 probe" shortcut:** a page-1 304 only proves
page 1 unchanged. GitHub event feeds shift between pages — one new event
pushes yesterday's page 1 content to page 2 — so skipping pages 2..N after a
cached page 1 would silently DROP data. The kernel-level cache already
makes every unchanged page a 304 (cheap, no rate cost), which is the correct
granularity: per-page revalidation without page-skip inferences.

Standalone development: the module pins a released parent version and builds
with `go build ./...` from this directory with `GOWORK=off`; inside the
repository, the root `go.work` wires it against the local core module.

## Live smoke test

The mock-based suite never touches the network. One env-gated round trip
proves the released wiring (auth, pagination, rate-limit metadata, error
classification) against the real API:

```bash
GITHUB_PAT=ghp_yourtoken go test -run TestLivePAT ./...
```

Without `GITHUB_PAT` the test skips. A public-read PAT is sufficient; it
fetches one page of `torvalds`' public event feed.
