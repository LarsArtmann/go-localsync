package crdt

import (
	"testing"
)

// concurrentClocks returns a pair of vector clocks that are concurrent to
// each other (neither happened-before the other).
func concurrentClocks() (VectorClock, VectorClock) {
	return VectorClock{NodeID("a"): 3, NodeID("b"): 1}, VectorClock{NodeID("a"): 1, NodeID("b"): 3}
}

func TestVectorClock_Cmp_Table(t *testing.T) {
	t.Parallel()

	identicalAB := VectorClock{NodeID("a"): 1, NodeID("b"): 2}
	greaterA := VectorClock{NodeID("a"): 5, NodeID("b"): 2}
	lessB := VectorClock{NodeID("a"): 3, NodeID("b"): 2}
	superset := VectorClock{NodeID("a"): 3, NodeID("b"): 2}
	subset := VectorClock{NodeID("a"): 3}

	concurrentA, concurrentB := concurrentClocks()

	tests := []struct {
		name     string
		a        VectorClock
		b        VectorClock
		expected ClockOrder
	}{
		{"empty clocks are equal", NewVectorClock(), NewVectorClock(), OrderEqual},
		{"identical clocks are equal", superset, superset, OrderEqual},
		{
			"a < b (happened before)",
			VectorClock{NodeID("a"): 1, NodeID("b"): 2},
			lessB,
			OrderBefore,
		},
		{"a > b (happened after)", greaterA, lessB, OrderAfter},
		{"concurrent clocks", concurrentA, concurrentB, OrderConcurrent},
		{"one node vs empty", VectorClock{NodeID("a"): 1}, NewVectorClock(), OrderAfter},
		{"empty vs one node", NewVectorClock(), VectorClock{NodeID("a"): 1}, OrderBefore},
		{"superset clock is greater", superset, subset, OrderAfter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.a.Cmp(tt.b)
			if got != tt.expected {
				t.Errorf("Cmp() = %v, want %v", got, tt.expected)
			}
		})
	}

	if got := identicalAB.Get(NodeID("a")); got != 1 {
		t.Errorf("identicalAB sanity: got %d", got)
	}
}

func TestVectorClock_Compare_Symmetric(t *testing.T) {
	t.Parallel()

	clockA := VectorClock{NodeID("a"): 1}
	clockB := VectorClock{NodeID("a"): 3}

	if clockA.Cmp(clockB) != OrderBefore {
		t.Error("a < b expected")
	}

	if clockB.Cmp(clockA) != OrderAfter {
		t.Error("b > a expected")
	}
}

func TestVectorClock_Clone(t *testing.T) {
	t.Parallel()

	original := VectorClock{NodeID("a"): 3, NodeID("b"): 5}
	cloned := original.Clone()

	if !original.Equal(cloned) {
		t.Fatal("clone should be equal to original")
	}

	cloned.Increment(NodeID("a"))

	if original.Get(NodeID("a")) != 3 {
		t.Fatalf("modifying clone should not affect original, got %d", original.Get(NodeID("a")))
	}
}

func TestVectorClock_Clone_Empty(t *testing.T) {
	t.Parallel()

	original := NewVectorClock()
	cloned := original.Clone()

	if len(cloned) != 0 {
		t.Fatalf("clone of empty should be empty, got %d entries", len(cloned))
	}
}

func TestVectorClock_Equal(t *testing.T) {
	t.Parallel()

	identicalAB := VectorClock{NodeID("a"): 1, NodeID("b"): 2}
	concurrent1 := VectorClock{NodeID("a"): 3, NodeID("b"): 1}
	concurrent2 := VectorClock{NodeID("a"): 1, NodeID("b"): 3}
	disjointA := VectorClock{NodeID("a"): 2}
	disjointB := VectorClock{NodeID("b"): 3}

	tests := []struct {
		name     string
		a        VectorClock
		b        VectorClock
		expected bool
	}{
		{"empty clocks equal", NewVectorClock(), NewVectorClock(), true},
		{"identical clocks equal", identicalAB, identicalAB, true},
		{"concurrent clocks not equal", concurrent1, concurrent2, false},
		{"different sizes not equal", disjointA, identicalAB, false},
		{"same compare result but different nodes", disjointA, disjointB, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.a.Equal(tt.b); got != tt.expected {
				t.Errorf("Equal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVectorClock_Equal_Symmetric(t *testing.T) {
	t.Parallel()

	a := VectorClock{NodeID("a"): 1, NodeID("b"): 2}
	b := VectorClock{NodeID("a"): 1, NodeID("b"): 2}

	if !a.Equal(b) || !b.Equal(a) {
		t.Error("Equal should be symmetric")
	}
}

func TestVectorClock_Cmp(t *testing.T) {
	t.Parallel()

	concurrentA, concurrentB := concurrentClocks()

	tests := []struct {
		name     string
		a        VectorClock
		b        VectorClock
		expected ClockOrder
	}{
		{"empty clocks are equal", NewVectorClock(), NewVectorClock(), OrderEqual},
		{
			"identical clocks are equal",
			VectorClock{NodeID("a"): 1},
			VectorClock{NodeID("a"): 1},
			OrderEqual,
		},
		{"before", VectorClock{NodeID("a"): 1}, VectorClock{NodeID("a"): 3}, OrderBefore},
		{"after", VectorClock{NodeID("a"): 3}, VectorClock{NodeID("a"): 1}, OrderAfter},
		{"concurrent", concurrentA, concurrentB, OrderConcurrent},
		{"empty vs non-empty", NewVectorClock(), VectorClock{NodeID("a"): 1}, OrderBefore},
		{
			"superset is after",
			VectorClock{NodeID("a"): 3, NodeID("b"): 2},
			VectorClock{NodeID("a"): 3},
			OrderAfter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.a.Cmp(tt.b)
			if got != tt.expected {
				t.Errorf("Cmp() = %v (%s), want %v (%s)", got, got, tt.expected, tt.expected)
			}
		})
	}
}

func TestClockOrder_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		order    ClockOrder
		expected string
	}{
		{OrderBefore, "before"},
		{OrderAfter, "after"},
		{OrderConcurrent, "concurrent"},
		{OrderEqual, "equal"},
		{ClockOrder(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			if got := tt.order.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestVectorClock_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		vc       VectorClock
		expected string
	}{
		{"empty", NewVectorClock(), "{}"},
		{"single entry", VectorClock{NodeID("a"): 1}, "{a:1}"},
		{
			"multiple entries sorted",
			VectorClock{NodeID("b"): 5, NodeID("a"): 3},
			"{a:3, b:5}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.vc.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}
