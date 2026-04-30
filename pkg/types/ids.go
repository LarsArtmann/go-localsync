// Package types provides domain-specific identifier types using branded IDs
// from go-branded-id. These types provide compile-time safety by preventing
// accidental mixing of different identifier kinds.
//
// go-cqrs-lite provides a related ID system (id.Of[T] backed by ULID) for
// CQRS aggregates and events. The two systems share the same underlying
// library (go-branded-id) but use different generic parameters and are not
// directly interoperable at the type level.
//
// # Usage
//
//	itemID := types.NewItemID("event-123")
//	actorID := types.NewActorID("larsartmann")
//
//	// This is a compile error - cannot use ItemID where ActorID is expected:
//	// ProcessActor(itemID) // ERROR: type mismatch
package types

import (
	"crypto/rand"
	"time"

	id "github.com/larsartmann/go-branded-id"
	"github.com/oklog/ulid/v2"
)

// Brand types (phantom types for type safety).
// These are empty structs used only as type parameters.
type (
	// EventBrand distinguishes EventID from other identifier types.
	EventBrand struct{}
	// SourceItemBrand distinguishes SourceItemID from other identifier types.
	SourceItemBrand struct{}
	// ItemBrand distinguishes ItemID from other identifier types.
	ItemBrand struct{}
	// ProviderBrand distinguishes ProviderID from other identifier types.
	ProviderBrand struct{}
	// ActorBrand distinguishes ActorID from other identifier types.
	ActorBrand struct{}
	// RepoBrand distinguishes RepoID from other identifier types.
	RepoBrand struct{}
	// EventTypeBrand distinguishes EventTypeID from other identifier types.
	EventTypeBrand struct{}
)

// ID type aliases for domain-specific identifiers.
type (
	// EventID is the internal database identifier for events using ULID.
	// Example: "01H0G0K1P1V2J3M4N5O6P7Q8R9" (ULID, time-sortable unique identifier).
	EventID = id.ID[EventBrand, ulid.ULID]
	// SourceItemID is the provider-specific item identifier used for upsert operations.
	// Example: "1234567890" (GitHub event ID as string for compatibility).
	SourceItemID = id.ID[SourceItemBrand, string]
	// ItemID is a unique identifier for sync items.
	// Example: "1234567890" (GitHub event ID).
	ItemID = id.ID[ItemBrand, string]
	// ProviderID identifies the source provider.
	// Example: "github", "gitlab".
	ProviderID = id.ID[ProviderBrand, string]
	// ActorID identifies the user/actor who triggered an event.
	// Example: "larsartmann".
	ActorID = id.ID[ActorBrand, string]
	// RepoID identifies a repository.
	// Example: "larsartmann/go-localsync".
	RepoID = id.ID[RepoBrand, string]
	// EventTypeID identifies the type of event.
	// Example: "PushEvent", "CreateEvent".
	EventTypeID = id.ID[EventTypeBrand, string]
)

// NewEventID creates a new EventID with a freshly generated ULID.
func NewEventID() EventID {
	return id.NewID[EventBrand](ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader))
}

// MustParseEventID parses a ULID string into an EventID. Panics on invalid input.
func MustParseEventID(s string) EventID {
	return id.NewID[EventBrand](ulid.MustParse(s))
}

// NewSourceItemID creates a new SourceItemID from a string value.
func NewSourceItemID(v string) SourceItemID { return id.NewID[SourceItemBrand](v) }

// NewItemID creates a new ItemID from a string value.
func NewItemID(v string) ItemID { return id.NewID[ItemBrand](v) }

// NewProviderID creates a new ProviderID from a string value.
func NewProviderID(v string) ProviderID { return id.NewID[ProviderBrand](v) }

// NewActorID creates a new ActorID from a string value.
func NewActorID(v string) ActorID { return id.NewID[ActorBrand](v) }

// NewRepoID creates a new RepoID from a string value.
func NewRepoID(v string) RepoID { return id.NewID[RepoBrand](v) }

// NewEventTypeID creates a new EventTypeID from a string value.
func NewEventTypeID(v string) EventTypeID { return id.NewID[EventTypeBrand](v) }
