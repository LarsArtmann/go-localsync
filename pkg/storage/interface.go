package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/larsartmann/go-localsync/internal/db"
)

type Event struct {
	GithubID       string          `json:"githubId"`
	Type           string          `json:"type"`
	ActorLogin     string          `json:"actorLogin,omitempty"`
	ActorAvatarURL string          `json:"actorAvatarUrl,omitempty"`
	RepoName       string          `json:"repoName,omitempty"`
	RepoURL        string          `json:"repoUrl,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	RawJSON        json.RawMessage `json:"rawJson"`
}

type Storage interface {
	UpsertEvent(ctx context.Context, event *Event) error
	GetLatestEvent(ctx context.Context) (*Event, error)
	GetEvents(ctx context.Context, limit, offset int) ([]*Event, error)
	GetEventsByType(ctx context.Context, eventType string, limit, offset int) ([]*Event, error)
	GetEventsByActor(ctx context.Context, actorLogin string, limit, offset int) ([]*Event, error)
	GetEventsByRepo(ctx context.Context, repoName string, limit, offset int) ([]*Event, error)
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

func toDBEvent(e *Event) *db.UpsertEventParams {
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

func fromDBEvent(e *db.Events) *Event {
	return &Event{
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
