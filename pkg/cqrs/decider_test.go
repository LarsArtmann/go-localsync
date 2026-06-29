package cqrs

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v3"
	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestAggregateID_Deterministic(t *testing.T) {
	t.Parallel()

	aggID := AggregateID("github", id.NewExternalID("123"))

	if aggID != AggregateID("github", id.NewExternalID("123")) {
		t.Error("same inputs must produce same AggregateID")
	}
}

func TestAggregateID_DifferentInputs(t *testing.T) {
	t.Parallel()

	if AggregateID(
		"github",
		id.NewExternalID("123"),
	) == AggregateID(
		"github",
		id.NewExternalID("456"),
	) {
		t.Error("different inputs must produce different AggregateIDs")
	}
}

func TestFold_ItemSynced(t *testing.T) {
	t.Parallel()

	payload := testSyncedPayload("123", "PushEvent")

	evt := mustNewTestEvent(EventItemSynced, payload)

	state, err := fold(InitialState, evt)
	testutil.MustNoError(t, err)
	if state.Item == nil {
		t.Fatal("expected non-nil Item")
	}
	testutil.AssertEqual(t, state.Item.Source.Get(), "github", "Source")
	testutil.AssertExternalID(t, state.Item, "123")
	testutil.AssertType(t, state.Item, "PushEvent")
	if state.IsTombstoned() {
		t.Error("expected not tombstoned")
	}
}

func TestFold_ItemSyncedOverwritesState(t *testing.T) {
	t.Parallel()

	existing := testActiveState("123", "PushEvent")

	updatedPayload := testSyncedPayload("123", "IssueEvent")

	evt := mustNewTestEvent(EventItemSynced, updatedPayload)

	state, err := fold(existing, evt)
	testutil.MustNoError(t, err)

	testutil.AssertType(t, state.Item, "IssueEvent")
}

func TestDecideSync_Fold_PreservesItemID(t *testing.T) {
	t.Parallel()

	dataItem := testDataItem("123", "PushEvent")
	dataItem.ID = id.NewItemID()
	originalID := dataItem.ID.String()

	events, err := decideSync(dataItem, nil, nil)(InitialState, 0)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 1)

	var payload ItemSyncedPayload
	if err := json.Unmarshal(events[0].Payload(), &payload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload.ItemID != originalID {
		t.Errorf("expected ItemID=%s, got %s", originalID, payload.ItemID)
	}

	state, err := fold(InitialState, events[0])
	testutil.MustNoError(t, err)
	if state.Item == nil {
		t.Fatal("expected non-nil Item")
	}
	if state.Item.ID.String() != originalID {
		t.Errorf("expected ID=%s, got %s", originalID, state.Item.ID.String())
	}
}

func TestFold_ItemTombstoned(t *testing.T) {
	t.Parallel()

	existing := testActiveState("123", "")

	evt := mustNewTestEvent(EventItemTombstoned, ItemTombstonedPayload{
		Source:       "github",
		SourceID:     "123",
		Reason:       string(model.ReasonUpstreamGone),
		TombstonedAt: time.Now().UnixNano(),
	})

	state, err := fold(existing, evt)
	testutil.MustNoError(t, err)

	if !state.IsTombstoned() {
		t.Fatal("expected tombstoned state")
	}

	if state.Item == nil {
		t.Fatal("tombstoned state must keep the item for history")
	}

	if state.Item.Tombstone.Reason != model.ReasonUpstreamGone {
		t.Errorf("expected reason upstream_gone, got %s", state.Item.Tombstone.Reason)
	}
}

func TestFold_ItemConflictFound(t *testing.T) {
	t.Parallel()

	existing := testActiveState("123", "PushEvent")

	evt := mustNewTestEvent(EventItemConflictFound, ItemConflictFoundPayload{
		Source: "github", SourceID: "123", Winner: "remote",
	})

	state, err := fold(existing, evt)
	testutil.MustNoError(t, err)

	testutil.AssertType(t, state.Item, "PushEvent")
}

