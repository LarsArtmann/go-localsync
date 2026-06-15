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

var (
	resolverTestNow    = time.Now().Truncate(time.Millisecond)
	resolverTestFuture = resolverTestNow.Add(2 * time.Hour)
)

func TestDecideSync_WithResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		resolver   crdt.ConflictResolver[*model.Item]
		localTime  time.Time
		remoteTime time.Time
		wantWinner string
	}{
		{
			name:       "custom_resolver_remote_wins",
			resolver:   &pickSideResolver{pickSide: "remote"},
			localTime:  resolverTestNow,
			remoteTime: resolverTestFuture,
			wantWinner: "remote",
		},
		{
			name:       "custom_resolver_local_wins",
			resolver:   &pickSideResolver{pickSide: "local"},
			localTime:  resolverTestNow,
			remoteTime: resolverTestFuture,
			wantWinner: "local",
		},
		{
			name:       "error_resolver_falls_back_to_remote",
			resolver:   new(errorResolver),
			localTime:  time.Now(),
			remoteTime: time.Now().Add(time.Hour),
			wantWinner: "remote",
		},
		{
			name:       "lww_resolver_remote_newer",
			resolver:   newUpdatedAtLWWResolver(t),
			localTime:  resolverTestNow,
			remoteTime: resolverTestFuture,
			wantWinner: "remote",
		},
		{
			name:       "lww_resolver_local_newer",
			resolver:   newUpdatedAtLWWResolver(t),
			localTime:  testFutureNow(3 * time.Hour),
			remoteTime: resolverTestNow,
			wantWinner: "local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			remoteItem := testItem("123", "PushEvent")
			remoteItem.UpdatedAt = tt.remoteTime

			state := testStateWithTimestamp("123", "PushEvent", tt.localTime)

			events, err := decideSync(toDataItem(remoteItem), nil, tt.resolver)(state, 1)
			testutil.MustNoError(t, err)
			testutil.RequireLen(t, events, 2)

			assertEventType(t, events[0], EventItemConflictFound)
			assertEventType(t, events[1], EventItemSynced)

			conflictPayload := unmarshalConflictPayload(t, events[0])
			testutil.AssertEqual(t, conflictPayload.Winner, tt.wantWinner, "winner")

			syncedPayload := unmarshalSyncedPayload(t, events[1])
			wantSyncedTime := tt.remoteTime.UnixNano()
			if tt.wantWinner == "local" {
				wantSyncedTime = tt.localTime.UnixNano()
			}
			if syncedPayload.UpdatedAt != wantSyncedTime {
				t.Errorf("expected synced payload timestamp from %s item, got %d",
					tt.wantWinner, syncedPayload.UpdatedAt)
			}
		})
	}
}
