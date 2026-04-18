// Package main provides a GitHub sync example using the go-localsync SDK.
// This demonstrates how to use the SDK to build a local sync application.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/internal/database"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/providers/github"
	"github.com/larsartmann/go-localsync/pkg/storage"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

// Semantic exit codes (sysexits.h conventions).
const (
	exitUsage       = 64 // EX_USAGE - command line usage error
	exitDataErr     = 65 // EX_DATAERR - data format error
	exitNoInput     = 66 // EX_NOINPUT - cannot open input
	exitUnavailable = 69 // EX_UNAVAILABLE - service unavailable
	exitSoftware    = 70 // EX_SOFTWARE - internal software error
	exitTempFail    = 75 // EX_TEMPFAIL - temporary failure
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		token = flag.String(
			"token",
			os.Getenv("GITHUB_TOKEN"),
			"GitHub personal access token (or set GITHUB_TOKEN env)",
		)
		username = flag.String("user", "", "GitHub username to sync events for")
		dbPath   = flag.String(
			"db",
			"",
			"Path to SQLite database (default: ~/.local/share/go-localsync/events.db)",
		)
		maxPages       = flag.Int("pages", 10, "Maximum number of pages to fetch")
		incremental    = flag.Bool("incremental", true, "Only sync new events")
		conflictAware  = flag.Bool("conflict-aware", false, "Use conflict-aware sync with CRDT resolution")
		showStats      = flag.Bool("stats", false, "Show database statistics and exit")
		showVersion = flag.Bool("version", false, "Show version information and exit")
		verbose     = flag.Bool("verbose", false, "Enable verbose logging")
	)

	flag.Parse()

	if *showVersion {
		fmt.Printf("gh-sync %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	logger := log.New(os.Stderr)
	if *verbose {
		logger.SetLevel(log.DebugLevel)
	}

	if *dbPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory", "error", err)
			os.Exit(exitNoInput)
		}

		*dbPath = filepath.Join(homeDir, ".local", "share", "go-localsync", "events.db")
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0o755); err != nil {
		logger.Error("Failed to create database directory", "error", err)
		os.Exit(exitNoInput)
	}

	dbc, err := database.Open(*dbPath)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(exitNoInput)
	}

	defer func() {
		err := dbc.Close()
		if err != nil {
			logger.Error("Failed to close database", "error", err)
		}
	}()

	store := storage.NewSQLiteStorage(dbc)

	if *showStats {
		stats, err := store.Count(context.Background())
		if err != nil {
			logger.Error("Failed to get stats", "error", err)
			os.Exit(exitSoftware)
		}

		types, err := store.GetTypes(context.Background())
		if err != nil {
			logger.Error("Failed to get item types", "error", err)
		}

		fmt.Printf("Total items: %d\n", stats)
		fmt.Printf("Item types: %v\n", types)
		os.Exit(0)
	}

	if *token == "" {
		logger.Error(
			"GitHub token is required. Use -token flag or set GITHUB_TOKEN environment variable",
		)
		os.Exit(exitUsage)
	}

	if *username == "" {
		logger.Error("Username is required. Use -user flag")
		os.Exit(exitUsage)
	}

	// Create GitHub provider
	ghProvider := github.NewClient(*token)
	baseSyncer := synclib.NewSyncer(ghProvider, store, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received interrupt signal, shutting down...")
		cancel()
	}()

	opts := &synclib.SyncOptions{
		Source:   *username,
		MaxPages: *maxPages,
		OnProgress: func(fetched, skipped, errors int) {
			logger.Info("Progress", "fetched", fetched, "skipped", skipped, "errors", errors)
		},
	}

	if *conflictAware {
		cas := synclib.NewConflictAwareSyncer(baseSyncer)
		cr, err := cas.SyncWithConflictDetection(ctx, opts)
		if err != nil {
			logger.Error("Conflict-aware sync failed", "error", err)
			os.Exit(exitCodeForError(err))
		}

		fmt.Printf(
				"Sync completed: fetched=%d, upserted=%d, conflicts=%d, skipped=%d, errors=%d\n",
				cr.Fetched, cr.Upserted, cr.Conflicts, cr.Skipped, cr.Errors,
			)

		return
	}

	var result *synclib.SyncResult
	if *incremental {
		result, err = baseSyncer.SyncIncremental(ctx, opts)
	} else {
		result, err = baseSyncer.Sync(ctx, opts)
	}

	if err != nil {
		logger.Error("Sync failed", "error", err)
		os.Exit(exitCodeForError(err))
	}

	fmt.Printf(
		"Sync completed: fetched=%d, skipped=%d, errors=%d\n",
		result.Fetched,
		result.Skipped,
		result.Errors,
	)
}

// exitCodeForError maps errors to semantic exit codes.
func exitCodeForError(err error) int {
	switch {
	case errors.Is(err, pkgerrors.ErrRateLimited):
		return exitTempFail
	case errors.Is(err, pkgerrors.ErrInvalidToken):
		return exitUsage
	case errors.Is(err, pkgerrors.ErrUserNotFound):
		return exitDataErr
	case errors.Is(err, pkgerrors.ErrStorage):
		return exitNoInput
	case errors.Is(err, pkgerrors.ErrSyncFailed):
		return exitSoftware
	default:
		return exitUnavailable
	}
}
