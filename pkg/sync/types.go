package sync

import (
	"context"
	"fmt"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
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
	SourceID id.ExternalID
	Action   SyncAction
	Error    error
}

func (r ItemSyncResult) String() string {
	if r.Error != nil {
		return fmt.Sprintf("%s:%s(%s)", r.SourceID, r.Action, r.Error)
	}

	return fmt.Sprintf("%s:%s", r.SourceID, r.Action)
}

// SyncSummary aggregates results from a batch SyncItems call.
type SyncSummary struct {
	Synced    int
	Conflicts int
	Errors    int
	Results   []ItemSyncResult
}

func (s SyncSummary) String() string {
	return fmt.Sprintf("synced=%d conflicts=%d errors=%d", s.Synced, s.Conflicts, s.Errors)
}

func (a SyncAction) String() string { return string(a) }

func (a SyncAction) IsValid() bool {
	switch a {
	case ActionCreated, ActionUpdated, ActionConflictRemote, ActionConflictLocal, ActionUnchanged, ActionError:
		return true
	default:
		return false
	}
}

// SyncStore is the minimal interface that decouples sync logic from concrete storage.
// *cqrs.CQRSStack implements this interface by embedding the cqrs ReadModel and
// adding SyncItems + Close. The read-side methods come from model.ItemReader.
type SyncStore interface {
	SyncItems(ctx context.Context, items []*provider.Item) *SyncSummary
	model.ItemReader
	Close() error
}
