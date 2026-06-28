package sync

import (
	"context"
	"errors"
	"fmt"
	stdsync "sync"
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
	retry    provider.RetryConfig
	sourceMu stdsync.Mutex
	locks    map[string]*stdsync.Mutex
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
		retry:    provider.DefaultRetryConfig,
		locks:    make(map[string]*stdsync.Mutex),
	}
}

func (s *Syncer) Store() SyncStore { //nolint:ireturn
	return s.store
}

// lockSource returns a release function that serializes concurrent syncs for the
// same source. It prevents a TOCTOU race where two syncs read the "latest item"
// timestamp concurrently and both process overlapping windows. Different
// sources run in parallel.
func (s *Syncer) lockSource(source string) func() {
	s.sourceMu.Lock()

	mu, ok := s.locks[source]
	if !ok {
		mu = &stdsync.Mutex{}
		s.locks[source] = mu
	}

	s.sourceMu.Unlock()

	mu.Lock()

	return mu.Unlock
}

// SyncProgressFunc is called after each sync batch to report progress.
type SyncProgressFunc func(fetched, skipped, errors int)

// SyncOptions configures a sync operation.
type SyncOptions struct {
	Source     string
	MaxPages   int
	OnProgress SyncProgressFunc
	// Reconcile, when true, runs an upstream-gone reconciliation pass after the
	// items are synced: live items for Source absent from the fetched set are
	// tombstoned (ReasonUpstreamGone). Only set this when the fetch was COMPLETE
	// (every item the provider holds), otherwise still-present items would be
	// wrongly tombstoned.
	Reconcile bool
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
	Tombstoned int
	Errors     int
	ItemErrors []ItemSyncResult
}

// Stats holds aggregate statistics about synced items.
type Stats struct {
	TotalItems int64
	ItemTypes  []string
	TypeCounts map[string]int64
}

// errCompletedWithErrors is a static sentinel wrapped when a sync finishes with
// per-item failures, so callers can errors.Is it (and it satisfies err113).
var errCompletedWithErrors = errors.New("sync completed with item errors")

// Sync fetches all items from the provider and persists them.
func (s *Syncer) Sync(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	err := s.validateOpts(opts)
	if err != nil {
		return nil, err
	}

	defer s.lockSource(opts.Source)()

	return s.runSync(ctx, opts)
}

// runSync is the lock-free full-sync body. Callers must already hold the source
// lock (acquired by Sync / SyncIncremental) so concurrent syncs for one source
// are serialized without re-entrant locking.
func (s *Syncer) runSync(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	result, err := s.fetchItems(ctx, opts, "Starting sync", "sync")
	if err != nil {
		return nil, err
	}

	syncResult := &SyncResult{Fetched: len(result.Items), Skipped: 0, Errors: 0}

	valid := s.filterValidItems(result.Items, syncResult)

	if len(valid) == 0 {
		s.reconcile(ctx, opts, result, syncResult)

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

	s.reconcile(ctx, opts, result, syncResult)

	s.logger.Info(
		"Sync completed",
		"fetched",
		syncResult.Fetched,
		"synced",
		summary.Synced,
		"tombstoned",
		syncResult.Tombstoned,
		"errors",
		syncResult.Errors,
	)

	if syncResult.Errors > 0 {
		return syncResult, fmt.Errorf(
			"%w: %d of %d items failed",
			errCompletedWithErrors,
			syncResult.Errors,
			len(valid),
		)
	}

	return syncResult, nil
}

// reconcile runs the opt-in upstream-gone reconciliation pass: live items for
// the source that are absent from the fetched set are tombstoned. It is a no-op
// unless opts.Reconcile is set, and best-effort (failures are logged, not fatal).
//
// SAFETY: reconcile is refused when fetched.HasMore is true. Tombstoning assumes
// the fetched set is the COMPLETE picture of what the provider holds; a partial
// (still-paginating) fetch would wrongly declare still-present items as gone.
func (s *Syncer) reconcile(
	ctx context.Context,
	opts *SyncOptions,
	fetched *provider.FetchResult,
	syncResult *SyncResult,
) {
	if !opts.Reconcile || ctx.Err() != nil {
		return
	}

	if fetched.HasMore {
		s.logger.Warn(
			"Skipping reconciliation: fetch was not complete (provider reports more pages)",
			"source", opts.Source,
		)

		return
	}

	seen := make([]model.Key, 0, len(fetched.Items))

	for _, item := range fetched.Items {
		seen = append(seen, model.Key{Source: item.Source, ExternalID: item.ExternalID})
	}

	tombstoned, err := s.store.Reconcile(ctx, opts.Source, seen)
	if err != nil {
		s.logger.Warn("Reconciliation failed", "source", opts.Source, "error", err)

		return
	}

	syncResult.Tombstoned = tombstoned

	if tombstoned > 0 {
		s.logger.Info("Reconciliation tombstoned upstream-gone items", "source", opts.Source, "tombstoned", tombstoned)
	}
}

func (s *Syncer) SyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	err := s.validateOpts(opts)
	if err != nil {
		return nil, err
	}

	defer s.lockSource(opts.Source)()

	return s.runSyncIncremental(ctx, opts)
}

// runSyncIncremental is the lock-free incremental-sync body. It falls back to
// runSync (not the public Sync) to avoid re-entrant source locking.
func (s *Syncer) runSyncIncremental(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
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
		return s.runSync(ctx, opts)
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

	wrapErr := func(e error) error {
		return pkgerrors.Wrapf(
			e,
			"%s (%s) failed for source %q (maxPages=%d)",
			errPrefix,
			logMsg,
			opts.Source,
			opts.MaxPages,
		)
	}

	fetch := func() (*provider.FetchResult, error) {
		return s.provider.FetchAll(ctx, opts.Source, opts.MaxPages)
	}

	result, err := fetch()
	if err == nil {
		return result, nil
	}

	cfg := s.retry
	if !cfg.Enabled {
		return nil, wrapErr(err)
	}

	lastErr := err

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Only retry errors the taxonomy marks retryable; permanent failures surface immediately.
		if !pkgerrors.IsRetryable(lastErr) {
			return nil, wrapErr(lastErr)
		}

		wait := backoff(cfg, attempt)

		if ra, ok := lastErr.(retryAfterer); ok { // honor a server-advised Retry-After when present
			if d := ra.RetryAfter(); d > 0 {
				wait = d
			}
		}

		if wait > cfg.MaxBackoff {
			wait = cfg.MaxBackoff
		}

		s.logger.Warn(
			"Retrying fetch after transient error",
			"source", opts.Source, "attempt", attempt, "wait", wait, "error", lastErr,
		)

		sleepErr := sleepCtx(ctx, wait)
		if sleepErr != nil {
			return nil, sleepErr
		}

		result, err = fetch()
		if err == nil {
			return result, nil
		}

		lastErr = err
	}

	return nil, wrapErr(lastErr)
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
