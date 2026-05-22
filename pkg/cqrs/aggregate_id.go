package cqrs

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// aggIDCache stores computed AggregateIDs to avoid repeated SHA256 hashing.
// Thread-safe via sync.Map. Intentionally global — cache persists across all CQRSStack instances.
//
//nolint:gochecknoglobals
var aggIDCache sync.Map

// itemKey returns the composite key for a source+externalID pair.
// Used for map lookups and aggregate ID generation.
func itemKey(source string, externalID types.ExternalID) string {
	return source + ":" + externalID.Get()
}

// AggregateID returns a deterministic AggregateID derived from (source, externalID).
// Same inputs always produce the same ID. Thread-safe with sync.Map caching.
func AggregateID(source string, externalID types.ExternalID) id.AggregateID {
	key := itemKey(source, externalID)

	if cached, ok := aggIDCache.Load(key); ok {
		return cached.(id.AggregateID) //nolint:forcetypeassert // sync.Map stores our exact type
	}

	h := sha256.Sum256([]byte(key))
	aggID := id.MustParseAggregateID(hex.EncodeToString(h[:16]))
	aggIDCache.Store(key, aggID)

	return aggID
}
