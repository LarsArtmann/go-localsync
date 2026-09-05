package cqrs

import (
	"context"
	"io"
	"log/slog"

	"charm.land/log/v2"
	"github.com/larsartmann/go-cqrs-lite/codec/v4"
	"github.com/larsartmann/go-cqrs-lite/command/v4"
	"github.com/larsartmann/go-cqrs-lite/decider/v4"
	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/middleware/v4"
	"github.com/larsartmann/go-cqrs-lite/snapshot/v4"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// newSlogLogger creates an *slog.Logger backed by charm.land/log/v2.
// Required by middleware.EventLogging which expects *slog.Logger.
func newSlogLogger() *slog.Logger {
	return slog.New(log.Default())
}

const (
	backendMemory  = "memory"
	backendSQLite  = "sqlite"
	dbPathInMemory = ":memory:"
)

// CQRSConfig configures the CQRS stack's storage backend and conflict resolution.
type CQRSConfig struct {
	Backend          string
	DBPath           string
	ConflictResolver crdt.ConflictResolver[*model.Item]
	// OTel is the opt-in observability surface: when non-nil, its command and
	// event middleware chains (spans + cqrs.operation.* metrics) attach to the
	// dispatcher and bus, SyncItems opens a span, and the projection host
	// reports through the same instruments. Nil (default) leaves behavior and
	// performance unchanged. Build one with middleware.NewOTelBundle from
	// your own TracerProvider/MeterProvider.
	OTel *middleware.OTelBundle
}

// CQRSStack wires together the event store, bus, decider repository, read model,
// command/query dispatchers, and projection runner.
//
// ReadModel is embedded so the read-side methods (List, Count, CountByType, Get, Upsert, Tombstone)
// are promoted onto *CQRSStack. This lets *CQRSStack satisfy both the
// internal ReadModel contract and the external sync.SyncStore contract
// without duplicate wrapper methods.
type CQRSStack struct {
	event.Store
	event.Bus
	ReadModel

	Repo              *decider.Repository[SyncItemState]
	CommandDispatcher *command.Dispatcher
	otel              *middleware.OTelBundle
	cancelRunner      context.CancelFunc
	drainDone         <-chan struct{}
}

var _ synclib.SyncStore = (*CQRSStack)(nil)

// OTel returns the observability bundle the stack was configured with, or
// nil when telemetry is off. Consumers can reuse it (e.g. HTTP middleware)
// so all signals share one tracer/meter.
func (s *CQRSStack) OTel() *middleware.OTelBundle { return s.otel }

// NewCQRSStack creates a fully wired CQRS stack based on the given config.
func NewCQRSStack(cfg CQRSConfig) (stack *CQRSStack, err error) { //nolint:nonamedreturns
	ctx := context.Background()

	sr, storeErr := createStoreAndBus(ctx, cfg)
	if storeErr != nil {
		return nil, storeErr
	}

	// Track resources opened during construction so the defer can release them
	// if any subsequent step fails — preventing store/bus/db/goroutine leaks.
	var rm ReadModel

	var cancelRunner context.CancelFunc

	var drainDone <-chan struct{}

	defer func() {
		if err == nil {
			return
		}

		cleanupFailedConstruction(sr, rm, cancelRunner, drainDone)
	}()

	rm, err = createReadModel(ctx, cfg, sr)
	if err != nil {
		return nil, err
	}

	proj := newProjector(rm)

	if err = sr.bus.Use(
		middleware.EventLogging(newSlogLogger()),
	); err != nil {
		return nil, pkgerrors.Wrap(err, "wire event logging middleware")
	}

	if cfg.OTel != nil {
		if err = sr.bus.Use(cfg.OTel.Event()...); err != nil {
			return nil, pkgerrors.Wrap(err, "wire otel event middleware")
		}
	}

	cancelRunner, drainDone, err = startProjectionRunner(sr, proj, cfg.OTel)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "start projection runner")
	}

	deciderSpec := decider.Decider[SyncItemState]{
		Initial: InitialState,
		Apply:   fold,
	}

	var snapshotStore snapshot.SnapshotStore

	snapshotStore, err = createSnapshotStore(cfg, sr.db)
	if err != nil {
		return nil, err
	}

	var snapshotStrategy snapshot.SnapshotStrategy

	snapshotStrategy, err = snapshot.EveryNEvents(10)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "create snapshot strategy")
	}

	var repo *decider.Repository[SyncItemState]

	repo, err = decider.NewRepository(
		sr.store, sr.bus, deciderSpec,
		decider.WithSnapshotStore[SyncItemState](snapshotStore),
		decider.WithCodec[SyncItemState](codec.CBORCodec{}),
		decider.WithSnapshotStrategy[SyncItemState](snapshotStrategy),
		// Propagate command causation from the handler context into event
		// metadata, so every event names the command that produced it.
		decider.WithEnricher[SyncItemState](event.CommandCausalityEnricher),
	)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "create decider repository")
	}

	var commandDispatcher *command.Dispatcher

	commandDispatcher, err = wireCommandDispatcher(repo, cfg.ConflictResolver, cfg.OTel)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "wire command dispatcher")
	}

	return &CQRSStack{
		Store:             sr.store,
		Bus:               sr.bus,
		Repo:              repo,
		ReadModel:         rm,
		CommandDispatcher: commandDispatcher,
		otel:              cfg.OTel,
		cancelRunner:      cancelRunner,
		drainDone:         drainDone,
	}, nil
}

