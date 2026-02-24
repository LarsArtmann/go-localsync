package sync

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
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

// SyncOptions configures a sync operation.
type SyncOptions struct {
	// Source identifies what to sync (e.g., username for GitHub).
	Source string
	// MaxPages is the maximum number of pages to fetch.
	MaxPages int
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
		return nil, nil
	}

	s.logger.Info("Starting sync", "provider", s.provider.Name(), "source", opts.Source)

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, err
	}

	syncResult := &SyncResult{Fetched: len(result.Items)}

	for _, item := range result.Items {
		if err := s.storage.Upsert(ctx, item); err != nil {
			s.logger.Warn("Failed to upsert item", "id", item.ID, "error", err)
			syncResult.Errors++
			continue
		}
	}

	s.logger.Info("Sync completed", "fetched", syncResult.Fetched, "errors", syncResult.Errors)
	return syncResult, nil
}

// SyncIncremental performs an incremental sync, only fetching items newer than the latest stored.
func (s *Syncer) SyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	if opts == nil {
		return nil, nil
	}

	latestItem, err := s.storage.GetLatest(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Starting incremental sync", "provider", s.provider.Name(), "source", opts.Source)

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, err
	}

	syncResult := &SyncResult{Fetched: len(result.Items)}

	var cutoff time.Time
	if latestItem != nil {
		cutoff = latestItem.CreatedAt
		s.logger.Debug("Using cutoff time", "cutoff", cutoff)
	}

	for _, item := range result.Items {
		if !cutoff.IsZero() && item.CreatedAt.Before(cutoff) {
			syncResult.Skipped++
			continue
		}

		if err := s.storage.Upsert(ctx, item); err != nil {
			s.logger.Warn("Failed to upsert item", "id", item.ID, "error", err)
			syncResult.Errors++
			continue
		}
	}

	s.logger.Info("Incremental sync completed", "fetched", syncResult.Fetched, "skipped", syncResult.Skipped, "errors", syncResult.Errors)
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
