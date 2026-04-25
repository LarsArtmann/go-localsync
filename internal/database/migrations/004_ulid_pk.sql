-- Migration: 004_ulid_pk
-- Description: Change events.id from INTEGER AUTOINCREMENT to TEXT (ULID)
-- Data will be re-synced from providers on next sync run.
DROP TABLE IF EXISTS events;

CREATE TABLE events (
    id TEXT PRIMARY KEY NOT NULL,
    source_id TEXT UNIQUE NOT NULL,
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
CREATE INDEX IF NOT EXISTS idx_events_source_id ON events(source_id);
CREATE INDEX IF NOT EXISTS idx_events_actor_login ON events(actor_login);
CREATE INDEX IF NOT EXISTS idx_events_repo_name ON events(repo_name);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_source_source_id ON events(source, source_id);
