package cqrs

import (
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/scenario/v4"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// The scenario DSL (go-cqrs-lite/scenario) is library-native BDD for deciders
// and projections: Given events folded into state, When a command decides,
// Then the emitted event types match. These specs document the flagship
// behaviors the way consumers will test their own deciders.

// mustScenarioEvents builds events for a fresh aggregate at the given base version.
func mustScenarioEvents(
	t *testing.T, sourceID string, base event.Version, types []event.Type, payloads []any,
) []event.Event {
	t.Helper()

	evts, err := event.NewEvents(
		MustStreamID("github", id.NewSourceID(sourceID)),
		aggregateType,
		base,
		types,
		payloads,
	)
	testutil.MustNoError(t, err)

	return evts
}

// syncCmd carries what the scenario's When needs for a sync decision.
type syncCmd struct {
	item     *model.Item
	rawJSON  []byte
	resolver crdt.ConflictResolver[*model.Item]
}

// decideForScenario adapts our version-first DecideFunc to the scenario DSL's
// command-first signature: the version is the folded history length.
func decideForScenario(state SyncItemState, cmd syncCmd) ([]event.Event, error) {
	version := stateVersionOf(state)

	decided, err := decideWithOutcome(cmd.item, cmd.rawJSON, cmd.resolver, nil)(state, version)
	if err != nil {
		return nil, err
	}

	return decided, nil
}

// stateVersionOf estimates the stream version from the item's mutation
// history length; scenario tests keep it simple by using 1 for live and
// tombstoned single-event histories.
func stateVersionOf(SyncItemState) event.Version { return event.Version(1) }

// TestScenario_Resurrection: a tombstoned item that syncs again emits
// ItemSynced — the projection upsert resurrects it.
func TestScenario_Resurrection(t *testing.T) {
	t.Parallel()

	tombstoned := mustScenarioEvents(t, "sc-resurrect", event.Version(1),
		[]event.Type{EventItemSynced, EventItemTombstoned},
		[]any{
			ItemSyncedPayload{
				Source:    "github",
				SourceID:  "sc-resurrect",
				Type:      "PushEvent",
				CreatedAt: 1,
				UpdatedAt: 1,
			},
			ItemTombstonedPayload{Source: "github", SourceID: "sc-resurrect", Reason: "upstream_gone", TombstonedAt: 2},
		})

	scenario.Given[syncCmd, SyncItemState](t, fold, InitialState, tombstoned...).
		When(syncCmd{item: dataItemForScenario("sc-resurrect", 2)}, decideForScenario).
		Then(EventItemSynced)
}

// TestScenario_ConflictLocalWins: with a resolver preferring local, the
// conflict event records the local winner and the sync event follows.
func TestScenario_ConflictLocalWins(t *testing.T) {
	t.Parallel()

	existing := mustScenarioEvents(
		t,
		"sc-conflict",
		event.Version(1),
		[]event.Type{EventItemSynced},
		[]any{
			ItemSyncedPayload{Source: "github", SourceID: "sc-conflict", Type: "PushEvent", CreatedAt: 1, UpdatedAt: 1},
		},
	)

	scenario.Given[syncCmd, SyncItemState](t, fold, InitialState, existing...).
		When(syncCmd{item: dataItemForScenario("sc-conflict", 2), resolver: localWinsResolver{}}, decideForScenario).
		Then(EventItemConflictFound, EventItemSynced)
}

// TestScenario_NewItemEmitsSynced: a brand-new aggregate with no history
// emits exactly one ItemSynced on first sync.
func TestScenario_NewItemEmitsSynced(t *testing.T) {
	t.Parallel()

	scenario.Given[syncCmd, SyncItemState](t, fold, InitialState).
		When(syncCmd{item: dataItemForScenario("sc-new", 0)}, decideForScenario).
		Then(EventItemSynced)
}

// localWinsResolver prefers the local item in conflicts.
type localWinsResolver struct{}

func (localWinsResolver) Resolve(conflict *crdt.Conflict[*model.Item]) (*model.Item, error) {
	return conflict.Local, nil
}

func dataItemForScenario(sourceID string, updatedAtOffset int64) *model.Item {
	now := time.Now().UTC().Add(time.Duration(updatedAtOffset) * time.Second)

	return &model.Item{
		ID:          id.NewItemID(),
		SourceID:    id.NewSourceID(sourceID),
		Source:      id.NewProviderID("github"),
		Type:        id.NewEventTypeID("PushEvent"),
		Attributes:  map[string]string{"actor_login": "scen"},
		ContentHash: "scenario-hash",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
