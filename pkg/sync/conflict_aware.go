package sync

import (
	"context"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
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

func newConflictResult(fetched int) *ConflictResult {
	return &ConflictResult{
		Fetched:   fetched,
		Upserted:  0,
		Skipped:   0,
		Conflicts: 0,
		Errors:    0,
	}
}

func (s *ConflictAwareSyncer) SyncWithConflictDetection(
	ctx context.Context,
	opts *SyncOptions,
) (*ConflictResult, error) {
	if opts == nil {
		return nil, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	err := opts.Validate()
	if err != nil {
		return nil, err
	}

	s.logger.Info("Starting conflict-aware sync",
		"provider", s.provider.Name(),
		"source", opts.Source,
	)

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, pkgerrors.Wrapf(err,
			"conflict-aware sync failed for source %q (maxPages=%d)",
			opts.Source,
			opts.MaxPages,
		)
	}

	cr := newConflictResult(len(result.Items))

	for _, item := range result.Items {
		existing, _ := s.stack.ReadModel.Get(ctx, item.Source.Get(), item.ExternalID.Get())
		s.processItem(ctx, item, existing, cr)
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

func (s *ConflictAwareSyncer) logError(
	msg string,
	item *provider.Item,
	err error,
	cr *ConflictResult,
) {
	s.logger.Warn(msg, "id", item.ID, "error", err)

	cr.Errors++
}

func (s *ConflictAwareSyncer) processItem(
	ctx context.Context,
	item *provider.Item,
	existing *provider.Item,
	cr *ConflictResult,
) {
	err := item.Validate()
	if err != nil {
		s.logError("Invalid item", item, err, cr)

		return
	}

	if existing == nil {
		s.upsertNewItem(ctx, item, cr)

		return
	}

	if s.isConflict(existing, item) {
		s.resolveConflict(ctx, existing, item, cr)
	} else {
		cr.Skipped++
	}
}

func (s *ConflictAwareSyncer) upsertNewItem(
	ctx context.Context,
	item *provider.Item,
	cr *ConflictResult,
) {
	err := s.stack.SyncItem(ctx, item)
	if err != nil {
		s.logError("Failed to sync item", item, err, cr)

		return
	}

	cr.Upserted++
}

func (s *ConflictAwareSyncer) resolveConflict(
	ctx context.Context,
	local, remote *provider.Item,
	cr *ConflictResult,
) {
	if remote.UpdatedAt.After(local.UpdatedAt) {
		err := s.stack.SyncItem(ctx, remote)
		if err != nil {
			s.logError("Failed to sync resolved item", remote, err, cr)

			return
		}

		cr.Conflicts++
		cr.Upserted++

		s.logger.Debug("Resolved conflict: remote wins", "id", remote.ID)
	} else {
		cr.Conflicts++
		cr.Skipped++

		s.logger.Debug("Resolved conflict: local wins, skipping write", "id", remote.ID)
	}
}

func (s *ConflictAwareSyncer) isConflict(local, remote *provider.Item) bool {
	return local.UpdatedAt != remote.UpdatedAt ||
		local.Type != remote.Type ||
		local.ActorLogin != remote.ActorLogin ||
		local.RepoName != remote.RepoName
}
