package testutil

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/provider"
)

// BlockingProvider blocks every Fetch/FetchAll until the request context is
// done, then surfaces the context error — the shared double for timeout,
// cancellation, and span-lifecycle tests that need a provider which never
// returns on its own. FetchOptions is ignored: the block IS the behavior.
type BlockingProvider struct{}

// Compile-time contract check.
var _ provider.Provider = BlockingProvider{}

func (p BlockingProvider) Name() string { return "blocking" }

func (p BlockingProvider) Fetch(ctx context.Context, _ *provider.FetchOptions) (*provider.FetchResult, error) {
	<-ctx.Done()

	return nil, ctx.Err()
}

func (p BlockingProvider) FetchAll(
	ctx context.Context, _ string, _ int,
) (*provider.FetchResult, error) {
	<-ctx.Done()

	return nil, ctx.Err()
}

func (p BlockingProvider) GetRateLimit(_ context.Context) (*provider.RateLimitInfo, error) {
	return nil, nil //nolint:nilnil // test double: the sync path never reads the rate limit
}
