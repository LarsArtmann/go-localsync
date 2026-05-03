package cqrs

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-cqrs-lite/core/event"
)

// Event type constants for the SyncItem aggregate.
const (
	// EventItemSynced is emitted when a new item is synced or an existing item is updated.
	EventItemSynced event.Type = "sync_item.synced"
	// EventItemConflictFound is emitted when a conflict is detected between local and remote items.
	EventItemConflictFound event.Type = "sync_item.conflict_found"
	// EventItemDeleted is emitted when an item is deleted.
	EventItemDeleted event.Type = "sync_item.deleted"
)

// ItemSyncedPayload is the payload for EventItemSynced events.
type ItemSyncedPayload struct {
	Source         string          `json:"source"`
	SourceID       string          `json:"sourceId"`
	Type           string          `json:"type"`
	ActorLogin     string          `json:"actorLogin,omitempty"`
	ActorAvatarURL string          `json:"actorAvatarUrl,omitempty"`
	RepoName       string          `json:"repoName,omitempty"`
	RepoURL        string          `json:"repoUrl,omitempty"`
	CreatedAt      int64           `json:"createdAt"`
	UpdatedAt      int64           `json:"updatedAt"`
	RawJSON        json.RawMessage `json:"rawJson,omitempty"`
}

// ItemConflictFoundPayload is the payload for EventItemConflictFound events.
type ItemConflictFoundPayload struct {
	Source          string `json:"source"`
	SourceID        string `json:"sourceId"`
	LocalUpdatedAt  int64  `json:"localUpdatedAt"`
	RemoteUpdatedAt int64  `json:"remoteUpdatedAt"`
	Winner          string `json:"winner"`
}

// ItemDeletedPayload is the payload for EventItemDeleted events.
type ItemDeletedPayload struct {
	Source   string `json:"source"`
	SourceID string `json:"sourceId"`
}

// unixNano returns the Unix nano timestamp for the given time, or 0 if zero.
func unixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}

	return t.UnixNano()
}

// fromUnixNano converts a Unix nano timestamp back to time.Time.
func fromUnixNano(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}

	return time.Unix(0, n)
}
