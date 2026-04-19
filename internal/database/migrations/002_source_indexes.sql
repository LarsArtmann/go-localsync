-- Migration: 002_source_indexes
-- Description: Add indexes for multi-provider support
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_source_github_id ON events(source, github_id);
