// Package model provides canonical domain types for the data layer.
// These types are purpose-built for the sync domain and carry no
// provider-specific baggage (e.g., RawJSON lives in the provider DTO).
package model

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// Item is the canonical domain entity for a synced item.
// It is provider-agnostic — no RawJSON, no API concerns.
type Item struct {
	ID             id.ItemID
	ExternalID     id.ExternalID
	Source         id.ProviderID
	Type           id.EventTypeID
	ActorLogin     id.ActorID
	ActorAvatarURL string
	RepoName       id.RepoID
	RepoURL        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	SchemaVersion  schema.Version
}

// Key returns the composite identifier for this item.
// The composite key (Source, ExternalID) is the stable identity
// across all systems — it determines the aggregate ID.
func (item Item) Key() Key {
	return Key{Source: item.Source, ExternalID: item.ExternalID}
}

// IsZero reports whether the item is the zero value.
func (item Item) IsZero() bool {
	return item.ExternalID.IsZero() && item.Source.IsZero()
}

// GetSource returns the source for criterion matching.
func (item Item) GetSource() id.ProviderID { return item.Source }

// GetType returns the type for criterion matching.
func (item Item) GetType() id.EventTypeID { return item.Type }

// GetActorLogin returns the actor for criterion matching.
func (item Item) GetActorLogin() id.ActorID { return item.ActorLogin }

// GetRepoName returns the repo for criterion matching.
func (item Item) GetRepoName() id.RepoID { return item.RepoName }

// GetCreatedAt returns created_at for criterion matching.
func (item Item) GetCreatedAt() time.Time { return item.CreatedAt }

// GetUpdatedAt returns updated_at for criterion matching.
func (item Item) GetUpdatedAt() time.Time { return item.UpdatedAt }

// ProviderItem is the DTO returned by provider implementations.
// It carries the raw payload for full-fidelity storage.
type ProviderItem struct {
	ExternalID     id.ExternalID
	Source         id.ProviderID
	Type           id.EventTypeID
	ActorLogin     id.ActorID
	ActorAvatarURL string
	RepoName       id.RepoID
	RepoURL        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RawPayload     []byte
}

// Validate checks that all required identity fields are present.
func (item Item) Validate() error {
	if item.ExternalID.IsZero() {
		return errMissingExternalID
	}

	if item.Source.IsZero() {
		return errMissingSource
	}

	if item.Type.IsZero() {
		return errMissingType
	}

	if item.CreatedAt.IsZero() {
		return errMissingCreatedAt
	}

	return nil
}

// Validate checks that all required identity fields are present.
func (p ProviderItem) Validate() error {
	if p.ExternalID.IsZero() {
		return errMissingExternalID
	}

	if p.Source.IsZero() {
		return errMissingSource
	}

	if p.Type.IsZero() {
		return errMissingType
	}

	if p.CreatedAt.IsZero() {
		return errMissingCreatedAt
	}

	return nil
}
