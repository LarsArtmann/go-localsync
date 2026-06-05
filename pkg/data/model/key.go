package model

import (
	"fmt"

	"github.com/larsartmann/go-localsync/pkg/id"
)

// Key is the composite identifier for an Item.
// It makes the (Source, ExternalID) pairing explicit and type-safe.
// You cannot construct a Key without both fields, and you cannot
// look up an item by partial key.
type Key struct {
	Source     id.ProviderID
	ExternalID id.ExternalID
}

// String returns the canonical string representation: "source/externalId".
func (k Key) String() string {
	return fmt.Sprintf("%s/%s", k.Source.Get(), k.ExternalID.Get())
}

// IsZero reports whether this is the zero key.
func (k Key) IsZero() bool {
	return k.Source.IsZero() && k.ExternalID.IsZero()
}

// Equals reports whether two keys identify the same item.
func (k Key) Equals(other Key) bool {
	return k.Source == other.Source && k.ExternalID == other.ExternalID
}

// ItemKey creates a Key from an Item's identity fields.
func ItemKey(source id.ProviderID, externalID id.ExternalID) Key {
	return Key{Source: source, ExternalID: externalID}
}
