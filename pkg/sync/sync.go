package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/log/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/storage"
)

// Syncer orchestrates syncing items from a provider to storage.
type Syncer struct {
	provider provider.Provider
	storage  storage.Storage
	logger   *log.Logger
}

// NewSyncer creates a new Syncer with the given provider and storage.
func NewSyncer(p provider.Provider, store storage.Storage, logger *log.Logger) *Syncer {
	if logger == nil {
		logger = log.Default()
	}

	return &Syncer{
		provider: p,
		storage:  store,
		logger:   logger,
	}
}

// SyncProgressFunc is called after each batch operation to report progress.
type SyncProgressFunc func(fetched, skipped, errors int)

// SyncOptions configures a sync operation.
type SyncOptions struct {
	// Source identifies what to sync (e.g., username for GitHub).
	Source string
	// MaxPages is the maximum number of pages to fetch.
	MaxPages int
	// OnProgress is an optional callback invoked after each batch operation.
	OnProgress SyncProgressFunc
}

// Validate checks that the SyncOptions has required fields set.
func (o *SyncOptions) Validate() error {
	if o.Source == "" {
		return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "SyncOptions.Source is required")
	}

	return nil
}

// SyncResult contains the results of a sync operation.
type SyncResult struct {
	Fetched int
	Skipped int
	Errors  int
}

// Stats contains storage statistics.
type Stats struct {
	TotalItems int64
	ItemTypes  []string
	TypeCounts map[string]int64
}

// Sync performs a full sync from the provider to storage.
func (s *Syncer) Sync(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	if opts == nil {
		return nil, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	s.logger.Info("Starting sync", "provider", s.provider.Name(), "source", opts.Source)

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, fmt.Errorf(
			"sync failed for source %q (maxPages=%d): %w",
			opts.Source,
			opts.MaxPages,
			err,
		)
	}

	syncResult := &SyncResult{Fetched: len(result.Items), Skipped: 0, Errors: 0}

	if err := s.storage.UpsertBatch(ctx, result.Items); err != nil {
		syncResult.Errors = len(result.Items)
		s.logger.Warn("Batch upsert failed", "error", err, "itemCount", len(result.Items))

		return syncResult, fmt.Errorf("batch upsert failed: %w", err)
	}

	if opts.OnProgress != nil {
		opts.OnProgress(syncResult.Fetched, syncResult.Skipped, syncResult.Errors)
	}

	s.logger.Info("Sync completed", "fetched", syncResult.Fetched, "errors", syncResult.Errors)

	return syncResult, nil
}

// SyncIncremental performs an incremental sync, only fetching items newer than the latest stored.
func (s *Syncer) SyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	if opts == nil {
		return nil, pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	latestItem, err := s.storage.GetLatest(ctx)
	if err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			return s.Sync(ctx, opts)
		}

		return nil, fmt.Errorf(
			"failed to get latest item for incremental sync (source=%q): %w",
			opts.Source,
			err,
		)
	}

	s.logger.Info("Starting incremental sync", "provider", s.provider.Name(), "source", opts.Source)

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, fmt.Errorf(
			"incremental sync failed for source %q (maxPages=%d): %w",
			opts.Source,
			opts.MaxPages,
			err,
		)
	}

	syncResult, err := s.processIncrementalItems(ctx, latestItem, result.Items)
	if err != nil {
		return nil, err
	}

	if opts.OnProgress != nil {
		opts.OnProgress(syncResult.Fetched, syncResult.Skipped, syncResult.Errors)
	}

	s.logger.Info(
		"Incremental sync completed",
		"fetched",
		syncResult.Fetched,
		"skipped",
		syncResult.Skipped,
		"errors",
		syncResult.Errors,
	)

	return syncResult, nil
}

// GetStats returns statistics about stored items.
func (s *Syncer) GetStats(ctx context.Context) (*Stats, error) {
	count, err := s.storage.Count(ctx)
	if err != nil {
		return nil, err
	}

	types, err := s.storage.GetTypes(ctx)
	if err != nil {
		return nil, err
	}

	typeCounts := make(map[string]int64)

	for _, t := range types {
		c, err := s.storage.CountByType(ctx, t)
		if err != nil {
			s.logger.Warn("Failed to count items by type", "type", t, "error", err)

			continue
		}

		typeCounts[t] = c
	}

	return &Stats{
		TotalItems: count,
		ItemTypes:  types,
		TypeCounts: typeCounts,
	}, nil
}

// Close releases resources.
func (s *Syncer) Close() error {
	return s.storage.Close()
}

// processIncrementalItems processes items from FetchAll, skipping those older than cutoff.
func (s *Syncer) processIncrementalItems(
	ctx context.Context,
	latestItem *provider.Item,
	items []*provider.Item,
) (*SyncResult, error) {
	syncResult := &SyncResult{Fetched: len(items), Skipped: 0, Errors: 0}

	cutoff := time.Time{}
	if latestItem != nil {
		cutoff = latestItem.CreatedAt
		s.logger.Debug("Using cutoff time", "cutoff", cutoff)
	}

	toUpsert := make([]*provider.Item, 0, len(items))

	for _, item := range items {
		if !cutoff.IsZero() && item.CreatedAt.Before(cutoff) {
			syncResult.Skipped++

			continue
		}

		toUpsert = append(toUpsert, item)
	}

	if len(toUpsert) > 0 {
		if err := s.storage.UpsertBatch(ctx, toUpsert); err != nil {
			syncResult.Errors = len(toUpsert)
			s.logger.Warn("Batch upsert failed", "error", err, "itemCount", len(toUpsert))

			return syncResult, fmt.Errorf("batch upsert failed: %w", err)
		}
	}

	return syncResult, nil
}
