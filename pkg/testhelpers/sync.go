package testhelpers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// NewTestItem creates a test item with sensible defaults.
func NewTestItem(id, eventType string, createdAt time.Time) *provider.Item {
	return &provider.Item{
		ID:         types.NewItemID(id),
		Source:     types.NewProviderID("fake"),
		Type:       types.NewEventTypeID(eventType),
		ActorLogin: types.NewActorID("testuser"),
		RepoName:   types.NewRepoID("test/repo"),
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
		RawJSON:    json.RawMessage(`{"id":"` + id + `"}`),
	}
}

// NewMinimalTestItem creates a test item with only the essential fields for sync tests.
// This is useful when the tests only care about ID, Type, and CreatedAt.
func NewMinimalTestItem(id, eventType string, createdAt time.Time) *provider.Item {
	return &provider.Item{
		ID:        types.NewItemID(id),
		Source:    types.NewProviderID("mock"),
		Type:      types.NewEventTypeID(eventType),
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}

// mockProvider implements provider.Provider for testing.
type MockProvider struct {
	NameVal       string
	ItemsVal      []*provider.Item
	FetchErr      error
	FetchCalls    int
	RateLimitVal  *provider.RateLimitInfo
	RateLimitErr  error
}

func (m *MockProvider) Name() string {
	if m.NameVal == "" {
		return "mock"
	}

	return m.NameVal
}

func (m *MockProvider) Fetch(
	ctx context.Context,
	opts *provider.FetchOptions,
) (*provider.FetchResult, error) {
	m.FetchCalls++
	if m.FetchErr != nil {
		return nil, m.FetchErr
	}

	return &provider.FetchResult{Items: m.ItemsVal, HasMore: false}, nil
}

func (m *MockProvider) FetchAll(
	ctx context.Context,
	source string,
	maxPages int,
) (*provider.FetchResult, error) {
	return m.Fetch(ctx, &provider.FetchOptions{Source: source})
}

func (m *MockProvider) GetRateLimit(ctx context.Context) (*provider.RateLimitInfo, error) {
	return m.RateLimitVal, m.RateLimitErr
}

// NewMockProviderWithItems creates a mock provider with standard test items.
func NewMockProviderWithItems() *MockProvider {
	return &MockProvider{
		ItemsVal: []*provider.Item{
			NewTestItem("1", "PushEvent", time.Now()),
			NewTestItem("2", "IssuesEvent", time.Now()),
		},
	}
}

// mockStorage implements storage.Storage for testing.
type MockStorage struct {
	ItemsVal          []*provider.Item
	LatestItemVal     *provider.Item
	UpsertErrVal      error
	LatestErrVal      error
	CountResultVal    int64
	CountErrVal       error
	TypesResultVal    []string
	TypesErrVal       error
	CountByTypeVal    int64
	CountByTypeErrVal error
	CloseErrVal       error
}

func (m *MockStorage) Upsert(ctx context.Context, item *provider.Item) error {
	if m.UpsertErrVal != nil {
		return m.UpsertErrVal
	}

	m.ItemsVal = append(m.ItemsVal, item)

	return nil
}

func (m *MockStorage) UpsertBatch(_ context.Context, items []*provider.Item) error {
	if m.UpsertErrVal != nil {
		return m.UpsertErrVal
	}

	m.ItemsVal = append(m.ItemsVal, items...)

	return nil
}

func (m *MockStorage) GetByID(ctx context.Context, id string) (*provider.Item, error) {
	for _, item := range m.ItemsVal {
		if item.ID.Get() == id {
			return item, nil
		}
	}

	return nil, nil //nolint:nilnil // not found is not an error condition
}

func (m *MockStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	if m.LatestErrVal != nil {
		return nil, m.LatestErrVal
	}

	return m.LatestItemVal, nil
}

func (m *MockStorage) GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error) {
	return m.ItemsVal, nil
}

// getItemsByFilter returns items for GetItemsByType/Actor/Repo.
// This eliminates duplication across the storage interface mock methods.
func (m *MockStorage) getItemsByFilter(
	_ context.Context,
	_ string,
	_, _ int,
) ([]*provider.Item, error) {
	return m.ItemsVal, nil
}