// cleanupFailedConstruction releases the resources opened before a failed
// NewCQRSStack construction, preventing store/bus/db/goroutine leaks. It is
// only called on the error path; on success the CQRSStack takes ownership.
// Close failures are logged, not returned — the primary construction error
// takes precedence, but a failed close (leaked fd, WAL flush) must not be
// invisible.
func cleanupFailedConstruction(
	sr storeResult,
	rm ReadModel,
	cancelRunner context.CancelFunc,
	drainDone <-chan struct{},
) {
	if cancelRunner != nil {
		cancelRunner()
	}

	if drainDone != nil {
		<-drainDone
	}

	if rm != nil {
		closeLogged("read model", rm)
	}

	if c, ok := sr.store.(io.Closer); ok {
		closeLogged("event store", c)
	}

	if c, ok := sr.bus.(io.Closer); ok {
		closeLogged("event bus", c)
	}
}

// closeLogged closes c, logging failures instead of returning them. For
// best-effort cleanup paths where a primary error is already surfacing: a
// silently swallowed close error would hide resource leaks.
func closeLogged(name string, c io.Closer) {
	if err := c.Close(); err != nil {
		log.Warn("cleanup close failed", "component", name, "error", err)
	}
}

// SyncItem dispatches a SyncItemCommand for a single item. A fresh
// correlation ID is attached so the emitted events are traceable to this
// call even outside a batch run.
func (s *CQRSStack) SyncItem(ctx context.Context, item *provider.Item) error {
	aggID := AggregateID(item.Source.Get(), item.ExternalID)

	return s.CommandDispatcher.Dispatch(ctx, &SyncItemCommand{
		BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
		Item:         toDataItem(item),
		RawJSON:      item.RawJSON,
		Options:      []event.Option{event.WithCorrelationID(cqrsid.NewCorrelationID())},
		outcome:      nil,
	})
}

// TombstoneItem dispatches a TombstoneItemCommand for the given source/externalID
// and reason, hiding the item from the default read model while preserving its
// history. A later sync resurrects it automatically.
func (s *CQRSStack) TombstoneItem(
	ctx context.Context,
	source string,
	sourceID id.ExternalID,
	reason model.TombstoneReason,
) error {
	aggID := AggregateID(source, sourceID)

	return s.CommandDispatcher.Dispatch(ctx, &TombstoneItemCommand{
		BasicCommand: mustNewCommand(commandTypeTombstone, aggID),
		Source:       source,
		SourceID:     sourceID,
		Reason:       reason,
		Options:      []event.Option{event.WithCorrelationID(cqrsid.NewCorrelationID())},
	})
}

