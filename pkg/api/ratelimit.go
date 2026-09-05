package api

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// WithRateLimit guards POST /sync with a token bucket refilled at
// requestsPerMinute. Bursts up to the bucket capacity (equal to one minute of
// budget) are allowed; beyond that clients receive 429 with a Retry-After
// header. Zero or a negative rate disables the guard. Reads stay unlimited —
// they are cheap and cacheable; sync runs hit the upstream provider and are
// the scarce resource.
func WithRateLimit(requestsPerMinute int) ServerOption {
	return func(o *serverOptions) {
		o.ratePerMinute = requestsPerMinute
	}
}

// tokenBucket is a minimal, mutex-guarded token bucket. Capacity equals one
// refill period of budget so a quiet period banks at most one minute of
// bursts — deliberately modest for a single-node SDK API.
type tokenBucket struct {
	mu       sync.Mutex
	rate     float64 // tokens per second
	capacity float64
	tokens   float64
	last     time.Time
}

func newTokenBucket(requestsPerMinute int) *tokenBucket {
	perSecond := float64(requestsPerMinute) / 60.0

	return &tokenBucket{
		rate:     perSecond,
		capacity: float64(requestsPerMinute),
		tokens:   float64(requestsPerMinute),
		last:     time.Now(),
	}
}

// take removes one token, reporting whether it was available and how long to
// wait for the next one when it was not.
func (b *tokenBucket) take() (ok bool, retryAfter time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.tokens = math.Min(b.capacity, b.tokens+now.Sub(b.last).Seconds()*b.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--

		return true, 0
	}

	missing := 1 - b.tokens

	return false, time.Duration(missing / b.rate * float64(time.Second))
}

func (s *Server) rateLimited(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.opts == nil || s.opts.ratePerMinute <= 0 || s.opts.bucket == nil ||
			r.Method != http.MethodPost || r.URL.Path != "/sync" {
			h.ServeHTTP(w, r)

			return
		}

		ok, retryAfter := s.opts.bucket.take()
		if ok {
			h.ServeHTTP(w, r)

			return
		}

		seconds := max(int(math.Ceil(retryAfter.Seconds())), 1)

		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)

		_, _ = w.Write(
			[]byte(
				`{"error":"rate_limited","message":"sync rate limit exceeded, retry after the advised delay"}` + "\n",
			),
		)
	})
}
