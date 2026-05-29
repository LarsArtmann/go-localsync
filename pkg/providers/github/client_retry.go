package github

import (
	"context"
	"errors"
	"net/http"
	"time"

	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

func (c *Client) waitForRateLimit(ctx context.Context) error {
	if !c.rateLimitConfig.Enabled {
		return nil
	}

	limits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to check rate limit")
	}

	core := limits.GetCore()
	if core == nil {
		return nil
	}

	if core.Remaining > c.rateLimitConfig.MinRemaining {
		return nil
	}

	resetTime := core.Reset.Time
	waitDuration := time.Until(resetTime)

	if waitDuration <= 0 {
		return nil
	}

	if waitDuration > c.rateLimitConfig.MaxWait {
		return pkgerrors.Wrapf(pkgerrors.ErrRateLimited, "reset in %v (exceeds max wait %v)",
			waitDuration, c.rateLimitConfig.MaxWait)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitDuration):
		return nil
	}
}

func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	if !c.retryConfig.Enabled {
		return fn()
	}

	var lastErr error

	backoff := c.retryConfig.InitialBackoff

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		err := ctx.Err()
		if err != nil {
			return pkgerrors.Wrapf(err, "retry loop interrupted (attempt %d)", attempt)
		}

		err = fn()
		if err == nil {
			return nil
		}

		lastErr = err
		if !isRetryableError(err) {
			return pkgerrors.Wrapf(err, "non-retryable error during retry (attempt %d)", attempt)
		}

		if attempt < c.retryConfig.MaxRetries {
			if backoff > c.retryConfig.MaxBackoff {
				backoff = c.retryConfig.MaxBackoff
			}

			select {
			case <-ctx.Done():
				return pkgerrors.Wrapf(
					ctx.Err(),
					"retry loop cancelled after %d attempts (last error: %v)",
					attempt,
					lastErr,
				)
			case <-time.After(backoff):
				backoff *= 2
			}
		}
	}

	return pkgerrors.Wrapf(lastErr, "retry exhausted after %d attempts", c.retryConfig.MaxRetries+1)
}

func isRetryableError(err error) bool {
	ghErr := &gh.ErrorResponse{} //nolint:exhaustruct // Intentionally empty for errors.As type assertion
	if errors.As(err, &ghErr) {
		statusCode := ghErr.Response.StatusCode

		return statusCode >= 500 || statusCode == 429
	}

	return pkgerrors.IsRetryable(err)
}

func wrapGitHubError(err error, username string) error {
	ghErr := &gh.ErrorResponse{} //nolint:exhaustruct // Intentionally empty for errors.As type assertion
	if errors.As(err, &ghErr) {
		switch ghErr.Response.StatusCode {
		case http.StatusUnauthorized:
			return pkgerrors.WithUserDetail(pkgerrors.ErrInvalidToken, username)
		case http.StatusForbidden:
			return pkgerrors.WithUserDetail(pkgerrors.ErrRateLimited, username)
		case http.StatusNotFound:
			return pkgerrors.WithUserDetail(pkgerrors.ErrUserNotFound, username)
		}
	}

	return pkgerrors.WithUserDetail(pkgerrors.ErrSyncFailed, username)
}
