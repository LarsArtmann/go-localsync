package sync

import (
	"context"
	"fmt"
	"time"

	localsync "github.com/larsartmann/go-localfirst/pkg/sync"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// ConflictAwareSyncer extends Syncer with vector clock tracking and conflict resolution.
// It uses go-localfirst sync primitives for causal ordering and Last-Write-Wins conflict resolution.
type ConflictAwareSyncer struct {
	*Syncer
	resolver localsync.ConflictResolver[*provider.Item]
	clock    localsync.VectorClock
	nodeID   string
}

// ConflictAwareSyncerOption configures a ConflictAwareSyncer.
type ConflictAwareSyncerOption func(*ConflictAwareSyncer)

// WithConflictResolver sets a custom conflict resolver.
// Defaults to LWW using provider.Item.UpdatedAt.
func WithConflictResolver(
	resolver localsync.ConflictResolver[*provider.Item],
) ConflictAwareSyncerOption {
	return func(s *ConflictAwareSyncer) {
		s.resolver = resolver
	}
}

// WithNodeID sets the node identifier for vector clock tracking.
// Defaults to the provider name.
func WithNodeID(nodeID string) ConflictAwareSyncerOption {
	return func(s *ConflictAwareSyncer) {
		s.nodeID = nodeID
	}
}

// NewConflictAwareSyncer creates a new ConflictAwareSyncer wrapping the given Syncer.
func NewConflictAwareSyncer(
	base *Syncer,
	opts ...ConflictAwareSyncerOption,
) *ConflictAwareSyncer {
	syncer := &ConflictAwareSyncer{
		Syncer:   base,
		resolver: nil,
		clock:    localsync.NewVectorClock(),
		nodeID:   base.provider.Name(),
	}

	for _, opt := range opts {
		opt(syncer)
	}

	if syncer.resolver == nil {
		syncer.resolver = localsync.NewLWWResolver(func(item *provider.Item) time.Time {
			return item.UpdatedAt
		})
	}

	return syncer
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
// Each fetched item is compared against the stored version using vector clocks.
// Conflicts are resolved using the configured ConflictResolver (LWW by default).
func (s *ConflictAwareSyncer) SyncWithConflictDetection(
	ctx context.Context,
	opts *SyncOptions,
) (*ConflictResult, error) {
	if opts == nil {
		return nil, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	s.logger.Info("Starting conflict-aware sync",
		"provider", s.provider.Name(),
		"source", opts.Source,
		"nodeID", s.nodeID,
	)

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, fmt.Errorf(
			"conflict-aware sync failed for source %q (maxPages=%d): %w",
			opts.Source,
			opts.MaxPages,
			err,
		)
	}

	cr := newConflictResult(len(result.Items))

	for _, item := range result.Items {
		s.clock.Increment(s.nodeID)

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

// resolveConflict resolves a conflict between local and remote items.
func (s *ConflictAwareSyncer) resolveConflict(
	ctx context.Context,
	local, remote *provider.Item,
	cr *ConflictResult,
) {
	resolved, err := s.resolver.Resolve(&localsync.Conflict[*provider.Item]{
		Local:     local,
		Remote:    remote,
		LocalVC:   s.buildClockForItem(local),
		RemoteVC:  s.buildClockForItem(remote),
		Timestamp: time.Now(),
	})
	if err != nil {
		s.logError("Conflict resolution failed", remote, err, cr)

		return
	}

	err = s.storage.Upsert(ctx, resolved)
	if err != nil {
		s.logError("Failed to upsert resolved item", resolved, err, cr)

		return
	}

	cr.Conflicts++
	cr.Upserted++

	s.logger.Debug("Resolved conflict", "id", remote.ID, "winner_source", resolved.Source)
}

// GetVectorClock returns a clone of the current vector clock state.
func (s *ConflictAwareSyncer) GetVectorClock() localsync.VectorClock {
	return s.clock.Clone()
}

// SyncOperations converts fetched items into sync Operations.
// This enables operation-based sync protocols using go-localfirst primitives.
func (s *ConflictAwareSyncer) SyncOperations(
	ctx context.Context,
	opts *SyncOptions,
) ([]*localsync.Operation[*provider.Item], *ConflictResult, error) {
	if opts == nil {
		return nil, nil, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch operations failed for source %q: %w", opts.Source, err)
	}

	operations := make([]*localsync.Operation[*provider.Item], 0, len(result.Items))
	cr := newConflictResult(len(result.Items))

	for idx, item := range result.Items {
		s.clock.Increment(s.nodeID)

		opType := localsync.OpCreate

		existing, err := s.findExistingItem(ctx, item)
		if err != nil {
			cr.Errors++

			continue
		}

		if existing != nil {
			opType = localsync.OpUpdate
		}

		op := localsync.NewOperation(
			fmt.Sprintf("%s-%d", s.nodeID, idx),
			opType,
			s.nodeID,
			item,
		)
		op.VectorClock = s.clock.Clone()

		operations = append(operations, op)

		err = s.storage.Upsert(ctx, item)
		if err != nil {
			cr.Errors++

			continue
		}

		cr.Upserted++
	}

	return operations, cr, nil
}

// findExistingItem checks if an item with the same ID already exists in storage.
func (s *ConflictAwareSyncer) findExistingItem(
	ctx context.Context,
	item *provider.Item,
) (*provider.Item, error) {
	existing, err := s.storage.GetByID(ctx, item.ID.Get())
	if err != nil {
		return nil, fmt.Errorf("failed to find existing item %q: %w", item.ID.Get(), err)
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

// buildClockForItem creates a vector clock snapshot for an item.
func (s *ConflictAwareSyncer) buildClockForItem(item *provider.Item) localsync.VectorClock {
	vc := s.clock.Clone()

	vc.Increment(item.Source.Get())

	return vc
}
