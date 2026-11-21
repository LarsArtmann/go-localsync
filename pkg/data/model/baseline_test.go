package model

import (
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

func TestBaselineDiffFirstSight(t *testing.T) {
	t.Parallel()

	var baseline Baseline

	remote := &Item{
		SourceID:    id.NewSourceID("123"),
		Type:        id.NewEventTypeID("PushEvent"),
		ContentHash: id.NewContentHash("hash-1"),
		UpdatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if got := baseline.Diff(remote); got != DiffFirstSight {
		t.Errorf("zero baseline Diff = %s, want %s", got, DiffFirstSight)
	}

	if got := BaselineFromItem(nil).Diff(remote); got != DiffFirstSight {
		t.Errorf("nil-item baseline Diff = %s, want %s", got, DiffFirstSight)
	}
}

func TestBaselineDiffUnchangedGoldenVector(t *testing.T) {
	t.Parallel()

	item := &Item{
		SourceID:    id.NewSourceID("123"),
		Source:      id.NewProviderID("github"),
		Type:        id.NewEventTypeID("PushEvent"),
		ContentHash: id.NewContentHash("9f2a"),
		UpdatedAt:   time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC),
	}

	if got := BaselineFromItem(item).Diff(item); got != DiffUnchanged {
		t.Errorf("identical payload Diff = %s, want %s", got, DiffUnchanged)
	}
}

func TestBaselineDiffSupersededVectors(t *testing.T) {
	t.Parallel()

	base := func() *Item {
		return &Item{
			SourceID:    id.NewSourceID("123"),
			Type:        id.NewEventTypeID("PushEvent"),
			ContentHash: id.NewContentHash("hash-1"),
			UpdatedAt:   time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC),
		}
	}

	tests := []struct {
		name string
		mut  func(remote *Item)
	}{
		{
			name: "content hash moved on",
			mut:  func(r *Item) { r.ContentHash = id.NewContentHash("hash-2") },
		},
		{
			name: "updated-at moved on",
			mut:  func(r *Item) { r.UpdatedAt = r.UpdatedAt.Add(time.Hour) },
		},
		{
			name: "type moved on",
			mut:  func(r *Item) { r.Type = id.NewEventTypeID("WatchEvent") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			local := base()
			remote := base()
			tt.mut(remote)

			if got := BaselineFromItem(local).Diff(remote); got != DiffSuperseded {
				t.Errorf("Diff = %s, want %s (the new facts supersede the baseline)", got, DiffSuperseded)
			}
		})
	}
}

func TestBaselineDiffEmptyHashOneSideStaysUnchanged(t *testing.T) {
	t.Parallel()

	local := &Item{
		Type:        id.NewEventTypeID("PushEvent"),
		ContentHash: id.NewContentHash(""),
		UpdatedAt:   time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC),
	}

	remote := *local
	remote.ContentHash = id.NewContentHash("hash-2")

	if got := BaselineFromItem(local).Diff(&remote); got != DiffUnchanged {
		t.Errorf("hashless baseline Diff = %s, want %s (hashes only compare when both sides carry one)", got, DiffUnchanged)
	}
}
