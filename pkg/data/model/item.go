// Package model provides canonical domain types for the data layer.
// These types are purpose-built for the sync domain and carry no
// provider-specific baggage (e.g., RawJSON lives in the provider DTO).
package model

import (
	"context"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
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

// ItemReader is the read-side contract for any storage backend that can
// list, count, and enumerate item types. Both the CQRS ReadModel and the
// sync.SyncStore embed this interface so the read-side method signatures
// are declared exactly once in a shared, import-safe package.
type ItemReader interface {
	List(ctx context.Context, filter provider.ItemFilter) ([]*Item, error)
	Count(ctx context.Context, filter provider.ItemFilter) (int64, error)
	GetTypes(ctx context.Context) ([]string, error)
}

// Validate checks that all required identity fields are present.
func (item Item) Validate() error {
	return validateIdentity(item.ExternalID, item.Source, item.Type, item.CreatedAt)
}

func validateIdentity(
	externalID id.ExternalID,
	source id.ProviderID,
	eventType id.EventTypeID,
	createdAt time.Time,
) error {
	switch {
	case externalID.IsZero():
		return errMissingExternalID
	case source.IsZero():
		return errMissingSource
	case eventType.IsZero():
		return errMissingType
	case createdAt.IsZero():
		return errMissingCreatedAt
	default:
		return nil
	}
}
