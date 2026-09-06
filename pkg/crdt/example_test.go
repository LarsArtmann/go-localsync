package crdt_test

import (
	"fmt"
	"time"

	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
)

func ExampleLWWResolver() {
	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	resolver, err := crdt.NewLWWResolver[*model.Item](func(item *model.Item) time.Time {
		return item.UpdatedAt
	})
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	local := &model.Item{
		SourceID: id.NewSourceID("123"),
		Source:     id.NewProviderID("github"),
		UpdatedAt:  ts,
	}

	remote := &model.Item{
		SourceID: id.NewSourceID("123"),
		Source:     id.NewProviderID("github"),
		UpdatedAt:  ts.Add(time.Hour),
	}

	conflict := &crdt.Conflict[*model.Item]{
		Local:  local,
		Remote: remote,
	}

	winner, err := resolver.Resolve(conflict)
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	fmt.Println("winner updated at:", winner.UpdatedAt.Format(time.RFC3339))

	// Output:
	// winner updated at: 2026-01-15T11:30:00Z
}
