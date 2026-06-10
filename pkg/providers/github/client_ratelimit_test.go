package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/google/go-github/v69/github"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestFetch_RetryOnServerError(t *testing.T) {
	server, callCount := newFailingThenSucceedingTestServer(3)
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRetryConfig(testRetryConfig())

	_, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
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

	client := newTestClient(server)
	client = client.WithRetryConfig(testRetryConfig())

	_, err := client.Fetch(context.Background(), &provider.FetchOptions{Source: "testuser"})
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
		_ = json.NewEncoder(w).Encode(rateLimitResponse(4999))
	}))
	defer server.Close()

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

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

	httpClient := &http.Client{}
	client := NewClientWithHTTP(httpClient)
	client.client.BaseURL = mustParseURL(server.URL)

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

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"5xx server error", http.StatusInternalServerError, true},
		{"429 rate limited", http.StatusTooManyRequests, true},
		{"400 bad request", http.StatusBadRequest, false},
		{"401 unauthorized", http.StatusUnauthorized, false},
		{"403 forbidden", http.StatusForbidden, false},
		{"404 not found", http.StatusNotFound, false},
		{"200 OK", http.StatusOK, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &gh.ErrorResponse{Response: &http.Response{StatusCode: tt.statusCode}}
			got := isRetryableError(err)
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestWrapGitHubError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    error
	}{
		{"unauthorized", http.StatusUnauthorized, pkgerrors.ErrInvalidToken},
		{"forbidden", http.StatusForbidden, pkgerrors.ErrRateLimited},
		{"not found", http.StatusNotFound, pkgerrors.ErrUserNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ghErr := &gh.ErrorResponse{
				Response: &http.Response{StatusCode: tt.statusCode},
			}
			wrapped := wrapGitHubError(ghErr, "testuser")
			if !errors.Is(wrapped, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, wrapped)
			}
		})
	}
}

func TestWaitForRateLimit_Disabled(t *testing.T) {
	t.Parallel()

	client := NewClient("test-token")
	client = client.WithRateLimitConfig(provider.RateLimitConfig{Enabled: false})

	err := client.waitForRateLimit(context.Background())
	if err != nil {
		t.Errorf("expected no error when rate limiting disabled, got %v", err)
	}
}

func TestWaitForRateLimit_SufficientRemaining(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rateLimitResponse(4999))
	}))
	defer server.Close()

	client := newTestClient(server)

	testutil.MustNoError(t, client.waitForRateLimit(context.Background()))
}

func TestWaitForRateLimit_ExceedsMaxWait(t *testing.T) {
	t.Parallel()

	resetTime := time.Now().Add(2 * time.Hour)
	server := newRateLimitCoreServer(t, exhaustedRate(resetTime))
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{
		Enabled:      true,
		MinRemaining: 100,
		MaxWait:      1 * time.Minute,
	})

	err := client.waitForRateLimit(context.Background())
	if err == nil {
		t.Fatal("expected error when wait exceeds max wait")
	}
	if !errors.Is(err, pkgerrors.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestWaitForRateLimit_ContextCanceled(t *testing.T) {
	t.Parallel()

	resetTime := time.Now().Add(5 * time.Second)
	server := newRateLimitCoreServer(t, exhaustedRate(resetTime))
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{
		Enabled:      true,
		MinRemaining: 100,
		MaxWait:      10 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.waitForRateLimit(ctx)
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
}

func TestWaitForRateLimit_NilCore(t *testing.T) {
	t.Parallel()

	server := newRateLimitCoreServer(t, nil)
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{Enabled: true})

	err := client.waitForRateLimit(context.Background())
	if err != nil {
		t.Errorf("expected no error when core is nil, got %v", err)
	}
}
