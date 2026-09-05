package github

import (
	"errors"
	"net/http"

	githubkit "github.com/LarsArtmann/go-github-kit"
	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// rateLimitOption translates the provider's RateLimitConfig onto the kit's
// kernel options. A disabled config must opt out explicitly: the kit's
// WithRateLimitOptions only overrides the numeric fields and always keeps
// the gate enabled.
func rateLimitOption(cfg RateLimitConfig) githubkit.Option {
	if !cfg.Enabled {
		return githubkit.WithoutRateLimit()
	}

	return githubkit.WithRateLimitOptions(githubkit.RateLimitOptions{
		MinRemaining: cfg.MinRemaining,
		MaxWait:      cfg.MaxWait,
	})
}

// retryOption translates the provider's RetryConfig onto the kit's kernel
// options, mirroring rateLimitOption's explicit opt-out for a disabled config.
func retryOption(cfg provider.RetryConfig) githubkit.Option {
	if !cfg.Enabled {
		return githubkit.WithoutRetry()
	}

	return githubkit.WithRetryOptions(githubkit.RetryOptions{
		MaxRetries:     cfg.MaxRetries,
		InitialBackoff: cfg.InitialBackoff,
		MaxBackoff:     cfg.MaxBackoff,
	})
}

// wrapGitHubError maps any GitHub API failure onto the go-localsync error
// family so callers can rely on errors.Is(pkgerrors.Err*) checks.
//
// go-github's dedicated rate-limit error types are handled first: the kit
// v0.2.0 classifier does not recognize them, and the kernel's gate only
// rejects requests whose budget it already knows is empty, so the first
// teaching 403 with X-RateLimit-Remaining: 0 surfaces raw. Kit sentinels
// are checked next (they cover gate rejections that never produced an HTTP
// response), then raw status codes for unclassified errors. Everything
// else is a provider outage.
func wrapGitHubError(err error, username string) error {
	if _, ok := errors.AsType[*gh.RateLimitError](err); ok {
		return pkgerrors.WithDetail(pkgerrors.ErrRateLimited, username)
	}

	if _, ok := errors.AsType[*gh.AbuseRateLimitError](err); ok {
		return pkgerrors.WithDetail(pkgerrors.ErrRateLimited, username)
	}

	switch {
	case errors.Is(err, githubkit.ErrRateLimited):
		return pkgerrors.WithDetail(pkgerrors.ErrRateLimited, username)
	case errors.Is(err, githubkit.ErrAuthRequired):
		return pkgerrors.WithDetail(pkgerrors.ErrInvalidToken, username)
	case errors.Is(err, githubkit.ErrNotFound):
		return pkgerrors.WithDetail(pkgerrors.ErrUserNotFound, username)
	case errors.Is(err, githubkit.ErrAPIUnavailable):
		return pkgerrors.WithDetail(pkgerrors.ErrProviderUnavailable, username)
	}

	if ghErr, ok := errors.AsType[*gh.ErrorResponse](err); ok && ghErr.Response != nil {
		switch ghErr.Response.StatusCode {
		case http.StatusUnauthorized:
			return pkgerrors.WithDetail(pkgerrors.ErrInvalidToken, username)
		case http.StatusForbidden:
			return pkgerrors.WithDetail(pkgerrors.ErrRateLimited, username)
		case http.StatusNotFound:
			return pkgerrors.WithDetail(pkgerrors.ErrUserNotFound, username)
		}
	}

	return pkgerrors.WithDetail(pkgerrors.ErrProviderUnavailable, username)
}
