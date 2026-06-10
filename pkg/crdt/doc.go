// Package crdt provides generic synchronization primitives for building
// local-first and distributed applications with event sourcing.
//
// This package offers transport-agnostic building blocks:
//
// # Active Types (used in production)
//
// These types are wired into the sync pipeline and exercised in production:
//
//   - [Conflict] represents a conflict between local and remote versions of an entity
//   - [ConflictResolver] is the interface for pluggable conflict resolution strategies
//   - [LWWResolver] implements Last-Writer-Wins resolution using a timestamp extractor
//
// # Future Types (planned for multi-node sync)
//
// These types are fully implemented and tested but not yet wired into the sync pipeline.
// They are designed for a future multi-node synchronization story where nodes exchange
// operations and need causal ordering:
//
//   - [VectorClock] for causal ordering across distributed nodes
//   - [Operation] for representing typed sync operations with generic payloads
//   - [SyncMessage], [SyncRequest], [SyncResponse] for the sync protocol wire format
//   - [NodeID], [OperationID] for identifying sync participants and their operations
//
// Currently, conflict resolution in the CQRS decider creates empty vector clocks
// ([crdt.NewVectorClock]) and the LWW resolver falls through to timestamp comparison.
// When multi-node sync is implemented, vector clocks will be populated with actual
// causal state and the Operation type will be used to exchange deltas between nodes.
//
// # Quick Start
//
//	resolver, err := crdt.NewLWWResolver[*MyEntity](func(e *MyEntity) time.Time {
//	    return e.UpdatedAt
//	})
//	if err != nil {
//	    return err
//	}
//	winner, err := resolver.Resolve(&crdt.Conflict[*MyEntity]{...})
package crdt
