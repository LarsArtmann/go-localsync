package cqrs

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/crdt"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

type localWinsResolver struct{}

func (localWinsResolver) Resolve(c *crdt.Conflict[*model.Item]) (*model.Item, error) {
	return c.Local, nil
}

type remoteWinsResolver struct{}

func (remoteWinsResolver) Resolve(c *crdt.Conflict[*model.Item]) (*model.Item, error) {
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

	events, err := DecideSync(ToDataItem(remoteItem), nil, new(remoteWinsResolver))(state, 1)
	testutil.MustNoError(t, err)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	assertEventType(t, events[0], EventItemConflictFound)
	assertEventType(t, events[1], EventItemSynced)

	var conflictPayload ItemConflictFoundPayload
	if unmarshalErr := json.Unmarshal(events[0].Payload(), &conflictPayload); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

	if conflictPayload.Winner != "remote" {
		t.Errorf("expected winner=remote, got %s", conflictPayload.Winner)
	}

	if conflictPayload.RemoteUpdatedAt != remoteTime.UnixNano() {
		t.Errorf("expected remote timestamp from original remote item")
	}

	var syncedPayload ItemSyncedPayload
	if unmarshalErr := json.Unmarshal(events[1].Payload(), &syncedPayload); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

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

	events, err := DecideSync(ToDataItem(remoteItem), nil, new(localWinsResolver))(state, 1)
	testutil.MustNoError(t, err)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	assertEventType(t, events[0], EventItemConflictFound)

	var conflictPayload ItemConflictFoundPayload
	if unmarshalErr := json.Unmarshal(events[0].Payload(), &conflictPayload); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

	if conflictPayload.Winner != "local" {
		t.Errorf("expected winner=local, got %s", conflictPayload.Winner)
	}

	if conflictPayload.LocalUpdatedAt != localTime.UnixNano() {
		t.Errorf("expected local timestamp from existing state")
	}

	if conflictPayload.RemoteUpdatedAt != remoteTime.UnixNano() {
		t.Errorf("expected remote timestamp from incoming item")
	}

	assertEventType(t, events[1], EventItemSynced)

	var syncedPayload ItemSyncedPayload
	if unmarshalErr := json.Unmarshal(events[1].Payload(), &syncedPayload); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

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
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	var conflictPayload ItemConflictFoundPayload
	if unmarshalErr := json.Unmarshal(events[0].Payload(), &conflictPayload); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

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
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	var conflictPayload ItemConflictFoundPayload
	if unmarshalErr := json.Unmarshal(events[0].Payload(), &conflictPayload); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

	if conflictPayload.Winner != "remote" {
		t.Errorf("LWW with newer remote should pick remote, got %s", conflictPayload.Winner)
	}
}

func TestDecideSync_LWWResolver_LocalNewer(t *testing.T) {
	t.Parallel()

	resolver := newUpdatedAtLWWResolver(t)

	localTime := time.Now().Truncate(time.Millisecond).Add(3 * time.Hour)
	remoteTime := time.Now().Truncate(time.Millisecond)

	remoteItem := testItem("123", "PushEvent")
	remoteItem.UpdatedAt = remoteTime

	state := testStateWithTimestamp("123", "PushEvent", localTime)

	events, err := DecideSync(ToDataItem(remoteItem), nil, resolver)(state, 1)
	testutil.MustNoError(t, err)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	var conflictPayload ItemConflictFoundPayload
	if unmarshalErr := json.Unmarshal(events[0].Payload(), &conflictPayload); unmarshalErr != nil {
		t.Fatalf("unexpected error: %v", unmarshalErr)
	}

	if conflictPayload.Winner != "local" {
		t.Errorf("LWW with newer local should pick local, got %s", conflictPayload.Winner)
	}
}
