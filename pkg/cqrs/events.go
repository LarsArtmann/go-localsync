package cqrs

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
)

const aggregateType event.StreamType = "sync_item"

const (
	// EventItemSynced is emitted when an item is created or updated. Folding it
	// always yields a live item, so re-syncing a tombstoned item resurrects it.
	EventItemSynced event.Type = "sync_item.synced"
	// EventItemConflictFound is emitted when a conflict is detected (remote wins).
	EventItemConflictFound event.Type = "sync_item.conflict_found"
	// EventItemTombstoned is emitted when an item is hidden from the default read
	// model. The row is kept (history preserved); a later EventItemSynced resurrects.
	EventItemTombstoned event.Type = "sync_item.tombstoned"
)

// ItemSyncedPayload is the event payload for ItemSynced events.
//
// Schema V3 replaces ActorLogin/ActorAvatarURL/RepoName/RepoURL with a
// single Attributes map. The legacy fields are kept (omitempty) so that
// events persisted under V1/V2 decode correctly; dataItemFromPayload
// upcasts them into Attributes when Attributes is nil.
type ItemSyncedPayload struct {
	ItemID        string            `json:"itemId"`
	Source        string            `json:"source"`
	SourceID      string            `json:"sourceId"`
	Type          string            `json:"type"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	ContentHash   string            `json:"contentHash,omitempty"`
	CreatedAt     int64             `json:"createdAt"`
	UpdatedAt     int64             `json:"updatedAt"`
	RawJSON       json.RawMessage   `json:"rawJson,omitempty"`
	SchemaVersion int               `json:"schemaVersion,omitempty"`

	// Legacy fields (schema V1/V2). Kept for backward-compatible event replay.
	// New events (V3) leave these empty and use Attributes instead.
	ActorLogin     string `json:"actorLogin,omitempty"`
	ActorAvatarURL string `json:"actorAvatarUrl,omitempty"`
	RepoName       string `json:"repoName,omitempty"`
	RepoURL        string `json:"repoUrl,omitempty"`
}

// ItemConflictFoundPayload is the event payload for ItemConflictFound events.
type ItemConflictFoundPayload struct {
	Source          string `json:"source"`
	SourceID        string `json:"sourceId"`
	LocalUpdatedAt  int64  `json:"localUpdatedAt"`
	RemoteUpdatedAt int64  `json:"remoteUpdatedAt"`
	Winner          string `json:"winner"`
}

// ItemTombstonedPayload is the event payload for ItemTombstoned events.
type ItemTombstonedPayload struct {
	Source       string `json:"source"`
	SourceID     string `json:"sourceId"`
	Reason       string `json:"reason"`
	TombstonedAt int64  `json:"tombstonedAt"`
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

	return time.Unix(0, n).UTC()
}
