package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/larsartmann/go-localsync/internal/database"
	"github.com/larsartmann/go-localsync/internal/db"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
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

func Open(path string) (*SQLiteStorage, error) {
	dbc, err := database.Open(path)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to open storage at %s", path)
	}

	return NewSQLiteStorage(dbc), nil
}

func (s *SQLiteStorage) Close() error {
	return s.dbc.Close()
}

func (s *SQLiteStorage) Upsert(ctx context.Context, item *provider.Item) error {
	if err := s.querier.UpsertEvent(ctx, toDBParams(item)); err != nil {
		return fmt.Errorf("%w: upsert item %q: %w", pkgerrors.ErrDatabase, item.ID.Get(), err)
	}

	return nil
}

func (s *SQLiteStorage) UpsertBatch(ctx context.Context, items []*provider.Item) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.dbc.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: begin transaction for batch upsert: %w", pkgerrors.ErrDatabase, err)
	}

	qtx := db.New(tx)

	for _, item := range items {
		if err := qtx.UpsertEvent(ctx, toDBParams(item)); err != nil {
			_ = tx.Rollback()

			return fmt.Errorf("%w: batch upsert item %q: %w", pkgerrors.ErrDatabase, item.ID.Get(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: commit batch upsert: %w", pkgerrors.ErrDatabase, err)
	}

	return nil
}

func (s *SQLiteStorage) GetByID(ctx context.Context, id string) (*provider.Item, error) {
	e, err := s.querier.GetEventByGithubID(ctx, types.NewGithubEventID(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, fmt.Errorf("%w: get item by ID %q: %w", pkgerrors.ErrDatabase, id, err)
	}

	return toItem(e), nil
}

func (s *SQLiteStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	e, err := s.querier.GetLatestEvent(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, fmt.Errorf("%w: get latest event: %w", pkgerrors.ErrDatabase, err)
	}

	return toItem(e), nil
}

func (s *SQLiteStorage) GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error) {
	events, err := s.querier.GetEvents(ctx, &db.GetEventsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: get events (limit=%d, offset=%d): %w", pkgerrors.ErrDatabase, limit, offset, err)
	}

	return convertItems(events), nil
}

func (s *SQLiteStorage) GetItemsByType(
	ctx context.Context,
	itemType string,
	limit, offset int,
) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsByType(ctx, &db.GetEventsByTypeParams{
		Type:   itemType,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: get events by type %q (limit=%d, offset=%d): %w",
			pkgerrors.ErrDatabase,
			itemType,
			limit,
			offset,
			err,
		)
	}

	return convertItems(events), nil
}

func (s *SQLiteStorage) GetItemsByActor(
	ctx context.Context,
	actorLogin string,
	limit, offset int,
) ([]*provider.Item, error) {
	return s.queryWithFilter(
		ctx,
		"actor",
		actorLogin,
		limit,
		offset,
		func(ctx context.Context) ([]*db.Events, error) {
			return s.querier.GetEventsByActor(ctx, &db.GetEventsByActorParams{
				ActorLogin: sql.NullString{String: actorLogin, Valid: true},
				Limit:      int64(limit),
				Offset:     int64(offset),
			})
		},
	)
}

func (s *SQLiteStorage) GetItemsByRepo(
	ctx context.Context,
	repoName string,
	limit, offset int,
) ([]*provider.Item, error) {
	return s.queryWithFilter(
		ctx,
		"repo",
		repoName,
		limit,
		offset,
		func(ctx context.Context) ([]*db.Events, error) {
			return s.querier.GetEventsByRepo(ctx, &db.GetEventsByRepoParams{
				RepoName: sql.NullString{String: repoName, Valid: true},
				Limit:    int64(limit),
				Offset:   int64(offset),
			})
		},
	)
}

func (s *SQLiteStorage) Count(ctx context.Context) (int64, error) {
	count, err := s.querier.CountEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: count events: %w", pkgerrors.ErrDatabase, err)
	}

	return count, nil
}

func (s *SQLiteStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
	count, err := s.querier.CountEventsByType(ctx, itemType)
	if err != nil {
		return 0, fmt.Errorf("%w: count events by type %q: %w", pkgerrors.ErrDatabase, itemType, err)
	}

	return count, nil
}

func (s *SQLiteStorage) GetTypes(ctx context.Context) ([]string, error) {
	types, err := s.querier.GetEventTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: get event types: %w", pkgerrors.ErrDatabase, err)
	}

	return types, nil
}

func (s *SQLiteStorage) GetItemsBySource(
	ctx context.Context,
	source string,
	limit, offset int,
) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsBySource(ctx, &db.GetEventsBySourceParams{
		Source: source,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: get events by source %q (limit=%d, offset=%d): %w",
			pkgerrors.ErrDatabase,
			source,
			limit,
			offset,
			err,
		)
	}

	return convertItems(events), nil
}

func (s *SQLiteStorage) GetItemsSince(
	ctx context.Context,
	since time.Time,
) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("%w: get events since %v: %w", pkgerrors.ErrDatabase, since, err)
	}

	return convertItems(events), nil
}

func (s *SQLiteStorage) Delete(ctx context.Context, id string) error {
	if err := s.querier.DeleteEventByGithubID(ctx, types.NewGithubEventID(id)); err != nil {
		return fmt.Errorf("%w: delete item %q: %w", pkgerrors.ErrDatabase, id, err)
	}

	return nil
}

func (s *SQLiteStorage) DeleteAll(ctx context.Context) error {
	if err := s.querier.DeleteAllEvents(ctx); err != nil {
		return fmt.Errorf("%w: delete all events: %w", pkgerrors.ErrDatabase, err)
	}

	return nil
}

func convertItems(events []*db.Events) []*provider.Item {
	result := make([]*provider.Item, len(events))
	for i, e := range events {
		result[i] = toItem(e)
	}

	return result
}

type queryFunc func(ctx context.Context) ([]*db.Events, error)

func (s *SQLiteStorage) queryWithFilter(
	ctx context.Context,
	filterName, filterValue string,
	limit, offset int,
	f queryFunc,
) ([]*provider.Item, error) {
	items, err := f(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: get events by %s %q (limit=%d, offset=%d): %w",
			pkgerrors.ErrDatabase,
			filterName,
			filterValue,
			limit,
			offset,
			err,
		)
	}

	return convertItems(items), nil
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{String: "", Valid: false}
	}

	return sql.NullString{String: s, Valid: true}
}

func fromNullString(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}

	return ns.String
}

func toItem(e *db.Events) *provider.Item {
	return &provider.Item{
		ID:             types.NewItemID(e.GithubID.Get()),
		Source:         types.NewProviderID(e.Source),
		Type:           types.NewEventTypeID(e.Type),
		ActorLogin:     types.NewActorID(fromNullString(e.ActorLogin)),
		ActorAvatarURL: fromNullString(e.ActorAvatarUrl),
		RepoName:       types.NewRepoID(fromNullString(e.RepoName)),
		RepoURL:        fromNullString(e.RepoUrl),
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
		RawJSON:        e.RawJson,
	}
}

func toDBParams(item *provider.Item) *db.UpsertEventParams {
	return &db.UpsertEventParams{
		GithubID:       types.NewGithubEventID(item.ID.Get()),
		Source:         item.Source.Get(),
		Type:           item.Type.Get(),
		ActorLogin:     toNullString(item.ActorLogin.Get()),
		ActorAvatarUrl: toNullString(item.ActorAvatarURL),
		RepoName:       toNullString(item.RepoName.Get()),
		RepoUrl:        toNullString(item.RepoURL),
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		RawJson:        item.RawJSON,
	}
}
