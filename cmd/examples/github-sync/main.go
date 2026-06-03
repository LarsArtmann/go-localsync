package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"charm.land/log/v2"
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
	pkgerrors.RegisterErrorTemplates()

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
			"Storage backend: memory, sqlite",
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

	//nolint:exhaustruct // ConflictResolver intentionally omitted (uses default)
	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{
		Backend: *backend,
		DBPath:  *dbPath,
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
}
