package cqrs

import (
	"crypto/sha256"
	"encoding/hex"

	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v2"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// itemKey returns the composite key for a source+externalID pair.
func itemKey(source string, externalID id.ExternalID) string {
	return source + ":" + externalID.Get()
}

// AggregateID returns a deterministic AggregateID derived from (source, externalID).
// Same inputs always produce the same ID.
func AggregateID(source string, externalID id.ExternalID) cqrsid.AggregateID {
	key := itemKey(source, externalID)
	h := sha256.Sum256([]byte(key))

	return cqrsid.MustParseAggregateID(hex.EncodeToString(h[:16]))
}
