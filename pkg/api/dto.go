package api

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// ListItemsInput defines the query parameters for listing items.
//
//nolint:tagalign // doc/example/query tags intentionally kept close to values for readability
type ListItemsInput struct {
	Type   string    `doc:"Filter by event type"                           example:"PushEvent" query:"type"`
	Source string    `doc:"Filter by source provider"                      example:"github"    query:"source"`
	Since  time.Time `doc:"Filter items updated since this time (RFC3339)"                     query:"since"`
	Limit  int       `doc:"Maximum items to return"                                            query:"limit"  default:"100"`
	Offset int       `doc:"Offset for pagination"                                              query:"offset" default:"0"`
	Cursor string    `doc:"Opaque pagination cursor (from X-Next-Cursor); overrides offset"    query:"cursor" example:"b2Zmc2V0PTEwMA=="`
}

// ItemResponse is the API DTO for a synced item.
type ItemResponse struct {
	ID         string            `json:"id"`
	ExternalID string            `json:"externalId"`
	Source     string            `json:"source"`
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

func toItemResponse(item *model.Item) *ItemResponse {
	if item == nil {
		return nil
	}

	return &ItemResponse{
		ID:         item.ID.String(),
		ExternalID: item.ExternalID.Get(),
		Source:     item.Source.Get(),
		Type:       item.Type.Get(),
		Attributes: item.Attributes,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}
}

// ListItemsOutput defines the response for listing items.
type ListItemsOutput struct {
	Body struct {
		Items []*ItemResponse `doc:"List of sync items"                        json:"items"`
		Total int64           `doc:"Total number of items matching the filter" json:"total"`
	}

	XTotalCount int64  `header:"X-Total-Count" doc:"Total items matching the filter"`
	NextCursor  string `header:"X-Next-Cursor" doc:"Opaque cursor for the next page; empty on the last page"`
}

// StatsOutput defines the response for statistics.
type StatsOutput struct {
	Body struct {
		TotalItems int64            `doc:"Total number of synced items" json:"totalItems"`
		ItemTypes  []string         `doc:"List of distinct item types"  json:"itemTypes"`
		TypeCounts map[string]int64 `doc:"Count of items per type"      json:"typeCounts"`
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
