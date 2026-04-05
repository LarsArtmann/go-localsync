package sync

import (
	"context"
	"fmt"
	"time"

	"charm.land/log/v2"
	localsync "github.com/larsartmann/go-localfirst/pkg/sync"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
)

// ConflictAwareSyncer extends Syncer with vector clock tracking and conflict resolution.
// It uses go-localfirst sync primitives for causal ordering and Last-Write-Wins conflict resolution.
type ConflictAwareSyncer struct {
	provider provider.Provider
	storage  storage.Storage
	resolver localsync.ConflictResolver[*provider.Item]
	clock    localsync.VectorClock
	nodeID   string
	logger   *log.Logger
}

// ConflictAwareSyncerOption configures a ConflictAwareSyncer.
type ConflictAwareSyncerOption func(*ConflictAwareSyncer)

// WithConflictResolver sets a custom conflict resolver.
// Defaults to LWW using provider.Item.CreatedAt.
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

// NewConflictAwareSyncer creates a new ConflictAwareSyncer with go-localfirst primitives.
func NewConflictAwareSyncer(
	p provider.Provider,
	store storage.Storage,
	logger *log.Logger,
	opts ...ConflictAwareSyncerOption,
) *ConflictAwareSyncer {
	if logger == nil {
		logger = log.Default()
	}

	s := &ConflictAwareSyncer{
		provider: p,
		storage:  store,
		clock:    localsync.NewVectorClock(),
		nodeID:   p.Name(),
		logger:   logger,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.resolver == nil {
		s.resolver = localsync.NewLWWResolver[*provider.Item](func(item *provider.Item) time.Time {
			return item.CreatedAt
		})
	}

	return s
}

// ConflictResult extends SyncResult with conflict resolution details.
type ConflictResult struct {
	Fetched   int
	Upserted  int
	Skipped   int
	Conflicts int
	Errors    int
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

	cr := &ConflictResult{Fetched: len(result.Items)}

	for _, item := range result.Items {
		s.clock.Increment(s.nodeID)

		existing, err := s.findExistingItem(ctx, item)
		if err != nil {
			s.logger.Warn("Failed to check existing item", "id", item.ID, "error", err)

			cr.Errors++

			continue
		}

		if existing == nil {
			err := s.storage.Upsert(ctx, item)
			if err != nil {
				s.logger.Warn("Failed to upsert item", "id", item.ID, "error", err)

				cr.Errors++

				continue
			}

			cr.Upserted++

			continue
		}

		if s.isConflict(existing, item) {
			resolved, err := s.resolver.Resolve(&localsync.Conflict[*provider.Item]{
				Local:     existing,
				Remote:    item,
				LocalVC:   s.buildClockForItem(existing),
				RemoteVC:  s.buildClockForItem(item),
				Timestamp: time.Now(),
			})
			if err != nil {
				s.logger.Warn("Conflict resolution failed", "id", item.ID, "error", err)

				cr.Errors++

				continue
			}

			if err := s.storage.Upsert(ctx, resolved); err != nil {
				s.logger.Warn("Failed to upsert resolved item", "id", resolved.ID, "error", err)

				cr.Errors++

				continue
			}

			cr.Conflicts++
			cr.Upserted++

			s.logger.Debug("Resolved conflict", "id", item.ID, "winner_source", resolved.Source)
		} else {
			cr.Skipped++
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
	cr := &ConflictResult{Fetched: len(result.Items)}

	for i, item := range result.Items {
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
			fmt.Sprintf("%s-%d", s.nodeID, i),
			opType,
			s.nodeID,
			item,
		)
		op.VectorClock = s.clock.Clone()

		operations = append(operations, op)

		if err := s.storage.Upsert(ctx, item); err != nil {
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
	items, err := s.storage.GetItems(ctx, 1, 0)
	if err != nil {
		return nil, err
	}

	for _, stored := range items {
		if stored.ID == item.ID {
			return stored, nil
		}
	}

	return nil, nil
}

// isConflict determines if the remote item conflicts with the existing local item.
func (s *ConflictAwareSyncer) isConflict(local, remote *provider.Item) bool {
	return local.CreatedAt != remote.CreatedAt ||
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

// Close releases resources.
func (s *ConflictAwareSyncer) Close() error {
	return s.storage.Close()
}
