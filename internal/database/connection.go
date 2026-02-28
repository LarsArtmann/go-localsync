package database

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

var schema = `
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    github_id TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL,
    actor_login TEXT,
    actor_avatar_url TEXT,
    repo_name TEXT,
    repo_url TEXT,
    created_at DATETIME NOT NULL,
    raw_json JSON NOT NULL,
    synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_github_id ON events(github_id);
CREATE INDEX IF NOT EXISTS idx_events_actor_login ON events(actor_login);
CREATE INDEX IF NOT EXISTS idx_events_repo_name ON events(repo_name);
`

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()

		return nil, err
	}

	return db, nil
}
