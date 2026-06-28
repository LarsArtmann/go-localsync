package sync

import (
	"context"
	stdsync "sync"
	"testing"
	"time"

	"charm.land/log/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// flakyProvider fails its first failN FetchAll calls with a retryable
// (Transient) error, then succeeds.
type flakyProvider struct {
	testutil.MockProvider

	failN int
	calls int
	mu    stdsync.Mutex
}

func (f *flakyProvider) FetchAll(ctx context.Context, source string, maxPages int) (*provider.FetchResult, error) {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()

	if call <= f.failN {
		return nil, pkgerrors.ErrRateLimited // Transient -> retryable
	}

	return f.MockProvider.FetchAll(ctx, source, maxPages)
}

// permanentErrProvider always fails with a permanent (Rejection) error.
type permanentErrProvider struct {
	testutil.MockProvider

	calls int
	mu    stdsync.Mutex
}

func (p *permanentErrProvider) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()

	return nil, pkgerrors.ErrInvalidInput // Rejection -> not retryable
}

func fastRetry() provider.RetryConfig {
	return provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
	}
}

// TestRegression_Sync_RetriesTransientFetchError guards P4.1/P4.4: a transient
// fetch error is retried up to the budget and the sync ultimately succeeds.
func TestRegression_Sync_RetriesTransientFetchError(t *testing.T) {
	t.Parallel()

	items := testSyncItems("1", "PushEvent")
	p := &flakyProvider{MockProvider: testutil.MockProvider{Items: items}, failN: 2}
	store := &mockSyncStore{}

	syncer := NewSyncer(p, store, log.Default())
	syncer.retry = fastRetry()

	res, err := syncer.Sync(context.Background(), testSyncOpts())
	testutil.MustNoError(t, err)

	if res.Fetched != 1 {
		t.Errorf("expected 1 fetched after retries, got %d", res.Fetched)
	}

	if p.calls != 3 { // 2 failed + 1 success
		t.Errorf("expected 3 fetch attempts, got %d", p.calls)
	}
}

// TestRegression_Sync_DoesNotRetryPermanentError guards P4.4: a permanent
// (non-retryable) error surfaces immediately without burning the retry budget.
func TestRegression_Sync_DoesNotRetryPermanentError(t *testing.T) {
	t.Parallel()

	p := &permanentErrProvider{}
	store := &mockSyncStore{}

	syncer := NewSyncer(p, store, log.Default())
	syncer.retry = fastRetry()

	_, err := syncer.Sync(context.Background(), testSyncOpts())
	if err == nil {
		t.Fatal("expected error for permanent fetch failure")
	}

	if p.calls != 1 {
		t.Errorf("permanent error must not be retried, got %d calls", p.calls)
	}
}

// TestNew_WithRetry verifies the WithRetry option overrides the default retry
// config, so consumers can tune backoff per deployment via the public New.
func TestNew_WithRetry(t *testing.T) {
	t.Parallel()

	cfg := provider.RetryConfig{
		Enabled:        false,
		MaxRetries:     7,
		InitialBackoff: 250 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
	}

	syncer := New(&testutil.MockProvider{}, &mockSyncStore{}, WithRetry(cfg))

	if syncer.retry != cfg {
		t.Errorf("WithRetry did not apply config: got %+v, want %+v", syncer.retry, cfg)
	}
}
