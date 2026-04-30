package database

import (
	"context"
	"database/sql"
	"embed"
	"sort"
	"strconv"
	"strings"
	"sync"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

const migrationFilenameParts = 2

var (
	migrationsOnce    sync.Once   //nolint:gochecknoglobals // lazy init via sync.Once is standard Go
	loadedMigrations  []migration //nolint:gochecknoglobals // populated once by sync.Once
	errMigrationsLoad error       //nolint:gochecknoglobals // populated once by sync.Once
)

func loadMigrations() {
	entries, readErr := migrationFS.ReadDir("migrations")
	if readErr != nil {
		errMigrationsLoad = readErr

		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		mig, parseErr := parseMigrationFile(entry.Name())
		if parseErr != nil {
			errMigrationsLoad = parseErr

			return
		}

		loadedMigrations = append(loadedMigrations, mig)
	}
}

func getMigrations() ([]migration, error) {
	migrationsOnce.Do(loadMigrations)

	return loadedMigrations, errMigrationsLoad
}

func parseMigrationFile(filename string) (migration, error) {
	content, err := migrationFS.ReadFile("migrations/" + filename)
	if err != nil {
		return migration{}, pkgerrors.Wrapf(err, "read migration file %s", filename)
	}

	version, name, err := parseMigrationFilename(filename)
	if err != nil {
		return migration{}, err
	}

	return migration{
		version: version,
		name:    name,
		sql:     string(content),
	}, nil
}

func parseMigrationFilename(filename string) (int, string, error) {
	base := strings.TrimSuffix(filename, ".sql")

	parts := strings.SplitN(base, "_", migrationFilenameParts)
	if len(parts) != migrationFilenameParts {
		return 0, "", pkgerrors.WithDetail(
			pkgerrors.ErrInvalidInput,
			"migration filename must match NNN_name.sql: "+filename,
		)
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", pkgerrors.WithDetail(
			pkgerrors.ErrInvalidInput,
			"migration version must be numeric: "+parts[0],
		)
	}

	return version, parts[1], nil
}

func RunMigrations(db *sql.DB) error {
	err := ensureMigrationsTable(db)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to create migrations table")
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to get applied migrations")
	}

	allMigrations, err := getMigrations()
	if err != nil {
		return pkgerrors.Wrap(err, "failed to load migrations")
	}

	sorted := make([]migration, len(allMigrations))
	copy(sorted, allMigrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].version < sorted[j].version
	})

	for _, mig := range sorted {
		if applied[mig.version] {
			continue
		}

		err = applyMigration(db, mig)
		if err != nil {
			return pkgerrors.Wrapf(err, "migration %d (%s) failed", mig.version, mig.name)
		}
	}

	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	return err
}

func getAppliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(context.Background(), "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()

	applied := make(map[int]bool)

	for rows.Next() {
		var version int

		err = rows.Scan(&version)
		if err != nil {
			return nil, err
		}

		applied[version] = true
	}

	return applied, rows.Err()
}

func applyMigration(db *sql.DB, mig migration) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return pkgerrors.Wrap(err, "begin transaction")
	}

	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(context.Background(), mig.sql)
	if err != nil {
		return pkgerrors.Wrap(err, "execute migration SQL")
	}

	_, err = tx.ExecContext(
		context.Background(),
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		mig.version, mig.name,
	)
	if err != nil {
		return pkgerrors.Wrap(err, "record migration")
	}

	return tx.Commit()
}
