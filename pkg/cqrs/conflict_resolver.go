package cqrs

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// NewUpdatedAtLWWResolver creates a Last-Writer-Wins conflict resolver
// that compares items by their UpdatedAt timestamp.
//
// This is the canonical conflict resolver for items that track a single
// last-modified time. Use it directly with CQRSConfig.ConflictResolver:
//
//	stack, _ := cqrs.NewCQRSStack(cqrs.CQRSConfig{
//	    ConflictResolver: cqrs.NewUpdatedAtLWWResolver(),
//	})
//
// The helper exists so callers (tests, examples, production wiring) share
// one definition of "resolve by UpdatedAt" rather than restating the
// closure body in each place.
func NewUpdatedAtLWWResolver() *crdt.LWWResolver[*model.Item] {
	// crdt.NewLWWResolver only errors when given a nil extractor; we pass a
	// non-nil closure here so the error cannot occur.
	resolver, _ := crdt.NewLWWResolver(func(item *model.Item) time.Time {
		return item.UpdatedAt
	})

	return resolver
}
