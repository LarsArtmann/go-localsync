package database

import (
	"context"
	"database/sql"
	"embed"
	"sort"
	"strconv"
	"strings"

	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

var migrations []migration

func init() {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		panic("failed to read embedded migrations: " + err.Error())
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		m, err := parseMigrationFile(entry.Name())
		if err != nil {
			panic("failed to parse migration " + entry.Name() + ": " + err.Error())
		}

		migrations = append(migrations, m)
	}
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

	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
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
	if err := ensureMigrationsTable(db); err != nil {
		return pkgerrors.Wrap(err, "failed to create migrations table")
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return pkgerrors.Wrap(err, "failed to get applied migrations")
	}

	sorted := make([]migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].version < sorted[j].version
	})

	for _, m := range sorted {
		if applied[m.version] {
			continue
		}

		err := applyMigration(db, m)
		if err != nil {
			return pkgerrors.Wrapf(err, "migration %d (%s) failed", m.version, m.name)
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
	defer rows.Close()

	applied := make(map[int]bool)

	for rows.Next() {
		var v int

		err := rows.Scan(&v)
		if err != nil {
			return nil, err
		}

		applied[v] = true
	}

	return applied, rows.Err()
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return pkgerrors.Wrap(err, "begin transaction")
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(context.Background(), m.sql); err != nil {
		return pkgerrors.Wrap(err, "execute migration SQL")
	}

	if _, err := tx.ExecContext(
		context.Background(),
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		m.version, m.name,
	); err != nil {
		return pkgerrors.Wrap(err, "record migration")
	}

	return tx.Commit()
}
