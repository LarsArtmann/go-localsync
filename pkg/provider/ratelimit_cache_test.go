package provider

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimitCache_UpdateAndGet(t *testing.T) {
	t.Parallel()

	cache := NewRateLimitCache()

	info, ok := cache.Get()
	if ok {
		t.Fatalf("expected empty cache, got %v", info)
	}

	expected := &RateLimitInfo{
		Limit:     5000,
		Remaining: 4999,
		ResetAt:   time.Now().Add(1 * time.Hour),
	}

	cache.Update(expected)

	got, ok := cache.Get()
	if !ok {
		t.Fatal("expected cache hit after Update")
	}

	if got.Limit != expected.Limit || got.Remaining != expected.Remaining {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestRateLimitCache_UpdateNil(t *testing.T) {
	t.Parallel()

	cache := NewRateLimitCache()
	cache.Update(nil)

	_, ok := cache.Get()
	if ok {
		t.Fatal("expected cache miss after nil Update")
	}
}

func TestRateLimitCache_UpdateZeroLimit(t *testing.T) {
	t.Parallel()

	cache := NewRateLimitCache()
	cache.Update(&RateLimitInfo{Limit: 0, Remaining: 0})

	_, ok := cache.Get()
	if ok {
		t.Fatal("expected cache miss after zero-limit Update")
	}
}

func TestRateLimitCache_Decrement(t *testing.T) {
	t.Parallel()

	cache := NewRateLimitCache()
	cache.Update(&RateLimitInfo{Limit: 5000, Remaining: 100, ResetAt: time.Now().Add(1 * time.Hour)})

	cache.Decrement(1)

	got, _ := cache.Get()
	if got.Remaining != 99 {
		t.Errorf("expected Remaining=99, got %d", got.Remaining)
	}

	cache.Decrement(5)
	got, _ = cache.Get()
	if got.Remaining != 94 {
		t.Errorf("expected Remaining=94, got %d", got.Remaining)
	}
}

func TestRateLimitCache_DecrementToZero(t *testing.T) {
	t.Parallel()

	cache := NewRateLimitCache()
	cache.Update(&RateLimitInfo{Limit: 5000, Remaining: 3, ResetAt: time.Now().Add(1 * time.Hour)})

	cache.Decrement(10)

	got, _ := cache.Get()
	if got.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %d", got.Remaining)
	}
}

func TestRateLimitCache_DecrementEmptyCache(t *testing.T) {
	t.Parallel()

	cache := NewRateLimitCache()
	cache.Decrement(1)

	_, ok := cache.Get()
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestRateLimitCache_NilSafe(t *testing.T) {
	var cache *RateLimitCache

	info, ok := cache.Get()
	if ok || info != nil {
		t.Fatal("nil cache should return nil, false")
	}

	cache.Update(&RateLimitInfo{Limit: 5000, Remaining: 100, ResetAt: time.Now()})
	cache.Decrement(1)
}

func TestRateLimitCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cache := NewRateLimitCache()
	cache.Update(&RateLimitInfo{Limit: 10000, Remaining: 10000, ResetAt: time.Now().Add(1 * time.Hour)})

	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			cache.Decrement(1)
		}()
	}

	wg.Wait()

	got, _ := cache.Get()
	if got.Remaining != 9900 {
		t.Errorf("expected Remaining=9900 after 100 decrements, got %d", got.Remaining)
	}
}
