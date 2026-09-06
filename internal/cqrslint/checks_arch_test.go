package cqrslint_test

import (
	"strings"
	"testing"
)

// Tests for the C0011-C0015 architectural checks. Each mutates
// compliantSource() to introduce exactly one violation, so any firing rule
// other than the one under test would be a false positive.

// withImports swaps the single import for a block that also imports "time",
// for fixtures whose violation calls time.Now.
func withTimeImport(source string) string {
	return strings.Replace(source,
		`import "github.com/larsartmann/go-cqrs-lite/event/v4"`,
		"import (\n\t\"time\"\n\n\t\"github.com/larsartmann/go-cqrs-lite/event/v4\"\n)", 1)
}

func TestC0011_SecondProjection(t *testing.T) {
	runMutation(t, "second-eventtypes", mutation{
		old: "func (p *Projector) Handle(evt event.Event) error {\n\tp.mu.Lock()\n\treturn nil\n}",
		new: "func (p *Projector) Handle(evt event.Event) error {\n\tp.mu.Lock()\n\treturn nil\n}\n" +
			"\nfunc (s State) EventTypes() []event.Type {\n\treturn nil\n}",
		rules: []string{"C0011"},
	})
}

func TestC0012_FoldUsesTimeNow(t *testing.T) {
	runFixture(t, fixtureCase{
		name: "fold-time-now",
		files: map[string]string{"cqrs.go": withTimeImport(strings.Replace(compliantSource(),
			"func hasChanged(local, remote *Item) bool {",
			"func foldLegacy(state State, evt event.Event) (State, error) {\n\t_ = time.Now()\n\treturn state, nil\n}\n"+
				"\nfunc hasChanged(local, remote *Item) bool {", 1))},
		wantRules:     []string{"C0012"},
		wantRuleCount: -1,
	})
}

func TestC0012_TimeOutsideFoldCompliant(t *testing.T) {
	runFixture(t, fixtureCase{
		name: "time-outside-fold",
		files: map[string]string{"cqrs.go": withTimeImport(strings.Replace(compliantSource(),
			"func hasChanged(local, remote *Item) bool {",
			"func stampNow() string { return time.Now().String() }\n"+
				"\nfunc hasChanged(local, remote *Item) bool {", 1))},
		wantNoRules:   []string{"C0012"},
		wantRuleCount: -1,
	})
}

func TestC0013_ProjectorWritesEvents(t *testing.T) {
	runMutation(t, "projector-append", mutation{
		old:   "func (p *Projector) Handle(evt event.Event) error {\n\tp.mu.Lock()\n\treturn nil\n}",
		new:   "func (p *Projector) Handle(evt event.Event) error {\n\tstore.Append(evt)\n\tp.mu.Lock()\n\treturn nil\n}",
		rules: []string{"C0013"},
	})
}

func TestC0013_AppendOutsideProjectorCompliant(t *testing.T) {
	runMutation(t, "append-elsewhere", mutation{
		old:    "func makeEvents(aggID string) {",
		new:    "func grow(list []int) []int { return append(list, 1) }\n\nfunc makeEvents(aggID string) {",
		absent: []string{"C0013"},
	})
}

func TestC0014_WireLiteralOutsideOwnerFile(t *testing.T) {
	runFixture(t, fixtureCase{
		name: "wire-literal-drift",
		files: map[string]string{
			"events.go": compliantSource(),
			"other.go":  "package cqrs\n\nfunc staleValue() string {\n\treturn \"sync_item.synced\"\n}\n",
		},
		wantRules:     []string{"C0014"},
		wantRuleCount: 1,
	})
}

func TestC0014_LiteralInOwnerFileCompliant(t *testing.T) {
	runFixture(t, fixtureCase{
		name:          "wire-literal-owner",
		files:         map[string]string{"events.go": compliantSource()},
		wantNoRules:   []string{"C0014"},
		wantRuleCount: 0,
	})
}

func TestC0015_NewEventsInlineTypeLiteral(t *testing.T) {
	runMutation(t, "newevents-inline-type", mutation{
		old:   "event.NewEvents(aggID, aggregateType, 0, nil, nil)",
		new:   `event.NewEvents(aggID, aggregateType, 0, []event.Type{"sync_item.synced"}, nil)`,
		rules: []string{"C0015"},
	})
}

func TestC0015_NewEventsConstTypeCompliant(t *testing.T) {
	runMutation(t, "newevents-const-type", mutation{
		old:    "event.NewEvents(aggID, aggregateType, 0, nil, nil)",
		new:    "event.NewEvents(aggID, aggregateType, 0, []event.Type{EventItemSynced}, nil)",
		absent: []string{"C0015"},
	})
}