func TestFold_UnknownEventType(t *testing.T) {
	t.Parallel()

	evt := mustNewTestEvent(event.Type("unknown"), map[string]string{"test": "data"})

	_, err := fold(InitialState, evt)
	if err == nil {
		t.Fatal("expected error for unknown event type")
	}
}

func TestDecideSync_NewItem(t *testing.T) {
	t.Parallel()

	item := testDataItem("123", "PushEvent")

	events, err := decideSync(item, nil, nil)(InitialState, 0)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 1)
	assertEventType(t, events[0], EventItemSynced)
}

func TestDecideSync_UnchangedItem(t *testing.T) {
	t.Parallel()

	now := time.Now()
	item := testItem("123", "PushEvent")
	item.UpdatedAt = now

	state := SyncItemState{
		Item: &model.Item{
			ExternalID: id.NewExternalID("123"),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			ActorLogin: id.NewActorLogin("testuser"),
			RepoName:   id.NewRepoID("owner/repo"),
			UpdatedAt:  now,
		},
	}

	events, err := decideSync(toDataItem(item), nil, nil)(state, 1)
	testutil.MustNoError(t, err)
	if events != nil {
		t.Errorf("unchanged item produces no events, got %d", len(events))
	}
}

func TestDecideSync_ConflictResolution(t *testing.T) {
	t.Parallel()

	item := testDataItem("123", "PushEvent")
	item.UpdatedAt = time.Now().Add(time.Hour)

	state := testStateWithTimestamp("123", "PushEvent", time.Now())

	events, err := decideSync(item, nil, nil)(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 2)
	assertEventType(t, events[0], EventItemConflictFound)
	assertEventType(t, events[1], EventItemSynced)
}

// TestDecideSync_ConflictTimestampIsUTC locks the invariant that the
// conflict-detection timestamp handed to a resolver is UTC, matching every
// other persisted timestamp in the system. A non-UTC host would otherwise feed
// a local-time value to custom resolvers that read Conflict.Timestamp.
func TestDecideSync_ConflictTimestampIsUTC(t *testing.T) {
	t.Parallel()

	local := testDataItem("123", "PushEvent")
	local.UpdatedAt = time.Now().Add(-time.Hour).UTC()

	remote := testDataItem("123", "PushEvent")
	remote.UpdatedAt = time.Now().UTC()

	state := SyncItemState{Item: local}

	var captured time.Time

	capturing := conflictCapturingResolver{onResolve: func(c *crdt.Conflict[*model.Item]) {
		captured = c.Timestamp
	}}

	_, err := decideSync(remote, nil, capturing)(state, 1)
	testutil.MustNoError(t, err)

	if captured.IsZero() {
		t.Fatal("resolver was never called, conflict not detected")
	}

	if captured.Location() != time.UTC {
		t.Errorf("Conflict.Timestamp must be UTC, got %s", captured.Location())
	}
}

// conflictCapturingResolver is a test ConflictResolver that records the
// conflict and lets the remote side win.
type conflictCapturingResolver struct {
	onResolve func(*crdt.Conflict[*model.Item])
}

func (r conflictCapturingResolver) Resolve(c *crdt.Conflict[*model.Item]) (*model.Item, error) {
	r.onResolve(c)
	return c.Remote, nil
}

