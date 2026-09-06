package cqrs

import (
	"context"

	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// Package cqrs dead-letter operations runbook (ADR-0006).
//
// Events that poison a projection handler past the retry threshold are
// captured, not dropped. The ops loop is list → replay → delete → purge:
//
//	// 1. Inspect what was captured (per projection, or "" for all).
//	entries, err := stack.DeadLetters(ctx, "sync_item_projection")
//
//	// 2. Re-deliver everything for that projection.
//	result, err := stack.ReplayDeadLetters(ctx, "sync_item_projection")
//
//	// 3. Delete exactly the entries that now replay cleanly — replay does
//	//    NOT auto-delete, so the caller keeps surgical control.
//	for _, replayed := range result.Replayed {
//	    err := stack.DeleteDeadLetter(ctx, replayed.ProjectionName, replayed.EventID)
//	}
//
//	// 4. Purge is the blunt instrument: only when every entry is resolved.
//	err = stack.PurgeDeadLetters(ctx, "sync_item_projection")
//
// Still-failing entries stay captured and are reported in result.StillFailing;
// fix the underlying cause (usually a schema-upcast gap) and replay again.

// DeadLetters lists the projection dead-letter entries: events that poisoned a
// projection handler past the retry threshold and were captured for later
// inspection or replay. An empty projectionName lists entries across all
// projections. The DLQ persists across restarts on the sqlite backend
// (ADR-0006); on the memory backend its lifetime matches the store's.
func (s *CQRSStack) DeadLetters(ctx context.Context, projectionName string) ([]projectionhost.DeadLetterEntry, error) {
	if s.dlq == nil {
		return nil, pkgerrors.Wrapf(pkgerrors.ErrDatabase, "no dead-letter store configured")
	}

	entries, err := s.dlq.List(ctx, projectionName)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "list dead letters for projection %q", projectionName)
	}

	return entries, nil
}

// DeadLetterCount returns how many dead-letter entries are currently captured.
// It falls back to listing when the store does not expose a native Count, so
// it works with every DeadLetterStore implementation.
func (s *CQRSStack) DeadLetterCount(ctx context.Context) (int, error) {
	if admin, ok := s.dlq.(projectionhost.DeadLetterStoreAdmin); ok {
		n, err := admin.Count(ctx)
		if err != nil {
			return 0, pkgerrors.Wrapf(err, "count dead letters")
		}

		return int(n), nil
	}

	entries, err := s.DeadLetters(ctx, "")
	if err != nil {
		return 0, err
	}

	return len(entries), nil
}

// DeleteDeadLetter removes a single dead-letter entry after it replayed
// successfully (surgical cleanup that leaves still-failing entries in place).
func (s *CQRSStack) DeleteDeadLetter(ctx context.Context, projectionName, eventID string) error {
	if s.dlq == nil {
		return pkgerrors.Wrapf(pkgerrors.ErrDatabase, "no dead-letter store configured")
	}

	if err := s.dlq.Delete(ctx, projectionName, eventID); err != nil {
		return pkgerrors.Wrapf(err, "delete dead letter %s/%s", projectionName, eventID)
	}

	return nil
}

// PurgeDeadLetters removes ALL dead-letter entries for the given projection
// (empty name purges every projection). Use only when every entry has been
// resolved; prefer ReplayDeadLetters + DeleteDeadLetter for partial cleanup.
func (s *CQRSStack) PurgeDeadLetters(ctx context.Context, projectionName string) error {
	if s.dlq == nil {
		return pkgerrors.Wrapf(pkgerrors.ErrDatabase, "no dead-letter store configured")
	}

	if err := s.dlq.Purge(ctx, projectionName); err != nil {
		return pkgerrors.Wrapf(err, "purge dead letters for projection %q", projectionName)
	}

	return nil
}

// ReplayDeadLetters re-delivers every captured poison event for the given
// projection (empty name = all projections) to its handler. Events that still
// fail are reported in the result and remain in the DLQ. Successfully
// replayed events are NOT auto-deleted — the caller deletes exactly those via
// DeleteDeadLetter (see the runbook at the top of this file), keeping cleanup
// surgical and re-replayable.
func (s *CQRSStack) ReplayDeadLetters(
	ctx context.Context,
	projectionName string,
) (projectionhost.ReplayResult, error) {
	if s.projHost == nil {
		return projectionhost.ReplayResult{}, pkgerrors.Wrapf(
			pkgerrors.ErrDatabase,
			"no projection host running; replay unavailable",
		)
	}

	result, err := s.projHost.ReplayDeadLetters(ctx, projectionName)
	if err != nil {
		return projectionhost.ReplayResult{}, pkgerrors.Wrapf(
			err,
			"replay dead letters for projection %q",
			projectionName,
		)
	}

	return result, nil
}
