package cqrs

import (
	"crypto/sha256"
	"sync"

	"github.com/larsartmann/go-cqrs-lite/core/pkg/id"
	"github.com/oklog/ulid/v2"
)

var aggIDCache sync.Map

// AggregateID returns a deterministic AggregateID derived from (source, sourceID).
// Same inputs always produce the same ULID. Thread-safe with sync.Map caching.
func AggregateID(source, sourceID string) id.AggregateID {
	key := source + ":" + sourceID

	if cached, ok := aggIDCache.Load(key); ok {
		return cached.(id.AggregateID) //nolint:forcetypeassert // sync.Map stores our exact type
	}

	h := sha256.Sum256([]byte(key))
	var entropy [10]byte
	copy(entropy[:], h[:10])

	u := ulid.MustNew(1, &deterministicReader{data: entropy[:]})

	aggID := id.MustParseAggregateID(u.String())
	aggIDCache.Store(key, aggID)

	return aggID
}

type deterministicReader struct {
	data []byte
	pos  int
}

func (r *deterministicReader) Read(p []byte) (int, error) {
	n := copy(p, r.data[r.pos:])
	r.pos += n

	return n, nil
}