func TestDecideSync_ConflictTimestamps(t *testing.T) {
	t.Parallel()

	localTime := time.Now().Truncate(time.Millisecond)
	remoteTime := localTime.Add(2 * time.Hour)

	item := testDataItem("123", "PushEvent")
	item.UpdatedAt = remoteTime

	state := testStateWithTimestamp("123", "PushEvent", localTime)

	events, err := decideSync(item, nil, nil)(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 2)

	var conflictPayload ItemConflictFoundPayload
	if err := json.Unmarshal(events[0].Payload(), &conflictPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conflictPayload.LocalUpdatedAt == conflictPayload.RemoteUpdatedAt {
		t.Error("LocalUpdatedAt and RemoteUpdatedAt must differ in conflict")
	}
	if conflictPayload.LocalUpdatedAt != localTime.UnixNano() {
		t.Errorf("LocalUpdatedAt must come from existing state")
	}
	if conflictPayload.RemoteUpdatedAt != remoteTime.UnixNano() {
		t.Errorf("RemoteUpdatedAt must come from incoming item")
	}
}

func TestDecideSync_ResurrectTombstonedItem(t *testing.T) {
	t.Parallel()

	item := testDataItem("123", "PushEvent")

	state := testTombstonedState("123")

	events, err := decideSync(item, nil, nil)(state, 2)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 1)
	assertEventType(t, events[0], EventItemSynced)
}

func TestDecideTombstone_ActiveItem(t *testing.T) {
	t.Parallel()

	state := testActiveState("123", "")

	events, err := decideTombstone("github", id.NewExternalID("123"), model.ReasonUpstreamGone)(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 1)
	assertEventType(t, events[0], EventItemTombstoned)
}

func TestDecideTombstone_AlreadyTombstoned(t *testing.T) {
	t.Parallel()

	state := testTombstonedState("123")

	events, err := decideTombstone("github", id.NewExternalID("123"), model.ReasonUpstreamGone)(state, 1)
	testutil.MustNoError(t, err)
	if events != nil {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestDecideTombstone_NewItem(t *testing.T) {
	t.Parallel()

	events, err := decideTombstone("github", id.NewExternalID("123"), model.ReasonUpstreamGone)(InitialState, 0)
	testutil.MustNoError(t, err)
	if events != nil {
		t.Errorf("expected no events, got %d", len(events))
	}
}

func TestSyncItemState_IsNew(t *testing.T) {
	t.Parallel()

	if !InitialState.IsNew() {
		t.Error("expected initial state to be new")
	}

	existing := SyncItemState{Item: &model.Item{}}
	if existing.IsNew() {
		t.Error("expected existing state to not be new")
	}
}

func TestHasChanged(t *testing.T) {
	t.Parallel()

	base := func() *model.Item {
		return &model.Item{
			ExternalID: id.NewExternalID("123"),
			Source:     id.NewProviderID("github"),
			Type:       id.NewEventTypeID("PushEvent"),
			ActorLogin: id.NewActorLogin("octocat"),
			RepoName:   id.NewRepoID("org/repo"),
			RepoURL:    "https://github.com/org/repo",
			UpdatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}
	}

	tests := []struct {
		name string
		mut  func(remote *model.Item)
		want bool
	}{
		{
			name: "identical items",
			mut:  func(_ *model.Item) {},
			want: false,
		},
		{
			name: "UpdatedAt differs",
			mut:  func(r *model.Item) { r.UpdatedAt = r.UpdatedAt.Add(time.Hour) },
			want: true,
		},
		{
			name: "Type differs",
			mut:  func(r *model.Item) { r.Type = id.NewEventTypeID("WatchEvent") },
			want: true,
		},
		{
			name: "ActorLogin differs",
			mut:  func(r *model.Item) { r.ActorLogin = id.NewActorLogin("other") },
			want: true,
		},
		{
			name: "RepoName differs",
			mut:  func(r *model.Item) { r.RepoName = id.NewRepoID("org/other") },
			want: true,
		},
		{
			name: "RepoURL differs",
			mut:  func(r *model.Item) { r.RepoURL = "https://github.com/org/other" },
			want: true,
		},
		{
			name: "only ID fields differ (not tracked)",
			mut:  func(r *model.Item) { r.ExternalID = id.NewExternalID("other") },
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			local := base()
			remote := base()
			tt.mut(remote)

			got := hasChanged(local, remote)
			if got != tt.want {
				t.Errorf("hasChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
