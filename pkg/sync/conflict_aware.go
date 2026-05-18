package sync

import (
	"context"

	"github.com/larsartmann/go-localsync/pkg/cqrs"
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

	valid := s.filterValidItems(result.Items, &SyncResult{Errors: 0})

	if len(valid) == 0 {
		return cr, nil
	}

	summary := s.stack.SyncItems(ctx, valid)

	for _, r := range summary.Results {
		switch r.Action {
		case cqrs.ActionCreated:
			cr.Upserted++
		case cqrs.ActionUpdated:
			cr.Upserted++
		case cqrs.ActionConflictRemote:
			cr.Conflicts++
			cr.Upserted++

			s.logger.Debug("Resolved conflict: remote wins", "sourceID", r.SourceID)
		case cqrs.ActionUnchanged:
			cr.Skipped++
		case cqrs.ActionError:
			cr.Errors++

			s.logger.Warn("Failed to sync item", "sourceID", r.SourceID, "error", r.Error)
		}
	}

	s.logger.Info("Conflict-aware sync completed",
		"fetched", cr.Fetched,
		"upserted", cr.Upserted,
		"conflicts", cr.Conflicts,
		"skipped", cr.Skipped,
		"errors", cr.Errors,
	)

	return cr, nil
}
