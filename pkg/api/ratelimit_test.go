package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charm.land/log/v2"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func newRateLimitedServer(t *testing.T, perMinute int) *Server {
	t.Helper()

	syncer := synclib.NewSyncer(&testutil.MockProvider{}, &mockSyncStore{}, log.Default())

	return NewServer(syncer, log.Default(), WithRateLimit(perMinute))
}

func TestRateLimit_BurstThenThrottle(t *testing.T) {
	t.Parallel()

	// 60/min => burst capacity 60 is too many requests to make in a test;
	// use a small bucket instead.
	srv := newRateLimitedServer(t, 1)

	payload := `{"source":"github","max_pages":1}`

	var lastCode int

	for range 5 {
		req := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "/sync", strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		lastCode = rec.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Errorf("expected eventual 429, got %d", lastCode)
	}
}

func TestRateLimit_429CarriesRetryAfter(t *testing.T) {
	t.Parallel()

	srv := newRateLimitedServer(t, 1)

	payload := `{"source":"github","max_pages":1}`

	for range 3 {
		req := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "/sync", strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			if ra := rec.Header().Get("Retry-After"); ra == "" {
				t.Fatal("429 must carry a Retry-After header")
			}

			if !strings.Contains(rec.Body.String(), "rate_limited") {
				t.Errorf("429 body should name rate_limited, got %q", rec.Body.String())
			}

			return
		}
	}

	t.Fatal("bucket never throttled within 3 requests with rate 1/min")
}

func TestRateLimit_ReadsUnlimited(t *testing.T) {
	t.Parallel()

	srv := newRateLimitedServer(t, 1)

	for range 10 {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/stats", nil)
		rec := httptest.NewRecorder()

		srv.ServeHTTP(rec, req)

		testutil.AssertStatus(t, rec, http.StatusOK)
	}
}

func TestRateLimit_OffByDefault(t *testing.T) {
	t.Parallel()

	srv := newTestServer(&mockSyncStore{})

	payload := `{"source":"github","max_pages":1}`

	for range 20 {
		req := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "/sync", strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			t.Fatal("without WithRateLimit nothing must throttle")
		}
	}
}

func TestRateLimit_CarriesRateLimitHeaders(t *testing.T) {
	t.Parallel()

	srv := newRateLimitedServer(t, 1)

	payload := `{"source":"github","max_pages":1}`

	var sawAllowed, sawThrottled bool

	for range 3 {
		req := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "/sync", strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if limit := rec.Header().Get("X-Ratelimit-Limit"); limit != "1" {
			t.Errorf("X-RateLimit-Limit = %q, want 1", limit)
		}

		// The allowed path carries headers no matter what the handler status
		// is (the middleware sets them before next.ServeHTTP); with the mock
		// provider the handler itself answers 422, which still consumed a
		// token.
		if rec.Code != http.StatusTooManyRequests {
			sawAllowed = true

			if remaining := rec.Header().Get("X-Ratelimit-Remaining"); remaining == "" {
				t.Error("allowed request must carry X-RateLimit-Remaining")
			}
		} else {
			sawThrottled = true

			if remaining := rec.Header().Get("X-Ratelimit-Remaining"); remaining != "0" {
				t.Errorf("429 X-RateLimit-Remaining = %q, want 0", remaining)
			}
		}
	}

	if !sawAllowed || !sawThrottled {
		t.Fatalf("expected both an allowed and a throttled request (allowed=%v throttled=%v)", sawAllowed, sawThrottled)
	}
}

func TestRateLimiter_PerClientIsolation(t *testing.T) {
	t.Parallel()

	syncer := synclib.NewSyncer(&testutil.MockProvider{}, &mockSyncStore{}, log.Default())
	srv := NewServer(syncer, log.Default(), WithRateLimiter(1, func(r *http.Request) string {
		return r.Header.Get("X-Test-Client")
	}))

	payload := `{"source":"github","max_pages":1}`
	post := func(client string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "/sync", strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Test-Client", client)

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		return rec
	}

	// Client A exhausts its own 1-per-minute budget (the mock provider's
	// handler status is irrelevant here: only 429 means the limiter fired).
	if rec := post("alice"); rec.Code == http.StatusTooManyRequests {
		t.Fatalf("alice's first request must pass the limiter, got %d", rec.Code)
	}

	if rec := post("alice"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("alice's second request must be throttled, got %d", rec.Code)
	}

	// ...while client B still has a full independent bucket.
	if rec := post("bob"); rec.Code == http.StatusTooManyRequests {
		t.Fatalf("bob must have an independent budget, got %d", rec.Code)
	}

	// An empty extracted key falls back to the shared bucket, not a panic.
	if rec := post(""); rec.Code == 0 {
		t.Fatal("empty key must still yield a response")
	}
}

// TestRateLimit_APIKeyClientRecipe pins the documented per-client recipe
// (WithAPIKey + WithRateLimiter(_, APIKeyClient)): buckets are keyed by the
// presented credential, so one client exhausting its budget leaves another
// client's bucket untouched — and both header spellings (X-Api-Key and
// Authorization: Bearer) resolve to the same bucket.
func TestRateLimit_APIKeyClientRecipe(t *testing.T) {
	t.Parallel()

	syncer := synclib.NewSyncer(&testutil.MockProvider{}, &mockSyncStore{}, log.Default())
	server := NewServer(syncer, log.Default(),
		WithRateLimiter(1, APIKeyClient), // 1/min: first POST spends the burst
	)

	payload := `{"source":"github","maxPages":0}`

	post := func(keyHeader, keyValue string) int {
		t.Helper()

		req := httptest.NewRequestWithContext(
			context.Background(), http.MethodPost, "/sync", strings.NewReader(payload),
		)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(keyHeader, keyValue)

		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		return rec.Code
	}

	// Client one (via X-Api-Key) burns its whole budget, then gets 429.
	if code := post("X-Api-Key", "secret-1"); code != http.StatusOK && code != http.StatusPartialContent {
		t.Fatalf("first sync for client one must pass, got %d", code)
	}
	if code := post("X-Api-Key", "secret-1"); code != http.StatusTooManyRequests {
		t.Fatalf("client one exhausted its bucket: want 429, got %d", code)
	}

	// Client two (via Bearer, same acceptance path) still has its full budget.
	if code := post("Authorization", "Bearer secret-2"); code != http.StatusOK && code != http.StatusPartialContent {
		t.Fatalf("client two must be rate-limited independently, got %d", code)
	}
}

// TestAPIKeyClient_ExtractorContract covers the extractor itself: header
// precedence and the empty fallback.
func TestAPIKeyClient_ExtractorContract(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	if got := APIKeyClient(req); got != "" {
		t.Errorf("no headers must extract empty, got %q", got)
	}

	req.Header.Set("Authorization", "Bearer via-auth")
	if got := APIKeyClient(req); got != "via-auth" {
		t.Errorf("bearer fallback must extract, got %q", got)
	}

	req.Header.Set("X-Api-Key", "via-header")
	if got := APIKeyClient(req); got != "via-header" {
		t.Errorf("X-Api-Key must win over bearer, got %q", got)
	}
}
