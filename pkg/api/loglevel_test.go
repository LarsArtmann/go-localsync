package api

import (
	"os"
	"testing"

	"charm.land/log/v2"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func newSyncerForTests(t *testing.T) *synclib.Syncer {
	t.Helper()

	return synclib.NewSyncer(&testutil.MockProvider{}, nil, log.New(os.Stderr))
}

func TestWithLogLevel_AppliesToServerLogger(t *testing.T) {
	logger := log.New(os.Stderr) // dedicated logger: avoids mutating the process-global default

	NewServer(newSyncerForTests(t), logger, WithLogLevel(log.WarnLevel))

	if got := logger.GetLevel(); got != log.WarnLevel {
		t.Fatalf("expected server logger level warn, got %v", got)
	}
}

func TestWithLogLevel_DefaultLeavesLevelUntouched(t *testing.T) {
	logger := log.New(os.Stderr)

	NewServer(newSyncerForTests(t), logger)

	if got := logger.GetLevel(); got != log.InfoLevel {
		t.Fatalf("expected default level info untouched, got %v", got)
	}
}

func TestWithLogLevel_GlobalFallbackDocumented(t *testing.T) {
	t.Cleanup(func() { log.Default().SetLevel(log.InfoLevel) })

	// Nil logger falls back to log.Default(); WithLogLevel then applies to the
	// global default (documented behavior of the option).
	NewServer(newSyncerForTests(t), nil, WithLogLevel(log.DebugLevel))

	if got := log.Default().GetLevel(); got != log.DebugLevel {
		t.Fatalf("expected global default logger level debug, got %v", got)
	}
}
