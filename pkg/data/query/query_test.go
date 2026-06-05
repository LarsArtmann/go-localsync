package query

import (
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

// testItem is a minimal type that satisfies all criterion interface constraints.
type testItem struct {
	source     id.ProviderID
	itemType   id.EventTypeID
	actor      id.ActorID
	repo       id.RepoID
	createdAt  time.Time
	updatedAt  time.Time
}

func (t testItem) GetSource() id.ProviderID     { return t.source }
func (t testItem) GetType() id.EventTypeID      { return t.itemType }
func (t testItem) GetActorLogin() id.ActorID    { return t.actor }
func (t testItem) GetRepoName() id.RepoID       { return t.repo }
func (t testItem) GetCreatedAt() time.Time      { return t.createdAt }
func (t testItem) GetUpdatedAt() time.Time      { return t.updatedAt }

func TestHasSource(t *testing.T) {
	t.Parallel()

	c := HasSource[testItem](id.NewProviderID("github"))

	if !c.Match(testItem{source: id.NewProviderID("github")}) {
		t.Error("expected match for same source")
	}

	if c.Match(testItem{source: id.NewProviderID("gitlab")}) {
		t.Error("expected no match for different source")
	}

	clause, args := c.ToSQL()
	if clause != "source = ?" {
		t.Errorf("SQL clause = %q, want %q", clause, "source = ?")
	}

	if len(args) != 1 || args[0] != "github" {
		t.Errorf("SQL args = %v, want [github]", args)
	}
}

func TestHasType(t *testing.T) {
	t.Parallel()

	c := HasType[testItem](id.NewEventTypeID("PushEvent"))

	if !c.Match(testItem{itemType: id.NewEventTypeID("PushEvent")}) {
		t.Error("expected match for same type")
	}

	if c.Match(testItem{itemType: id.NewEventTypeID("IssueEvent")}) {
		t.Error("expected no match for different type")
	}
}

func TestHasActor(t *testing.T) {
	t.Parallel()

	c := HasActor[testItem](id.NewActorID("octocat"))

	if !c.Match(testItem{actor: id.NewActorID("octocat")}) {
		t.Error("expected match for same actor")
	}

	if c.Match(testItem{actor: id.NewActorID("torvalds")}) {
		t.Error("expected no match for different actor")
	}
}

func TestHasRepo(t *testing.T) {
	t.Parallel()

	c := HasRepo[testItem](id.NewRepoID("org/repo"))

	if !c.Match(testItem{repo: id.NewRepoID("org/repo")}) {
		t.Error("expected match for same repo")
	}

	if c.Match(testItem{repo: id.NewRepoID("other/repo")}) {
		t.Error("expected no match for different repo")
	}
}

func TestCreatedAfter(t *testing.T) {
	t.Parallel()

	cutoff := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	c := CreatedAfter[testItem](cutoff)

	if !c.Match(testItem{createdAt: cutoff.Add(time.Hour)}) {
		t.Error("expected match for item after cutoff")
	}

	if c.Match(testItem{createdAt: cutoff.Add(-time.Hour)}) {
		t.Error("expected no match for item before cutoff")
	}
}

func TestAndCriterion(t *testing.T) {
	t.Parallel()

	c1 := HasSource[testItem](id.NewProviderID("github"))
	c2 := HasType[testItem](id.NewEventTypeID("PushEvent"))
	and := And(c1, c2)

	if !and.Match(testItem{source: id.NewProviderID("github"), itemType: id.NewEventTypeID("PushEvent")}) {
		t.Error("expected match when both criteria match")
	}

	if and.Match(testItem{source: id.NewProviderID("github"), itemType: id.NewEventTypeID("IssueEvent")}) {
		t.Error("expected no match when one criterion fails")
	}

	if and.Match(testItem{source: id.NewProviderID("gitlab"), itemType: id.NewEventTypeID("PushEvent")}) {
		t.Error("expected no match when other criterion fails")
	}

	clause, args := and.ToSQL()
	if clause != "(source = ?) AND (type = ?)" {
		t.Errorf("SQL clause = %q", clause)
	}

	if len(args) != 2 {
		t.Errorf("SQL args count = %d, want 2", len(args))
	}
}

func TestAndEmpty(t *testing.T) {
	t.Parallel()

	and := And[testItem]()

	if !and.Match(testItem{}) {
		t.Error("expected empty And to match everything")
	}

	clause, _ := and.ToSQL()
	if clause != "1=1" {
		t.Errorf("empty And SQL = %q, want 1=1", clause)
	}
}

func TestOrCriterion(t *testing.T) {
	t.Parallel()

	c1 := HasSource[testItem](id.NewProviderID("github"))
	c2 := HasSource[testItem](id.NewProviderID("gitlab"))
	or := Or(c1, c2)

	if !or.Match(testItem{source: id.NewProviderID("github")}) {
		t.Error("expected match for first criterion")
	}

	if !or.Match(testItem{source: id.NewProviderID("gitlab")}) {
		t.Error("expected match for second criterion")
	}

	if or.Match(testItem{source: id.NewProviderID("bitbucket")}) {
		t.Error("expected no match for neither criterion")
	}

	clause, _ := or.ToSQL()
	if clause != "(source = ?) OR (source = ?)" {
		t.Errorf("SQL clause = %q", clause)
	}
}

func TestOrEmpty(t *testing.T) {
	t.Parallel()

	or := Or[testItem]()

	if or.Match(testItem{}) {
		t.Error("expected empty Or to match nothing")
	}

	clause, _ := or.ToSQL()
	if clause != "1=0" {
		t.Errorf("empty Or SQL = %q, want 1=0", clause)
	}
}

func TestNotCriterion(t *testing.T) {
	t.Parallel()

	c := HasSource[testItem](id.NewProviderID("github"))
	notC := Not(c)

	if notC.Match(testItem{source: id.NewProviderID("github")}) {
		t.Error("expected Not to invert match")
	}

	if !notC.Match(testItem{source: id.NewProviderID("gitlab")}) {
		t.Error("expected Not to match inverse")
	}

	clause, _ := notC.ToSQL()
	if clause != "NOT (source = ?)" {
		t.Errorf("SQL clause = %q", clause)
	}
}

func TestQueryMatch(t *testing.T) {
	t.Parallel()

	q := Query[testItem]{
		Criteria: []Criterion[testItem]{
			HasSource[testItem](id.NewProviderID("github")),
			HasType[testItem](id.NewEventTypeID("PushEvent")),
		},
	}

	if !q.Match(testItem{source: id.NewProviderID("github"), itemType: id.NewEventTypeID("PushEvent")}) {
		t.Error("expected match for item satisfying all criteria")
	}

	if q.Match(testItem{source: id.NewProviderID("gitlab"), itemType: id.NewEventTypeID("PushEvent")}) {
		t.Error("expected no match for item failing one criterion")
	}
}

func TestQuerySort(t *testing.T) {
	t.Parallel()

	items := []testItem{
		{createdAt: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
		{createdAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{createdAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)},
	}

	q := Query[testItem]{
		OrderBy: []Order[testItem]{ByCreatedAtDesc[testItem]()},
	}

	q.Sort(items)

	if !items[0].createdAt.Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected first item to be latest")
	}

	if !items[2].createdAt.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected last item to be earliest")
	}
}

