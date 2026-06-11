package sync

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// SyncAction classifies the outcome of syncing a single item.
type SyncAction string

const (
	ActionCreated        SyncAction = "created"
	ActionUpdated        SyncAction = "updated"
	ActionConflictRemote SyncAction = "conflict_remote"
	ActionConflictLocal  SyncAction = "conflict_local"
	ActionUnchanged      SyncAction = "unchanged"
	ActionError          SyncAction = "error"
)

// ItemSyncResult holds the result of syncing a single item.
type ItemSyncResult struct {
	SourceID string
	Action   SyncAction
	Error    error
}

// SyncSummary aggregates results from a batch SyncItems call.
type SyncSummary struct {
	Synced    int
	Conflicts int
	Errors    int
	Results   []ItemSyncResult
}

// SyncStore is the minimal interface that decouples sync logic from concrete storage.
// *cqrs.CQRSStack implements this interface by embedding the cqrs ReadModel and
// adding SyncItems + Close. The read-side methods come from model.ItemReader.
type SyncStore interface {
	SyncItems(ctx context.Context, items []*provider.Item) *SyncSummary
	model.ItemReader
	Close() error
}
