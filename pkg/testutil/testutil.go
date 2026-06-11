// Package testutil provides shared test helpers for use across test files.
//
// All functions take *testing.T to integrate with the standard testing package.
package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// MustNoError fails the test immediately if err is non-nil.
func MustNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertEqual fails the test if got != want.
func AssertEqual[T comparable](t *testing.T, got, want T, label string) {
	t.Helper()

	if got != want {
		t.Errorf("expected %s=%v, got %v", label, want, got)
	}
}

// AssertInt is shorthand for AssertEqual with int values.
func AssertInt(t *testing.T, got, want int, label string) {
	t.Helper()

	AssertEqual(t, got, want, label)
}

// AssertInt64 is shorthand for AssertEqual with int64 values.
func AssertInt64(t *testing.T, got, want int64, label string) {
	t.Helper()

	AssertEqual(t, got, want, label)
}

// AssertContains fails the test if needle is not found in haystack.
func AssertContains[T comparable](t *testing.T, haystack []T, needle T, label string) {
	t.Helper()

	if !slices.Contains(haystack, needle) {
		t.Errorf("expected %s to contain %v, got %v", label, needle, haystack)
	}
}

// AssertExternalID fails the test if the item's ExternalID does not match want.
func AssertExternalID(t *testing.T, item *model.Item, want string) {
	t.Helper()

	AssertEqual(t, item.ExternalID.Get(), want, "ExternalID")
}

// AssertType fails the test if the item's Type does not match want.
func AssertType(t *testing.T, item *model.Item, want string) {
	t.Helper()

	AssertEqual(t, item.Type.Get(), want, "Type")
}

// AssertStatus fails the test if the recorder's HTTP status is not the expected code.
func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, wantCode int) {
	t.Helper()

	if rec.Code != wantCode {
		t.Fatalf("expected status %d, got %d: %s", wantCode, rec.Code, rec.Body.String())
	}
}

// AssertStatusOK fails the test if the recorder's HTTP status is not 200.
func AssertStatusOK(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	AssertStatus(t, rec, http.StatusOK)
}

// AssertLen fails the test if len(got) != want.
func AssertLen[T any](t *testing.T, got []T, want int, label string) {
	t.Helper()

	if len(got) != want {
		t.Errorf("expected %s count=%d, got %d", label, want, len(got))
	}
}

// RequireLen fails the test (using Fatalf) if len(got) != want.
// Use for assertions where the test cannot continue if the length is wrong
// (e.g. accessing events[0] afterwards).
func RequireLen[T any](t *testing.T, got []T, want int) {
	t.Helper()

	if len(got) != want {
		t.Fatalf("expected %d events, got %d", want, len(got))
	}
}

// WaitForCount polls counter until it returns the expected count.
// counter is a function that returns the current count (e.g., stack.Count).
func WaitForCount(t *testing.T, ctx context.Context, counter func(context.Context) (int64, error), want int64) {
	t.Helper()

	for {
		got, _ := counter(ctx)
		if got == want {
			return
		}
	}
}

// BuildPairs constructs a slice of T from alternating (id, type) string pairs,
// using the provided factory function. Panics if pairs has odd length.
func BuildPairs[T any](factory func(id, typ string) T, pairs ...string) []T {
	if len(pairs)%2 != 0 {
		panic("BuildPair requires an even number of arguments (id, type pairs)")
	}

	result := make([]T, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		result = append(result, factory(pairs[i], pairs[i+1]))
	}

	return result
}

// AssertPanics fails the test if fn does not panic.
func AssertPanics(t *testing.T, fn func(), label string) {
	t.Helper()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for %s", label)
		}
	}()

	fn()
}
