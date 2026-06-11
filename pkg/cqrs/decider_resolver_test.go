package cqrs

import (
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

type pickSideResolver struct {
	pickSide string
}

func (r pickSideResolver) Resolve(c *crdt.Conflict[*model.Item]) (*model.Item, error) {
	if r.pickSide == "local" {
		return c.Local, nil
	}

	return c.Remote, nil
}

type errorResolver struct{}

func (errorResolver) Resolve(_ *crdt.Conflict[*model.Item]) (*model.Item, error) {
	return nil, errors.New("resolver failed")
}

func TestDecideSync_CustomResolver_RemoteWins(t *testing.T) {
	t.Parallel()

	localTime := time.Now().Truncate(time.Millisecond)
	remoteTime := localTime.Add(2 * time.Hour)

	remoteItem := testItem("123", "PushEvent")
	remoteItem.UpdatedAt = remoteTime

	state := testStateWithTimestamp("123", "PushEvent", localTime)

	events, err := DecideSync(ToDataItem(remoteItem), nil, &pickSideResolver{pickSide: "remote"})(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 2)

	assertEventType(t, events[0], EventItemConflictFound)
	assertEventType(t, events[1], EventItemSynced)

	conflictPayload := unmarshalConflictPayload(t, events[0])

	testutil.AssertEqual(t, conflictPayload.Winner, "remote", "winner")

	if conflictPayload.RemoteUpdatedAt != remoteTime.UnixNano() {
		t.Errorf("expected remote timestamp from original remote item")
	}

	syncedPayload := unmarshalSyncedPayload(t, events[1])

	if syncedPayload.UpdatedAt != remoteTime.UnixNano() {
		t.Errorf("expected synced payload to contain remote item data")
	}
}

func TestDecideSync_CustomResolver_LocalWins(t *testing.T) {
	t.Parallel()

	localTime := time.Now().Truncate(time.Millisecond)
	remoteTime := localTime.Add(2 * time.Hour)

	localItem := testStateWithTimestamp("123", "PushEvent", localTime).Item

	remoteItem := testItem("123", "IssueEvent")
	remoteItem.UpdatedAt = remoteTime

	state := SyncItemState{Item: localItem}

	events, err := DecideSync(ToDataItem(remoteItem), nil, &pickSideResolver{pickSide: "local"})(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 2)

	assertEventType(t, events[0], EventItemConflictFound)

	conflictPayload := unmarshalConflictPayload(t, events[0])

	testutil.AssertEqual(t, conflictPayload.Winner, "local", "winner")

	if conflictPayload.LocalUpdatedAt != localTime.UnixNano() {
		t.Errorf("expected local timestamp from existing state")
	}

	if conflictPayload.RemoteUpdatedAt != remoteTime.UnixNano() {
		t.Errorf("expected remote timestamp from incoming item")
	}

	assertEventType(t, events[1], EventItemSynced)

	syncedPayload := unmarshalSyncedPayload(t, events[1])

	if syncedPayload.UpdatedAt != localTime.UnixNano() {
		t.Errorf("expected synced payload to contain local item data")
	}
}

func TestDecideSync_CustomResolver_Error_FallsBackToRemote(t *testing.T) {
	t.Parallel()

	remoteItem := testItem("123", "PushEvent")
	remoteItem.UpdatedAt = time.Now().Add(time.Hour)

	state := testStateWithTimestamp("123", "PushEvent", time.Now())

	events, err := DecideSync(ToDataItem(remoteItem), nil, new(errorResolver))(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 2)

	conflictPayload := unmarshalConflictPayload(t, events[0])

	if conflictPayload.Winner != "remote" {
		t.Errorf("expected fallback to remote on resolver error, got %s", conflictPayload.Winner)
	}
}

func TestDecideSync_LWWResolver_RemoteNewer(t *testing.T) {
	t.Parallel()

	resolver := newUpdatedAtLWWResolver(t)

	localTime := time.Now().Truncate(time.Millisecond)
	remoteTime := localTime.Add(2 * time.Hour)

	remoteItem := testItem("123", "PushEvent")
	remoteItem.UpdatedAt = remoteTime

	state := testStateWithTimestamp("123", "PushEvent", localTime)

	events, err := DecideSync(ToDataItem(remoteItem), nil, resolver)(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 2)

	conflictPayload := unmarshalConflictPayload(t, events[0])

	if conflictPayload.Winner != "remote" {
		t.Errorf("LWW with newer remote should pick remote, got %s", conflictPayload.Winner)
	}
}

func TestDecideSync_LWWResolver_LocalNewer(t *testing.T) {
	t.Parallel()

	resolver := newUpdatedAtLWWResolver(t)

	localTime := testFutureNow(3 * time.Hour)
	remoteTime := time.Now().Truncate(time.Millisecond)

	remoteItem := testItem("123", "PushEvent")
	remoteItem.UpdatedAt = remoteTime

	state := testStateWithTimestamp("123", "PushEvent", localTime)

	events, err := DecideSync(ToDataItem(remoteItem), nil, resolver)(state, 1)
	testutil.MustNoError(t, err)
	testutil.RequireLen(t, events, 2)

	conflictPayload := unmarshalConflictPayload(t, events[0])

	if conflictPayload.Winner != "local" {
		t.Errorf("LWW with newer local should pick local, got %s", conflictPayload.Winner)
	}
}
