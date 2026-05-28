package provider

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

type ItemFilter struct {
	Type       *id.EventTypeID
	ActorLogin *id.ActorID
	RepoName   *id.RepoID
	Source     *id.ProviderID
	Since      *time.Time
	Limit      int
	Offset     int
}
