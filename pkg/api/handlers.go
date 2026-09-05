package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/larsartmann/go-localsync/pkg/data/model"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

func (s *Server) listItems(ctx context.Context, input *ListItemsInput) (*ListItemsOutput, error) {
	var filter model.ItemFilter

	filter.Limit = input.Limit
	filter.Offset = input.Offset

	if input.Cursor != "" {
		decoded, curErr := decodeCursor(input.Cursor)
		if curErr != nil {
			return nil, mapSyncError(curErr)
		}

		filter.Offset = decoded
	}

	if input.Type != "" {
		t := id.NewEventTypeID(input.Type)
		filter.Type = &t
	}

	if input.Source != "" {
		src := id.NewProviderID(input.Source)
		filter.Source = &src
	}

	if !input.Since.IsZero() {
		filter.Since = &input.Since
	}

	items, err := s.store.List(ctx, filter)
	if err != nil {
		return nil, mapSyncError(err)
	}

	total, err := s.store.Count(ctx, filter)
	if err != nil {
		return nil, mapSyncError(err)
	}

	var resp ListItemsOutput

	resp.Body.Items = make([]*ItemResponse, 0, len(items))
	for _, item := range items {
		resp.Body.Items = append(resp.Body.Items, toItemResponse(item))
	}

	resp.Body.Total = total
	resp.XTotalCount = total

	if next := nextCursor(filter.Offset, filter.Limit, len(items), total); next != "" {
		resp.NextCursor = next
	}

	return &resp, nil
}

func (s *Server) getStats(ctx context.Context, _ *struct{}) (*StatsOutput, error) {
	stats, err := s.syncer.GetStats(ctx)
	if err != nil {
		return nil, mapSyncError(err)
	}

	var resp StatsOutput

	resp.Body.TotalItems = stats.TotalItems
	resp.Body.ItemTypes = stats.ItemTypes
	resp.Body.TypeCounts = stats.TypeCounts

	return &resp, nil
}

func (s *Server) triggerSync(ctx context.Context, input *SyncInput) (*SyncOutput, error) {
	var opts synclib.SyncOptions

	opts.Source = input.Body.Source
	opts.MaxPages = input.Body.MaxPages

	err := opts.Validate()
	if err != nil {
		return nil, mapSyncError(err)
	}

	result, err := s.syncer.Sync(ctx, &opts)
	if err != nil {
		// A partial sync (some items failed) still returns a populated result;
		// surface it as 200 with the per-item error count rather than discarding
		// the successfully-synced items. Any other error is mapped to its status.
		if !errors.Is(err, pkgerrors.ErrPartialSync) || result == nil {
			return nil, mapSyncError(err)
		}
	}

	var resp SyncOutput

	resp.Body.Fetched = result.Fetched
	resp.Body.Skipped = result.Skipped
	resp.Body.Errors = result.Errors

	return &resp, nil
}

func (s *Server) healthCheck(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
	_, err := s.store.Count(ctx, model.ItemFilter{})
	if err != nil {
		return nil, huma.Error503ServiceUnavailable("store unavailable", err)
	}

	var resp HealthOutput

	resp.Body.Status = "healthy"

	return &resp, nil
}

// mapSyncError translates a domain error into a huma StatusError using the central
// pkgerrors.HTTPStatus mapping (per-sentinel overrides + error-family fallback).
// It is the single HTTP-boundary translator: every handler routes errors here so
// status codes stay consistent and newly added sentinels map automatically instead
// of silently hitting a brittle catch-all default.
func mapSyncError(err error) error {
	status := pkgerrors.HTTPStatus(err)

	return huma.NewError(status, http.StatusText(status), err)
}
