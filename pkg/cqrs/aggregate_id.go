package cqrs

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// itemKey returns the composite key for a source+externalID pair.
func itemKey(source string, externalID types.ExternalID) string {
	return source + ":" + externalID.Get()
}

// AggregateID returns a deterministic AggregateID derived from (source, externalID).
// Same inputs always produce the same ID.
func AggregateID(source string, externalID types.ExternalID) id.AggregateID {
	key := itemKey(source, externalID)
	h := sha256.Sum256([]byte(key))

	return id.MustParseAggregateID(hex.EncodeToString(h[:16]))
}
