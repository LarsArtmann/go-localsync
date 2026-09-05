package github

import (
	"time"
)

// FetchProgressFunc reports multi-page fetch progress: page number, total pages,
// and fetched is the cumulative item count so far.
type FetchProgressFunc func(page, total, fetched int)

// FetchConfig configures multi-page fetch behavior for FetchAll.
type FetchConfig struct {
	// MaxConcurrentFetches controls how many pages are fetched in parallel.
	// 0 or 1 means sequential fetching. Defaults to 3 when unset.
	MaxConcurrentFetches int
	// OnProgress is an optional callback invoked after each page completes.
	OnProgress FetchProgressFunc
}

// DefaultFetchConfig provides sensible defaults for multi-page fetching.
var DefaultFetchConfig = FetchConfig{ //nolint:gochecknoglobals // intentional default config for callers
	MaxConcurrentFetches: 3,
}

// RateLimitConfig configures pre-flight rate-limit gating for GitHub API calls.
// The gate itself lives in the go-github-kit kernel; these values translate
// onto its RateLimitOptions.
type RateLimitConfig struct {
	// Enabled controls whether rate limit checking is performed.
	Enabled bool
	// MinRemaining is the minimum remaining calls before waiting for reset.
	MinRemaining int
	// MaxWait is the maximum time to wait for the rate limit to reset.
	MaxWait time.Duration
}

// DefaultRateLimitConfig gates when fewer than 10 requests remain, waiting up to 15 minutes.
var DefaultRateLimitConfig = RateLimitConfig{ //nolint:gochecknoglobals // intentional default config for callers
	Enabled:      true,
	MinRemaining: 10,
	MaxWait:      15 * time.Minute,
}
