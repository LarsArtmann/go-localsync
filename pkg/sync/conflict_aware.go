package sync

import (
	"context"
)

// ConflictAwareSyncer adds per-item conflict detection on top of a Syncer.
// It delegates fetching and validation to the base Syncer, then classifies
// conflict outcomes from the CQRS decider's BatchOutcome.
type ConflictAwareSyncer struct {
	syncer *Syncer
}

// NewConflictAwareSyncer creates a ConflictAwareSyncer that delegates to the base Syncer.
func NewConflictAwareSyncer(base *Syncer) *ConflictAwareSyncer {
	return &ConflictAwareSyncer{
		syncer: base,
	}
}

// Close delegates to the underlying Syncer's Close.
func (s *ConflictAwareSyncer) Close() error {
	return s.syncer.Close()
}

// ConflictResult holds the result of a conflict-aware sync operation. Its
// surface mirrors SyncResult so the two sync paths report the same data:
// per-item errors are retained in ItemErrors, and Tombstoned reflects the
// optional reconciliation pass (run when SyncOptions.Reconcile is set).
type ConflictResult struct {
	Fetched    int
	Upserted   int
	Skipped    int
	Conflicts  int
	Tombstoned int
	Errors     int
	ItemErrors []ItemSyncResult
}

// SyncWithConflictDetection fetches items and syncs them, tracking conflicts separately.
func (s *ConflictAwareSyncer) SyncWithConflictDetection(
	ctx context.Context,
	opts *SyncOptions,
) (*ConflictResult, error) {
	release, err := s.syncer.lockAndValidate(opts)
	if err != nil {
		return nil, err
	}

	// Acquire the per-source lock so two concurrent conflict-aware syncs for the
	// same source can't interleave a fetch+store window (the same TOCTOU guard
	// the base Syncer uses). Different sources still run in parallel.
	defer release()

	result, err := s.syncer.fetchItems(ctx, opts, "Starting conflict-aware sync", "conflict-aware sync")
	if err != nil {
		return nil, err
	}

	cr := &ConflictResult{Fetched: len(result.Items)}

	validationResult := &SyncResult{Errors: 0}
	valid := s.syncer.filterValidItems(result.Items, validationResult)
	cr.Errors += validationResult.Errors

	if len(valid) == 0 {
		return cr, partialSyncError(cr.Errors, len(result.Items))
	}

	batch := s.syncer.store.SyncItems(ctx, valid)
	s.classify(batch, cr)

	// Reuse the guarded reconciliation helper (refuses incomplete fetches) so the
	// conflict-aware path has the same upstream-gone detection as the base Syncer.
	reconcileResult := &SyncResult{}
	s.syncer.reconcile(ctx, opts, result, reconcileResult)
	cr.Tombstoned = reconcileResult.Tombstoned

	s.syncer.logger.Info(
		"Conflict-aware sync completed",
		"source", opts.Source,
		"fetched", cr.Fetched,
		"upserted", cr.Upserted,
		"conflicts", cr.Conflicts,
		"skipped", cr.Skipped,
		"tombstoned", cr.Tombstoned,
		"errors", cr.Errors,
	)

	return cr, partialSyncError(cr.Errors, len(result.Items))
}

// classify folds a BatchOutcome into a ConflictResult, counting upserts,
// conflicts, skips, and errors while retaining per-item error detail.
func (s *ConflictAwareSyncer) classify(batch *BatchOutcome, cr *ConflictResult) {
	for _, r := range batch.Results {
		switch r.Action {
		case ActionCreated:
			cr.Upserted++
		case ActionUpdated:
			cr.Upserted++
		case ActionConflictRemote:
			cr.Conflicts++
			cr.Upserted++

			s.syncer.logger.Debug("Resolved conflict: remote wins", "sourceID", r.SourceID)
		case ActionConflictLocal:
			cr.Conflicts++

			s.syncer.logger.Debug("Resolved conflict: local wins", "sourceID", r.SourceID)
		case ActionUnchanged:
			cr.Skipped++
		case ActionTombstoned:
			// Never produced by SyncItems; only reconciliation tombstones, handled by reconcile().
		case ActionError:
			cr.Errors++
			cr.ItemErrors = append(cr.ItemErrors, r)

			s.syncer.logger.Warn("Failed to sync item", "sourceID", r.SourceID, "error", r.Error)
		}
	}
}
