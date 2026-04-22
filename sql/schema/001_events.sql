-- Events table for storing synced items from any provider
-- Stores raw JSON payloads for 100% data fidelity
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
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

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_source_id ON events(source_id);
CREATE INDEX IF NOT EXISTS idx_events_actor_login ON events(actor_login);
CREATE INDEX IF NOT EXISTS idx_events_repo_name ON events(repo_name);
