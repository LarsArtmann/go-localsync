package api

import (
	"os"
	"testing"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestWithLogLevel_AppliesToServerLogger(t *testing.T) {
	logger := log.New(os.Stderr) // dedicated logger: avoids mutating the process-global default

	server := NewServer(newSyncerForTests(t), logger, WithLogLevel(log.WarnLevel))
	t.Cleanup(func() { _ = server.Close() })

	if got := logger.GetLevel(); got != log.WarnLevel {
		t.Fatalf("expected server logger level warn, got %v", got)
	}
}

func TestWithLogLevel_DefaultLeavesLevelUntouched(t *testing.T) {
	logger := log.New(os.Stderr)

	server := NewServer(newSyncerForTests(t), logger)
	t.Cleanup(func() { _ = server.Close() })

	if got := logger.GetLevel(); got != log.InfoLevel {
		t.Fatalf("expected default level info untouched, got %v", got)
	}
}

func TestWithLogLevel_GlobalFallbackDocumented(t *testing.T) {
	t.Cleanup(func() { log.Default().SetLevel(log.InfoLevel) })

	// Nil logger falls back to log.Default(); WithLogLevel then applies to the
	// global default (documented behavior of the option).
	server := NewServer(newSyncerForTests(t), nil, WithLogLevel(log.DebugLevel))
	t.Cleanup(func() { _ = server.Close() })

	if got := log.Default().GetLevel(); got != log.DebugLevel {
		t.Fatalf("expected global default logger level debug, got %v", got)
	}
	testutil.AssertTrue(t, server != nil, "server constructed")
}
