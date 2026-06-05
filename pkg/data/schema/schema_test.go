package schema

import "testing"

func TestVersionInt(t *testing.T) {
	t.Parallel()

	if got, want := V1.Int(), 1; got != want {
		t.Errorf("V1.Int() = %d, want %d", got, want)
	}

	if got, want := V2.Int(), 2; got != want {
		t.Errorf("V2.Int() = %d, want %d", got, want)
	}
}

func TestVersionValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		v    Version
		want bool
	}{
		{"V1", V1, true},
		{"V2", V2, true},
		{"unknown-0", 0, false},
		{"unknown-3", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.v.Valid(); got != tt.want {
				t.Errorf("Version(%d).Valid() = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}

func TestCurrentVersion(t *testing.T) {
	t.Parallel()

	if got := CurrentVersion(); got != V2 {
		t.Errorf("CurrentVersion() = %v, want %v", got, V2)
	}
}

func TestVersionString(t *testing.T) {
	t.Parallel()

	if got, want := V1.String(), "v1"; got != want {
		t.Errorf("V1.String() = %q, want %q", got, want)
	}

	if got, want := V2.String(), "v2"; got != want {
		t.Errorf("V2.String() = %q, want %q", got, want)
	}
}
