package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/api"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/providers/github"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
)

const (
	exitUsage       = 64
	exitDataErr     = 65
	exitNoInput     = 66
	exitUnavailable = 69
	exitSoftware    = 70
	exitTempFail    = 75
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	envCfg, err := LoadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)

		os.Exit(exitSoftware)
	}

	var (
		token = flag.String(
			"token",
			envCfg.Token,
			"GitHub personal access token (or set GITHUB_TOKEN env)",
		)
		username = flag.String("user", envCfg.Username, "GitHub username to sync events for")
		dbPath   = flag.String(
			"db",
			envCfg.DBPath,
			"Path to database directory (default: ~/.local/share/go-localsync)",
		)
		backend = flag.String(
			"backend",
			envCfg.Backend,
			"Storage backend: memory, turso",
		)
		maxPages      = flag.Int("pages", envCfg.MaxPages, "Maximum number of pages to fetch")
		incremental   = flag.Bool("incremental", envCfg.Incremental, "Only sync new events")
		conflictAware = flag.Bool(
			"conflict-aware",
			envCfg.ConflictAware,
			"Use conflict-aware sync with CRDT resolution",
		)
		showStats   = flag.Bool("stats", envCfg.ShowStats, "Show database statistics and exit")
		showVersion = flag.Bool("version", false, "Show version information and exit")
		syncPush    = flag.Bool("push", false, "Push local changes to remote Turso after sync")
		syncPull    = flag.Bool("pull", false, "Pull remote changes from Turso before sync")
		verbose     = flag.Bool("verbose", envCfg.Verbose, "Enable verbose logging")
		jsonOutput  = flag.Bool("json", false, "Output results as JSON")
		serverMode  = flag.Bool("server", false, "Start HTTP API server")
		serverPort  = flag.Int("port", 8080, "HTTP server port (only with -server)")
	)

	flag.Parse()

	if *showVersion {
		printVersion(os.Stdout)
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

		*dbPath = filepath.Join(homeDir, ".local", "share", "go-localsync")
	}

	if err := os.MkdirAll(*dbPath, 0o755); err != nil {
		logger.Error("Failed to create database directory", "error", err)
		os.Exit(exitNoInput)
	}

	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{
		Backend:   *backend,
		DBPath:    *dbPath,
		RemoteURL: envCfg.RemoteURL,
		AuthToken: envCfg.AuthToken,
	})
	if err != nil {
		logger.Error("Failed to create CQRS stack", "error", err)
		os.Exit(exitNoInput)
	}

	defer func() {
		err := stack.Close()
		if err != nil {
			logger.Error("Failed to close stack", "error", err)
		}
	}()

	if *showStats {
		runStats(stack, *jsonOutput, logger)
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

	ghProvider := github.NewClient(*token)
	baseSyncer := synclib.NewSyncer(ghProvider, stack, logger)

	if *serverMode {
		runAPIServer(baseSyncer, stack, *serverPort, logger)
	}

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

	if *syncPull {
		changed, err := stack.Pull(ctx)
		if err != nil {
			logger.Warn("Pull failed (non-fatal)", "error", err)
		} else if changed {
			logger.Info("Pulled remote changes")
		}
	}

	if *conflictAware {
		runConflictAwareSync(ctx, baseSyncer, opts, *jsonOutput, logger)
	}

	var result *synclib.SyncResult
	if *incremental {
		result, err = baseSyncer.SyncIncremental(ctx, opts)
	} else {
		result, err = baseSyncer.Sync(ctx, opts)
	}

	if err != nil {
		logFatalAndExit(logger, "Sync failed", err)
	}

	if *jsonOutput {
		printSyncResultJSON(result)
	} else {
		fmt.Printf(
			"Sync completed: fetched=%d, skipped=%d, errors=%d\n",
			result.Fetched,
			result.Skipped,
			result.Errors,
		)
	}

	if *syncPush {
		if err := stack.Push(ctx); err != nil {
			logger.Warn("Push failed (non-fatal)", "error", err)
		} else {
			logger.Info("Pushed local changes to remote")
		}
	}
}

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
	stats, err := stack.Count(context.Background())
	if err != nil {
		logger.Error("Failed to get stats", "error", err)
		os.Exit(exitSoftware)
	}

	types, err := stack.GetTypes(context.Background())
	if err != nil {
		logger.Error("Failed to get item types", "error", err)
	}

	if jsonOutput {
		out := statsOutput{TotalItems: stats, ItemTypes: types}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(out); err != nil {
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
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if err := enc.Encode(out); err != nil {
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
	out := syncResultOutput{Fetched: result.Fetched, Skipped: result.Skipped, Errors: result.Errors}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	if err := enc.Encode(out); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to encode sync result JSON: %v\n", err)
	}
}

func printVersion(w io.Writer) {
	_, _ = fmt.Fprintf(w, "gh-sync %s (commit: %s, built: %s)\n", version, commit, date)
}

func logFatalAndExit(logger *log.Logger, msg string, err error) {
	logger.Error(msg, "error", err)
	os.Exit(exitCodeForError(err))
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
	store synclib.SyncStore,
	port int,
	logger *log.Logger,
) {
	server := api.NewServer(syncer, store, logger)
	addr := fmt.Sprintf(":%d", port)

	logger.Info("Starting API server", "address", addr)

	if err := http.ListenAndServe(addr, server); err != nil {
		logger.Error("API server error", "error", err)
		os.Exit(exitSoftware)
	}
}
