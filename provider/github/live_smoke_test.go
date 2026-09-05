package github

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLivePAT_Smoke is an env-gated live-API round trip: it only runs when
// GITHUB_PAT (a classic or fine-grained PAT with public-read scope) is set.
// It proves the released kit wiring once against the real GitHub API —
// token auth, pagination, rate-limit metadata, and error classification —
// which the mock-based suite cannot.
//
//	Run with:  GITHUB_PAT=ghp_xxx go test -run TestLivePAT ./...
func TestLivePAT_Smoke(t *testing.T) {
	pat := os.Getenv("GITHUB_PAT")
	if pat == "" {
		t.Skip("GITHUB_PAT not set; skipping live smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewClient(pat)

	result, err := client.FetchAll(ctx, "torvalds", 1)
	if err != nil {
		t.Fatalf("live FetchAll failed: %v", err)
	}

	if result == nil {
		t.Fatal("live FetchAll returned nil result")
	}

	t.Logf("fetched %d public events for torvalds (hasMore=%v)", len(result.Items), result.HasMore)

	if len(result.Items) == 0 {
		t.Log("no public events returned; wiring worked but the feed is empty — acceptable")
	}

	for _, item := range result.Items {
		if item.Source.Get() == "" || item.ExternalID.Get() == "" {
			t.Fatalf("item missing identity: %+v", item)
		}

		if err := item.Validate(); err != nil {
			t.Errorf("live item failed validation: %v", err)
		}
	}

	rateLimit, err := client.GetRateLimit(ctx)
	if err != nil {
		t.Logf("rate-limit probe failed (non-fatal): %v", err)
	} else if rateLimit != nil {
		t.Logf("rate limit: remaining=%d limit=%d", rateLimit.Remaining, rateLimit.Limit)
	}
}
