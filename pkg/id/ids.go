// Package id provides domain-specific identifier types using branded IDs
// from go-branded-id. These types provide compile-time safety by preventing
// accidental mixing of different identifier kinds.
//
// All entity identifiers use ULID as the value type, aligning with go-cqrs-lite's
// id.Of[T] which wraps cbid.ID[T, ulid.ULID]. External provider IDs (strings from
// GitHub, GitLab, etc.) are stored as SourceID, not as entity IDs.
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
	// SourceBrand distinguishes SourceID from other identifier types.
	SourceBrand struct{}
	// ProviderBrand distinguishes ProviderID from other identifier types.
	ProviderBrand struct{}
	// EventTypeBrand distinguishes EventTypeID from other identifier types.
	EventTypeBrand struct{}
)

type (
	// ItemID is the internal ULID-based identifier for sync items.
	// Aligned with go-cqrs-lite's id.Of[T] which uses ULID-only identifiers.
	ItemID = brandid.ID[ItemBrand, ulid.ULID]
	// SourceID is the provider's own key for an item (e.g. a GitHub event ID),
	// used for upsert operations and deterministic stream derivation. It is the
	// v0.6 vocabulary-aligned name (ADR-0009): event payloads already persist
	// this value as "sourceId".
	SourceID = brandid.ID[SourceBrand, string]
	// ProviderID identifies the source provider.
	// Example: "github", "gitlab".
	ProviderID = brandid.ID[ProviderBrand, string]
	// EventTypeID identifies the type of event.
	// Example: "PushEvent", "CreateEvent".
	EventTypeID = brandid.ID[EventTypeBrand, string]
)

// ExternalBrand is the former name of SourceBrand.
//
// Deprecated: use SourceBrand (ADR-0009 vocabulary alignment). Type alias kept
// for one minor cycle so pre-v0.6 code compiles unchanged.
type ExternalBrand = SourceBrand

// ExternalID is the former name of SourceID. It is a true type alias, so
// existing code compiles unchanged.
//
// Deprecated: use SourceID (ADR-0009 vocabulary alignment); removed in the
// next breaking window.
type ExternalID = SourceID

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

// NewSourceID creates a new SourceID from a string value.
func NewSourceID(v string) SourceID { return brandid.NewID[SourceBrand](v) }

// NewProviderID creates a new ProviderID from a string value.
func NewProviderID(v string) ProviderID { return brandid.NewID[ProviderBrand](v) }

// NewEventTypeID creates a new EventTypeID from a string value.
func NewEventTypeID(v string) EventTypeID { return brandid.NewID[EventTypeBrand](v) }

// NewExternalID is the former name of NewSourceID.
//
// Deprecated: use NewSourceID (ADR-0009 vocabulary alignment); removed in the
// next breaking window.
var NewExternalID = NewSourceID //nolint:gochecknoglobals // deprecated one-cycle alias, not mutable state
