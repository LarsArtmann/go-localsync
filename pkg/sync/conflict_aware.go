package sync

import (
	"context"
)

// ConflictAwareSyncer adds per-item conflict detection on top of a Syncer.
// It delegates fetching and validation to the base Syncer, then classifies
// conflict outcomes from the CQRS decider's SyncSummary.
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

// ConflictResult holds the result of a conflict-aware sync operation.
type ConflictResult struct {
	Fetched   int
	Upserted  int
	Skipped   int
	Conflicts int
	Errors    int
}

// SyncWithConflictDetection fetches items and syncs them, tracking conflicts separately.
func (s *ConflictAwareSyncer) SyncWithConflictDetection(
	ctx context.Context,
	opts *SyncOptions,
) (*ConflictResult, error) {
	err := s.syncer.validateOpts(opts)
	if err != nil {
		return nil, err
	}

	result, err := s.syncer.fetchItems(ctx, opts, "Starting conflict-aware sync", "conflict-aware sync")
	if err != nil {
		return nil, err
	}

	cr := &ConflictResult{Fetched: len(result.Items)}

	validationResult := &SyncResult{Errors: 0}
	valid := s.syncer.filterValidItems(result.Items, validationResult)
	cr.Errors += validationResult.Errors

	if len(valid) == 0 {
		return cr, nil
	}

	summary := s.syncer.store.SyncItems(ctx, valid)

	for _, r := range summary.Results {
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
			// Never produced by SyncItems; reconciliation tombstones outside this path.
		case ActionError:
			cr.Errors++

			s.syncer.logger.Warn("Failed to sync item", "sourceID", r.SourceID, "error", r.Error)
		}
	}

	s.syncer.logger.Info(
		"Conflict-aware sync completed",
		"fetched", cr.Fetched,
		"upserted", cr.Upserted,
		"conflicts", cr.Conflicts,
		"skipped", cr.Skipped,
		"errors", cr.Errors,
	)

	return cr, nil
}
