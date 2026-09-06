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

// TestBackoff_JitterBounds pins the jitter contract: every sampled wait for
// an attempt stays within ±25% of the pure exponential delay and never
// exceeds MaxBackoff — enough spread to de-synchronize retriers, not enough
// to distort the schedule.
func TestBackoff_JitterBounds(t *testing.T) {
	t.Parallel()

	cfg := provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     8,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
	}

	for attempt := 1; attempt <= 8; attempt++ {
		shift := min(attempt-1, 30)
		base := cfg.InitialBackoff << uint(shift)
		if base > cfg.MaxBackoff || base <= 0 {
			base = cfg.MaxBackoff
		}

		lo := base - base/jitterFraction
		hi := min(base+base/jitterFraction, cfg.MaxBackoff)

		for range 200 {
			got := backoff(cfg, attempt)
			if got < lo || got > hi {
				t.Fatalf("attempt %d: backoff %v outside jitter band [%v, %v]", attempt, got, lo, hi)
			}
		}
	}
}

// TestBackoff_EdgeConfigs covers the degenerate configurations: zero/absent
// initial backoff disables waiting, MaxBackoff below InitialBackoff clamps
// immediately, and absurd attempt counts cap at MaxBackoff instead of
// overflowing the shift.
func TestBackoff_EdgeConfigs(t *testing.T) {
	t.Parallel()

	t.Run("disabled initial backoff", func(t *testing.T) {
		t.Parallel()

		cfg := provider.RetryConfig{InitialBackoff: 0, MaxBackoff: time.Second}
		if got := backoff(cfg, 3); got != 0 {
			t.Errorf("zero InitialBackoff must wait 0, got %v", got)
		}
	})

	t.Run("max below initial clamps", func(t *testing.T) {
		t.Parallel()

		cfg := provider.RetryConfig{InitialBackoff: time.Second, MaxBackoff: 10 * time.Millisecond}
		for range 50 {
			if got := backoff(cfg, 1); got > cfg.MaxBackoff {
				t.Fatalf("backoff must never exceed MaxBackoff, got %v", got)
			}
		}
	})

	t.Run("absurd attempt caps at max", func(t *testing.T) {
		t.Parallel()

		cfg := provider.RetryConfig{InitialBackoff: time.Millisecond, MaxBackoff: 50 * time.Millisecond}
		for range 50 {
			if got := backoff(cfg, 1000); got > cfg.MaxBackoff {
				t.Fatalf("attempt=1000 must cap at MaxBackoff=%v, got %v", cfg.MaxBackoff, got)
			}
		}
	})

	t.Run("zero max with positive initial yields zero", func(t *testing.T) {
		t.Parallel()

		cfg := provider.RetryConfig{InitialBackoff: time.Millisecond, MaxBackoff: 0}
		if got := backoff(cfg, 2); got != 0 {
			t.Errorf("MaxBackoff=0 means no waiting budget, got %v", got)
		}
	})
}

// retryAfterError is a retryable provider error carrying a server-advised
// wait (the parsed Retry-After of a 429) through the retryAfterer seam.
type retryAfterError struct {
	advice time.Duration
}

func (e retryAfterError) Error() string { return "server said slow down" }

func (e retryAfterError) RetryAfter() time.Duration { return e.advice }

// retryAfterProvider fails its first FetchAll with the given error, then
// succeeds like a plain mock.
type retryAfterProvider struct {
	testutil.MockProvider

	err   error
	calls int
}

func (p *retryAfterProvider) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	p.calls++
	if p.calls == 1 {
		return nil, p.err
	}

	return p.MockProvider.FetchAll(ctx, source, maxPages)
}

// TestSync_RetryAfterOverrideBeatsBackoff: when the provider error advises a
// short Retry-After, the retry loop must wait THAT, not the computed backoff
// — proven by total elapsed time, not by mocking the clock.
func TestSync_RetryAfterOverrideBeatsBackoff(t *testing.T) {
	t.Parallel()

	p := &retryAfterProvider{
		MockProvider: testutil.MockProvider{Items: testSyncItems("ra-1", "PushEvent")},
		err:          retryAfterError{advice: 2 * time.Millisecond},
	}

	syncer := NewSyncer(p, &mockSyncStore{}, log.Default())
	syncer.retry = provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond, // would dominate without the override
		MaxBackoff:     time.Second,
	}

	start := time.Now()
	_, err := syncer.Sync(context.Background(), testSyncOpts())
	elapsed := time.Since(start)

	testutil.MustNoError(t, err)

	if p.calls != 2 {
		t.Fatalf("expected 2 fetch attempts (1 advised retry), got %d", p.calls)
	}

	if elapsed > 300*time.Millisecond {
		t.Fatalf(
			"Retry-After advice (2ms) must override the backoff (>=375ms even with jitter); took %v",
			elapsed,
		)
	}
}

// TestSync_RetryAfterCappedByMaxBackoff: even a huge server-advised wait is
// clamped to MaxBackoff — the client keeps its own waiting budget.
func TestSync_RetryAfterCappedByMaxBackoff(t *testing.T) {
	t.Parallel()

	p := &retryAfterProvider{
		MockProvider: testutil.MockProvider{Items: testSyncItems("ra-2", "PushEvent")},
		err:          retryAfterError{advice: time.Hour},
	}

	syncer := NewSyncer(p, &mockSyncStore{}, log.Default())
	syncer.retry = provider.RetryConfig{
		Enabled:        true,
		MaxRetries:     2,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
	}

	start := time.Now()
	_, err := syncer.Sync(context.Background(), testSyncOpts())
	elapsed := time.Since(start)

	testutil.MustNoError(t, err)

	if elapsed > 300*time.Millisecond {
		t.Fatalf("Retry-After of 1h must be clamped to MaxBackoff=5ms; took %v", elapsed)
	}
}

// TestSync_RetryAfterZeroFallsBackToBackoff: a non-positive advised duration
// is ignored and the normal backoff schedule applies.
func TestSync_RetryAfterZeroFallsBackToBackoff(t *testing.T) {
	t.Parallel()

	p := &retryAfterProvider{
		MockProvider: testutil.MockProvider{Items: testSyncItems("ra-3", "PushEvent")},
		err:          retryAfterError{advice: 0},
	}

	syncer := NewSyncer(p, &mockSyncStore{}, log.Default())
	syncer.retry = fastRetry()

	res, err := syncer.Sync(context.Background(), testSyncOpts())
	testutil.MustNoError(t, err)

	if res.Fetched != 1 || p.calls != 2 {
		t.Errorf("zero advice must fall back to backoff and retry once: fetched=%d calls=%d", res.Fetched, p.calls)
	}
}
