package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/larsartmann/go-localsync/internal/database"
	"github.com/larsartmann/go-localsync/pkg/github"
	"github.com/larsartmann/go-localsync/pkg/storage"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		token       = flag.String("token", os.Getenv("GITHUB_TOKEN"), "GitHub personal access token (or set GITHUB_TOKEN env)")
		username    = flag.String("user", "", "GitHub username to sync events for")
		dbPath      = flag.String("db", "", "Path to SQLite database (default: ~/.local/share/go-localsync/events.db)")
		maxPages    = flag.Int("pages", 10, "Maximum number of pages to fetch")
		incremental = flag.Bool("incremental", true, "Only sync new events")
		showStats   = flag.Bool("stats", false, "Show database statistics and exit")
		showVersion = flag.Bool("version", false, "Show version information and exit")
		verbose     = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("go-localsync %s (commit: %s, built: %s)\n", version, commit, date)
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
			os.Exit(1)
		}
		*dbPath = filepath.Join(homeDir, ".local", "share", "go-localsync", "events.db")
	}

	if err := os.MkdirAll(filepath.Dir(*dbPath), 0755); err != nil {
		logger.Error("Failed to create database directory", "error", err)
		os.Exit(1)
	}

	dbc, err := database.Open(*dbPath)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer dbc.Close()

	store := storage.NewSQLiteStorage(dbc)

	if *showStats {
		stats, err := store.CountEvents(context.Background())
		if err != nil {
			logger.Error("Failed to get stats", "error", err)
			os.Exit(1)
		}
		types, _ := store.GetEventTypes(context.Background())
		fmt.Printf("Total events: %d\n", stats)
		fmt.Printf("Event types: %v\n", types)
		os.Exit(0)
	}

	if *token == "" {
		logger.Error("GitHub token is required. Use -token flag or set GITHUB_TOKEN environment variable")
		os.Exit(1)
	}

	if *username == "" {
		logger.Error("Username is required. Use -user flag")
		os.Exit(1)
	}

	ghClient := github.NewClient(*token)
	syncer := synclib.NewSyncer(ghClient, store, logger)

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
		Username: *username,
		MaxPages: *maxPages,
	}

	var result *synclib.SyncResult
	if *incremental {
		result, err = syncer.SyncIncremental(ctx, opts)
	} else {
		result, err = syncer.Sync(ctx, opts)
	}

	if err != nil {
		logger.Error("Sync failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Sync completed: fetched=%d, skipped=%d, errors=%d\n", result.Fetched, result.Skipped, result.Errors)
}
