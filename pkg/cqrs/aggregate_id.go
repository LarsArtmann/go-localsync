package cqrs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"

	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v3"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// itemKey returns the composite key for a source+externalID pair.
// Uses length-prefixed encoding to prevent delimiter-injection collisions
// (e.g., source="github:" + externalID="42" vs source="github" + externalID=":42").
func itemKey(source string, externalID id.ExternalID) string {
	return strconv.Itoa(len(source)) + ":" + source + externalID.Get()
}

// aggIDCache caches deterministic AggregateID computations to avoid
// repeated SHA256 hashing of the same (source, externalID) pairs.
// Keys are immutable so cache entries never need invalidation.
//
//nolint:gochecknoglobals // immutable append-only cache, not mutable global state
var aggIDCache sync.Map

// AggregateID returns a deterministic AggregateID derived from (source, externalID).
// Same inputs always produce the same ID. Results are cached for O(1) repeat lookups.
func AggregateID(source string, externalID id.ExternalID) cqrsid.AggregateID {
	key := itemKey(source, externalID)

	if cached, ok := aggIDCache.Load(key); ok {
		return cached.(cqrsid.AggregateID) //nolint:forcetypeassert // type is always cqrsid.AggregateID
	}

	h := sha256.Sum256([]byte(key))
	hexID := hex.EncodeToString(h[:16])

	result, err := cqrsid.ParseAggregateID(hexID)
	if err != nil {
		// ParseAggregateID only fails on empty input. A SHA256 hex of 16
		// bytes is always 32 chars, so this is unreachable — fail fast
		// rather than caching a zero AggregateID that would collide every
		// subsequent call for different keys.
		panic(fmt.Sprintf("cqrs: ParseAggregateID failed for valid hex %q: %v", hexID, err))
	}

	aggIDCache.Store(key, result)

	return result
}
