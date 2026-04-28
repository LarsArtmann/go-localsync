package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-localsync/internal/database"
	"github.com/larsartmann/go-localsync/internal/db"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
	turso "turso.tech/database/tursogo"
)

type TursoStorage struct {
	dbc     *sql.DB
	querier db.Querier
}

func NewTursoStorage(dbc *sql.DB) *TursoStorage {
	return &TursoStorage{
		dbc:     dbc,
		querier: db.New(dbc),
	}
}

func OpenTurso(url, authToken string) (*TursoStorage, error) {
	ctx := context.Background()

	var dbc *sql.DB

	if isRemoteURL(url) {
		//nolint:exhaustruct // optional fields have sensible defaults
		syncDb, err := turso.NewTursoSyncDb(ctx, turso.TursoSyncDbConfig{
			Path:      ":memory:",
			RemoteUrl: url,
			AuthToken: authToken,
		})
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to create turso sync db for %s", url)
		}

		dbc, err = syncDb.Connect(ctx)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to connect turso sync db at %s", url)
		}
	} else {
		path := strings.TrimPrefix(url, "file:")

		var err error

		dbc, err = sql.Open("turso", path)
		if err != nil {
			return nil, pkgerrors.Wrapf(err, "failed to open turso at %s", url)
		}
	}

	err := dbc.PingContext(ctx)
	if err != nil {
		_ = dbc.Close()

		return nil, pkgerrors.Wrapf(err, "failed to ping turso at %s", url)
	}

	err = database.RunMigrations(dbc)
	if err != nil {
		_ = dbc.Close()

		return nil, pkgerrors.Wrapf(err, "failed to run migrations at %s", url)
	}

	dbc.SetMaxOpenConns(1)

	return NewTursoStorage(dbc), nil
}

func isRemoteURL(url string) bool {
	return strings.HasPrefix(url, "libsql://") ||
		strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "http://")
}

func (s *TursoStorage) Close() error {
	return s.dbc.Close()
}

func (s *TursoStorage) Upsert(ctx context.Context, item *provider.Item) error {
	err := s.querier.UpsertEvent(ctx, toDBParams(item))
	if err != nil {
		return fmt.Errorf("%w: upsert item %q: %w", pkgerrors.ErrDatabase, item.ID.Get(), err)
	}

	return nil
}

func (s *TursoStorage) UpsertBatch(ctx context.Context, items []*provider.Item) error {
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

func (s *TursoStorage) GetByID(ctx context.Context, id types.ItemID) (*provider.Item, error) {
	e, err := s.querier.GetEventBySourceID(ctx, types.NewSourceItemID(id.Get()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, fmt.Errorf("%w: get item by ID %q: %w", pkgerrors.ErrDatabase, id.Get(), err)
	}

	return toItem(e), nil
}

func (s *TursoStorage) BatchGetByIDs(
	ctx context.Context,
	ids []types.ItemID,
) ([]*provider.Item, error) {
	return batchGetByIDs(ctx, s.dbc, ids)
}

func (s *TursoStorage) GetLatest(ctx context.Context) (*provider.Item, error) {
	e, err := s.querier.GetLatestEvent(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, pkgerrors.ErrNotFound
		}

		return nil, fmt.Errorf("%w: get latest event: %w", pkgerrors.ErrDatabase, err)
	}

	return toItem(e), nil
}

func (s *TursoStorage) GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error) {
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

func (s *TursoStorage) GetItemsByType(
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

func (s *TursoStorage) GetItemsByActor(
	ctx context.Context,
	actorLogin string,
	limit, offset int,
) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsByActor(ctx, &db.GetEventsByActorParams{
		ActorLogin: sql.NullString{String: actorLogin, Valid: true},
		Limit:      int64(limit),
		Offset:     int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: get events by actor %q (limit=%d, offset=%d): %w",
			pkgerrors.ErrDatabase,
			actorLogin,
			limit,
			offset,
			err,
		)
	}

	return convertItems(events), nil
}

func (s *TursoStorage) GetItemsByRepo(
	ctx context.Context,
	repoName string,
	limit, offset int,
) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsByRepo(ctx, &db.GetEventsByRepoParams{
		RepoName: sql.NullString{String: repoName, Valid: true},
		Limit:    int64(limit),
		Offset:   int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"%w: get events by repo %q (limit=%d, offset=%d): %w",
			pkgerrors.ErrDatabase,
			repoName,
			limit,
			offset,
			err,
		)
	}

	return convertItems(events), nil
}

func (s *TursoStorage) GetItemsBySource(
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

func (s *TursoStorage) GetItemsSince(
	ctx context.Context,
	since time.Time,
) ([]*provider.Item, error) {
	events, err := s.querier.GetEventsSince(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("%w: get events since %v: %w", pkgerrors.ErrDatabase, since, err)
	}

	return convertItems(events), nil
}

func (s *TursoStorage) Count(ctx context.Context) (int64, error) {
	count, err := s.querier.CountEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("%w: count events: %w", pkgerrors.ErrDatabase, err)
	}

	return count, nil
}

func (s *TursoStorage) CountByType(ctx context.Context, itemType string) (int64, error) {
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

func (s *TursoStorage) GetTypes(ctx context.Context) ([]string, error) {
	types, err := s.querier.GetEventTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: get event types: %w", pkgerrors.ErrDatabase, err)
	}

	return types, nil
}

func (s *TursoStorage) Delete(ctx context.Context, id types.ItemID) error {
	err := s.querier.DeleteEventBySourceID(ctx, types.NewSourceItemID(id.Get()))
	if err != nil {
		return fmt.Errorf("%w: delete item %q: %w", pkgerrors.ErrDatabase, id.Get(), err)
	}

	return nil
}

func (s *TursoStorage) DeleteAll(ctx context.Context) error {
	err := s.querier.DeleteAllEvents(ctx)
	if err != nil {
		return fmt.Errorf("%w: delete all events: %w", pkgerrors.ErrDatabase, err)
	}

	return nil
}
