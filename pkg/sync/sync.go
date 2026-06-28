package sync

import (
	"context"
	"fmt"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

// Syncer orchestrates fetching items from a provider and persisting them via a SyncStore.
type Syncer struct {
	provider provider.Provider
	store    SyncStore
	logger   *log.Logger
}

// NewSyncer creates a Syncer with the given provider, store, and optional logger.
func NewSyncer(p provider.Provider, store SyncStore, logger *log.Logger) *Syncer {
	if logger == nil {
		logger = log.Default()
	}

	return &Syncer{
		provider: p,
		store:    store,
		logger:   logger,
	}
}

func (s *Syncer) Store() SyncStore { //nolint:ireturn
	return s.store
}

// SyncProgressFunc is called after each sync batch to report progress.
type SyncProgressFunc func(fetched, skipped, errors int)

// SyncOptions configures a sync operation.
type SyncOptions struct {
	Source     string
	MaxPages   int
	OnProgress SyncProgressFunc
}

// Validate checks that required fields are set.
func (o *SyncOptions) Validate() error {
	if o.Source == "" {
		return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "SyncOptions.Source is required")
	}

	return nil
}

// SyncResult holds the result of a sync operation.
type SyncResult struct {
	Fetched    int
	Skipped    int
	Errors     int
	ItemErrors []ItemSyncResult
}

// Stats holds aggregate statistics about synced items.
type Stats struct {
	TotalItems int64
	ItemTypes  []string
	TypeCounts map[string]int64
}

// Sync fetches all items from the provider and persists them.
func (s *Syncer) Sync(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	err := s.validateOpts(opts)
	if err != nil {
		return nil, err
	}

	result, err := s.fetchItems(ctx, opts, "Starting sync", "sync")
	if err != nil {
		return nil, err
	}

	syncResult := &SyncResult{Fetched: len(result.Items), Skipped: 0, Errors: 0}

	valid := s.filterValidItems(result.Items, syncResult)

	if len(valid) == 0 {
		s.logger.Info(
			"Sync completed: no valid items",
			"fetched",
			syncResult.Fetched,
			"errors",
			syncResult.Errors,
		)

		return syncResult, nil
	}

	summary := s.store.SyncItems(ctx, valid)
	syncResult.Errors += summary.Errors
	syncResult.Skipped = len(valid) - summary.Synced - summary.Errors

	for _, r := range summary.Results {
		if r.Action == ActionError {
			syncResult.ItemErrors = append(syncResult.ItemErrors, r)
		}
	}

	s.reportProgress(opts, syncResult)

	s.logger.Info(
		"Sync completed",
		"fetched",
		syncResult.Fetched,
		"synced",
		summary.Synced,
		"errors",
		syncResult.Errors,
	)

	if syncResult.Errors > 0 {
		return syncResult, fmt.Errorf("sync completed with %d errors out of %d items", syncResult.Errors, len(valid))
	}

	return syncResult, nil
}

// SyncIncremental fetches items and skips those already present based on the latest item timestamp.
func (s *Syncer) SyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	err := s.validateOpts(opts)
	if err != nil {
		return nil, err
	}

	source := id.NewProviderID(opts.Source)

	items, err := s.store.List(
		ctx,
		model.ItemFilter{
			Source: &source,
			Limit:  1,
		},
	)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to list items for incremental sync")
	}

	if len(items) == 0 {
		return s.Sync(ctx, opts)
	}

	latestItem := items[0]

	result, err := s.fetchItems(ctx, opts, "Starting incremental sync", "incremental sync")
	if err != nil {
		return nil, err
	}

	syncResult := s.processIncrementalItems(ctx, latestItem, result.Items)

	s.reportProgress(opts, syncResult)

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

// GetStats returns aggregate statistics including per-type counts.
// Uses a single CountByType query instead of N+1 per-type Count calls.
func (s *Syncer) GetStats(ctx context.Context) (*Stats, error) {
	typeCounts, err := s.store.CountByType(ctx, model.ItemFilter{})
	if err != nil {
		return nil, err
	}

	var total int64

	types := make([]string, 0, len(typeCounts))

	for t, c := range typeCounts {
		total += c

		types = append(types, t)
	}

	return &Stats{
		TotalItems: total,
		ItemTypes:  types,
		TypeCounts: typeCounts,
	}, nil
}

func (s *Syncer) Close() error {
	return s.store.Close()
}

func (s *Syncer) processIncrementalItems(
	ctx context.Context,
	latestItem *model.Item,
	items []*provider.Item,
) *SyncResult {
	syncResult := &SyncResult{Fetched: len(items), Skipped: 0, Errors: 0}

	cutoff := time.Time{}
	if latestItem != nil {
		cutoff = latestItem.CreatedAt
		s.logger.Debug("Using cutoff time", "cutoff", cutoff)
	}

	filtered := make([]*provider.Item, 0, len(items))
	for _, item := range items {
		if !cutoff.IsZero() && item.CreatedAt.Before(cutoff) {
			syncResult.Skipped++

			continue
		}

		filtered = append(filtered, item)
	}

	toSync := s.filterValidItems(filtered, syncResult)

	if len(toSync) > 0 {
		summary := s.store.SyncItems(ctx, toSync)
		syncResult.Errors += summary.Errors
		syncResult.Skipped += len(toSync) - summary.Synced - summary.Errors
	}

	return syncResult
}

func (s *Syncer) filterValidItems(items []*provider.Item, syncResult *SyncResult) []*provider.Item {
	valid := make([]*provider.Item, 0, len(items))

	for _, item := range items {
		err := item.Validate()
		if err != nil {
			syncResult.Errors++

			s.logger.Warn("Skipping invalid item", "id", item.ID, "error", err)

			continue
		}

		valid = append(valid, item)
	}

	return valid
}

func (s *Syncer) fetchItems(
	ctx context.Context,
	opts *SyncOptions,
	logMsg, errPrefix string,
) (*provider.FetchResult, error) {
	s.logger.Info(logMsg, "provider", s.provider.Name(), "source", opts.Source)

	result, err := s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	if err != nil {
		return nil, pkgerrors.Wrapf(
			err,
			"%s (%s) failed for source %q (maxPages=%d)",
			errPrefix,
			logMsg,
			opts.Source,
			opts.MaxPages,
		)
	}

	return result, nil
}

func (s *Syncer) reportProgress(opts *SyncOptions, syncResult *SyncResult) {
	if opts.OnProgress != nil {
		opts.OnProgress(syncResult.Fetched, syncResult.Skipped, syncResult.Errors)
	}
}

func (s *Syncer) validateOpts(opts *SyncOptions) error {
	if opts == nil {
		return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	return opts.Validate()
}
