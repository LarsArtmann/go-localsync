package cqrs

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-cqrs-lite/core/event"
)

const aggregateType event.AggregateType = "sync_item"

const (
	// EventItemSynced is emitted when an item is created or updated.
	EventItemSynced event.Type = "sync_item.synced"
	// EventItemConflictFound is emitted when a conflict is detected (remote wins).
	EventItemConflictFound event.Type = "sync_item.conflict_found"
	// EventItemDeleted is emitted when an item is soft-deleted.
	EventItemDeleted event.Type = "sync_item.deleted"
)

// ItemSyncedPayload is the event payload for ItemSynced events.
type ItemSyncedPayload struct {
	ItemID         string          `json:"itemId"`
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

// ItemConflictFoundPayload is the event payload for ItemConflictFound events.
type ItemConflictFoundPayload struct {
	Source          string `json:"source"`
	SourceID        string `json:"sourceId"`
	LocalUpdatedAt  int64  `json:"localUpdatedAt"`
	RemoteUpdatedAt int64  `json:"remoteUpdatedAt"`
	Winner          string `json:"winner"`
}

// ItemDeletedPayload is the event payload for ItemDeleted events.
type ItemDeletedPayload struct {
	Source   string `json:"source"`
	SourceID string `json:"sourceId"`
}

func unixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}

	return t.UnixNano()
}

func fromUnixNano(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}

	return time.Unix(0, n)
}
