package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/api"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

type statsOutput struct {
	TotalItems int64    `json:"totalItems"`
	ItemTypes  []string `json:"itemTypes"`
}

type conflictResultOutput struct {
	Fetched   int `json:"fetched"`
	Upserted  int `json:"upserted"`
	Conflicts int `json:"conflicts"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

type syncResultOutput struct {
	Fetched int `json:"fetched"`
	Skipped int `json:"skipped"`
	Errors  int `json:"errors"`
}

func runStats(stack *cqrs.CQRSStack, jsonOutput bool, logger *log.Logger) {
	stats, err := stack.Count(context.Background(), provider.ItemFilter{})
	if err != nil {
		logErrorAndExit(logger, "Failed to get stats", err, exitSoftware)
	}

	types, err := stack.GetTypes(context.Background())
	if err != nil {
		logger.Error("Failed to get item types", "error", err)
	}

	if jsonOutput {
		if err := encodeIndentedJSON(os.Stdout, statsOutput{TotalItems: stats, ItemTypes: types}); err != nil {
			logger.Error("Failed to encode stats JSON", "error", err)
		}
	} else {
		fmt.Printf("Total items: %d\n", stats)
		fmt.Printf("Item types: %v\n", types)
	}

	os.Exit(0)
}

func runConflictAwareSync(
	ctx context.Context,
	baseSyncer *synclib.Syncer,
	opts *synclib.SyncOptions,
	jsonOutput bool,
	logger *log.Logger,
) {
	cas := synclib.NewConflictAwareSyncer(baseSyncer)

	cr, err := cas.SyncWithConflictDetection(ctx, opts)
	if err != nil {
		logFatalAndExit(logger, "Conflict-aware sync failed", err)
	}

	if jsonOutput {
		out := conflictResultOutput{
			Fetched:   cr.Fetched,
			Upserted:  cr.Upserted,
			Conflicts: cr.Conflicts,
			Skipped:   cr.Skipped,
			Errors:    cr.Errors,
		}

		if err := encodeIndentedJSON(os.Stdout, out); err != nil {
			logger.Error("Failed to encode conflict result JSON", "error", err)
		}
	} else {
		fmt.Printf(
			"Sync completed: fetched=%d, upserted=%d, conflicts=%d, skipped=%d, errors=%d\n",
			cr.Fetched, cr.Upserted, cr.Conflicts, cr.Skipped, cr.Errors,
		)
	}

	os.Exit(0)
}

func printSyncResultJSON(result *synclib.SyncResult) {
	printSyncResultJSONToWriter(result, os.Stdout)
}

func printSyncResultJSONToWriter(result *synclib.SyncResult, w io.Writer) {
	if err := encodeIndentedJSON(w, syncResultOutput{
		Fetched: result.Fetched,
		Skipped: result.Skipped,
		Errors:  result.Errors,
	}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to encode sync result JSON: %v\n", err)
	}
}

func encodeIndentedJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(v)
}

func printVersion(w io.Writer) {
	_, _ = fmt.Fprintf(w, "gh-sync %s (commit: %s, built: %s)\n", version, commit, date)
}

func logFatalAndExit(logger *log.Logger, msg string, err error) {
	logger.Error(msg, "error", err)
	os.Exit(exitCodeForError(err))
}

func logErrorAndExit(logger *log.Logger, msg string, err error, code int) {
	logger.Error(msg, "error", err)
	os.Exit(code)
}

func exitCodeForError(err error) int {
	switch {
	case errors.Is(err, pkgerrors.ErrRateLimited):
		return exitTempFail
	case errors.Is(err, pkgerrors.ErrInvalidToken):
		return exitUsage
	case errors.Is(err, pkgerrors.ErrUserNotFound):
		return exitDataErr
	case errors.Is(err, pkgerrors.ErrDatabase):
		return exitNoInput
	case errors.Is(err, pkgerrors.ErrSyncFailed):
		return exitSoftware
	default:
		return exitUnavailable
	}
}

func runAPIServer(
	syncer *synclib.Syncer,
	port int,
	logger *log.Logger,
) {
	server := api.NewServer(syncer, logger)
	addr := fmt.Sprintf(":%d", port)

	logger.Info("Starting API server", "address", addr)

	if err := http.ListenAndServe(addr, server); err != nil {
		logger.Error("API server error", "error", err)
		os.Exit(exitSoftware)
	}
}
