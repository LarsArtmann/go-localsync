package sync

import (
	"context"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

type Syncer struct {
	provider provider.Provider
	stack    *cqrs.CQRSStack
	logger   *log.Logger
}

func NewSyncer(p provider.Provider, stack *cqrs.CQRSStack, logger *log.Logger) *Syncer {
	if logger == nil {
		logger = log.Default()
	}

	return &Syncer{
		provider: p,
		stack:    stack,
		logger:   logger,
	}
}

type SyncProgressFunc func(fetched, skipped, errors int)

type SyncOptions struct {
	Source     string
	MaxPages   int
	OnProgress SyncProgressFunc
}

func (o *SyncOptions) Validate() error {
	if o.Source == "" {
		return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "SyncOptions.Source is required")
	}

	return nil
}

type SyncResult struct {
	Fetched int
	Skipped int
	Errors  int
}

type Stats struct {
	TotalItems int64
	ItemTypes  []string
	TypeCounts map[string]int64
}

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

	synced, _, errs := s.stack.SyncItems(ctx, valid)
	syncResult.Errors += errs
	syncResult.Skipped = len(valid) - synced - errs

	s.reportProgress(opts, syncResult)

	s.logger.Info(
		"Sync completed",
		"fetched",
		syncResult.Fetched,
		"synced",
		synced,
		"errors",
		syncResult.Errors,
	)

	return syncResult, nil
}

func (s *Syncer) SyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	err := s.validateOpts(opts)
	if err != nil {
		return nil, err
	}

	items, err := s.stack.ReadModel.List(
		ctx,
		cqrs.ItemFilter{
			Type:       nil,
			ActorLogin: nil,
			RepoName:   nil,
			Source:     nil,
			Since:      nil,
			Limit:      1,
			Offset:     0,
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

func (s *Syncer) GetStats(ctx context.Context) (*Stats, error) {
	count, err := s.stack.Count(ctx)
	if err != nil {
		return nil, err
	}

	eventTypes, err := s.stack.GetTypes(ctx)
	if err != nil {
		return nil, err
	}

	typeCounts := make(map[string]int64)

	for _, t := range eventTypes {
		eventType := types.NewEventTypeID(t)

		count, err := s.stack.ReadModel.Count(
			ctx,
			cqrs.ItemFilter{
				Type:       &eventType,
				ActorLogin: nil,
				RepoName:   nil,
				Source:     nil,
				Since:      nil,
				Limit:      0,
				Offset:     0,
			},
		)
		if err != nil {
			s.logger.Warn("Failed to count items by type", "type", t, "error", err)

			continue
		}

		typeCounts[t] = count
	}

	return &Stats{
		TotalItems: count,
		ItemTypes:  eventTypes,
		TypeCounts: typeCounts,
	}, nil
}

func (s *Syncer) Close() error {
	return s.stack.Close()
}

func (s *Syncer) processIncrementalItems(
	ctx context.Context,
	latestItem *provider.Item,
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
		synced, _, errs := s.stack.SyncItems(ctx, toSync)
		syncResult.Errors += errs
		syncResult.Skipped += len(toSync) - synced - errs
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
