package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/larsartmann/go-localsync/internal/db"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

type sqlStorage struct {
	dbc     *sql.DB
	querier db.Querier
}

func newSQLStorage(dbc *sql.DB) sqlStorage {
	return sqlStorage{
		dbc:     dbc,
		querier: db.New(dbc),
	}
}

func (s *sqlStorage) Close() error {
	return s.dbc.Close()
}

func (s *sqlStorage) Upsert(ctx context.Context, item *provider.Item) error {
	err := s.querier.UpsertEvent(ctx, toDBParams(item))
	if err != nil {
		return fmt.Errorf("%w: upsert item %q: %w", pkgerrors.ErrDatabase, item.ID.Get(), err)
	}

	return nil
}

func (s *sqlStorage) UpsertBatch(ctx context.Context, items []*provider.Item) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.dbc.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: begin transaction for batch upsert: %w", pkgerrors.ErrDatabase, err)
	}

	qtx := db.New(tx)

	for _, item := range items {
		err := qtx.UpsertEvent(ctx, toDBParams(item))
		if err != nil {
			_ = tx.Rollback()

			return fmt.Errorf(
				"%w: batch upsert item %q: %w",
				pkgerrors.ErrDatabase,
				item.ID.Get(),
				err,
			)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("%w: commit batch upsert: %w", pkgerrors.ErrDatabase, err)
	}

	return nil
}

func (s *sqlStorage) GetByID(ctx context.Context, id types.ItemID) (*provider.Item, error) {
	e, err := s.querier.GetEventBySourceID(ctx, types.NewSourceItemID(id.Get()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, fmt.Errorf("%w: get item by ID %q: %w", pkgerrors.ErrDatabase, id.Get(), err)
	}

	return toItem(e), nil
}

func (s *sqlStorage) BatchGetByIDs(
	ctx context.Context,
	ids []types.ItemID,
) ([]*provider.Item, error) {
	return batchGetByIDs(ctx, s.dbc, ids)
}

func (s *sqlStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	e, err := s.querier.GetLatestEvent(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, fmt.Errorf("%w: get latest event: %w", pkgerrors.ErrDatabase, err)
	}

	return toItem(e), nil
}

func (s *sqlStorage) GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error) {
	events, err := s.querier.GetEvents(ctx, &db.GetEventsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: get events (limit=%d, offset=%d): %w",
			pkgerrors.ErrDatabase,
			limit,
			offset,
			err,
		)
	}

	return convertItems(events), nil
}

func (s *sqlStorage) GetItemsByType(
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

func (s *sqlStorage) GetItemsByActor(
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

func (s *sqlStorage) GetItemsByRepo(
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

func (s *sqlStorage) GetItemsBySource(
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

func (s *sqlStorage) GetItemsSince(
	ctx context.Context,
	since time.Time,
) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("%w: get events since %v: %w", pkgerrors.ErrDatabase, since, err)
	}

	return convertItems(events), nil
}

func (s *sqlStorage) Count(ctx context.Context) (int64, error) {
	count, err := s.querier.CountEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: count events: %w", pkgerrors.ErrDatabase, err)
	}

	return count, nil
}

func (s *sqlStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
	count, err := s.querier.CountEventsByType(ctx, itemType)
	if err != nil {
		return 0, fmt.Errorf(
			"%w: count events by type %q: %w",
			pkgerrors.ErrDatabase,
			itemType,
			err,
		)
	}

	return count, nil
}

func (s *sqlStorage) GetTypes(ctx context.Context) ([]string, error) {
	types, err := s.querier.GetEventTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: get event types: %w", pkgerrors.ErrDatabase, err)
	}

	return types, nil
}

func (s *sqlStorage) Delete(ctx context.Context, id types.ItemID) error {
	err := s.querier.DeleteEventBySourceID(ctx, types.NewSourceItemID(id.Get()))
	if err != nil {
		return fmt.Errorf("%w: delete item %q: %w", pkgerrors.ErrDatabase, id.Get(), err)
	}

	return nil
}

func (s *sqlStorage) DeleteAll(ctx context.Context) error {
	err := s.querier.DeleteAllEvents(ctx)
	if err != nil {
		return fmt.Errorf("%w: delete all events: %w", pkgerrors.ErrDatabase, err)
	}

	return nil
}

type queryFunc func(ctx context.Context) ([]*db.Events, error)

func (s *sqlStorage) queryWithFilter(
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

func convertItems(events []*db.Events) []*provider.Item {
	result := make([]*provider.Item, len(events))
	for i, e := range events {
		result[i] = toItem(e)
	}

	return result
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
		ID:             types.NewItemID(e.SourceID.Get()),
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
		ID:             types.NewEventID(),
		SourceID:       types.NewSourceItemID(item.ID.Get()),
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
