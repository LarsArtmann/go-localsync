// Package crdt provides generic synchronization primitives for building
// local-first and distributed applications with event sourcing.
//
// This package offers transport-agnostic building blocks:
//
//   - [VectorClock] for causal ordering across distributed nodes
//   - [Operation] for representing typed sync operations with generic payloads
//   - [ConflictResolver] and [LWWResolver] for pluggable conflict resolution
//
// The types in this package are domain-agnostic with minimal external dependencies
// (only github.com/larsartmann/go-error-family for error classification).
//
// # Quick Start
//
//	import "github.com/larsartmann/go-localsync/pkg/crdt"
//
//	vc := crdt.NewVectorClock()
//	vc.Increment(crdt.NodeID("node-1"))
//	vc.Increment(crdt.NodeID("node-2"))
//	vc.Clone()
//	vc.Cmp(otherVC)
//
//	resolver, err := crdt.NewLWWResolver[*MyEntity](func(e *MyEntity) time.Time {
//	    return e.UpdatedAt
//	})
//	if err != nil {
//	    return err
//	}
//	winner, err := resolver.Resolve(&crdt.Conflict[*MyEntity]{...})
package crdt
