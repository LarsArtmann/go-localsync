// Package model provides canonical domain types for the data layer.
// These types are purpose-built for the sync domain and carry no
// provider-specific baggage (e.g., RawJSON lives in the provider DTO).
package model

import (
	"context"
	"errors"
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
	ActorLogin     id.ActorLogin
	ActorAvatarURL string
	RepoName       id.RepoID
	RepoURL        string
	ContentHash    string
	Tombstone      Tombstone
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

// IsTombstoned reports whether the item is hidden from the default read model.
func (item Item) IsTombstoned() bool {
	return !item.Tombstone.IsZero()
}

// ItemReader is the read-side contract for any storage backend that can
// list, count, and enumerate item types. Both the CQRS ReadModel and the
// sync.SyncStore embed this interface so the read-side method signatures
// are declared exactly once in a shared, import-safe package.
type ItemReader interface {
	List(ctx context.Context, filter ItemFilter) ([]*Item, error)
	Count(ctx context.Context, filter ItemFilter) (int64, error)
	CountByType(ctx context.Context, filter ItemFilter) (map[string]int64, error)
	GetTypes(ctx context.Context) ([]string, error)
}

// Validate checks that all required identity fields are present.
// All field errors are collected and returned together via errors.Join
// so callers see every problem in a single call instead of fixing
// them one at a time.
func (item Item) Validate() error {
	return validateIdentity(item.ExternalID, item.Source, item.Type, item.CreatedAt, item.UpdatedAt)
}

func validateIdentity(
	externalID id.ExternalID,
	source id.ProviderID,
	eventType id.EventTypeID,
	createdAt time.Time,
	updatedAt time.Time,
) error {
	var errs []error

	if externalID.IsZero() {
		errs = append(errs, errMissingExternalID)
	}

	if source.IsZero() {
		errs = append(errs, errMissingSource)
	}

	if eventType.IsZero() {
		errs = append(errs, errMissingType)
	}

	if createdAt.IsZero() {
		errs = append(errs, errMissingCreatedAt)
	}

	if updatedAt.IsZero() {
		errs = append(errs, errMissingUpdatedAt)
	}

	return errors.Join(errs...)
}
