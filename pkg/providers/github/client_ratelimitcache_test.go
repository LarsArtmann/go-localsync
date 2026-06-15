package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
)

func TestRateLimitCache_HitAvoidsAPICall(t *testing.T) {
	t.Parallel()

	var rateLimitCalls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "" && containsSubstring(r.URL.Path, "rate_limit") {
			rateLimitCalls.Add(1)
		}

		w.Header().Set("Content-Type", "application/json")

		if containsSubstring(r.URL.Path, "rate_limit") {
			_ = json.NewEncoder(w).Encode(rateLimitResponse(4999))
			return
		}

		_ = json.NewEncoder(w).Encode([]json.RawMessage{})
	}))
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{
		Enabled:      true,
		MinRemaining: 10,
		MaxWait:      1 * 1e9,
	})

	// First call: cache is empty, makes API call
	_ = client.waitForRateLimit(context.Background())
	firstCalls := rateLimitCalls.Load()
	if firstCalls != 1 {
		t.Fatalf("expected 1 rate_limit API call on first check, got %d", firstCalls)
	}

	// Populate cache from a simulated API response
	client.rateCache.Update(&provider.RateLimitInfo{
		Limit:     5000,
		Remaining: 4999,
		ResetAt:   time.Time{},
	})

	// Second call: cache hit, should NOT make API call
	_ = client.waitForRateLimit(context.Background())
	secondCalls := rateLimitCalls.Load()
	if secondCalls != 1 {
		t.Fatalf("expected 1 rate_limit API call after cache hit, got %d", secondCalls)
	}
}

func TestRateLimitCache_FallbackWhenEmpty(t *testing.T) {
	t.Parallel()

	var rateLimitCalls atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if containsSubstring(r.URL.Path, "rate_limit") {
			rateLimitCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rateLimitResponse(4999))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]json.RawMessage{})
	}))
	defer server.Close()

	client := newTestClient(server)
	client = client.WithRateLimitConfig(provider.RateLimitConfig{
		Enabled:      true,
		MinRemaining: 10,
		MaxWait:      1 * 1e9,
	})

	// Cache is empty → should fall back to API call
	err := client.waitForRateLimit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rateLimitCalls.Load() != 1 {
		t.Errorf("expected 1 API call when cache is empty, got %d", rateLimitCalls.Load())
	}

	// Cache should now be populated
	cached, ok := client.rateCache.Get()
	if !ok {
		t.Fatal("expected cache to be populated after API call")
	}

	if cached.Remaining != 4999 {
		t.Errorf("expected cached Remaining=4999, got %d", cached.Remaining)
	}
}
