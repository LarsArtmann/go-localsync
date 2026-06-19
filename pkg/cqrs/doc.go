// Package cqrs implements the event-sourced CQRS storage layer for go-localsync.
//
// It combines go-cqrs-lite decider, projection, and snapshot primitives with a
// SyncStore adapter (see stack_adapters.go) that bridges CQRSStack to the sync
// package. Commands are dispatched through a typed command.Dispatcher; the
// read-side adapters call the ReadModel directly for hot-path performance.
package cqrs
