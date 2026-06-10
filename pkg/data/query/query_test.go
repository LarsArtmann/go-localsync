package query

import (
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
)

type testItem struct {
	source    id.ProviderID
	itemType  id.EventTypeID
	actor     id.ActorID
	repo      id.RepoID
	createdAt time.Time
	updatedAt time.Time
}

func (t testItem) GetSource() id.ProviderID  { return t.source }
func (t testItem) GetType() id.EventTypeID   { return t.itemType }
func (t testItem) GetActorLogin() id.ActorID { return t.actor }
func (t testItem) GetRepoName() id.RepoID    { return t.repo }
func (t testItem) GetCreatedAt() time.Time   { return t.createdAt }
func (t testItem) GetUpdatedAt() time.Time   { return t.updatedAt }

func TestFieldCriteria(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		c       Criterion[testItem]
		match   testItem
		noMatch testItem
		sql     string
		sqlArg  string
	}{
		{
			"HasSource", HasSource[testItem](id.NewProviderID("github")),
			testItem{source: id.NewProviderID("github")},
			testItem{source: id.NewProviderID("gitlab")},
			"source = ?", "github",
		},
		{
			"HasType", HasType[testItem](id.NewEventTypeID("PushEvent")),
			testItem{itemType: id.NewEventTypeID("PushEvent")},
			testItem{itemType: id.NewEventTypeID("IssueEvent")},
			"", "",
		},
		{
			"HasActor", HasActor[testItem](id.NewActorID("octocat")),
			testItem{actor: id.NewActorID("octocat")},
			testItem{actor: id.NewActorID("torvalds")},
			"", "",
		},
		{
			"HasRepo", HasRepo[testItem](id.NewRepoID("org/repo")),
			testItem{repo: id.NewRepoID("org/repo")},
			testItem{repo: id.NewRepoID("other/repo")},
			"", "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !tc.c.Match(tc.match) {
				t.Error("expected match")
			}
			if tc.c.Match(tc.noMatch) {
				t.Error("expected no match")
			}
			if tc.sql != "" {
				clause, args := tc.c.ToSQL()
				if clause != tc.sql {
					t.Errorf("SQL clause = %q, want %q", clause, tc.sql)
				}
				if len(args) != 1 || args[0] != tc.sqlArg {
					t.Errorf("SQL args = %v, want [%s]", args, tc.sqlArg)
				}
			}
		})
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

	and := And(HasSource[testItem](id.NewProviderID("github")), HasType[testItem](id.NewEventTypeID("PushEvent")))

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

	or := Or(HasSource[testItem](id.NewProviderID("github")), HasSource[testItem](id.NewProviderID("gitlab")))

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

	notC := Not(HasSource[testItem](id.NewProviderID("github")))

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

	q := Query[testItem]{OrderBy: []Order[testItem]{ByCreatedAtDesc[testItem]()}}
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

	t.Run("has_more", func(t *testing.T) {
		t.Parallel()
		page := NewPage([]int{1, 2, 3}, 100, 10, 0)
		if len(page.Items) != 3 {
			t.Errorf("items = %d, want 3", len(page.Items))
		}
		if !page.HasMore {
			t.Error("expected HasMore")
		}
	})

	t.Run("no_more", func(t *testing.T) {
		t.Parallel()
		page := NewPage([]int{1, 2, 3}, 3, 10, 0)
		if page.HasMore {
			t.Error("expected no HasMore")
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		page := EmptyPage[int]()
		if page.Items == nil {
			t.Error("expected non-nil Items")
		}
		if page.Total != 0 {
			t.Errorf("total = %d, want 0", page.Total)
		}
	})

	t.Run("map_preserves_metadata", func(t *testing.T) {
		t.Parallel()
		page := NewPage([]int{1, 2, 3}, 100, 10, 0)
		mapped := MapPage(page, func(n int) string { return string(rune('a' + n - 1)) }) //nolint:gosec // test: n is 1-3
		if mapped.Items[0] != "a" || mapped.Items[2] != "c" {
			t.Errorf("mapped = %v", mapped.Items)
		}
		if mapped.Total != 100 || !mapped.HasMore {
			t.Error("expected metadata preserved")
		}
	})
}
