package github

import (
	"errors"

	githubkit "github.com/LarsArtmann/go-github-kit"
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
// Since kit v0.3.0, ClassifyError recognizes go-github's dedicated
// *RateLimitError/*AbuseRateLimitError types, so one classification pass
// covers raw responses, gate rejections, and native rate-limit errors
// alike. Kit sentinels drive the mapping; the kit's ErrForbidden (a 403
// permission denial) reports as ErrRateLimited because the events
// endpoints only 403 on exhausted budgets in practice and the provider
// vocabulary has no permission sentinel. The original GitHub error stays
// reachable in the returned chain for errors.AsType diagnostics.
func wrapGitHubError(err error, username string) error {
	classified := githubkit.ClassifyError(err)

	var mapped error

	switch {
	case errors.Is(classified, githubkit.ErrRateLimited),
		errors.Is(classified, githubkit.ErrForbidden):
		mapped = pkgerrors.WithDetail(pkgerrors.ErrRateLimited, username)
	case errors.Is(classified, githubkit.ErrAuthRequired):
		mapped = pkgerrors.WithDetail(pkgerrors.ErrInvalidToken, username)
	case errors.Is(classified, githubkit.ErrNotFound):
		mapped = pkgerrors.WithDetail(pkgerrors.ErrUserNotFound, username)
	default:
		// Covers the kit's ErrAPIUnavailable (transport failures,
		// exhausted 5xx retries) and anything unclassified: from the
		// provider's point of view the source is unavailable either way.
		mapped = pkgerrors.WithDetail(pkgerrors.ErrProviderUnavailable, username)
	}

	return errors.Join(mapped, pkgerrors.Wrap(err, "github api call failed"))
}
