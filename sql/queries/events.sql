-- name: UpsertEvent :exec
-- Insert an event, updating if it already exists (conflict resolution)
INSERT INTO events (
    id,
    source_id,
    source,
    type,
    actor_login,
    actor_avatar_url,
    repo_name,
    repo_url,
    created_at,
    updated_at,
    raw_json,
    synced_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(source_id) DO UPDATE SET
    source = excluded.source,
    type = excluded.type,
    actor_login = excluded.actor_login,
    actor_avatar_url = excluded.actor_avatar_url,
    repo_name = excluded.repo_name,
    repo_url = excluded.repo_url,
    updated_at = excluded.updated_at,
    raw_json = excluded.raw_json;

-- name: GetLatestEvent :one
-- Get the most recent event by creation timestamp (for incremental sync)
SELECT * FROM events
ORDER BY created_at DESC
LIMIT 1;

-- name: GetEventBySourceID :one
-- Get a single event by its source ID
SELECT * FROM events
WHERE source_id = ?;

-- name: GetEvents :many
-- Get all events, ordered by creation date (newest first)
SELECT * FROM events
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetEventsByType :many
-- Get events filtered by type
SELECT * FROM events
WHERE type = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetEventsByActor :many
-- Get events filtered by actor login
SELECT * FROM events
WHERE actor_login = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetEventsByRepo :many
-- Get events filtered by repository name
SELECT * FROM events
WHERE repo_name = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetEventsBySource :many
-- Get events filtered by source provider
SELECT * FROM events
WHERE source = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: GetEventsSince :many
-- Get events created after a specific timestamp (for incremental sync)
SELECT * FROM events
WHERE created_at > ?
ORDER BY created_at DESC;

-- name: CountEvents :one
-- Count total number of events
SELECT COUNT(*) as count FROM events;

-- name: CountEventsByType :one
-- Count events by type
SELECT COUNT(*) as count FROM events
WHERE type = ?;

-- name: DeleteEventBySourceID :exec
-- Delete an event by its source ID
DELETE FROM events WHERE source_id = ?;

-- name: DeleteAllEvents :exec
-- Delete all events (use with caution)
DELETE FROM events;

-- name: GetEventTypes :many
-- Get distinct event types in the database
SELECT DISTINCT type FROM events
ORDER BY type;
