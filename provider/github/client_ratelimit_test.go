package github

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	githubkit "github.com/LarsArtmann/go-github-kit"
	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestFetch_RetryOnServerError(t *testing.T) {
	server, callCount := newFailingThenSucceedingTestServer(3)
	defer server.Close()

	client := newRetryTestClient(server)

	_, err := fetchFromTestClient(client, "testuser")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *callCount != 3 {
		t.Errorf("expected 3 calls, got %d", *callCount)
	}
}

func TestFetch_NoRetryOnClientError(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client := newRetryTestClient(server)

	_, err := fetchFromTestClient(client, "testuser")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestGetRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !containsSubstring(r.URL.Path, "/rate_limit") {
			t.Errorf("expected path to contain /rate_limit, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.MarshalWrite(w, rateLimitResponse(4999))
	}))
	defer server.Close()

	client := newServerClient(server, RateLimitConfig{Enabled: false})

	limits, err := client.GetRateLimit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if limits == nil {
		t.Fatal("expected non-nil limits")
	}

	if limits.Limit != 5000 {
		t.Errorf("expected Limit=5000, got %d", limits.Limit)
	}

	if limits.Remaining != 4999 {
		t.Errorf("expected Remaining=4999, got %d", limits.Remaining)
	}
}

func TestGetRateLimit_NilCore(t *testing.T) {
	server := newRateLimitCoreServer(t, nil)
	defer server.Close()

	client := newServerClient(server, RateLimitConfig{Enabled: false})

	limits, err := client.GetRateLimit(context.Background())
	testutil.MustNoError(t, err)

	if limits == nil {
		t.Fatal("expected non-nil limits")
	}

	if limits.Limit != 0 {
		t.Errorf("expected Limit=0, got %d", limits.Limit)
	}

	if limits.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %d", limits.Remaining)
	}

	if !limits.ResetAt.IsZero() {
		t.Errorf("expected zero ResetAt, got %v", limits.ResetAt)
	}
}

func TestWrapGitHubError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{"unauthorized", newGitHubErrorResponse(http.StatusUnauthorized), pkgerrors.ErrInvalidToken},
		{"forbidden", newGitHubErrorResponse(http.StatusForbidden), pkgerrors.ErrRateLimited},
		{"not found", newGitHubErrorResponse(http.StatusNotFound), pkgerrors.ErrUserNotFound},
		{
			"kit classified not found",
			githubkit.ClassifyError(newGitHubErrorResponse(http.StatusNotFound)),
			pkgerrors.ErrUserNotFound,
		},
		{
			"kit gate rejection",
			&githubkit.StatusError{Sentinel: githubkit.ErrRateLimited},
			pkgerrors.ErrRateLimited,
		},
		{
			"native rate limit error (classified by kit since v0.3.0)",
			&gh.RateLimitError{Response: &http.Response{StatusCode: http.StatusForbidden}},
			pkgerrors.ErrRateLimited,
		},
		{
			"native abuse rate limit error (classified by kit since v0.3.0)",
			&gh.AbuseRateLimitError{Response: &http.Response{StatusCode: http.StatusForbidden}},
			pkgerrors.ErrRateLimited,
		},
		{"server error", newGitHubErrorResponse(http.StatusInternalServerError), pkgerrors.ErrProviderUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapGitHubError(tt.err, "testuser")
			if !errors.Is(wrapped, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, wrapped)
			}

			// The original GitHub error must stay reachable for diagnostics.
			if !errors.Is(wrapped, tt.err) {
				t.Errorf("wrapped error lost the original cause %T: %v", tt.err, wrapped)
			}
		})
	}
}

// TestFetch_RateLimitGatePassesWithSufficientBudget verifies the kernel gate
// lets requests through when the budget (fed from X-RateLimit headers) is
// healthy, and that a second request reuses the cached budget instead of
// probing /rate_limit again.
func TestFetch_RateLimitGatePassesWithSufficientBudget(t *testing.T) {
	var probeCalls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if containsSubstring(r.URL.Path, "rate_limit") {
			probeCalls.Add(1)
			setRateLimitHeaders(w, 4999, time.Now().Add(1*time.Hour))
			w.WriteHeader(http.StatusOK)

			return
		}

		setRateLimitHeaders(w, 4998, time.Now().Add(1*time.Hour))
		writeEmptyEventsResponse(w)
	}))
	defer server.Close()

	client := newServerClient(server, RateLimitConfig{
		Enabled:      true,
		MinRemaining: 10,
		MaxWait:      1 * time.Minute,
	})

	for range 2 {
		_, err := fetchFromTestClient(client, "testuser")
		testutil.MustNoError(t, err)
	}

	if probeCalls.Load() != 1 {
		t.Errorf("expected exactly 1 /rate_limit probe (cache hit afterwards), got %d", probeCalls.Load())
	}
}

// TestFetch_RateLimitGateRejectsWhenResetTooFar verifies the kernel gate
// fails fast with ErrRateLimited when the budget is exhausted and the reset
// is further away than the configured MaxWait.
func TestFetch_RateLimitGateRejectsWhenResetTooFar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setRateLimitHeaders(w, 0, time.Now().Add(2*time.Hour))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newServerClient(server, RateLimitConfig{
		Enabled:      true,
		MinRemaining: 10,
		MaxWait:      1 * time.Minute,
	})

	_, err := fetchFromTestClient(client, "testuser")
	if err == nil {
		t.Fatal("expected error when wait exceeds max wait")
	}

	if !errors.Is(err, pkgerrors.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

// TestFetch_RateLimitGateHonorsCanceledContext verifies a canceled context
// aborts the request rather than blocking in the gate.
func TestFetch_RateLimitGateHonorsCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setRateLimitHeaders(w, 0, time.Now().Add(5*time.Second))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newServerClient(server, RateLimitConfig{
		Enabled:      true,
		MinRemaining: 10,
		MaxWait:      10 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Fetch(ctx, &provider.FetchOptions{Source: id.NewProviderID("testuser")})
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
}

// TestFetch_RateLimitProbeFailureIsAdvisory verifies that a probe endpoint
// reporting no usable budget information (e.g. a host that strips
// X-RateLimit headers) never blocks traffic: the gate proceeds and the first
// real response corrects the picture.
func TestFetch_RateLimitProbeFailureIsAdvisory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if containsSubstring(r.URL.Path, "rate_limit") {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		writeEmptyEventsResponse(w)
	}))
	defer server.Close()

	client := newServerClient(server, RateLimitConfig{
		Enabled:      true,
		MinRemaining: 10,
		MaxWait:      1 * time.Minute,
	}).WithRetryConfig(testRetryConfig())

	_, err := fetchFromTestClient(client, "testuser")
	testutil.MustNoError(t, err)
}

// newServerClient builds a client aimed at the test server with an
// explicit rate-limit gate configuration.
func newServerClient(server *httptest.Server, cfg RateLimitConfig) *Client {
	client, err := NewClientWithHTTP(&http.Client{}).WithBaseURL(server.URL + "/")
	if err != nil {
		panic(fmt.Sprintf("WithBaseURL failed: %v", err))
	}

	return client.WithRateLimitConfig(cfg)
}

// setRateLimitHeaders writes the X-RateLimit header family the kernel's
// budget tracking feeds on.
func setRateLimitHeaders(w http.ResponseWriter, remaining int, resetAt time.Time) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(defaultGitHubRateLimit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
}