func (m *MockStorage) GetItemsByType(
	ctx context.Context,
	itemType string,
	limit, offset int,
) ([]*provider.Item, error) {
	return m.getItemsByFilter(ctx, itemType, limit, offset)
}

func (m *MockStorage) GetItemsByActor(
	ctx context.Context,
	actorLogin string,
	limit, offset int,
) ([]*provider.Item, error) {
	return m.getItemsByFilter(ctx, actorLogin, limit, offset)
}

func (m *MockStorage) GetItemsByRepo(
	ctx context.Context,
	repoName string,
	limit, offset int,
) ([]*provider.Item, error) {
	return m.getItemsByFilter(ctx, repoName, limit, offset)
}

func (m *MockStorage) Count(ctx context.Context) (int64, error) {
	return m.CountResultVal, m.CountErrVal
}

func (m *MockStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
	return m.CountByTypeVal, m.CountByTypeErrVal
}

func (m *MockStorage) GetTypes(ctx context.Context) ([]string, error) {
	return m.TypesResultVal, m.TypesErrVal
}

func (m *MockStorage) GetItemsBySource(_ context.Context, _ string, _, _ int) ([]*provider.Item, error) {
	return m.ItemsVal, nil
}

func (m *MockStorage) Delete(_ context.Context, _ string) error {
	return nil
}

func (m *MockStorage) DeleteAll(_ context.Context) error {
	return nil
}

func (m *MockStorage) GetItemsSince(_ context.Context, _ time.Time) ([]*provider.Item, error) {
	return m.ItemsVal, nil
}

func (m *MockStorage) Close() error {
	return m.CloseErrVal
}

// failingStorage simulates a storage that always fails.
type FailingStorage struct{}

func (f *FailingStorage) Upsert(ctx context.Context, item *provider.Item) error {
	return errors.New("disk full")
}

func (f *FailingStorage) UpsertBatch(_ context.Context, _ []*provider.Item) error {
	return errors.New("disk full")
}

func (f *FailingStorage) GetByID(ctx context.Context, id string) (*provider.Item, error) {
	return nil, errors.New("not found")
}

func (f *FailingStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	return nil, errors.New("not found")
}

func (f *FailingStorage) GetItems(
	ctx context.Context,
	limit, offset int,
) ([]*provider.Item, error) {
	return nil, nil
}

// getItemsByFilter returns nil for GetItemsByType/Actor/Repo.
// This eliminates duplication across the failing storage interface mock methods.
func (*FailingStorage) getItemsByFilter(
	context.Context,
	string,
	int, int,
) ([]*provider.Item, error) {
	return nil, nil
}

func (f *FailingStorage) GetItemsByType(
	ctx context.Context,
	itemType string,
	limit, offset int,
) ([]*provider.Item, error) {
	return f.getItemsByFilter(ctx, itemType, limit, offset)
}

func (f *FailingStorage) GetItemsByActor(
	ctx context.Context,
	actorLogin string,
	limit, offset int,
) ([]*provider.Item, error) {
	return f.getItemsByFilter(ctx, actorLogin, limit, offset)
}

func (f *FailingStorage) GetItemsByRepo(
	ctx context.Context,
	repoName string,
	limit, offset int,
) ([]*provider.Item, error) {
	return f.getItemsByFilter(ctx, repoName, limit, offset)
}

func (f *FailingStorage) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

func (f *FailingStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
	return 0, nil
}

func (f *FailingStorage) GetTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (f *FailingStorage) GetItemsBySource(_ context.Context, _ string, _, _ int) ([]*provider.Item, error) {
	return nil, nil
}

func (f *FailingStorage) Delete(_ context.Context, _ string) error {
	return errors.New("disk full")
}

func (f *FailingStorage) DeleteAll(_ context.Context) error {
	return errors.New("disk full")
}

func (f *FailingStorage) GetItemsSince(_ context.Context, _ time.Time) ([]*provider.Item, error) {
	return nil, nil
}

func (f *FailingStorage) Close() error {
	return nil
}
