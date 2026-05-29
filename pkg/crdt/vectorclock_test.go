package crdt

import (
	"testing"
)

func TestNewVectorClock(t *testing.T) {
	t.Parallel()

	clock := NewVectorClock()
	if clock == nil {
		t.Fatal("NewVectorClock returned nil")
	}

	if len(clock) != 0 {
		t.Fatalf("expected empty vector clock, got %d entries", len(clock))
	}
}

func TestNewVectorClockFromMap(t *testing.T) {
	t.Parallel()

	entries := map[NodeID]int64{
		"node-a": 3,
		"node-b": 1,
	}

	clock, err := NewVectorClockFromMap(entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(clock) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(clock))
	}

	if clock.Get("node-a") != 3 {
		t.Errorf("expected node-a=3, got %d", clock.Get("node-a"))
	}

	if clock.Get("node-b") != 1 {
		t.Errorf("expected node-b=1, got %d", clock.Get("node-b"))
	}
}

func TestNewVectorClockFromMap_Empty(t *testing.T) {
	t.Parallel()

	clock, err := NewVectorClockFromMap(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clock == nil {
		t.Fatal("expected non-nil clock for nil input")
	}
	if len(clock) != 0 {
		t.Fatalf("expected empty clock, got %d entries", len(clock))
	}

	clock.Increment("node-a")
	if clock.Get("node-a") != 1 {
		t.Errorf("expected Increment to work on nil-input clock, got %d", clock.Get("node-a"))
	}
}

func TestNewVectorClockFromMap_NegativeCounter(t *testing.T) {
	t.Parallel()

	entries := map[NodeID]int64{"node-a": -1}
	_, err := NewVectorClockFromMap(entries)
	if err == nil {
		t.Fatal("expected error for negative counter")
	}
}

func TestVectorClock_Increment(t *testing.T) {
	t.Parallel()

	clock := NewVectorClock()

	clock.Increment(NodeID("node-a"))
	if got := clock.Get(NodeID("node-a")); got != 1 {
		t.Errorf("first increment: got %d, want 1", got)
	}

	clock.Increment(NodeID("node-a"))
	if got := clock.Get(NodeID("node-a")); got != 2 {
		t.Errorf("second increment: got %d, want 2", got)
	}

	clock.Increment(NodeID("node-b"))
	if got := clock.Get(NodeID("node-b")); got != 1 {
		t.Errorf("new node: got %d, want 1", got)
	}

	if got := clock.Get(NodeID("node-a")); got != 2 {
		t.Errorf("original unchanged: got %d, want 2", got)
	}
}

func TestVectorClock_Get_MissingNode(t *testing.T) {
	t.Parallel()

	clock := NewVectorClock()
	if got := clock.Get(NodeID("nonexistent")); got != 0 {
		t.Errorf("missing node: got %d, want 0", got)
	}
}

func TestVectorClock_Merge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     VectorClock
		other    VectorClock
		expected VectorClock
	}{
		{
			name:     "empty into empty",
			base:     NewVectorClock(),
			other:    NewVectorClock(),
			expected: NewVectorClock(),
		},
		{
			name:     "non-empty into empty",
			base:     NewVectorClock(),
			other:    VectorClock{NodeID("a"): 3, NodeID("b"): 5},
			expected: VectorClock{NodeID("a"): 3, NodeID("b"): 5},
		},
		{
			name:     "empty into non-empty",
			base:     VectorClock{NodeID("a"): 3, NodeID("b"): 5},
			other:    NewVectorClock(),
			expected: VectorClock{NodeID("a"): 3, NodeID("b"): 5},
		},
		{
			name:     "merge takes max per node",
			base:     VectorClock{NodeID("a"): 3, NodeID("b"): 2},
			other:    VectorClock{NodeID("a"): 1, NodeID("b"): 5, NodeID("c"): 4},
			expected: VectorClock{NodeID("a"): 3, NodeID("b"): 5, NodeID("c"): 4},
		},
		{
			name:     "disjoint nodes merged",
			base:     VectorClock{NodeID("a"): 2},
			other:    VectorClock{NodeID("b"): 3},
			expected: VectorClock{NodeID("a"): 2, NodeID("b"): 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.base.Merge(tt.other)

			for node, expected := range tt.expected {
				if got := tt.base.Get(node); got != expected {
					t.Errorf("node %q: got %d, want %d", node, got, expected)
				}
			}

			if len(tt.base) != len(tt.expected) {
				t.Errorf("expected %d nodes, got %d", len(tt.expected), len(tt.base))
			}
		})
	}
}
