package database

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// Migration version constants.
const (
	migrationVersionInitial       = 1
	migrationVersionSourceIndexes = 2
)

type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: migrationVersionInitial,
		name:    "initial",
		sql:     migration001Initial,
	},
	{
		version: migrationVersionSourceIndexes,
		name:    "source_indexes",
		sql:     migration002SourceIndexes,
	},
}

func RunMigrations(db *sql.DB) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	applied, err := getAppliedVersions(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
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
			return fmt.Errorf("migration %d (%s) failed: %w", m.version, m.name, err)
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
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(context.Background(), m.sql); err != nil {
		return fmt.Errorf("execute migration SQL: %w", err)
	}

	if _, err := tx.ExecContext(
		context.Background(),
		"INSERT INTO schema_migrations (version, name) VALUES (?, ?)",
		m.version, m.name,
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

const migration001Initial = `
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    github_id TEXT UNIQUE NOT NULL,
    source TEXT NOT NULL DEFAULT 'github',
    type TEXT NOT NULL,
    actor_login TEXT,
    actor_avatar_url TEXT,
    repo_name TEXT,
    repo_url TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    raw_json JSON NOT NULL,
    synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_github_id ON events(github_id);
CREATE INDEX IF NOT EXISTS idx_events_actor_login ON events(actor_login);
CREATE INDEX IF NOT EXISTS idx_events_repo_name ON events(repo_name);
`

const migration002SourceIndexes = `
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_source_github_id ON events(source, github_id);
`
