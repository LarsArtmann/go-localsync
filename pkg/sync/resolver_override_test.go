package sync

import (
	"context"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// localWins always prefers the local item; remoteWins always the remote.
type localWins struct{}

func (localWins) Resolve(c *crdt.Conflict[*model.Item]) (*model.Item, error) {
	return c.Local, nil
}

type remoteWins struct{}

func (remoteWins) Resolve(c *crdt.Conflict[*model.Item]) (*model.Item, error) {
	return c.Remote, nil
}

// resolverObservingStore wraps the CQRS stack so the test can read the final
// stored item after a conflict decision.
func newResolverTestStack(t *testing.T, cfg crdt.ConflictResolver[*model.Item]) *cqrs.CQRSStack {
	t.Helper()

	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{Backend: "memory", ConflictResolver: cfg})
	testutil.MustNoError(t, err)
	t.Cleanup(func() { _ = stack.Close() })

	return stack
}

func conflictPair(updatedAt time.Time) *provider.Item {
	now := updatedAt

	return &provider.Item{
		ID:         id.NewItemID(),
		ExternalID: id.NewExternalID("res-1"),
		Source:     id.NewProviderID("github"),
		Type:       id.NewEventTypeID("PushEvent"),
		Attributes: map[string]string{"actor_login": "resolver-test"},
		CreatedAt:  now,
		UpdatedAt:  now,
		RawJSON:    []byte(`{"resolver":true}`),
	}
}

// TestSyncOptions_ConflictResolver_OverridesConfig proves per-sync precedence:
// the stack is configured remote-wins, the option says local-wins, and the
// stored item keeps the LOCAL content hash after the conflict.
func TestSyncOptions_ConflictResolver_OverridesConfig(t *testing.T) {
	t.Parallel()

	stack := newResolverTestStack(t, remoteWins{})
	ctx := context.Background()

	first := conflictPair(time.Now())
	testutil.MustNoError(t, stack.SyncItem(ctx, first))
	waitForItem(t, stack)

	localBefore, err := stack.Get(ctx, "github", first.ExternalID)
	testutil.MustNoError(t, err)

	// Second sync with a CHANGED item while the option forces local-wins:
	// the conflict must keep the stored (local) version.
	changed := conflictPair(time.Now().Add(time.Hour))
	changed.RawJSON = []byte(`{"resolver":true,"changed":true}`)

	syncer := NewSyncer(
		&testutil.MockProvider{Items: []*provider.Item{changed}},
		stack,
		log.Default(),
	)

	result, err := syncer.Sync(ctx, &SyncOptions{
		Source:           "github",
		MaxPages:         1,
		ConflictResolver: localWins{},
	})
	testutil.MustNoError(t, err)

	if result.Fetched != 1 {
		t.Fatalf("expected 1 fetched item, got %+v", result)
	}

	localAfter, err := stack.Get(ctx, "github", first.ExternalID)
	testutil.MustNoError(t, err)

	if localAfter.ContentHash != localBefore.ContentHash {
		t.Errorf(
			"per-sync local-wins resolver must keep the local item: hash %q -> %q",
			localBefore.ContentHash, localAfter.ContentHash,
		)
	}
}

// TestSyncOptions_ConflictResolver_NilUsesConfigDefault pins the fallback:
// without the option, the stack-configured strategy (remote-wins default)
// applies.
func TestSyncOptions_ConflictResolver_NilUsesConfigDefault(t *testing.T) {
	t.Parallel()

	stack := newResolverTestStack(t, nil)
	ctx := context.Background()

	first := conflictPair(time.Now())
	testutil.MustNoError(t, stack.SyncItem(ctx, first))
	waitForItem(t, stack)

	changed := conflictPair(time.Now().Add(time.Hour))
	changed.RawJSON = []byte(`{"resolver":true,"changed":true}`)

	syncer := NewSyncer(
		&testutil.MockProvider{Items: []*provider.Item{changed}},
		stack,
		log.Default(),
	)

	_, err := syncer.Sync(ctx, &SyncOptions{Source: "github", MaxPages: 1})
	testutil.MustNoError(t, err)

	after, err := stack.Get(ctx, "github", first.ExternalID)
	testutil.MustNoError(t, err)

	if after.ContentHash.IsZero() {
		t.Fatal("expected a stored item")
	}
}

func waitForItem(t *testing.T, stack *cqrs.CQRSStack) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		count, err := stack.Count(context.Background(), model.ItemFilter{})
		testutil.MustNoError(t, err)

		if count == 1 {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for the item to project")
}
