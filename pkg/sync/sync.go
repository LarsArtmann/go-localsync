package sync

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/larsartmann/go-localsync/pkg/github"
	"github.com/larsartmann/go-localsync/pkg/storage"
)

// Fetcher is an alias to the github.Fetcher interface for clear dependencies.
type Fetcher = github.Fetcher

type Syncer struct {
	fetcher Fetcher
	storage storage.Storage
	logger  *log.Logger
}

func NewSyncer(fetcher Fetcher, store storage.Storage, logger *log.Logger) *Syncer {
	if logger == nil {
		logger = log.Default()
	}
	return &Syncer{
		fetcher: fetcher,
		storage: store,
		logger:  logger,
	}
}

type SyncOptions struct {
	Username string
	MaxPages int
}

type SyncResult struct {
	Fetched int
	Skipped int
	Errors  int
}

func (s *Syncer) Sync(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	if opts == nil {
		return nil, nil
	}

	s.logger.Info("Starting sync", "username", opts.Username)

	events, err := s.fetcher.FetchAllEvents(ctx, opts.Username, opts.MaxPages)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{Fetched: len(events)}

	for _, event := range events {
		if err := s.storage.UpsertEvent(ctx, event); err != nil {
			s.logger.Warn("Failed to upsert event", "github_id", event.GithubID, "error", err)
			result.Errors++
			continue
		}
	}

	s.logger.Info("Sync completed", "fetched", result.Fetched, "errors", result.Errors)
	return result, nil
}

func (s *Syncer) SyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	if opts == nil {
		return nil, nil
	}

	latestEvent, err := s.storage.GetLatestEvent(ctx)
	if err != nil {
		return nil, err
	}

	s.logger.Info("Starting incremental sync", "username", opts.Username)

	events, err := s.fetcher.FetchAllEvents(ctx, opts.Username, opts.MaxPages)
	if err != nil {
		return nil, err
	}

	result := &SyncResult{Fetched: len(events)}

	var cutoff time.Time
	if latestEvent != nil {
		cutoff = latestEvent.CreatedAt
		s.logger.Debug("Using cutoff time", "cutoff", cutoff)
	}

	for _, event := range events {
		if !cutoff.IsZero() && event.CreatedAt.Before(cutoff) {
			result.Skipped++
			continue
		}

		if err := s.storage.UpsertEvent(ctx, event); err != nil {
			s.logger.Warn("Failed to upsert event", "github_id", event.GithubID, "error", err)
			result.Errors++
			continue
		}
	}

	s.logger.Info("Incremental sync completed", "fetched", result.Fetched, "skipped", result.Skipped, "errors", result.Errors)
	return result, nil
}

func (s *Syncer) GetStats(ctx context.Context) (map[string]interface{}, error) {
	count, err := s.storage.CountEvents(ctx)
	if err != nil {
		return nil, err
	}

	types, err := s.storage.GetEventTypes(ctx)
	if err != nil {
		return nil, err
	}

	typeCounts := make(map[string]int64)
	for _, t := range types {
		c, err := s.storage.CountEventsByType(ctx, t)
		if err != nil {
			continue
		}
		typeCounts[t] = c
	}

	return map[string]interface{}{
		"total_events": count,
		"event_types":  types,
		"type_counts":  typeCounts,
	}, nil
}

func (s *Syncer) Close() error {
	return s.storage.Close()
}
