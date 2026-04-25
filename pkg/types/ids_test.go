package types

import (
	"strings"
	"testing"
)

func TestNewStringIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"NewItemID", NewItemID("event-123").Get(), "event-123"},
		{"NewProviderID", NewProviderID("github").Get(), "github"},
		{"NewActorID", NewActorID("octocat").Get(), "octocat"},
		{"NewRepoID", NewRepoID("org/repo").Get(), "org/repo"},
		{"NewEventTypeID", NewEventTypeID("PushEvent").Get(), "PushEvent"},
		{"NewSourceItemID", NewSourceItemID("999").Get(), "999"},
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

func TestNewEventID(t *testing.T) {
	t.Parallel()

	got := NewEventID()
	if got.Get().IsZero() {
		t.Error("expected non-zero ULID")
	}

	s := got.String()
	if len(s) != 26 {
		t.Errorf("expected 26-char ULID string, got %d chars: %s", len(s), s)
	}
}

func TestMustParseEventID(t *testing.T) {
	t.Parallel()

	original := NewEventID()
	parsed := MustParseEventID(original.String())

	if !original.Equal(parsed) {
		t.Errorf("expected %s, got %s", original.String(), parsed.String())
	}
}

func TestMustParseEventID_panics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid ULID string")
		}
	}()

	MustParseEventID("not-a-valid-ulid")
}

func TestEventIDIsZero(t *testing.T) {
	t.Parallel()

	var zero EventID
	if !zero.IsZero() {
		t.Error("expected zero EventID to be zero")
	}

	if NewEventID().IsZero() {
		t.Error("expected new EventID to not be zero")
	}
}

func TestIDIsZero(t *testing.T) {
	t.Parallel()

	if !NewItemID("").IsZero() {
		t.Error("expected empty ItemID to be zero")
	}

	if NewItemID("abc").IsZero() {
		t.Error("expected non-empty ItemID to not be zero")
	}
}

func TestIDEqual(t *testing.T) {
	t.Parallel()

	a := NewItemID("1")
	b := NewItemID("1")
	c := NewItemID("2")

	if !a.Equal(b) {
		t.Error("expected equal IDs to be equal")
	}

	if a.Equal(c) {
		t.Error("expected different IDs to not be equal")
	}
}

func TestIDString(t *testing.T) {
	t.Parallel()

	id := NewItemID("test-id")
	s := id.String()
	if s == "" {
		t.Error("expected non-empty string representation")
	}
}

func TestEventIDStringRoundTrip(t *testing.T) {
	t.Parallel()

	id := NewEventID()
	s := id.String()

	if !strings.HasPrefix(s, "0") {
		t.Errorf("ULID should start with '0' in this millennium, got: %s", s)
	}

	parsed := MustParseEventID(s)
	if !id.Equal(parsed) {
		t.Errorf("round-trip failed: %s != %s", id.String(), parsed.String())
	}
}
