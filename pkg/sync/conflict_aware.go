package sync

import (
	"context"
)

type ConflictAwareSyncer struct {
	*Syncer
}

func NewConflictAwareSyncer(base *Syncer) *ConflictAwareSyncer {
	return &ConflictAwareSyncer{
		Syncer: base,
	}
}

type ConflictResult struct {
	Fetched   int
	Upserted  int
	Skipped   int
	Conflicts int
	Errors    int
}

func (s *ConflictAwareSyncer) SyncWithConflictDetection(
	ctx context.Context,
	opts *SyncOptions,
) (*ConflictResult, error) {
	err := s.validateOpts(opts)
	if err != nil {
		return nil, err
	}

	result, err := s.fetchItems(ctx, opts, "Starting conflict-aware sync", "conflict-aware sync")
	if err != nil {
		return nil, err
	}

	cr := &ConflictResult{Fetched: len(result.Items)}

	validationResult := &SyncResult{Errors: 0}
	valid := s.filterValidItems(result.Items, validationResult)
	cr.Errors += validationResult.Errors

	if len(valid) == 0 {
		return cr, nil
	}

	summary := s.store.SyncItems(ctx, valid)

	for _, r := range summary.Results {
		switch r.Action {
		case ActionCreated:
			cr.Upserted++
		case ActionUpdated:
			cr.Upserted++
		case ActionConflictRemote:
			cr.Conflicts++
			cr.Upserted++

			s.logger.Debug("Resolved conflict: remote wins", "sourceID", r.SourceID)
		case ActionUnchanged:
			cr.Skipped++
		case ActionError:
			cr.Errors++

			s.logger.Warn("Failed to sync item", "sourceID", r.SourceID, "error", r.Error)
		}
	}

	s.logger.Info(
		"Conflict-aware sync completed",
		"fetched", cr.Fetched,
		"upserted", cr.Upserted,
		"conflicts", cr.Conflicts,
		"skipped", cr.Skipped,
		"errors", cr.Errors,
	)

	return cr, nil
}
