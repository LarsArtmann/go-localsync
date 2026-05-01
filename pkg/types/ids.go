// Package types provides domain-specific identifier types using branded IDs
// from go-branded-id. These types provide compile-time safety by preventing
// accidental mixing of different identifier kinds.
//
// All entity identifiers use ULID as the value type, aligning with go-cqrs-lite's
// id.Of[T] which wraps cbid.ID[T, ulid.ULID]. External provider IDs (strings from
// GitHub, GitLab, etc.) are stored as attributes via ExternalID, not as entity IDs.
//
// # Usage
//
//	itemID := types.NewItemID()
//	externalID := types.NewExternalID("1234567890")
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

type (
	// EventBrand distinguishes EventID from other identifier types.
	EventBrand struct{}
	// ItemBrand distinguishes ItemID from other identifier types.
	ItemBrand struct{}
	// ExternalBrand distinguishes ExternalID from other identifier types.
	ExternalBrand struct{}
	// ProviderBrand distinguishes ProviderID from other identifier types.
	ProviderBrand struct{}
	// ActorBrand distinguishes ActorID from other identifier types.
	ActorBrand struct{}
	// RepoBrand distinguishes RepoID from other identifier types.
	RepoBrand struct{}
	// EventTypeBrand distinguishes EventTypeID from other identifier types.
	EventTypeBrand struct{}
)

type (
	// EventID is the internal database identifier for events using ULID.
	// Example: "01H0G0K1P1V2J3M4N5O6P7Q8R9" (ULID, time-sortable unique identifier).
	EventID = id.ID[EventBrand, ulid.ULID]
	// ItemID is the internal ULID-based identifier for sync items.
	// Aligned with go-cqrs-lite's id.Of[T] which uses ULID-only identifiers.
	ItemID = id.ID[ItemBrand, ulid.ULID]
	// ExternalID is the provider-specific item identifier used for upsert operations.
	// Stores the original string ID from external providers (e.g., GitHub event "1234567890").
	ExternalID = id.ID[ExternalBrand, string]
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

// NewItemID creates a new ItemID with a freshly generated ULID.
func NewItemID() ItemID {
	return id.NewID[ItemBrand](ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader))
}

// MustParseItemID parses a ULID string into an ItemID. Panics on invalid input.
func MustParseItemID(s string) ItemID {
	return id.NewID[ItemBrand](ulid.MustParse(s))
}

// NewExternalID creates a new ExternalID from a string value.
func NewExternalID(v string) ExternalID { return id.NewID[ExternalBrand](v) }

// NewProviderID creates a new ProviderID from a string value.
func NewProviderID(v string) ProviderID { return id.NewID[ProviderBrand](v) }

// NewActorID creates a new ActorID from a string value.
func NewActorID(v string) ActorID { return id.NewID[ActorBrand](v) }

// NewRepoID creates a new RepoID from a string value.
func NewRepoID(v string) RepoID { return id.NewID[RepoBrand](v) }

// NewEventTypeID creates a new EventTypeID from a string value.
func NewEventTypeID(v string) EventTypeID { return id.NewID[EventTypeBrand](v) }
