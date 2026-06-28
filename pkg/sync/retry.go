package sync

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
)

// retryAfterer is optionally implemented by provider errors that carry a
// server-advised wait duration (e.g. a parsed Retry-After header on a 429).
// When present, the retry loop waits that long instead of its computed backoff.
// This is forward-compatible: providers that don't implement it simply use the
// exponential backoff.
type retryAfterer interface {
	RetryAfter() time.Duration
}

// jitterFraction sizes the backoff jitter band: the wait varies by ±1/4 of the
// computed delay, enough to de-synchronize retriers without distorting the mean.
const jitterFraction = 4

// backoff returns the wait duration for a retry attempt using exponential
// backoff (InitialBackoff * 2^(attempt-1), capped at MaxBackoff) with ±25%
// jitter to avoid thundering herds. attempt is 1-indexed.
func backoff(cfg provider.RetryConfig, attempt int) time.Duration {
	if cfg.InitialBackoff <= 0 {
		return 0
	}

	shift := attempt - 1
	shift = min(shift, 30) // guard against overflow on absurd configs

	delay := cfg.InitialBackoff << uint(shift)
	if delay > cfg.MaxBackoff || delay <= 0 {
		delay = cfg.MaxBackoff
	}

	if delay <= 0 {
		return 0
	}

	delta := delay / jitterFraction
	jitter := time.Duration(rand.Int64N(int64(2*delta))) - delta //nolint:gosec,mnd // jitter; magic is the ±band width

	return clampNonNeg(delay+jitter, cfg.MaxBackoff)
}

func clampNonNeg(duration, ceiling time.Duration) time.Duration {
	if duration < 0 {
		return 0
	}

	if ceiling > 0 {
		return min(duration, ceiling)
	}

	return duration
}

// sleepCtx sleeps for d, returning ctx.Err() early if the context is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
