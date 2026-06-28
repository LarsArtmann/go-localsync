package cqrs_test

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// exampleProvider is the smallest contract a consumer implements: the SDK ships no
// provider, so this is how any data source (GitHub, GitLab, …) plugs in.
type exampleProvider struct{ items []*provider.Item }

func (exampleProvider) Name() string { return "example" }

func (p exampleProvider) Fetch(context.Context, *provider.FetchOptions) (*provider.FetchResult, error) {
	return &provider.FetchResult{Items: p.items}, nil
}

func (p exampleProvider) FetchAll(context.Context, string, int) (*provider.FetchResult, error) {
	return &provider.FetchResult{Items: p.items}, nil
}

func (exampleProvider) GetRateLimit(context.Context) (*provider.RateLimitInfo, error) {
	return nil, nil //nolint:nilnil // Provider contract: nil means "no rate limiting"
}

// ExampleSyncer shows the full pull-mirror loop end to end: build a CQRS stack, wire a
// provider into a Syncer, sync, then read the projected items back.
func ExampleSyncer() {
	ctx := context.Background()

	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	if err != nil {
		panic(err)
	}
	defer func() { _ = stack.Close() }()

	now := time.Date(2026, 6, 28, 9, 0, 0, 0, time.UTC)
	src := id.NewProviderID("example")
	p := exampleProvider{items: []*provider.Item{
		{
			ID:         id.NewItemID(),
			ExternalID: id.NewExternalID("evt-1"),
			Source:     src,
			Type:       id.NewEventTypeID("PushEvent"),
			ActorLogin: id.NewActorLogin("octocat"),
			RepoName:   id.NewRepoID("octocat/Hello-World"),
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		{
			ID:         id.NewItemID(),
			ExternalID: id.NewExternalID("evt-2"),
			Source:     src,
			Type:       id.NewEventTypeID("WatchEvent"),
			ActorLogin: id.NewActorLogin("octocat"),
			RepoName:   id.NewRepoID("octocat/Hello-World"),
			CreatedAt:  now.Add(time.Hour),
			UpdatedAt:  now.Add(time.Hour),
		},
	}}

	syncer := synclib.NewSyncer(p, stack, nil)
	result, err := syncer.Sync(ctx, &synclib.SyncOptions{Source: "example", MaxPages: 1})
	if err != nil {
		panic(err)
	}

	fmt.Printf("fetched=%d skipped=%d errors=%d\n", result.Fetched, result.Skipped, result.Errors)

	items, err := stack.List(ctx, model.ItemFilter{Source: &src})
	if err != nil {
		panic(err)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ExternalID.Get() < items[j].ExternalID.Get()
	})

	for _, it := range items {
		fmt.Printf("%s %s by %s\n", it.ExternalID.Get(), it.Type.Get(), it.ActorLogin.Get())
	}

	fmt.Println("tombstoned:", items[0].IsTombstoned())

	// Output:
	// fetched=2 skipped=0 errors=0
	// evt-1 PushEvent by octocat
	// evt-2 WatchEvent by octocat
	// tombstoned: false
}

// ExampleTombstoneAndResurrect shows the soft-delete lifecycle: an item is
// tombstoned (hidden from the default view but not destroyed), then a later
// sync of the same item resurrects it automatically — no special-case call.
func ExampleSyncer_tombstoneResurrect() {
	ctx := context.Background()

	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory"})
	if err != nil {
		panic(err)
	}
	defer func() { _ = stack.Close() }()

	src := id.NewProviderID("example")
	now := time.Date(2026, 6, 28, 9, 0, 0, 0, time.UTC)
	p := exampleProvider{items: []*provider.Item{{
		ID:         id.NewItemID(),
		ExternalID: id.NewExternalID("evt-1"),
		Source:     src,
		Type:       id.NewEventTypeID("PushEvent"),
		ActorLogin: id.NewActorLogin("octocat"),
		RepoName:   id.NewRepoID("octocat/Hello-World"),
		CreatedAt:  now,
		UpdatedAt:  now,
	}}}
	syncer := synclib.NewSyncer(p, stack, nil)

	// 1) Sync: the item is live.
	if _, err := syncer.Sync(ctx, &synclib.SyncOptions{Source: "example", MaxPages: 1}); err != nil {
		panic(err)
	}

	// 2) Tombstone it: history is kept, but it leaves the default view.
	if err := stack.TombstoneItem(ctx, "example", id.NewExternalID("evt-1"), model.ReasonUserHidden); err != nil {
		panic(err)
	}

	hidden, _ := stack.List(ctx, model.ItemFilter{Source: &src, IncludeTombstoned: true})
	fmt.Println("after tombstone:", hidden[0].IsTombstoned())

	live, _ := stack.List(ctx, model.ItemFilter{Source: &src})
	fmt.Println("live items:", len(live))

	// 3) Sync the same item again: a sync event means "live", so it resurrects.
	if _, err := syncer.Sync(ctx, &synclib.SyncOptions{Source: "example", MaxPages: 1}); err != nil {
		panic(err)
	}

	resurrected, _ := stack.List(ctx, model.ItemFilter{Source: &src})
	fmt.Println("after re-sync:", resurrected[0].IsTombstoned())

	// Output:
	// after tombstone: true
	// live items: 0
	// after re-sync: false
}
