package cqrslint_test

import (
	"strings"
	"testing"
)

// compliantSource returns the source of a package that satisfies every rule.
// Per-rule tests mutate it (via strings.Replace) to introduce exactly one
// violation, so the only findings come from the rule under test. This mirrors
// how staticcheck and golangci-lint test their analyzers.
func compliantSource() string {
	return `package cqrs

import "github.com/larsartmann/go-cqrs-lite/event/v4"

const aggregateType event.AggregateType = "sync_item"

const (
	EventItemSynced        event.Type = "sync_item.synced"
	EventItemConflictFound event.Type = "sync_item.conflict_found"
	EventItemTombstoned    event.Type = "sync_item.tombstoned"
)

type ItemSyncedPayload struct {
	ItemID string ` + "`json:\"itemId\"`" + `
	Source string ` + "`json:\"source\"`" + `
}

type ItemConflictFoundPayload struct {
	Source string ` + "`json:\"source\"`" + `
}

type ItemTombstonedPayload struct {
	Source string ` + "`json:\"source\"`" + `
}

type State struct{}

func fold(state State, evt event.Event) (State, error) {
	switch evt.Type() {
	case EventItemSynced:
		return state, nil
	case EventItemConflictFound:
		return state, nil
	case EventItemTombstoned:
		return state, nil
	}
	return state, nil
}

type Projector struct {
	mu mutex
}

func (p *Projector) EventTypes() []event.Type {
	return []event.Type{EventItemSynced, EventItemConflictFound, EventItemTombstoned}
}

func (p *Projector) Handle(evt event.Event) error {
	p.mu.Lock()
	return nil
}

type mutex struct{}

func (m mutex) Lock() {}

type Item struct {
	ContentHash string
	UpdatedAt   int64
	Type        string
}

func hasChanged(local, remote *Item) bool {
	if local.ContentHash != remote.ContentHash {
		return true
	}
	if local.UpdatedAt != remote.UpdatedAt {
		return true
	}
	return local.Type != remote.Type
}

func makeEvents(aggID string) {
	event.NewEvents(aggID, aggregateType, 0, nil, nil)
}
`
}

// mutation holds a strings.Replace transformation and the rule(s) it should trigger.
type mutation struct {
	old    string
	new    string
	rules  []string // rules that MUST fire after the mutation
	absent []string // rules that must NOT fire (for positive controls)
	count  int      // expected finding count; -1 = skip exact-count check
}

func runMutation(t *testing.T, name string, mut mutation) {
	t.Helper()

	if mut.count == 0 {
		mut.count = -1 // default: don't assert exact count unless explicitly set
	}

	source := strings.Replace(compliantSource(), mut.old, mut.new, 1)

	runFixture(t, fixtureCase{
		name:          name,
		files:         map[string]string{"cqrs.go": source},
		wantRules:     mut.rules,
		wantNoRules:   mut.absent,
		wantRuleCount: mut.count,
	})
}

func TestCompliantBase_HasZeroFindings(t *testing.T) {
	runFixture(t, fixtureCase{
		name:          "compliant-base",
		files:         map[string]string{"cqrs.go": compliantSource()},
		wantRuleCount: 0,
	})
}

func TestC0001_AggregateTypeMissing(t *testing.T) {
	runMutation(t, "missing-aggregate-type", mutation{
		old:   `const aggregateType event.AggregateType = "sync_item"`,
		new:   `const placeholder = 1`,
		rules: []string{"C0001"},
	})
}

func TestC0001_AggregateTypeWrongValue(t *testing.T) {
	runMutation(t, "wrong-aggregate-value", mutation{
		old:   `const aggregateType event.AggregateType = "sync_item"`,
		new:   `const aggregateType event.AggregateType = "other_thing"`,
		rules: []string{"C0001"},
		count: 1,
	})
}

func TestC0002_EventConstMissing(t *testing.T) {
	runMutation(t, "missing-event-const", mutation{
		old:   `EventItemTombstoned    event.Type = "sync_item.tombstoned"`,
		new:   ``,
		rules: []string{"C0002"},
	})
}