// Reconcile tombstones live items for source that are absent from seen,
// detecting upstream deletions. It returns the number of items tombstoned.
//
// Only call this after a COMPLETE fetch (every item the provider currently
// holds), since any live item not present in seen is assumed gone upstream and
// will be tombstoned with ReasonUpstreamGone.
func (s *CQRSStack) Reconcile(ctx context.Context, source string, seen []model.Key) (int, error) {
	src := id.NewProviderID(source)

	live, err := s.List(ctx, model.ItemFilter{Source: &src})
	if err != nil {
		return 0, pkgerrors.Wrapf(err, "reconcile: list live items for %s", source)
	}

	seenSet := make(map[string]struct{}, len(seen))

	for _, k := range seen {
		seenSet[itemKey(k.Source.Get(), k.ExternalID)] = struct{}{}
	}

	var tombstoned int

	for _, item := range live {
		if ctx.Err() != nil {
			return tombstoned, ctx.Err()
		}

		if _, ok := seenSet[itemKey(item.Source.Get(), item.ExternalID)]; ok {
			continue
		}

		if err := s.TombstoneItem(ctx, source, item.ExternalID, model.ReasonUpstreamGone); err != nil {
			return tombstoned, pkgerrors.Wrapf(err, "reconcile: tombstone %s/%s", source, item.ExternalID)
		}

		tombstoned++
	}

	return tombstoned, nil
}

// SyncItems syncs a batch of items, returning a summary with per-item results.
func (s *CQRSStack) SyncItems(
	ctx context.Context,
	items []*provider.Item,
) *synclib.SyncSummary {
	if s.otel != nil {
		return withBatchSpan(ctx, s.otel, func(ctx context.Context) *synclib.SyncSummary {
			return s.syncItems(ctx, items)
		})
	}

	return s.syncItems(ctx, items)
}

func (s *CQRSStack) syncItems(
	ctx context.Context,
	items []*provider.Item,
) *synclib.SyncSummary {
	summary := &synclib.SyncSummary{
		Results:   make([]synclib.ItemSyncResult, 0, len(items)),
		Synced:    0,
		Conflicts: 0,
		Errors:    0,
	}

	corrID := cqrsid.NewCorrelationID()

	for _, item := range items {
		if ctx.Err() != nil {
			break
		}

		aggID := AggregateID(item.Source.Get(), item.ExternalID)
		dataItem := toDataItem(item)

		var outcome SyncOutcome

		syncOpts := []event.Option{event.WithCorrelationID(corrID)}

		cmd := &SyncItemCommand{
			BasicCommand: mustNewCommand(commandTypeSyncItem, aggID),
			Item:         dataItem,
			RawJSON:      item.RawJSON,
			Options:      syncOpts,
			outcome:      &outcome,
		}

		err := s.CommandDispatcher.Dispatch(ctx, cmd)
		if err != nil {
			err = pkgerrors.Wrapf(err, "sync %s/%s", item.Source.Get(), item.ExternalID.Get())
		}

		action := classifyAction(err, outcome)

		if err != nil {
			err = pkgerrors.Wrapf(err, "eventCount=%d, conflict=%v", outcome.EventCount, outcome.ConflictDetected)
		}

		result := synclib.ItemSyncResult{
			SourceID: item.ExternalID,
			Action:   action,
			Error:    err,
		}

		switch result.Action {
		case synclib.ActionError:
			summary.Errors++
		case synclib.ActionCreated, synclib.ActionUpdated, synclib.ActionConflictRemote, synclib.ActionConflictLocal:
			summary.Synced++

			if result.Action == synclib.ActionConflictRemote || result.Action == synclib.ActionConflictLocal {
				summary.Conflicts++
			}
		case synclib.ActionUnchanged, synclib.ActionTombstoned:
		}

		summary.Results = append(summary.Results, result)
	}

	return summary
}
