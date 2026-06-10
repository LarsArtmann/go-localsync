// Package testutil provides shared test helpers for use across test files.
//
// All functions take *testing.T to integrate with the standard testing package.
package testutil

import (
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

// AssertStatusOK fails the test if the recorder's HTTP status is not 200.
func AssertStatusOK(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
