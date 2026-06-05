// Package schema provides schema versioning for domain entities.
// Every item carries a SchemaVersion so old events can be migrated
// forward when the domain model evolves.
package schema

import "fmt"

// Version is the schema version of a domain entity.
type Version int

const (
	// V1 is the original schema — no SchemaVersion field.
	V1 Version = 1
	// V2 is the current schema — adds SchemaVersion field.
	V2 Version = 2
)

// CurrentVersion returns the latest schema version.
func CurrentVersion() Version { return V2 }

// Int returns the version as an int.
func (v Version) Int() int { return int(v) }

// Valid reports whether this is a known schema version.
func (v Version) Valid() bool { return v >= V1 && v <= V2 }

// String implements fmt.Stringer.
func (v Version) String() string { return fmt.Sprintf("v%d", v.Int()) }
