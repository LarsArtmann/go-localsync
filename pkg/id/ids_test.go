package id

import (
	"strings"
	"testing"
)

func assertItemIDsEqual(t *testing.T, want, got ItemID, context string) {
	t.Helper()

	if !want.Equal(got) {
		t.Errorf("%s: expected %s, got %s", context, want.String(), got.String())
	}
}

func TestNewExternalIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"NewExternalID", NewExternalID("event-123").Get(), "event-123"},
		{"NewProviderID", NewProviderID("github").Get(), "github"},
		{"NewActorID", NewActorID("octocat").Get(), "octocat"},
		{"NewRepoID", NewRepoID("org/repo").Get(), "org/repo"},
		{"NewEventTypeID", NewEventTypeID("PushEvent").Get(), "PushEvent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, tt.got)
			}
		})
	}
}

func TestNewItemID(t *testing.T) {
	t.Parallel()

	got := NewItemID()
	if got.Get().IsZero() {
		t.Error("expected non-zero ULID")
	}

	s := got.String()
	if len(s) != 26 {
		t.Errorf("expected 26-char ULID string, got %d chars: %s", len(s), s)
	}
}

func TestMustParseItemID(t *testing.T) {
	t.Parallel()

	original := NewItemID()
	parsed := MustParseItemID(original.String())

	assertItemIDsEqual(t, original, parsed, "MustParseItemID round-trip")
}

func TestMustParseItemID_panics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid ULID string")
		}
	}()

	MustParseItemID("not-a-valid-ulid")
}

func TestParseItemID(t *testing.T) {
	t.Parallel()

	original := NewItemID()
	parsed, err := ParseItemID(original.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertItemIDsEqual(t, original, parsed, "ParseItemID round-trip")
}

func TestParseItemID_invalid(t *testing.T) {
	t.Parallel()

	_, err := ParseItemID("not-a-valid-ulid")
	if err == nil {
		t.Error("expected error for invalid ULID string")
	}
}

func TestItemIDIsZero(t *testing.T) {
	t.Parallel()

	var zero ItemID
	if !zero.IsZero() {
		t.Error("expected zero ItemID to be zero")
	}

	if NewItemID().IsZero() {
		t.Error("expected new ItemID to not be zero")
	}
}

func TestExternalIDIsZero(t *testing.T) {
	t.Parallel()

	if !NewExternalID("").IsZero() {
		t.Error("expected empty ExternalID to be zero")
	}

	if NewExternalID("abc").IsZero() {
		t.Error("expected non-empty ExternalID to not be zero")
	}
}

func TestIDEqual(t *testing.T) {
	t.Parallel()

	a := NewExternalID("1")
	b := NewExternalID("1")
	c := NewExternalID("2")

	if !a.Equal(b) {
		t.Error("expected equal IDs to be equal")
	}

	if a.Equal(c) {
		t.Error("expected different IDs to not be equal")
	}
}

func TestItemIDEqual(t *testing.T) {
	t.Parallel()

	a := NewItemID()
	b := a
	c := NewItemID()

	if !a.Equal(b) {
		t.Error("expected equal IDs to be equal")
	}

	if a.Equal(c) {
		t.Error("expected different IDs to not be equal")
	}
}

func TestIDString(t *testing.T) {
	t.Parallel()

	testID := NewExternalID("test-id")
	s := testID.String()
	if s == "" {
		t.Error("expected non-empty string representation")
	}
}

func TestItemIDStringRoundTrip(t *testing.T) {
	t.Parallel()

	testID := NewItemID()
	s := testID.String()

	if !strings.HasPrefix(s, "0") {
		t.Errorf("ULID should start with '0' in this millennium, got: %s", s)
	}

	parsed := MustParseItemID(s)
	if !testID.Equal(parsed) {
		t.Errorf("round-trip failed: %s != %s", testID.String(), parsed.String())
	}
}
