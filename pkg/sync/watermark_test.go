package sync

import (
	"testing"
	"time"
)

func TestWatermarkCovers(t *testing.T) {
	t.Parallel()

	cutoff := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	watermark := NewWatermark(cutoff)

	tests := []struct {
		name   string
		at     time.Time
		covers bool
	}{
		{"strictly before the cutoff is covered", cutoff.Add(-time.Second), true},
		{"at the cutoff is not covered (baseline diff dedupes it)", cutoff, false},
		{"after the cutoff is not covered", cutoff.Add(time.Second), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := watermark.Covers(tt.at); got != tt.covers {
				t.Errorf("Covers(%v) = %v, want %v", tt.at, got, tt.covers)
			}
		})
	}
}

func TestWatermarkZeroCoversNothing(t *testing.T) {
	t.Parallel()

	var watermark Watermark

	if !watermark.IsZero() {
		t.Error("zero-value watermark must report IsZero")
	}

	if watermark.Covers(time.Unix(0, 0)) {
		t.Error("zero watermark must cover nothing (empty store: everything is new)")
	}
}
