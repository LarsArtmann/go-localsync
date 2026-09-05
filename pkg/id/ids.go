// Package id provides domain-specific identifier types using branded IDs
// from go-branded-id. These types provide compile-time safety by preventing
// accidental mixing of different identifier kinds.
//
// All entity identifiers use ULID as the value type, aligning with go-cqrs-lite's
// id.Of[T] which wraps cbid.ID[T, ulid.ULID]. External provider IDs (strings from
// GitHub, GitLab, etc.) are stored as attributes via ExternalID, not as entity IDs.
package id

import (
	"crypto/rand"
	"fmt"
	"time"

	brandid "github.com/larsartmann/go-branded-id"
	"github.com/oklog/ulid/v2"
)

type (
	// ItemBrand distinguishes ItemID from other identifier types.
	ItemBrand struct{}
	// ExternalBrand distinguishes ExternalID from other identifier types.
	ExternalBrand struct{}
	// ProviderBrand distinguishes ProviderID from other identifier types.
	ProviderBrand struct{}
	// EventTypeBrand distinguishes EventTypeID from other identifier types.
	EventTypeBrand struct{}
)

type (
	// ItemID is the internal ULID-based identifier for sync items.
	// Aligned with go-cqrs-lite's id.Of[T] which uses ULID-only identifiers.
	ItemID = brandid.ID[ItemBrand, ulid.ULID]
	// ExternalID is the provider-specific item identifier used for upsert operations.
	// Stores the original string ID from external providers (e.g., GitHub event "1234567890").
	ExternalID = brandid.ID[ExternalBrand, string]
	// ProviderID identifies the source provider.
	// Example: "github", "gitlab".
	ProviderID = brandid.ID[ProviderBrand, string]
	// EventTypeID identifies the type of event.
	// Example: "PushEvent", "CreateEvent".
	EventTypeID = brandid.ID[EventTypeBrand, string]
)

// NewItemID creates a new ItemID with a freshly generated ULID.
func NewItemID() ItemID {
	return brandid.NewID[ItemBrand](ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader))
}

// MustParseItemID parses a ULID string into an ItemID. Panics on invalid input.
func MustParseItemID(s string) ItemID {
	return brandid.NewID[ItemBrand](ulid.MustParse(s))
}

// ParseItemID parses a ULID string into an ItemID. Returns error on invalid input.
func ParseItemID(s string) (ItemID, error) {
	parsed, err := ulid.Parse(s)
	if err != nil {
		return ItemID{}, fmt.Errorf("parse item ID %q: %w", s, err)
	}

	return brandid.NewID[ItemBrand](parsed), nil
}

// NewExternalID creates a new ExternalID from a string value.
func NewExternalID(v string) ExternalID { return brandid.NewID[ExternalBrand](v) }

// NewProviderID creates a new ProviderID from a string value.
func NewProviderID(v string) ProviderID { return brandid.NewID[ProviderBrand](v) }

// NewEventTypeID creates a new EventTypeID from a string value.
func NewEventTypeID(v string) EventTypeID { return brandid.NewID[EventTypeBrand](v) }

// ContentHash is the SHA-256 hex digest of an item's raw provider payload.
// A named string type (not a struct): literal assignment and comparison keep
// compiling, while function signatures gain compile-time protection against
// mixing hashes with arbitrary strings.
type ContentHash string

// NewContentHash brands a SHA-256 hex string as a ContentHash.
func NewContentHash(hex string) ContentHash { return ContentHash(hex) }

// IsZero reports whether the hash is empty (provider set no raw payload).
func (h ContentHash) IsZero() bool { return h == "" }

// String returns the underlying hex digest.
func (h ContentHash) String() string { return string(h) }
