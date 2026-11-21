package sync

import "time"

// Watermark is the high-water mark of already-synced items: the newest
// CreatedAt the store holds for a source. The incremental sync skips
// every fetched item it already covers, so a re-run can never duplicate
// facts — the pull-mirror's idempotency boundary.
//
// The zero Watermark covers nothing (empty store: every fetched item is
// new).
type Watermark struct {
	at time.Time
}

// NewWatermark returns the watermark at the given instant.
func NewWatermark(at time.Time) Watermark {
	return Watermark{at: at}
}

// Cutoff returns the underlying instant (the zero time when the store is
// empty).
func (w Watermark) Cutoff() time.Time { return w.at }

// IsZero reports whether this watermark covers nothing.
func (w Watermark) IsZero() bool { return w.at.IsZero() }

// Covers reports whether an item created at t is already inside the
// synced high-water mark: strictly before the cutoff, never at it (an
// item created exactly at the cutoff may share the instant with the
// newest synced item, so it is fetched and deduplicated by the baseline
// diff instead).
func (w Watermark) Covers(t time.Time) bool {
	if w.IsZero() {
		return false
	}

	return t.Before(w.at)
}
