// Package storage defines the interface for storing and retrieving sync items.
package storage

import (
	"context"
	"database/sql"

	"github.com/larsartmann/go-localsync/internal/db"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/types"
)

// Storage defines the interface for storing and retrieving sync items.
type Storage interface {
	// Upsert inserts or updates an item. ID is used as the unique key.
	Upsert(ctx context.Context, item *provider.Item) error
	// GetLatest returns the most recently created item, or nil if empty.
	GetLatest(ctx context.Context) (*provider.Item, error)
	// GetItems retrieves items with pagination.
	GetItems(ctx context.Context, limit, offset int) ([]*provider.Item, error)
	// GetItemsByType retrieves items filtered by type.
	GetItemsByType(
		ctx context.Context,
		itemType string,
		limit, offset int,
	) ([]*provider.Item, error)
	// GetItemsByActor retrieves items filtered by actor login.
	GetItemsByActor(
		ctx context.Context,
		actorLogin string,
		limit, offset int,
	) ([]*provider.Item, error)
	// GetItemsByRepo retrieves items filtered by repository name.
	GetItemsByRepo(
		ctx context.Context,
		repoName string,
		limit, offset int,
	) ([]*provider.Item, error)
	// Count returns the total number of items.
	Count(ctx context.Context) (int64, error)
	// CountByType returns the number of items of a specific type.
	CountByType(ctx context.Context, itemType string) (int64, error)
	// GetTypes returns all unique item types.
	GetTypes(ctx context.Context) ([]string, error)
	// Close releases resources.
	Close() error
}

// Legacy Storage interface names (deprecated, use new methods)
// These are kept for backward compatibility during migration.

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

// toItem converts a database row to provider.Item.
// Note: The database column is named "github_id" for backward compatibility,
// but we treat it as a generic source ID.
func toItem(e *db.Events) *provider.Item {
	return &provider.Item{
		ID:             types.NewItemID(e.GithubID),   // Map github_id column to generic ID
		Source:         types.NewProviderID("github"), // Default to github for existing data
		Type:           types.NewEventTypeID(e.Type),  // Convert type string to branded ID
		ActorLogin:     types.NewActorID(fromNullString(e.ActorLogin)),
		ActorAvatarURL: fromNullString(e.ActorAvatarUrl),
		RepoName:       types.NewRepoID(fromNullString(e.RepoName)),
		RepoURL:        fromNullString(e.RepoUrl),
		CreatedAt:      e.CreatedAt,
		RawJSON:        e.RawJson,
	}
}

// toDBParams converts provider.Item to database parameters.
// Note: Item.ID is stored in the "github_id" column for backward compatibility.
func toDBParams(item *provider.Item) *db.UpsertEventParams {
	return &db.UpsertEventParams{
		GithubID:       item.ID.Get(), // Store generic ID in github_id column
		Type:           item.Type.Get(),
		ActorLogin:     toNullString(item.ActorLogin.Get()),
		ActorAvatarUrl: toNullString(item.ActorAvatarURL),
		RepoName:       toNullString(item.RepoName.Get()),
		RepoUrl:        toNullString(item.RepoURL),
		CreatedAt:      item.CreatedAt,
		RawJson:        item.RawJSON,
	}
}