func TestQueryBuilder(t *testing.T) {
	t.Parallel()

	q := NewBuilder[testItem]().
		Where(HasSource[testItem](id.NewProviderID("github"))).
		Where(HasType[testItem](id.NewEventTypeID("PushEvent"))).
		OrderBy(ByCreatedAtDesc[testItem]()).
		Limit(10).
		Offset(5).
		Build()

	if len(q.Criteria) != 2 {
		t.Errorf("criteria count = %d, want 2", len(q.Criteria))
	}

	if len(q.OrderBy) != 1 {
		t.Errorf("orderBy count = %d, want 1", len(q.OrderBy))
	}

	if q.Limit != 10 {
		t.Errorf("limit = %d, want 10", q.Limit)
	}

	if q.Offset != 5 {
		t.Errorf("offset = %d, want 5", q.Offset)
	}
}

func TestNewPage(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3}
	page := NewPage(items, 100, 10, 0)

	if len(page.Items) != 3 {
		t.Errorf("items count = %d, want 3", len(page.Items))
	}

	if page.Total != 100 {
		t.Errorf("total = %d, want 100", page.Total)
	}

	if !page.HasMore {
		t.Error("expected HasMore = true")
	}
}

func TestNewPageNoMore(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3}
	page := NewPage(items, 3, 10, 0)

	if page.HasMore {
		t.Error("expected HasMore = false")
	}
}

func TestEmptyPage(t *testing.T) {
	t.Parallel()

	page := EmptyPage[int]()

	if page.Items == nil {
		t.Error("expected non-nil Items slice")
	}

	if page.Total != 0 {
		t.Errorf("total = %d, want 0", page.Total)
	}
}

func TestMapPage(t *testing.T) {
	t.Parallel()

	page := NewPage([]int{1, 2, 3}, 100, 10, 0)
	mapped := MapPage(page, func(n int) string {
		return string(rune('a' + n - 1))
	})

	if len(mapped.Items) != 3 {
		t.Fatalf("mapped items count = %d, want 3", len(mapped.Items))
	}

	if mapped.Items[0] != "a" || mapped.Items[1] != "b" || mapped.Items[2] != "c" {
		t.Errorf("mapped items = %v, want [a b c]", mapped.Items)
	}

	if mapped.Total != 100 {
		t.Error("expected Total to be preserved")
	}

	if !mapped.HasMore {
		t.Error("expected HasMore to be preserved")
	}
}
