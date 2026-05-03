package cqrs

import (
	"context"
	"time"
)

// ReadModel is the projected read-side of the CQRS architecture.
// It maintains the current state of all sync items, updated by event projection.
type ReadModel interface {
	// Get retrieves a single item by source and sourceID.
	// Returns nil, ErrNotFound if not found.
	Get(ctx context.Context, source, sourceID string) (*itemState, error)
	// List retrieves items matching the filter with pagination.
	List(ctx context.Context, filter ItemFilter) ([]*itemState, error)
	// Count returns the number of items matching the filter.
	Count(ctx context.Context, filter ItemFilter) (int64, error)
	// GetTypes returns all unique item types.
	GetTypes(ctx context.Context) ([]string, error)
	// Upsert inserts or updates an item in the read model.
	Upsert(ctx context.Context, state *itemState) error
	// Delete removes an item from the read model.
	Delete(ctx context.Context, source, sourceID string) error
	// Close releases resources.
	Close() error
}

// ItemFilter defines filtering and pagination for list/count queries.
type ItemFilter struct {
	Type       *string
	ActorLogin *string
	RepoName   *string
	Source     *string
	Since      *time.Time
	Limit      int
	Offset     int
}

// itemState is the read-model representation of a sync item.
// This is the state that queries read from, separate from the event-sourced state.
type itemState struct {
	Source         string    `json:"source"`
	SourceID       string    `json:"sourceId"`
	Type           string    `json:"type"`
	ActorLogin     string    `json:"actorLogin"`
	ActorAvatarURL string    `json:"actorAvatarUrl"`
	RepoName       string    `json:"repoName"`
	RepoURL        string    `json:"repoUrl"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	RawJSON        []byte    `json:"rawJson"`
}

// fromSyncItemState converts a SyncItemState to an itemState for the read model.
func fromSyncItemState(s SyncItemState) *itemState {
	return &itemState{
		Source:         s.Source,
		SourceID:       s.SourceID,
		Type:           s.Type,
		ActorLogin:     s.ActorLogin,
		ActorAvatarURL: s.ActorAvatarURL,
		RepoName:       s.RepoName,
		RepoURL:        s.RepoURL,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
		RawJSON:        s.RawJSON,
	}
}

// key returns the storage key for this item.
func (s *itemState) key() string {
	return s.Source + ":" + s.SourceID
}

// matchesFilter returns true if this item matches all non-nil filter criteria.
func (s *itemState) matchesFilter(f ItemFilter) bool {
	if f.Type != nil && s.Type != *f.Type {
		return false
	}

	if f.ActorLogin != nil && s.ActorLogin != *f.ActorLogin {
		return false
	}

	if f.RepoName != nil && s.RepoName != *f.RepoName {
		return false
	}

	if f.Source != nil && s.Source != *f.Source {
		return false
	}

	if f.Since != nil && s.CreatedAt.Before(*f.Since) {
		return false
	}

	return true
}
