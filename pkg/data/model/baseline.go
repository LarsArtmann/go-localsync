package model

import "time"

// DiffKind classifies a fetched item against the stored baseline — the
// three-way answer to "what does this fetch mean for what we already
// know?".
type DiffKind string

const (
	// DiffFirstSight means no baseline exists yet: the item was never
	// synced, so this fetch creates it.
	DiffFirstSight DiffKind = "first_sight"
	// DiffUnchanged means the fetched version is identical to the
	// baseline; re-recording it would be a no-op.
	DiffUnchanged DiffKind = "unchanged"
	// DiffSuperseded means the fetched version's facts retire the
	// baseline's facts: the new payload supersedes the stored one
	// (history keeps both — the event log is append-only, so a superseded
	// baseline remains replayable forever).
	DiffSuperseded DiffKind = "superseded"
)

// Baseline is the stored snapshot of an item that the next fetched
// version is diffed against — the pull-mirror's answer to "what did we
// already know?". Change detection is fully provider-agnostic:
// ContentHash (a SHA-256 of the provider's raw JSON payload) is the
// primary signal, with UpdatedAt and Type as fallbacks for providers
// that don't set ContentHash.
type Baseline struct {
	ContentHash string
	UpdatedAt   time.Time
	Type        string
}

// BaselineFromItem captures the diff-relevant facts of a stored item.
// A nil item yields the zero Baseline, whose Diff is always
// DiffFirstSight.
func BaselineFromItem(item *Item) Baseline {
	if item == nil {
		return Baseline{}
	}

	return Baseline{
		ContentHash: item.ContentHash.String(),
		UpdatedAt:   item.UpdatedAt,
		Type:        item.Type.Get(),
	}
}

// Diff classifies a fetched item against this baseline: first sight,
// unchanged, or superseded.
func (b Baseline) Diff(remote *Item) DiffKind {
	if b.isZero() {
		return DiffFirstSight
	}

	if b.contentDiffers(remote) || !b.UpdatedAt.Equal(remote.UpdatedAt) || b.Type != remote.Type.Get() {
		return DiffSuperseded
	}

	return DiffUnchanged
}

func (b Baseline) isZero() bool {
	return b.ContentHash == "" && b.UpdatedAt.IsZero() && b.Type == ""
}

// contentDiffers reports a content change when both sides carry a
// ContentHash; hashes are only comparable when both are present.
func (b Baseline) contentDiffers(remote *Item) bool {
	return b.ContentHash != "" &&
		remote.ContentHash.String() != "" &&
		b.ContentHash != remote.ContentHash.String()
}
