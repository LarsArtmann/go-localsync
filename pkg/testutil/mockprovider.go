package testutil

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/provider"
)

// MockProvider is a configurable provider.Provider for tests. The zero value
// returns empty results with no error.
type MockProvider struct {
	Items []*provider.Item
	// HasMore simulates a partial (still-paginating) fetch when true. Default
	// false mirrors a complete fetch.
	HasMore   bool
	Err       error
	RateLimit *provider.RateLimitInfo
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) fetchResult() (*provider.FetchResult, error) {
	return &provider.FetchResult{Items: m.Items, HasMore: m.HasMore}, m.Err
}

func (m *MockProvider) Fetch(_ context.Context, _ *provider.FetchOptions) (*provider.FetchResult, error) {
	return m.fetchResult()
}

func (m *MockProvider) FetchAll(_ context.Context, _ string, _ int) (*provider.FetchResult, error) {
	return m.fetchResult()
}

func (m *MockProvider) GetRateLimit(_ context.Context) (*provider.RateLimitInfo, error) {
	return m.RateLimit, nil
}
