package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/larsartmann/go-localsync/internal/db"
	"github.com/larsartmann/go-localsync/pkg/event"
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

func (s *SQLiteStorage) UpsertEvent(ctx context.Context, event *event.Event) error {
	return s.querier.UpsertEvent(ctx, toDBEvent(event))
}

func (s *SQLiteStorage) GetLatestEvent(ctx context.Context) (*event.Event, error) {
	e, err := s.querier.GetLatestEvent(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return fromDBEvent(e), nil
}

func convertEvents(events []*db.Events) []*event.Event {
	result := make([]*event.Event, len(events))
	for i, e := range events {
		result[i] = fromDBEvent(e)
	}
	return result
}

func (s *SQLiteStorage) GetEvents(ctx context.Context, limit, offset int) ([]*event.Event, error) {
	return withConvertedEvents(s.querier.GetEvents(ctx, &db.GetEventsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	}))
}

func (s *SQLiteStorage) GetEventsByType(ctx context.Context, eventType string, limit, offset int) ([]*event.Event, error) {
	return withConvertedEvents(s.querier.GetEventsByType(ctx, &db.GetEventsByTypeParams{
		Type:   eventType,
		Limit:  int64(limit),
		Offset: int64(offset),
	}))
}

func (s *SQLiteStorage) GetEventsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*event.Event, error) {
	return withConvertedEvents(s.querier.GetEventsByActor(ctx, &db.GetEventsByActorParams{
		ActorLogin: toNullString(actorLogin),
		Limit:      int64(limit),
		Offset:     int64(offset),
	}))
}

func (s *SQLiteStorage) GetEventsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*event.Event, error) {
	return withConvertedEvents(s.querier.GetEventsByRepo(ctx, &db.GetEventsByRepoParams{
		RepoName: toNullString(repoName),
		Limit:    int64(limit),
		Offset:   int64(offset),
	}))
}

func withConvertedEvents(events []*db.Events, err error) ([]*event.Event, error) {
	if err != nil {
		return nil, err
	}
	return convertEvents(events), nil
}

func (s *SQLiteStorage) CountEvents(ctx context.Context) (int64, error) {
	return s.querier.CountEvents(ctx)
}

func (s *SQLiteStorage) CountEventsByType(ctx context.Context, eventType string) (int64, error) {
	return s.querier.CountEventsByType(ctx, eventType)
}

func (s *SQLiteStorage) GetEventTypes(ctx context.Context) ([]string, error) {
	return s.querier.GetEventTypes(ctx)
}
