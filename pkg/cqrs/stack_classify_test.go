package cqrs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestClassifyAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		eventCount     int
		wasNew         bool
		conflictWinner ConflictWinner
		want           synclib.SyncAction
	}{
		{
			name:       "error_returns_action_error",
			err:        errors.New("some error"),
			eventCount: 0,
			wasNew:     false,
			want:       synclib.ActionError,
		},
		{
			name:           "multiple_events_conflict_remote",
			err:            nil,
			eventCount:     2,
			wasNew:         false,
			conflictWinner: conflictWinnerRemote,
			want:           synclib.ActionConflictRemote,
		},
		{
			name:           "multiple_events_conflict_local",
			err:            nil,
			eventCount:     2,
			wasNew:         false,
			conflictWinner: conflictWinnerLocal,
			want:           synclib.ActionConflictLocal,
		},
		{
			name:       "one_event_new_item_created",
			err:        nil,
			eventCount: 1,
			wasNew:     true,
			want:       synclib.ActionCreated,
		},
		{
			name:       "one_event_existing_item_updated",
			err:        nil,
			eventCount: 1,
			wasNew:     false,
			want:       synclib.ActionUpdated,
		},
		{
			name:       "zero_events_unchanged",
			err:        nil,
			eventCount: 0,
			wasNew:     false,
			want:       synclib.ActionUnchanged,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := classifyAction(tt.err, tt.eventCount, tt.wasNew, tt.conflictWinner)
			if got != tt.want {
				t.Errorf("classifyAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCQRSStack_SyncItems_SameItem_Twice(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("1", "PushEvent")

	first := stack.SyncItems(ctx, []*provider.Item{item})
	if first.Synced != 1 {
		t.Errorf("first sync: expected Synced=1, got %d", first.Synced)
	}
	if first.Errors != 0 {
		t.Errorf("first sync: expected Errors=0, got %d", first.Errors)
	}

	var foundCreated bool

	for _, r := range first.Results {
		if r.Action == synclib.ActionCreated {
			foundCreated = true
		}
	}
	if !foundCreated {
		t.Error("first sync: expected ActionCreated in results")
	}

	second := stack.SyncItems(ctx, []*provider.Item{item})
	if second.Synced != 0 {
		t.Errorf("second sync of same item: expected Synced=0, got %d", second.Synced)
	}
	if second.Errors != 0 {
		t.Errorf("second sync of same item: expected Errors=0, got %d", second.Errors)
	}

	var foundUnchanged bool

	for _, r := range second.Results {
		if r.Action == synclib.ActionUnchanged {
			foundUnchanged = true
		}
	}
	if !foundUnchanged {
		t.Error("second sync of same item: expected ActionUnchanged in results")
	}
}

func TestCQRSStack_SyncItems_ConflictRemote(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	item := testItem("1", "PushEvent")

	first := stack.SyncItems(ctx, []*provider.Item{item})
	if first.Synced != 1 {
		t.Fatalf("expected Synced=1, got %d", first.Synced)
	}

	updated := testItem("1", "IssueEvent")
	updated.UpdatedAt = time.Now().Add(time.Hour)

	second := stack.SyncItems(ctx, []*provider.Item{updated})
	if second.Conflicts != 1 {
		t.Errorf("expected Conflicts=1, got %d", second.Conflicts)
	}
	if second.Synced != 1 {
		t.Errorf("expected Synced=1, got %d", second.Synced)
	}

	if !hasAction(second, synclib.ActionConflictRemote) {
		t.Error("expected ActionConflictRemote in results")
	}
}

func TestCQRSStack_SyncItems_LWWConflictResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		localTime      time.Time
		remoteTime     time.Time
		wantAction     synclib.SyncAction
		wantWinnerType string
	}{
		{
			name:           "local_wins_when_newer",
			localTime:      testFutureNow(3 * time.Hour),
			remoteTime:     time.Now().Truncate(time.Millisecond),
			wantAction:     synclib.ActionConflictLocal,
			wantWinnerType: "PushEvent",
		},
		{
			name:           "remote_wins_when_newer",
			localTime:      time.Now().Truncate(time.Millisecond),
			remoteTime:     time.Now().Truncate(time.Millisecond).Add(2 * time.Hour),
			wantAction:     synclib.ActionConflictRemote,
			wantWinnerType: "IssueEvent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolver := newUpdatedAtLWWResolver(t)

			stack, stackErr := NewCQRSStack(CQRSConfig{
				Backend:          "memory",
				ConflictResolver: resolver,
			})
			testutil.MustNoError(t, stackErr)
			defer func() { _ = stack.Close() }()

			ctx := context.Background()

			first := testItem("1", "PushEvent")
			first.UpdatedAt = tt.localTime

			firstResult := stack.SyncItems(ctx, []*provider.Item{first})
			if firstResult.Synced != 1 {
				t.Fatalf("expected Synced=1, got %d", firstResult.Synced)
			}

			updated := testItem("1", "IssueEvent")
			updated.UpdatedAt = tt.remoteTime

			second := stack.SyncItems(ctx, []*provider.Item{updated})
			if second.Conflicts != 1 {
				t.Errorf("expected Conflicts=1, got %d", second.Conflicts)
			}

			if !hasAction(second, tt.wantAction) {
				t.Errorf("expected %v in results", tt.wantAction)
			}

			got, getErr := stack.Get(ctx, "github", id.NewExternalID("1"))
			testutil.MustNoError(t, getErr)

			testutil.AssertEqual(t, got.Type.Get(), tt.wantWinnerType, "Type")
		})
	}
}

// hasAction returns true if any result in summary has the given action.
func hasAction(summary *synclib.SyncSummary, action synclib.SyncAction) bool {
	for _, r := range summary.Results {
		if r.Action == action {
			return true
		}
	}

	return false
}
