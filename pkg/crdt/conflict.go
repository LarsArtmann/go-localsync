// Package crdt holds the conflict-resolution strategy for the sync pipeline.
//
// Despite the historical name, this package carries no CRDT or vector clock
// machinery: go-localsync is a single-writer pull mirror, so there is no second
// writer and no causal ordering to track. What remains is the genuinely used
// conflict surface: [Conflict], [ConflictResolver], and the timestamp-based
// [LWWResolver].
package crdt

import "time"

// Conflict represents a synchronization conflict between a local and a remote
// version of an entity. T is the entity type being synced (e.g. *model.Item).
//
// Resolution needs no causal metadata here: the provider is the sole writer per
// aggregate, so the only question is which version to keep, decided by a
// ConflictResolver (timestamp-based by default).
type Conflict[T any] struct {
	Local     T         `json:"local"`
	Remote    T         `json:"remote"`
	Timestamp time.Time `json:"timestamp"`
}

// ConflictResolver determines the winner when a sync conflict occurs.
// Implementations choose a strategy: last-write-wins, custom merge, etc.
type ConflictResolver[T any] interface {
	// Resolve returns the winning value for the conflict.
	Resolve(conflict *Conflict[T]) (T, error)
}

// LWWResolver resolves conflicts with Last-Write-Wins: the entity whose
// timestamp (extracted via TimestampFunc) is newer wins. Ties fall back to the
// remote side, or to Tiebreaker when set.
//
// T is the entity type. TimestampFunc extracts the comparison timestamp.
type LWWResolver[T any] struct {
	// TimestampFunc extracts the comparison timestamp from an entity.
	TimestampFunc func(T) time.Time
	// Tiebreaker is called when timestamps are equal. Returns true if local wins.
	// If nil, remote wins on tie.
	Tiebreaker func(local, remote T) bool
}

// NewLWWResolver creates a Last-Write-Wins resolver with the given timestamp
// extractor. Returns ErrNilTimestampFunc if timestampFunc is nil.
func NewLWWResolver[T any](timestampFunc func(T) time.Time) (*LWWResolver[T], error) {
	if timestampFunc == nil {
		return nil, ErrNilTimestampFunc
	}

	return &LWWResolver[T]{
		TimestampFunc: timestampFunc,
		Tiebreaker:    nil,
	}, nil
}

// Resolve implements ConflictResolver by comparing timestamps. The newer side
// wins; on a tie the Tiebreaker decides (defaulting to remote).
func (r *LWWResolver[T]) Resolve(conflict *Conflict[T]) (T, error) {
	localTime := r.TimestampFunc(conflict.Local)
	remoteTime := r.TimestampFunc(conflict.Remote)

	if localTime.After(remoteTime) {
		return conflict.Local, nil
	}

	if remoteTime.After(localTime) {
		return conflict.Remote, nil
	}

	if r.Tiebreaker != nil && r.Tiebreaker(conflict.Local, conflict.Remote) {
		return conflict.Local, nil
	}

	return conflict.Remote, nil
}
