-- Events table for storing GitHub user events
-- Stores raw JSON payloads for 100% data fidelity
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

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_github_id ON events(github_id);
CREATE INDEX IF NOT EXISTS idx_events_actor_login ON events(actor_login);
CREATE INDEX IF NOT EXISTS idx_events_repo_name ON events(repo_name);
