package sync

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/log"
	gh "github.com/google/go-github/v69/github"
	"github.com/larsartmann/go-localsync/pkg/event"
	"github.com/larsartmann/go-localsync/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStorage implements storage.Storage for testing
type mockStorage struct {
	events       []*event.Event
	latestEvent  *event.Event
	upsertErr    error
	latestErr    error
	countResult  int64
	countErr     error
	typesResult  []string
	typesErr     error
	countByType  int64
	countByTypeErr error
	closeErr     error
}

func (m *mockStorage) UpsertEvent(ctx context.Context, e *event.Event) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.events = append(m.events, e)
	return nil
}

func (m *mockStorage) GetLatestEvent(ctx context.Context) (*event.Event, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}
	return m.latestEvent, nil
}

func (m *mockStorage) GetEvents(ctx context.Context, limit, offset int) ([]*event.Event, error) {
	return m.events, nil
}

func (m *mockStorage) GetEventsByType(ctx context.Context, eventType string, limit, offset int) ([]*event.Event, error) {
	return m.events, nil
}

func (m *mockStorage) GetEventsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*event.Event, error) {
	return m.events, nil
}

func (m *mockStorage) GetEventsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*event.Event, error) {
	return m.events, nil
}

func (m *mockStorage) CountEvents(ctx context.Context) (int64, error) {
	return m.countResult, m.countErr
}

func (m *mockStorage) CountEventsByType(ctx context.Context, eventType string) (int64, error) {
	return m.countByType, m.countByTypeErr
}

func (m *mockStorage) GetEventTypes(ctx context.Context) ([]string, error) {
	return m.typesResult, m.typesErr
}

func (m *mockStorage) Close() error {
	return m.closeErr
}

// mockGitHubClient implements github.Fetcher interface
type mockGitHubClient struct {
	events []*event.Event
	err    error
}

func (m *mockGitHubClient) FetchEvents(ctx context.Context, username string, opts *github.FetchOptions) ([]*event.Event, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func (m *mockGitHubClient) FetchAllEvents(ctx context.Context, username string, maxPages int) ([]*event.Event, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func (m *mockGitHubClient) GetRateLimit(ctx context.Context) (*gh.RateLimits, *gh.Response, error) {
	return nil, nil, m.err
}

func TestNewSyncer(t *testing.T) {
	t.Run("creates syncer with provided logger", func(t *testing.T) {
		mockStore := &mockStorage{}
		logger := log.New(nil)
		
		// Create a simple wrapper since we can't directly instantiate github.Client
		syncer := NewSyncer(nil, mockStore, logger)
		
		require.NotNil(t, syncer)
	})
	
	t.Run("uses default logger when nil", func(t *testing.T) {
		mockStore := &mockStorage{}
		
		syncer := NewSyncer(nil, mockStore, nil)
		
		require.NotNil(t, syncer)
		require.NotNil(t, syncer.logger)
	})
}

func TestSyncer_Sync(t *testing.T) {
	t.Run("returns nil for nil options", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)
		
		result, err := syncer.Sync(context.Background(), nil)
		
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestSyncer_SyncIncremental(t *testing.T) {
	t.Run("returns nil for nil options", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)
		
		result, err := syncer.SyncIncremental(context.Background(), nil)
		
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestSyncer_GetStats(t *testing.T) {
	t.Run("returns stats successfully", func(t *testing.T) {
		mockStore := &mockStorage{
			countResult: 100,
			typesResult: []string{"PushEvent", "IssuesEvent"},
			countByType: 50,
		}
		syncer := NewSyncer(nil, mockStore, nil)
		
		stats, err := syncer.GetStats(context.Background())
		
		require.NoError(t, err)
		require.NotNil(t, stats)
		assert.Equal(t, int64(100), stats["total_events"])
		assert.Equal(t, []string{"PushEvent", "IssuesEvent"}, stats["event_types"])
	})
	
	t.Run("returns error when count fails", func(t *testing.T) {
		mockStore := &mockStorage{
			countErr: errors.New("count error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)
		
		stats, err := syncer.GetStats(context.Background())
		
		require.Error(t, err)
		assert.Nil(t, stats)
		assert.Contains(t, err.Error(), "count error")
	})
	
	t.Run("returns error when get types fails", func(t *testing.T) {
		mockStore := &mockStorage{
			countResult: 100,
			typesErr:    errors.New("types error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)
		
		stats, err := syncer.GetStats(context.Background())
		
		require.Error(t, err)
		assert.Nil(t, stats)
		assert.Contains(t, err.Error(), "types error")
	})
}

func TestSyncer_Close(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		mockStore := &mockStorage{}
		syncer := NewSyncer(nil, mockStore, nil)
		
		err := syncer.Close()
		
		require.NoError(t, err)
	})
	
	t.Run("returns error on close failure", func(t *testing.T) {
		mockStore := &mockStorage{
			closeErr: errors.New("close error"),
		}
		syncer := NewSyncer(nil, mockStore, nil)
		
		err := syncer.Close()
		
		require.Error(t, err)
		assert.Contains(t, err.Error(), "close error")
	})
}

// Note: Full integration test would require a mock github client
// that implements the FetchAllEvents method. Since we can't easily
// inject a mock without interface extraction, we test the storage
// directly in the sqlite_test.go file.
func TestSyncResult(t *testing.T) {
	result := &SyncResult{
		Fetched: 100,
		Skipped: 20,
		Errors:  2,
	}
	
	assert.Equal(t, 100, result.Fetched)
	assert.Equal(t, 20, result.Skipped)
	assert.Equal(t, 2, result.Errors)
}