func TestC0002_ExtraEventConst(t *testing.T) {
	runMutation(t, "extra-event-const", mutation{
		old: `EventItemTombstoned    event.Type = "sync_item.tombstoned"
)`,
		new: `EventItemTombstoned    event.Type = "sync_item.tombstoned"
	EventItemArchived   event.Type = "sync_item.archived"
)`,
		rules: []string{"C0002"},
		count: 1,
	})
}

func TestC0003_FoldMissingCase(t *testing.T) {
	runMutation(t, "fold-missing-case", mutation{
		old: `	case EventItemTombstoned:
		return state, nil
	}
	return state, nil`,
		new: `	}
	return state, nil`,
		rules: []string{"C0003"},
		count: 1,
	})
}

func TestC0004_ProjectorMissingEvent(t *testing.T) {
	runMutation(t, "projector-missing-event", mutation{
		old:   `return []event.Type{EventItemSynced, EventItemConflictFound, EventItemTombstoned}`,
		new:   `return []event.Type{EventItemSynced, EventItemConflictFound}`,
		rules: []string{"C0004"},
		count: 1,
	})
}

func TestC0005_HasChangedBannedField(t *testing.T) {
	runMutation(t, "haschanged-banned-field", mutation{
		old: `	if local.ContentHash != remote.ContentHash {
		return true
	}`,
		new: `	if local.Title != remote.Title {
		return true
	}`,
		rules: []string{"C0005"},
		count: 2,
	})
}

func TestC0005_HasChangedClean(t *testing.T) {
	// Positive control: only allowed fields → no C0005 finding.
	runMutation(t, "haschanged-clean", mutation{
		old:    `local.ContentHash != remote.ContentHash`,
		new:    `local.ContentHash != remote.ContentHash`, // no-op mutation
		absent: []string{"C0005"},
		count:  0,
	})
}

func TestC0006_QueryDispatcherSelector(t *testing.T) {
	source := strings.Replace(compliantSource(),
		`import "github.com/larsartmann/go-cqrs-lite/event/v4"`,
		`import (
	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/query/v4"
)`, 1) + `
var _ query.Dispatcher
`
	runFixture(t, fixtureCase{
		name:          "query-dispatcher-selector",
		files:         map[string]string{"cqrs.go": source},
		wantRules:     []string{"C0006"},
		wantRuleCount: 1,
	})
}

func TestC0006_QueryDispatcherField(t *testing.T) {
	runMutation(t, "query-dispatcher-field", mutation{
		old:   `type State struct{}`,
		new:   `type State struct{ QueryDispatcher int }`,
		rules: []string{"C0006"},
		count: 1,
	})
}

func TestC0007_SyncActionType(t *testing.T) {
	runMutation(t, "syncaction-type", mutation{
		old: `type State struct{}`,
		new: `type State struct{}
type SyncAction string`,
		rules: []string{"C0007"},
		count: 1,
	})
}

func TestC0007_ItemSyncResultType(t *testing.T) {
	runMutation(t, "itemsyncresult-type", mutation{
		old: `type State struct{}`,
		new: `type State struct{}
type ItemSyncResult struct{}`,
		rules: []string{"C0007"},
		count: 1,
	})
}

func TestC0008_HandleNoLock(t *testing.T) {
	runMutation(t, "handle-no-lock", mutation{
		old: `func (p *Projector) Handle(evt event.Event) error {
	p.mu.Lock()
	return nil
}`,
		new: `func (p *Projector) Handle(evt event.Event) error {
	return nil
}`,
		rules: []string{"C0008"},
		count: 1,
	})
}

func TestC0009_PayloadMissingTag(t *testing.T) {
	runMutation(t, "payload-missing-tag", mutation{
		old:   `ItemID string ` + "`json:\"itemId\"`",
		new:   `ItemID string`,
		rules: []string{"C0009"},
		count: 1,
	})
}

func TestC0010_NewEventsLiteral(t *testing.T) {
	runMutation(t, "newevents-literal", mutation{
		old:   `event.NewEvents(aggID, aggregateType, 0, nil, nil)`,
		new:   `event.NewEvents(aggID, "literal", 0, nil, nil)`,
		rules: []string{"C0010"},
		count: 1,
	})
}
