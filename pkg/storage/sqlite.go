package storage

import (
	"database/sql"

	"github.com/larsartmann/go-localsync/internal/database"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

type SQLiteStorage struct {
	sqlStorage
}

func NewSQLiteStorage(dbc *sql.DB) *SQLiteStorage {
	return &SQLiteStorage{
		sqlStorage: newSQLStorage(dbc),
	}
}

func Open(path string) (*SQLiteStorage, error) {
	dbc, err := database.Open(path)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to open storage at %s", path)
	}

	return NewSQLiteStorage(dbc), nil
}
