package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

func MustNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func AssertEqual[T comparable](t *testing.T, got, want T, label string) {
	t.Helper()

	if got != want {
		t.Errorf("expected %s=%v, got %v", label, want, got)
	}
}

func AssertInt(t *testing.T, got, want int, label string) {
	t.Helper()

	AssertEqual(t, got, want, label)
}

func AssertInt64(t *testing.T, got, want int64, label string) {
	t.Helper()

	AssertEqual(t, got, want, label)
}

func AssertContains[T comparable](t *testing.T, haystack []T, needle T, label string) {
	t.Helper()

	if !slices.Contains(haystack, needle) {
		t.Errorf("expected %s to contain %v, got %v", label, needle, haystack)
	}
}

func AssertExternalID(t *testing.T, item *model.Item, want string) {
	t.Helper()

	AssertEqual(t, item.ExternalID.Get(), want, "ExternalID")
}

func AssertType(t *testing.T, item *model.Item, want string) {
	t.Helper()

	AssertEqual(t, item.Type.Get(), want, "Type")
}

func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, wantCode int) {
	t.Helper()

	if rec.Code != wantCode {
		t.Fatalf("expected status %d, got %d: %s", wantCode, rec.Code, rec.Body.String())
	}
}

func AssertStatusOK(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	AssertStatus(t, rec, http.StatusOK)
}

func AssertLen[T any](t *testing.T, got []T, want int, label string) {
	t.Helper()

	if len(got) != want {
		t.Errorf("expected %s count=%d, got %d", label, want, len(got))
	}
}

func RequireLen[T any](t *testing.T, got []T, want int) {
	t.Helper()

	if len(got) != want {
		t.Fatalf("expected %d events, got %d", want, len(got))
	}
}

func WaitForCount(t *testing.T, ctx context.Context, counter func(context.Context) (int64, error), want int64) {
	t.Helper()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		got, _ := counter(ctx)
		if got == want {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf(
				"WaitForCount: context canceled while waiting for count %d (last seen: %d): %v",
				want,
				got,
				ctx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func BuildPairs[T any](factory func(id, typ string) T, pairs ...string) []T {
	const pairSize = 2

	if len(pairs)%pairSize != 0 {
		panic("BuildPair requires an even number of arguments (id, type pairs)")
	}

	result := make([]T, 0, len(pairs)/pairSize)
	for i := 0; i < len(pairs); i += pairSize {
		result = append(result, factory(pairs[i], pairs[i+1]))
	}

	return result
}

func AssertPanics(t *testing.T, fn func(), label string) {
	t.Helper()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for %s", label)
		}
	}()

	fn()
}
