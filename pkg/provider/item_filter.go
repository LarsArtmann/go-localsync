package provider

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

// ItemFilter defines optional filters for querying items. Nil fields are ignored.
type ItemFilter struct {
	Type       *id.EventTypeID
	ActorLogin *id.ActorID
	RepoName   *id.RepoID
	Source     *id.ProviderID
	Since      *time.Time
	Limit      int
	Offset     int
}

// WithType returns a copy of f with Type set.
func (f ItemFilter) WithType(t id.EventTypeID) ItemFilter {
	f.Type = &t

	return f
}

// WithActorLogin returns a copy of f with ActorLogin set.
func (f ItemFilter) WithActorLogin(a id.ActorID) ItemFilter {
	f.ActorLogin = &a

	return f
}

// WithRepoName returns a copy of f with RepoName set.
func (f ItemFilter) WithRepoName(r id.RepoID) ItemFilter {
	f.RepoName = &r

	return f
}

// WithSource returns a copy of f with Source set.
func (f ItemFilter) WithSource(s id.ProviderID) ItemFilter {
	f.Source = &s

	return f
}

// WithSince returns a copy of f with Since set.
func (f ItemFilter) WithSince(s time.Time) ItemFilter {
	f.Since = &s

	return f
}

// WithLimit returns a copy of f with Limit set.
func (f ItemFilter) WithLimit(n int) ItemFilter {
	f.Limit = n

	return f
}

// WithOffset returns a copy of f with Offset set.
func (f ItemFilter) WithOffset(n int) ItemFilter {
	f.Offset = n

	return f
}
