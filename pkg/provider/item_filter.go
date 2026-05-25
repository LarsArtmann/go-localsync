package provider

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/types"
)

type ItemFilter struct {
	Type       *types.EventTypeID
	ActorLogin *types.ActorID
	RepoName   *types.RepoID
	Source     *types.ProviderID
	Since      *time.Time
	Limit      int
	Offset     int
}
