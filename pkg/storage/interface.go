package storage

import (
	"context"
	"database/sql"

	"github.com/larsartmann/go-localsync/internal/db"
	"github.com/larsartmann/go-localsync/pkg/event"
)

type Storage interface {
	UpsertEvent(ctx context.Context, event *event.Event) error
	GetLatestEvent(ctx context.Context) (*event.Event, error)
	GetEvents(ctx context.Context, limit, offset int) ([]*event.Event, error)
	GetEventsByType(ctx context.Context, eventType string, limit, offset int) ([]*event.Event, error)
	GetEventsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*event.Event, error)
	GetEventsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*event.Event, error)
	CountEvents(ctx context.Context) (int64, error)
	CountEventsByType(ctx context.Context, eventType string) (int64, error)
	GetEventTypes(ctx context.Context) ([]string, error)
	Close() error
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func fromNullString(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func toDBEvent(e *event.Event) *db.UpsertEventParams {
	return &db.UpsertEventParams{
		GithubID:       e.GithubID,
		Type:           e.Type,
		ActorLogin:     toNullString(e.ActorLogin),
		ActorAvatarUrl: toNullString(e.ActorAvatarURL),
		RepoName:       toNullString(e.RepoName),
		RepoUrl:        toNullString(e.RepoURL),
		CreatedAt:      e.CreatedAt,
		RawJson:        e.RawJSON,
	}
}

func fromDBEvent(e *db.Events) *event.Event {
	return &event.Event{
		GithubID:       e.GithubID,
		Type:           e.Type,
		ActorLogin:     fromNullString(e.ActorLogin),
		ActorAvatarURL: fromNullString(e.ActorAvatarUrl),
		RepoName:       fromNullString(e.RepoName),
		RepoURL:        fromNullString(e.RepoUrl),
		CreatedAt:      e.CreatedAt,
		RawJSON:        e.RawJson,
	}
}
