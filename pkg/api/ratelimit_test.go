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
