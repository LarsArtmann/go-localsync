package database

import (
	"database/sql"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to open database at %s", path)
	}

	if err := RunMigrations(db); err != nil {
		_ = db.Close()

		return nil, pkgerrors.Wrapf(err, "failed to run migrations at %s", path)
	}

	return db, nil
}
