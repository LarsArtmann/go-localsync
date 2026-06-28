package crdt

import (
	"encoding/json"
	"testing"
	"time"
)

type testItem struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func assertWinner(t *testing.T, winner testItem, want, context string) {
	t.Helper()

	if winner.Name != want {
		t.Errorf("expected %s to win%s, got %q", want, context, winner.Name)
	}
}

func itemTimestamp(item testItem) time.Time {
	return item.UpdatedAt
}

func newTestConflict(local, remote testItem) *Conflict[testItem] {
	return &Conflict[testItem]{Local: local, Remote: remote}
}

func TestLWWResolver_LocalWinsByTimestamp(t *testing.T) {
	t.Parallel()

	resolver, _ := NewLWWResolver(itemTimestamp)
	now := time.Now()
	local := testItem{Name: "local", UpdatedAt: now.Add(2 * time.Hour)}
	remote := testItem{Name: "remote", UpdatedAt: now}

	winner, err := resolver.Resolve(newTestConflict(local, remote))
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	assertWinner(t, winner, "local", " (later timestamp)")
}

func TestLWWResolver_RemoteWinsByTimestamp(t *testing.T) {
	t.Parallel()

	resolver, _ := NewLWWResolver(itemTimestamp)
	now := time.Now()
	local := testItem{Name: "local", UpdatedAt: now}
	remote := testItem{Name: "remote", UpdatedAt: now.Add(2 * time.Hour)}

	winner, err := resolver.Resolve(newTestConflict(local, remote))
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	assertWinner(t, winner, "remote", " (later timestamp)")
}

func TestLWWResolver_RemoteWinsOnTie_NoTiebreaker(t *testing.T) {
	t.Parallel()

	resolver, _ := NewLWWResolver(itemTimestamp)
	now := time.Now()
	local := testItem{Name: "local", UpdatedAt: now}
	remote := testItem{Name: "remote", UpdatedAt: now}

	winner, err := resolver.Resolve(newTestConflict(local, remote))
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}

	if winner.Name != "remote" {
		t.Errorf("expected remote to win on tie (no tiebreaker), got %q", winner.Name)
	}
}

func TestLWWResolver_Tiebreaker(t *testing.T) {
	t.Parallel()

	resolver, _ := NewLWWResolver(itemTimestamp)
	resolver.Tiebreaker = func(local, remote testItem) bool {
		return local.Name < remote.Name
	}

	now := time.Now()

	tests := []struct {
		name         string
		localName    string
		remoteName   string
		expectWinner string
	}{
		{"local wins with smaller name", "aaa", "zzz", "aaa"},
		{"remote wins with smaller name", "zzz", "aaa", "aaa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			local := testItem{Name: tt.localName, UpdatedAt: now}
			remote := testItem{Name: tt.remoteName, UpdatedAt: now}

			winner, err := resolver.Resolve(newTestConflict(local, remote))
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}

			assertWinner(t, winner, tt.expectWinner, " via tiebreaker")
		})
	}
}

func TestLWWResolver_NilTimestampFunc(t *testing.T) {
	t.Parallel()

	_, err := NewLWWResolver[testItem](nil)
	if err == nil {
		t.Fatal("expected error for nil timestamp func")
	}
}

func TestLWWResolver_ImplementsInterface(t *testing.T) {
	t.Parallel()

	var resolver ConflictResolver[testItem] = &LWWResolver[testItem]{
		TimestampFunc: itemTimestamp,
	}
	_ = resolver
}

func TestConflict_JSON_RoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now()
	conflict := Conflict[testItem]{
		Local:     testItem{Name: "local", UpdatedAt: now},
		Remote:    testItem{Name: "remote", UpdatedAt: now.Add(time.Hour)},
		Timestamp: now,
	}

	data, err := json.Marshal(conflict)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Conflict[testItem]
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Local.Name != "local" || decoded.Remote.Name != "remote" {
		t.Errorf("round-trip mismatch: local=%q remote=%q", decoded.Local.Name, decoded.Remote.Name)
	}
}
