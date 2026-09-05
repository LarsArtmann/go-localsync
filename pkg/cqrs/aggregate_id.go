package cqrs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"

	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
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

// AggregateID returns a deterministic stream ID derived from (source, externalID).
// Same inputs always produce the same ID. Results are cached for O(1) repeat lookups.
//
// DELIBERATE DIVERGENCE from cqrsid.DeriveStreamID (see ADR-0009): this uses
// length-prefixed SHA256 truncated to 16 bytes (hex-32); the library's uses
// NUL-separated full SHA256. Encodings are incompatible — switching would
// orphan every stored stream. Keep this encoding for existing streams; the
// rename to StreamID() is planned for v0.6 together with ADR-0009.
func AggregateID(source string, externalID id.ExternalID) cqrsid.StreamID {
	key := itemKey(source, externalID)

	if cached, ok := aggIDCache.Load(key); ok {
		return cached.(cqrsid.StreamID) //nolint:forcetypeassert // type is always cqrsid.StreamID
	}

	h := sha256.Sum256([]byte(key))
	hexID := hex.EncodeToString(h[:16])

	result, err := cqrsid.ParseStreamID(hexID)
	if err != nil {
		// ParseStreamID only fails on empty input. A SHA256 hex of 16
		// bytes is always 32 chars, so this is unreachable — fail fast
		// rather than caching a zero StreamID that would collide every
		// subsequent call for different keys.
		panic(fmt.Sprintf("cqrs: ParseStreamID failed for valid hex %q: %v", hexID, err))
	}

	aggIDCache.Store(key, result)

	return result
}
