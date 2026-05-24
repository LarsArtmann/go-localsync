// Package localsync provides generic synchronization primitives for building
// local-first and distributed applications with event sourcing.
//
// This package offers transport-agnostic building blocks:
//
//   - [VectorClock] for causal ordering across distributed nodes
//   - [Operation] for representing typed sync operations with generic payloads
//   - [ConflictResolver] and [LWWResolver] for pluggable conflict resolution
//
// The types in this package are domain-agnostic and have zero external dependencies
// beyond the Go standard library.
//
// # Quick Start
//
//	import "github.com/larsartmann/go-localsync/pkg/localsync"
//
//	vc := localsync.NewVectorClock()
//	vc.Increment(localsync.NodeID("node-1"))
//	vc.Increment(localsync.NodeID("node-2"))
//	vc.Clone()
//	vc.Cmp(otherVC)
//
//	resolver, err := localsync.NewLWWResolver[*MyEntity](func(e *MyEntity) time.Time {
//	    return e.UpdatedAt
//	})
//	if err != nil {
//	    return err
//	}
//	winner, err := resolver.Resolve(&localsync.Conflict[*MyEntity]{...})
package localsync
