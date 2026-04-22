-- Migration: 003_rename_github_id
-- Description: Rename github_id column to source_id for multi-provider support
ALTER TABLE events RENAME COLUMN github_id TO source_id;

DROP INDEX IF EXISTS idx_events_github_id;
CREATE INDEX IF NOT EXISTS idx_events_source_id ON events(source_id);

DROP INDEX IF EXISTS idx_events_source_github_id;
CREATE INDEX IF NOT EXISTS idx_events_source_source_id ON events(source, source_id);
