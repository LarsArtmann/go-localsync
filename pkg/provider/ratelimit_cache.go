package provider

import "sync"

// RateLimitCache stores rate-limit info extracted from API response headers.
// It is safe for concurrent use and designed to be shared across provider
// copies (e.g., via With* methods) so configuration changes don't lose the cache.
//
// The zero value is not usable — use NewRateLimitCache.
type RateLimitCache struct {
	mu   sync.Mutex
	info *RateLimitInfo
}

// NewRateLimitCache creates an empty RateLimitCache.
func NewRateLimitCache() *RateLimitCache {
	return &RateLimitCache{}
}

// Update stores the authoritative rate-limit info from an API response.
// Overwrites any local decrement-based estimate. Nil or zero-limit info is ignored.
func (c *RateLimitCache) Update(info *RateLimitInfo) {
	if c == nil || info == nil || info.Limit == 0 {
		return
	}

	c.mu.Lock()
	c.info = info
	c.mu.Unlock()
}

// Decrement locally reduces the remaining count by n after dispatching
// API calls, giving a conservative estimate between API responses.
// The next API response will overwrite with the authoritative value.
func (c *RateLimitCache) Decrement(n int) {
	if c == nil || n <= 0 {
		return
	}

	c.mu.Lock()
	if c.info != nil {
		c.info.Remaining -= n
		if c.info.Remaining < 0 {
			c.info.Remaining = 0
		}
	}
	c.mu.Unlock()
}

// Get returns the cached rate-limit info and true if a cache entry exists.
// Returns nil, false if the cache is empty or the receiver is nil.
func (c *RateLimitCache) Get() (*RateLimitInfo, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.info, c.info != nil
}
