package cqrs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"

	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// itemKey returns the composite key for a source+sourceID pair.
// Uses length-prefixed encoding to prevent delimiter-injection collisions
// (e.g., source="github:" + sourceID="42" vs source="github" + sourceID=":42").
func itemKey(source string, sourceID id.SourceID) string {
	return strconv.Itoa(len(source)) + ":" + source + sourceID.Get()
}

// streamIDCache caches deterministic StreamID computations to avoid
// repeated SHA256 hashing of the same (source, sourceID) pairs.
// Keys are immutable so cache entries never need invalidation.
//
//nolint:gochecknoglobals // immutable append-only cache, not mutable global state
var streamIDCache sync.Map

// StreamID returns a deterministic stream ID derived from (source, sourceID).
// Same inputs always produce the same ID. Results are cached for O(1) repeat lookups.
//
// DELIBERATE DIVERGENCE from cqrsid.DeriveStreamID (see ADR-0009): this uses
// length-prefixed SHA256 truncated to 16 bytes (hex-32); the library's uses
// NUL-separated full SHA256. Encodings are incompatible — switching would
// orphan every stored stream. Keep this encoding for existing streams.
func StreamID(source string, sourceID id.SourceID) (cqrsid.StreamID, error) {
	key := itemKey(source, sourceID)

	if cached, ok := streamIDCache.Load(key); ok {
		return cached.(cqrsid.StreamID), nil //nolint:forcetypeassert // type is always cqrsid.StreamID
	}

	h := sha256.Sum256([]byte(key))
	hexID := hex.EncodeToString(h[:16])

	result, err := cqrsid.ParseStreamID(hexID)
	if err != nil {
		// ParseStreamID only fails on empty input. A SHA256 hex of 16
		// bytes is always 32 chars, so this is unreachable for any real
		// input — but the error return keeps the signature honest instead
		// of panicking (v0.6 conversion per ADR-0009).
		return "", pkgerrors.Wrapf(
			pkgerrors.ErrInvalidInput,
			"cqrs: derive stream ID: ParseStreamID failed for valid hex %q: %v",
			hexID,
			err,
		)
	}

	streamIDCache.Store(key, result)

	return result, nil
}

// MustStreamID is StreamID for inputs whose error path is unreachable (the
// derived key is always non-empty, so the SHA256 hex always parses). Intended
// for tests and tooling where a derivation failure is a programming error.
func MustStreamID(source string, sourceID id.SourceID) cqrsid.StreamID {
	streamID, err := StreamID(source, sourceID)
	if err != nil {
		panic(err) //nolint:goerr113 // Must* contract: unreachable for valid inputs
	}

	return streamID
}

// Deprecated: use StreamID (ADR-0009 vocabulary alignment), which returns an
// error instead of panicking on the (unreachable) derivation failure. Alias
// kept for one minor cycle; removed in the next breaking window.
func AggregateID(source string, externalID id.ExternalID) cqrsid.StreamID {
	streamID, err := StreamID(source, externalID)
	if err != nil {
		// Preserves the pre-v0.6 fail-fast contract of this deprecated shim.
		panic(fmt.Sprintf("cqrs: AggregateID: %v", err)) //nolint:goerr113 // deprecated shim
	}

	return streamID
}
