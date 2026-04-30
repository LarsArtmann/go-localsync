package storage

import (
	"context"
	"database/sql"
	"strings"

	"github.com/larsartmann/go-localsync/internal/database"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	turso "turso.tech/database/tursogo"
)

type TursoStorage struct {
	sqlStorage
}

func NewTursoStorage(dbc *sql.DB) *TursoStorage {
	return &TursoStorage{
		sqlStorage: newSQLStorage(dbc),
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
