package model

import "time"

// TombstoneReason explains why a live item was hidden from the default view.
// It is the honest replacement for a hard delete: the item's history is kept
// and the action is reversible (a later sync resurrects it).
//
// The zero value "" is reserved for "not tombstoned" — every tombstoned item
// carries one of the named reasons below.
type TombstoneReason string

const (
	// ReasonUpstreamGone means the provider no longer returns the item, detected
	// by the reconciliation pass. This is the normal "deleted upstream" case.
	ReasonUpstreamGone TombstoneReason = "upstream_gone"
	// ReasonUserHidden means a consumer hid the item locally (e.g. muted/spam).
	ReasonUserHidden TombstoneReason = "user_hidden"
	// ReasonRedacted means the item was removed for policy or legal reasons.
	ReasonRedacted TombstoneReason = "redacted"
)

// ParseTombstoneReason converts a stored reason string into a TombstoneReason.
// Unknown values collapse to ReasonUpstreamGone — the safe, honest default that
// matches the reconciliation detection path, so an unknown reason never silently
// keeps a hidden item in the live view.
func ParseTombstoneReason(s string) TombstoneReason {
	switch TombstoneReason(s) {
	case ReasonUpstreamGone, ReasonUserHidden, ReasonRedacted:
		return TombstoneReason(s)
	default:
		return ReasonUpstreamGone
	}
}

// Tombstone marks an item as hidden from the default read model while
// preserving its full history. The zero value (Reason == "") means the item is
// live. Always construct an active tombstone via NewTombstone so the timestamp
// is set.
type Tombstone struct {
	// Reason is non-empty for a tombstoned item and "" for a live item.
	Reason TombstoneReason
	// At is when the tombstone was applied (UTC).
	At time.Time
}

// NewTombstone returns an active tombstone with the given reason and a UTC
// timestamp of now.
func NewTombstone(reason TombstoneReason) Tombstone {
	return Tombstone{Reason: reason, At: time.Now().UTC()}
}

// IsZero reports whether this is the zero value, i.e. the item is live.
// Reason is the discriminant: a tombstoned item always carries a reason, so an
// empty reason unambiguously means "not tombstoned" (regardless of At).
func (t Tombstone) IsZero() bool {
	return t.Reason == ""
}
