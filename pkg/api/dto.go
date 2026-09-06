package api

import (
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
)

// ListItemsInput defines the query parameters for listing items.
//
//nolint:tagalign // doc/example/query tags intentionally kept close to values for readability
type ListItemsInput struct {
	Type              string    `doc:"Filter by event type"                     example:"PushEvent"        query:"type"`
	Source            string    `doc:"Filter by source provider"                  example:"github"           query:"source"`
	IncludeTombstoned bool      `doc:"Include items hidden by a tombstone (they carry a tombstone object in the response)" query:"includeTombstoned"`
	Since             time.Time `doc:"Filter items updated since this time (RFC3339)"                                             query:"since"`
	Limit             int       `doc:"Maximum items to return"                                                                    query:"limit"  default:"100"`
	Offset            int       `doc:"Offset for pagination"                                                                      query:"offset" default:"0"`
	Cursor            string    `doc:"Opaque pagination cursor (from X-Next-Cursor); overrides offset" example:"b2Zmc2V0PTEwMA==" query:"cursor"`
}

// TombstoneInfo is the API DTO for an item's tombstone: why it is hidden
// from the default view and since when. It appears on ItemResponse only for
// tombstoned items (and only when the client asked to include them).
type TombstoneInfo struct {
	Reason string    `doc:"Why the item is hidden: upstream_gone, user_hidden, or redacted" example:"upstream_gone" json:"reason"`
	At     time.Time `doc:"When the tombstone was applied (RFC3339, UTC)"                   example:"2026-09-06T12:00:00Z" json:"tombstonedAt"`
}

// ItemResponse is the API DTO for a synced item.
type ItemResponse struct {
	ID         string            `json:"id"`
	SourceID   string            `json:"sourceId"`
	Source     string            `json:"source"`
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Tombstone  *TombstoneInfo    `doc:"Present only when the item is tombstoned" json:"tombstone,omitempty"`
}

func toItemResponse(item *model.Item) *ItemResponse {
	if item == nil {
		return nil
	}

	resp := &ItemResponse{
		ID:         item.ID.String(),
		SourceID:   item.SourceID.Get(),
		Source:     item.Source.Get(),
		Type:       item.Type.Get(),
		Attributes: item.Attributes,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}

	if item.IsTombstoned() {
		// ParseTombstoneReason degrades a reason string that predates the
		// typed vocabulary to upstream_gone, so the DTO never lies with an
		// empty or unknown reason.
		resp.Tombstone = &TombstoneInfo{
			Reason: string(model.ParseTombstoneReason(string(item.Tombstone.Reason))),
			At:     item.Tombstone.At,
		}
	}

	return resp
}

// ListItemsOutput defines the response for listing items.
type ListItemsOutput struct {
	Body struct {
		Items []*ItemResponse `doc:"List of sync items"                        json:"items"`
		Total int64           `doc:"Total number of items matching the filter" json:"total"`
	}

	XTotalCount int64  `doc:"Total items matching the filter"                         header:"X-Total-Count"`
	NextCursor  string `doc:"Opaque cursor for the next page; empty on the last page" header:"X-Next-Cursor"`
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
