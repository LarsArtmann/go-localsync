package types

import (
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

	got := NewEventID(42)
	if got.Get() != int64(42) {
		t.Errorf("expected 42, got %d", got.Get())
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
