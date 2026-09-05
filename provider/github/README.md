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
- **Retry with backoff** on 429 and idempotent 5xx.
- **Concurrent pagination**: `FetchAll` walks pages 2..N on a bounded pool
  (default 3) and stops at the first short page.
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

| Method                | Effect                                                                     |
| --------------------- | -------------------------------------------------------------------------- |
| `NewClient(token)`    | Explicit PAT; an empty token means unauthenticated (60 req/h core budget). |
| `WithRateLimitConfig` | `Enabled`, `MinRemaining` (default 10), `MaxWait` (default 15m).           |
| `WithFetchConfig`     | `MaxConcurrentFetches` (default 3), `OnProgress` callback.                 |
| `WithRetryConfig`     | Max retries (default 3), initial/max backoff (default 1s → 30s).           |
| `WithBaseURL`         | Point at a different API root (GitHub Enterprise, test servers).           |

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
