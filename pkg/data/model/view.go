package model

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

// ItemView is the read-model projection of an Item.
// It augments the domain entity with computed fields that
// only make sense in the query/read context.
type ItemView struct {
	Item
	LastSyncedAt  time.Time
	SyncCount     int64
	ConflictCount int64
	IsDeleted     bool
}

// GetCreatedAt delegates to the embedded Item for criterion matching.
func (v ItemView) GetCreatedAt() time.Time { return v.Item.CreatedAt }

// GetUpdatedAt delegates to the embedded Item for criterion matching.
func (v ItemView) GetUpdatedAt() time.Time { return v.Item.UpdatedAt }

// GetSource delegates to the embedded Item for criterion matching.
func (v ItemView) GetSource() id.ProviderID { return v.Item.Source }

// GetType delegates to the embedded Item for criterion matching.
func (v ItemView) GetType() id.EventTypeID { return v.Item.Type }

// GetActorLogin delegates to the embedded Item for criterion matching.
func (v ItemView) GetActorLogin() id.ActorID { return v.Item.ActorLogin }

// GetRepoName delegates to the embedded Item for criterion matching.
func (v ItemView) GetRepoName() id.RepoID { return v.Item.RepoName }

// StatsView is a denormalized aggregation for dashboard/API use.
type StatsView struct {
	TotalItems int64
	ItemTypes  []string
	TypeCounts map[string]int64
	Sources    map[string]int64
}

// EmptyStatsView returns a zero-value StatsView with initialized maps.
func EmptyStatsView() StatsView {
	return StatsView{
		ItemTypes:  []string{},
		TypeCounts: make(map[string]int64),
		Sources:    make(map[string]int64),
	}
}
