// Package testutil provides shared test helpers for use across test files.
//
// All functions take *testing.T to integrate with the standard testing package.
package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
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
