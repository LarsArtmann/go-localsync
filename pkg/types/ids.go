// Package types provides domain-specific identifier types using branded IDs
// from go-composable-business-types/id. These types provide compile-time safety
// by preventing accidental mixing of different identifier kinds.
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
	"github.com/larsartmann/go-composable-business-types/id"
)

// Brand types (phantom types for type safety).
// These are empty structs used only as type parameters.
type (
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
// All use string as the underlying value type for flexibility and JSON compatibility.
type (
	// ItemID is a unique identifier for sync items.
	// Example: "1234567890" (GitHub event ID)
	ItemID = id.ID[ItemBrand, string]
	// ProviderID identifies the source provider.
	// Example: "github", "gitlab"
	ProviderID = id.ID[ProviderBrand, string]
	// ActorID identifies the user/actor who triggered an event.
	// Example: "larsartmann"
	ActorID = id.ID[ActorBrand, string]
	// RepoID identifies a repository.
	// Example: "larsartmann/go-localsync"
	RepoID = id.ID[RepoBrand, string]
	// EventTypeID identifies the type of event.
	// Example: "PushEvent", "CreateEvent"
	EventTypeID = id.ID[EventTypeBrand, string]
)

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
