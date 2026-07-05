package model

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

// ItemFilter defines optional filters for querying items. Nil fields are ignored.
type ItemFilter struct {
	Type   *id.EventTypeID
	Source *id.ProviderID
	Since  *time.Time
	Limit  int
	Offset int
	// Attributes filters by key-value pairs. All specified attributes must
	// match (AND logic). A nil or empty map matches all items.
	Attributes map[string]string
	// IncludeTombstoned, when true, returns tombstoned (hidden) items too.
	// The zero value (false) excludes them — the safe default for a live view.
	IncludeTombstoned bool
}

// WithType returns a copy of f with Type set.
func (f ItemFilter) WithType(t id.EventTypeID) ItemFilter {
	f.Type = &t

	return f
}

// WithAttribute returns a copy of f with the given attribute key-value pair
// added to the Attributes filter. Multiple calls add multiple pairs (AND logic).
func (f ItemFilter) WithAttribute(key, value string) ItemFilter {
	if f.Attributes == nil {
		f.Attributes = map[string]string{}
	}

	f.Attributes[key] = value

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

// WithIncludeTombstoned returns a copy of f with IncludeTombstoned set.
func (f ItemFilter) WithIncludeTombstoned(include bool) ItemFilter {
	f.IncludeTombstoned = include

	return f
}
