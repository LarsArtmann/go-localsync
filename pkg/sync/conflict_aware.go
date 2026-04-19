package sync

import (
	"context"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// ConflictAwareSyncer extends Syncer with conflict detection and LWW resolution.
// It compares fetched items against stored versions and resolves conflicts using Last-Write-Wins.
type ConflictAwareSyncer struct {
	*Syncer
}

// NewConflictAwareSyncer creates a new ConflictAwareSyncer wrapping the given Syncer.
func NewConflictAwareSyncer(base *Syncer) *ConflictAwareSyncer {
	return &ConflictAwareSyncer{
		Syncer: base,
	}
}

// ConflictResult extends SyncResult with conflict resolution details.
type ConflictResult struct {
	Fetched   int
	Upserted  int
	Skipped   int
	Conflicts int
	Errors    int
}

// newConflictResult creates a ConflictResult initialized with the given fetched count.
func newConflictResult(fetched int) *ConflictResult {
	return &ConflictResult{
		Fetched:   fetched,
		Upserted:  0,
		Skipped:   0,
		Conflicts: 0,
		Errors:    0,
	}
}

// SyncWithConflictDetection performs a full sync with conflict detection and resolution.
// Each fetched item is compared against the stored version.
// Conflicts are resolved using Last-Write-Wins (UpdatedAt timestamp).
func (s *ConflictAwareSyncer) SyncWithConflictDetection(
	ctx context.Context,
	opts *SyncOptions,
) (*ConflictResult, error) {
	if opts == nil {
		return nil, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	if err := opts.Validate(); err != nil {
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
		s.processItem(ctx, item, cr)
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

// logError logs a warning and increments the error counter.
func (s *ConflictAwareSyncer) logError(
	msg string,
	item *provider.Item,
	err error,
	cr *ConflictResult,
) {
	s.logger.Warn(msg, "id", item.ID, "error", err)

	cr.Errors++
}

// processItem handles a single item during conflict-aware sync.
func (s *ConflictAwareSyncer) processItem(
	ctx context.Context,
	item *provider.Item,
	cr *ConflictResult,
) {
	existing, err := s.findExistingItem(ctx, item)
	if err != nil {
		s.logError("Failed to check existing item", item, err, cr)

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

// upsertNewItem inserts a new item into storage.
func (s *ConflictAwareSyncer) upsertNewItem(
	ctx context.Context,
	item *provider.Item,
	cr *ConflictResult,
) {
	err := s.storage.Upsert(ctx, item)
	if err != nil {
		s.logError("Failed to upsert item", item, err, cr)

		return
	}

	cr.Upserted++
}

// resolveConflict resolves a conflict between local and remote items using LWW.
// The item with the later UpdatedAt timestamp wins.
func (s *ConflictAwareSyncer) resolveConflict(
	ctx context.Context,
	local, remote *provider.Item,
	cr *ConflictResult,
) {
	var resolved *provider.Item
	if remote.UpdatedAt.After(local.UpdatedAt) {
		resolved = remote
	} else {
		resolved = local
	}

	err := s.storage.Upsert(ctx, resolved)
	if err != nil {
		s.logError("Failed to upsert resolved item", resolved, err, cr)

		return
	}

	cr.Conflicts++
	cr.Upserted++

	s.logger.Debug("Resolved conflict", "id", remote.ID, "winner_source", resolved.Source)
}

// findExistingItem checks if an item with the same ID already exists in storage.
func (s *ConflictAwareSyncer) findExistingItem(
	ctx context.Context,
	item *provider.Item,
) (*provider.Item, error) {
	existing, err := s.storage.GetByID(ctx, item.ID.Get())
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to find existing item %q", item.ID.Get())
	}

	return existing, nil
}

// isConflict determines if the remote item conflicts with the existing local item.
func (s *ConflictAwareSyncer) isConflict(local, remote *provider.Item) bool {
	return local.UpdatedAt != remote.UpdatedAt ||
		local.Type != remote.Type ||
		local.ActorLogin != remote.ActorLogin ||
		local.RepoName != remote.RepoName
}
