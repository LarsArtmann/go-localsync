package api

import (
	"context"
	"errors"

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

	if input.Type != "" {
		t := id.NewEventTypeID(input.Type)
		filter.Type = &t
	}

	if input.ActorLogin != "" {
		a := id.NewActorLogin(input.ActorLogin)
		filter.ActorLogin = &a
	}

	if input.RepoName != "" {
		r := id.NewRepoID(input.RepoName)
		filter.RepoName = &r
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
		return nil, err
	}

	total, err := s.store.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	var resp ListItemsOutput

	resp.Body.Items = make([]*ItemResponse, len(items))
	for i, item := range items {
		resp.Body.Items[i] = toItemResponse(item)
	}

	resp.Body.Total = total

	return &resp, nil
}

func (s *Server) getStats(ctx context.Context, _ *struct{}) (*StatsOutput, error) {
	stats, err := s.syncer.GetStats(ctx)
	if err != nil {
		return nil, err
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
		return nil, huma.Error400BadRequest("invalid sync options", err)
	}

	result, err := s.syncer.Sync(ctx, &opts)
	if err != nil {
		return nil, mapSyncError(err)
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

func mapSyncError(err error) error {
	switch {
	case errors.Is(err, pkgerrors.ErrNotFound):
		return huma.Error404NotFound("not found", err)
	case errors.Is(err, pkgerrors.ErrRateLimited):
		return huma.Error429TooManyRequests("rate limited", err)
	case errors.Is(err, pkgerrors.ErrInvalidToken):
		return huma.Error401Unauthorized("invalid token", err)
	case errors.Is(err, pkgerrors.ErrUserNotFound):
		return huma.Error404NotFound("user not found", err)
	case errors.Is(err, pkgerrors.ErrDatabase):
		return huma.Error500InternalServerError("database error", err)
	case errors.Is(err, pkgerrors.ErrInvalidInput):
		return huma.Error400BadRequest("invalid input", err)
	case errors.Is(err, pkgerrors.ErrUnknownBackend):
		return huma.Error500InternalServerError("unknown backend", err)
	default:
		return huma.Error503ServiceUnavailable("sync failed", err)
	}
}
