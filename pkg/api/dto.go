package api

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/data/schema"
	"github.com/larsartmann/go-localsync/pkg/id"
)

// ListItemsInput defines the query parameters for listing items.
//
//nolint:tagalign // doc/example/query tags intentionally kept close to values for readability
type ListItemsInput struct {
	Type       string    `doc:"Filter by event type"                           example:"PushEvent"                query:"type"`
	ActorLogin string    `doc:"Filter by actor login"                          example:"larsartmann"              query:"actor"`
	RepoName   string    `doc:"Filter by repository name"                      example:"larsartmann/go-localsync" query:"repo"`
	Source     string    `doc:"Filter by source provider"                      example:"github"                   query:"source"`
	Since      time.Time `doc:"Filter items updated since this time (RFC3339)"                                    query:"since"`
	Limit      int       `doc:"Maximum items to return"                                                           query:"limit"  default:"100"`
	Offset     int       `doc:"Offset for pagination"                                                             query:"offset" default:"0"`
}

// ItemResponse is the API DTO for a synced item.
type ItemResponse struct {
	ID             string    `json:"id"`
	ExternalID     string    `json:"externalId"`
	Source         string    `json:"source"`
	Type           string    `json:"type"`
	ActorLogin     string    `json:"actorLogin"`
	ActorAvatarURL string    `json:"actorAvatarUrl,omitempty"`
	RepoName       string    `json:"repoName"`
	RepoURL        string    `json:"repoUrl,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func toItemResponse(item *model.Item) *ItemResponse {
	if item == nil {
		return nil
	}

	return &ItemResponse{
		ID:             item.ID.String(),
		ExternalID:     item.ExternalID.Get(),
		Source:         item.Source.Get(),
		Type:           item.Type.Get(),
		ActorLogin:     item.ActorLogin.Get(),
		ActorAvatarURL: item.ActorAvatarURL,
		RepoName:       item.RepoName.Get(),
		RepoURL:        item.RepoURL,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

// ListItemsOutput defines the response for listing items.
type ListItemsOutput struct {
	Body struct {
		Items []*ItemResponse `doc:"List of sync items"                        json:"items"`
		Total int64           `doc:"Total number of items matching the filter" json:"total"`
	}
}

// StatsOutput defines the response for statistics.
type StatsOutput struct {
	Body struct {
		TotalItems int64    `doc:"Total number of synced items" json:"totalItems"`
		ItemTypes  []string `doc:"List of distinct item types"  json:"itemTypes"`
	}
}

// SyncInput defines the request body for triggering a sync.
type SyncInput struct {
	Body struct {
		Source   string `doc:"Source to sync" example:"larsartmann"        json:"source"`
		MaxPages int    `default:"0"          doc:"Maximum pages to fetch" json:"maxPages"`
	}
}

// SyncOutput defines the response for a sync operation.
type SyncOutput struct {
	Body struct {
		Fetched int `doc:"Number of items fetched" json:"fetched"`
		Skipped int `doc:"Number of items skipped" json:"skipped"`
		Errors  int `doc:"Number of errors"        json:"errors"`
	}
}

// HealthOutput defines the response for the health check.
type HealthOutput struct {
	Body struct {
		Status string `doc:"Health status" json:"status"`
	}
}

// newTestItem creates a model.Item for use in API tests.
func newTestItem(itemID, eventType string) *model.Item {
	now := time.Now()

	return &model.Item{
		ID:            id.NewItemID(),
		ExternalID:    id.NewExternalID(itemID),
		Source:        id.NewProviderID("github"),
		Type:          id.NewEventTypeID(eventType),
		ActorLogin:    id.NewActorID("testuser"),
		RepoName:      id.NewRepoID("test/repo"),
		CreatedAt:     now,
		UpdatedAt:     now,
		SchemaVersion: schema.CurrentVersion(),
	}
}
