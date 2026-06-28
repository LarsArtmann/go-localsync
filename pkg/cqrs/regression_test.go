package cqrs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v3"
	cqrsid "github.com/larsartmann/go-cqrs-lite/id/v3"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestRegression_AggregateID_NoDelimiterCollision guards the length-prefixed
// itemKey fix: "github" + ":42" must never collide with "github:" + "42".
func TestRegression_AggregateID_NoDelimiterCollision(t *testing.T) {
	t.Parallel()

	a := itemKey("github", id.NewExternalID(":42"))
	b := itemKey("github:", id.NewExternalID("42"))

	if a == b {
		t.Fatalf("length-prefixed key must distinguish %q from %q", a, b)
	}

	if AggregateID("github", id.NewExternalID(":42")) == AggregateID("github:", id.NewExternalID("42")) {
		t.Error("AggregateIDs must not collide across delimiter injection")
	}
}

// TestRegression_HasChanged_AvatarAndContentHash guards the P1.2 fix: changes
// to ActorAvatarURL alone, and to ContentHash (RawJSON fingerprint) alone, must
// be detected — previously they were silently dropped.
func TestRegression_HasChanged_AvatarAndContentHash(t *testing.T) {
	t.Parallel()

	now := time.Now()
	base := func() *model.Item {
		return &model.Item{
			ExternalID:     id.NewExternalID("1"),
			Source:         id.NewProviderID("github"),
			Type:           id.NewEventTypeID("PushEvent"),
			ActorLogin:     id.NewActorLogin("alice"),
			ActorAvatarURL: "old.png",
			RepoName:       id.NewRepoID("org/repo"),
			UpdatedAt:      now,
			ContentHash:    "hash-1",
		}
	}

	t.Run("avatar-only change", func(t *testing.T) {
		t.Parallel()

		local := base()
		remote := *local
		remote.ActorAvatarURL = "new.png"

		if !hasChanged(local, &remote) {
			t.Error("avatar-only change must be detected")
		}
	})

	t.Run("content-hash-only change", func(t *testing.T) {
		t.Parallel()

		local := base()
		remote := *local
		remote.ContentHash = "hash-2"

		if !hasChanged(local, &remote) {
			t.Error("content-hash-only change must be detected")
		}
	})

	t.Run("empty content-hash is backward compatible", func(t *testing.T) {
		t.Parallel()

		local := base()
		local.ContentHash = ""
		remote := *local
		remote.ContentHash = "hash-2" // differs, but local side empty → must NOT flag via hash

		if hasChanged(local, &remote) {
			t.Error("empty content-hash on one side must not trigger a change")
		}
	})
}

func newVersionedTestEvent(
	t *testing.T,
	eventType event.Type,
	aggID cqrsid.AggregateID,
	version event.Version,
	payload any,
) event.Event {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	evt, err := event.NewEvent(eventType, aggID, aggregateType, version, data)
	if err != nil {
		t.Fatalf("new event: %v", err)
	}

	return evt
}

// TestRegression_Projection_VersionGate_PreventsResurrect guards the P1.5 fix:
// a stale ItemSynced from journal replay (lower version) must not resurrect an
// item that a newer EventItemTombstoned has hidden.
func TestRegression_Projection_VersionGate_PreventsResurrect(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rm := NewMemoryReadModel()
	proj := newProjector(rm)

	aggID := AggregateID("github", id.NewExternalID("1"))
	now := time.Now().UnixNano()

	synced := testSyncedPayload("1", "PushEvent")
	synced.CreatedAt = now
	synced.UpdatedAt = now

	tomb := ItemTombstonedPayload{
		Source:       "github",
		SourceID:     "1",
		Reason:       string(model.ReasonUpstreamGone),
		TombstonedAt: now,
	}

	testutil.MustNoError(t, proj.Handle(ctx, newVersionedTestEvent(t, EventItemSynced, aggID, 1, synced)))
	testutil.MustNoError(t, proj.Handle(ctx, newVersionedTestEvent(t, EventItemTombstoned, aggID, 2, tomb)))

	live, err := rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if live != 0 {
		t.Fatalf("expected live count=0 after tombstone, got %d", live)
	}

	// Stale replay of the v1 sync event must NOT resurrect the tombstoned row.
	testutil.MustNoError(t, proj.Handle(ctx, newVersionedTestEvent(t, EventItemSynced, aggID, 1, synced)))

	live, err = rm.Count(ctx, model.ItemFilter{})
	testutil.MustNoError(t, err)
	if live != 0 {
		t.Fatalf("stale replay resurrected the tombstoned item; live count=%d", live)
	}
}

// TestRegression_Tombstone_Reconcile_UpstreamGone verifies reconciliation
// tombstones items absent from the seen set, and that re-syncing resurrects them.
func TestRegression_Tombstone_Reconcile_UpstreamGone(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	t.Cleanup(func() { _ = stack.Close() })
	ctx := context.Background()

	syncTestItem(t, stack, ctx, "1", "PushEvent")
	syncTestItem(t, stack, ctx, "2", "IssueEvent")
	waitForCount(t, stack, ctx, 2)

	// Provider now only returns item "1": item "2" is gone upstream.
	seen := []model.Key{
		{Source: id.NewProviderID("github"), ExternalID: id.NewExternalID("1")},
	}

	tombstoned, err := stack.Reconcile(ctx, "github", seen)
	testutil.MustNoError(t, err)
	if tombstoned != 1 {
		t.Fatalf("expected 1 item tombstoned, got %d", tombstoned)
	}

	waitForCount(t, stack, ctx, 1) // only "1" remains live

	// Item "2" reappears upstream → resurrect.
	syncTestItem(t, stack, ctx, "2", "IssueEvent")
	waitForCount(t, stack, ctx, 2)
}
