package sync

import (
	"context"
	"errors"
	stdsync "sync"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/crdt"
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

// New constructs a Syncer for provider p writing to store, applying the given
// options. Retry defaults to provider.DefaultRetryConfig and the logger defaults
// to log.Default(); override either with WithRetry / WithLogger. This is the
// preferred constructor; NewSyncer remains as a backwards-compatible wrapper.
func New(p provider.Provider, store SyncStore, opts ...Option) *Syncer {
	syncer := &Syncer{
		provider: p,
		store:    store,
		logger:   log.Default(),
		retry:    provider.DefaultRetryConfig,
		locks:    make(map[string]*stdsync.Mutex),
	}

	for _, opt := range opts {
		opt(syncer)
	}

	if syncer.logger == nil {
		syncer.logger = log.Default()
	}

	return syncer
}

// Option configures a Syncer built with New.
type Option func(*Syncer)

// WithRetry sets the retry configuration applied to transient fetch errors.
// Use this to tune MaxRetries / backoff per deployment (e.g. a rate-limited
// mirror vs a LAN source).
func WithRetry(cfg provider.RetryConfig) Option {
	return func(s *Syncer) { s.retry = cfg }
}

// WithLogger sets the structured logger. A nil logger falls back to log.Default().
func WithLogger(l *log.Logger) Option {
	return func(s *Syncer) { s.logger = l }
}

// NewSyncer creates a Syncer with the given provider, store, and optional logger.
func NewSyncer(p provider.Provider, store SyncStore, logger *log.Logger) *Syncer {
	return New(p, store, WithLogger(logger))
}

func (s *Syncer) Store() SyncStore { //nolint:ireturn
	return s.store
}

// lockSource returns a release function that serializes concurrent syncs for the
// same source. It prevents a TOCTOU race where two syncs read the "latest item"
// timestamp concurrently and both process overlapping windows. Different
// sources run in parallel.
//
// Entries persist for the lifetime of the Syncer (one *sync.Mutex per source).
// This is a bounded cache, not a leak: the source set is the set of
// provider/user identifiers the consumer mirrors, which is finite and small in
// this SDK's pull-mirror domain. Deleting entries on release would be unsafe —
// a finishing goroutine removing the entry while another holds the old mutex
// reference would let a fresh goroutine create a second mutex and break the
// serialization invariant. So we keep them.
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
	// ConflictResolver overrides the store's default conflict strategy for
	// THIS sync run (per-sync strategy without re-stacking). Nil uses whatever
	// the store was configured with. Only effective when the store implements
	// ResolverAwareStore (the CQRS stack does).
	ConflictResolver crdt.ConflictResolver[*model.Item]
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

// partialSyncError returns ErrPartialSync (Transient) when failed > 0, embedding
// the failed/attempted counts in the message. It is the single partial-failure
// contract shared by the full-sync and conflict-aware paths, so both surface the
// same retryable, errors.Is(err, pkgerrors.ErrPartialSync)-checkable error. This
// shared helper exists precisely so the two paths cannot diverge again (which is
// how the conflict-aware path previously dropped partial failures silently).
func partialSyncError(failed, total int) error {
	if failed == 0 {
		return nil
	}

	return pkgerrors.Wrapf(pkgerrors.ErrPartialSync, "%d of %d items failed", failed, total)
}

// errIfFailed surfaces the partial-sync error when result records item failures.
func errIfFailed(result *SyncResult, total int) error {
	return partialSyncError(result.Errors, total)
}

// withSourceLock acquires the per-source lock, defers its release, and invokes
// run. Shared by Sync and SyncIncremental so the lock-and-defer prologue cannot
// drift between them. On lock/validation failure the lock is not held and run
// is never invoked.
func (s *Syncer) withSourceLock(
	ctx context.Context,
	opts *SyncOptions,
	run func(ctx context.Context) (*SyncResult, error),
) (*SyncResult, error) {
	release, err := s.lockAndValidate(opts)
	if err != nil {
		return nil, err
	}

	defer release()

	return run(ctx)
}

// Sync fetches all items from the provider and persists them.
func (s *Syncer) Sync(ctx context.Context, opts *SyncOptions) (*SyncResult, error) {
	return s.withSourceLock(ctx, opts, func(ctx context.Context) (*SyncResult, error) {
		return s.runSync(ctx, opts)
	})
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
			"source",
			opts.Source,
			"fetched",
			syncResult.Fetched,
			"errors",
			syncResult.Errors,
		)

		return syncResult, errIfFailed(syncResult, len(result.Items))
	}

	summary := s.syncBatch(ctx, opts, valid)
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
		"source",
		opts.Source,
		"fetched",
		syncResult.Fetched,
		"synced",
		summary.Synced,
		"tombstoned",
		syncResult.Tombstoned,
		"errors",
		syncResult.Errors,
	)

	return syncResult, errIfFailed(syncResult, len(valid))
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
		seen = append(seen, model.Key{Source: item.Source, SourceID: item.SourceID})
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
	return s.withSourceLock(ctx, opts, func(ctx context.Context) (*SyncResult, error) {
		return s.runSyncIncremental(ctx, opts)
	})
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
		"source",
		opts.Source,
		"fetched",
		syncResult.Fetched,
		"skipped",
		syncResult.Skipped,
		"errors",
		syncResult.Errors,
	)

	return syncResult, errIfFailed(syncResult, syncResult.Fetched)
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

			s.logger.Warn(
				"Skipping invalid item",
				"source", item.Source.Get(),
				"id", item.ID,
				"error", err,
			)

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

		if ra, ok := errors.AsType[retryAfterer](lastErr); ok { // honor a server-advised Retry-After when present
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

// lockAndValidate validates opts and acquires the per-source lock. The returned
// release function must be deferred by the caller to drop the lock on return.
// If validation fails, the lock is NOT acquired and the error is returned.
// This consolidates the validate-then-lock pattern shared by Sync,
// SyncIncremental, and ConflictAwareSyncer.SyncWithConflictDetection.
func (s *Syncer) lockAndValidate(opts *SyncOptions) (func(), error) {
	if err := s.validateOpts(opts); err != nil {
		return nil, err
	}

	return s.lockSource(opts.Source), nil
}

func (s *Syncer) validateOpts(opts *SyncOptions) error {
	if opts == nil {
		return pkgerrors.WithDetail(pkgerrors.ErrInvalidInput, "opts is nil")
	}

	return opts.Validate()
}

// syncBatch dispatches a validated batch through the store, honoring a
// per-sync conflict resolver (SyncOptions.ConflictResolver) when the store
// supports the override seam; otherwise it falls back to plain SyncItems and
// the store's configured strategy applies.
func (s *Syncer) syncBatch(
	ctx context.Context,
	opts *SyncOptions,
	valid []*provider.Item,
) *BatchOutcome {
	if opts != nil && opts.ConflictResolver != nil {
		if aware, ok := s.store.(ResolverAwareStore); ok {
			return aware.SyncItemsWithResolver(ctx, valid, opts.ConflictResolver)
		}
	}

	return s.store.SyncItems(ctx, valid)
}
