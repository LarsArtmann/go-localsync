package api

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WithRateLimit guards POST /sync with a token bucket refilled at
// requestsPerMinute. Bursts up to the bucket capacity (equal to one minute of
// budget) are allowed; beyond that clients receive 429 with a Retry-After
// header. Zero or a negative rate disables the guard. Reads stay unlimited —
// they are cheap and cacheable; sync runs hit the upstream provider and are
// the scarce resource.
//
// Scope: ONE global bucket shared by every client. For per-client budgets use
// WithRateLimiter; when both are configured the per-client limiter wins and
// the global bucket is ignored for /sync.
func WithRateLimit(requestsPerMinute int) ServerOption {
	return func(o *serverOptions) {
		o.ratePerMinute = requestsPerMinute
	}
}

// WithRateLimiter guards POST /sync with one token bucket PER CLIENT, keyed by
// keyExtractor(r) (e.g. the API key or a username). Each client gets the full
// requestsPerMinute budget: one noisy client cannot starve the others. An
// empty extracted key falls back to a single shared bucket.
//
// Scope: per-client. When configured, it takes precedence over WithRateLimit's
// global bucket. The key→bucket map lives for the server's lifetime and grows
// with the number of distinct keys — keep the extractor to bounded identities
// (API keys, usernames), not unbounded remote addresses behind proxies.
//
// Recipe — key from the API key (pairs with WithAPIKey, which accepts the same
// two headers; X-Api-Key first, Authorization: Bearer as fallback):
//
//	server := api.NewServer(syncer, logger,
//		api.WithAPIKey(os.Getenv("LOCALSYNC_API_KEY")),
//		api.WithRateLimiter(30, api.APIKeyClient),
//	)
//
// APIKeyClient is the canonical extractor; use it directly instead of
// re-deriving the header dance in consumer code.
func WithRateLimiter(requestsPerMinute int, keyExtractor func(*http.Request) string) ServerOption {
	return func(o *serverOptions) {
		o.ratePerMinute = requestsPerMinute
		o.perClient = requestsPerMinute > 0
		o.keyExtractor = keyExtractor
	}
}

// APIKeyClient is the canonical per-client key extractor for WithRateLimiter:
// it keys the bucket by the presented API credential — X-Api-Key first, then
// Authorization: Bearer — mirroring WithAPIKey's acceptance rules. Requests
// without a credential share the "" fallback bucket.
func APIKeyClient(r *http.Request) string {
	if key := r.Header.Get(apiKeyHeader); key != "" {
		return key
	}

	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
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

// secondsPerMinute converts a per-minute budget to a per-second refill rate.
const secondsPerMinute = 60.0

func newTokenBucket(requestsPerMinute int) *tokenBucket {
	perSecond := float64(requestsPerMinute) / secondsPerMinute

	return &tokenBucket{
		rate:     perSecond,
		capacity: float64(requestsPerMinute),
		tokens:   float64(requestsPerMinute),
		last:     time.Now(),
	}
}

// take removes one token, reporting whether it was available, how long to
// wait for the next one when it was not, and how many tokens remain after the
// removal (surfaced as X-RateLimit-Remaining).
func (b *tokenBucket) take() (bool, time.Duration, int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.tokens = math.Min(b.capacity, b.tokens+now.Sub(b.last).Seconds()*b.rate)
	b.last = now

	if b.tokens >= 1 {
		b.tokens--

		return true, 0, int(b.tokens)
	}

	missing := 1 - b.tokens

	return false, time.Duration(missing / b.rate * float64(time.Second)), 0
}

func (s *Server) rateLimited(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.opts == nil || s.opts.ratePerMinute <= 0 || s.opts.bucket == nil ||
			r.Method != http.MethodPost || r.URL.Path != "/sync" {
			next.ServeHTTP(w, r)

			return
		}

		bucket := s.opts.bucket
		if s.opts.perClient {
			bucket = s.clientBucket(s.opts.keyExtractor(r))
		}

		ok, retryAfter, remaining := bucket.take()
		if ok {
			w.Header().Set("X-Ratelimit-Limit", strconv.Itoa(s.opts.ratePerMinute))
			w.Header().Set("X-Ratelimit-Remaining", strconv.Itoa(remaining))

			next.ServeHTTP(w, r)

			return
		}

		seconds := max(int(math.Ceil(retryAfter.Seconds())), 1)

		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		w.Header().Set("X-Ratelimit-Limit", strconv.Itoa(s.opts.ratePerMinute))
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)

		_, _ = w.Write(
			[]byte(
				`{"error":"rate_limited","message":"sync rate limit exceeded, retry after the advised delay"}` + "\n",
			),
		)
	})
}

// clientBucket returns the per-client token bucket for key, creating it on
// first use. The map's size is bounded by the number of distinct client keys
// (see WithRateLimiter's scope note).
func (s *Server) clientBucket(key string) *tokenBucket {
	if key == "" {
		key = "shared"
	}

	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.clientBuckets == nil {
		s.clientBuckets = map[string]*tokenBucket{}
	}

	bucket, ok := s.clientBuckets[key]
	if !ok {
		bucket = newTokenBucket(s.opts.ratePerMinute)
		s.clientBuckets[key] = bucket
	}

	return bucket
}
