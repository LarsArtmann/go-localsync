package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/larsartmann/go-localsync/internal/db"
	"github.com/larsartmann/go-localsync/pkg/provider"
)

type SQLiteStorage struct {
	dbc     *sql.DB
	querier db.Querier
}

func NewSQLiteStorage(dbc *sql.DB) *SQLiteStorage {
	return &SQLiteStorage{
		dbc:     dbc,
		querier: db.New(dbc),
	}
}

func (s *SQLiteStorage) Close() error {
	return s.dbc.Close()
}

func (s *SQLiteStorage) Upsert(ctx context.Context, item *provider.Item) error {
	return s.querier.UpsertEvent(ctx, toDBParams(item))
}

func (s *SQLiteStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	e, err := s.querier.GetLatestEvent(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toItem(e), nil
}

func (s *SQLiteStorage) GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error) {
	events, err := s.querier.GetEvents(ctx, &db.GetEventsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return convertItems(events), nil
}

func (s *SQLiteStorage) GetItemsByType(ctx context.Context, itemType string, limit, offset int) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsByType(ctx, &db.GetEventsByTypeParams{
		Type:   itemType,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return convertItems(events), nil
}

func (s *SQLiteStorage) GetItemsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsByActor(ctx, &db.GetEventsByActorParams{
		ActorLogin: toNullString(actorLogin),
		Limit:      int64(limit),
		Offset:     int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return convertItems(events), nil
}

func (s *SQLiteStorage) GetItemsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsByRepo(ctx, &db.GetEventsByRepoParams{
		RepoName: toNullString(repoName),
		Limit:    int64(limit),
		Offset:   int64(offset),
	})
	if err != nil {
		return nil, err
	}
	return convertItems(events), nil
}

func (s *SQLiteStorage) Count(ctx context.Context) (int64, error) {
	return s.querier.CountEvents(ctx)
}

func (s *SQLiteStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
	return s.querier.CountEventsByType(ctx, itemType)
}

func (s *SQLiteStorage) GetTypes(ctx context.Context) ([]string, error) {
	return s.querier.GetEventTypes(ctx)
}

func convertItems(events []*db.Events) []*provider.Item {
	result := make([]*provider.Item, len(events))
	for i, e := range events {
		result[i] = toItem(e)
	}
	return result
}
