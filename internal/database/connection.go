package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", path, err)
	}

	if err := RunMigrations(db); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("failed to run migrations at %s: %w", path, err)
	}

	return db, nil
}
