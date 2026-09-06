package sync_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"charm.land/log/v2"
	"github.com/larsartmann/go-localsync/pkg/cqrs"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	synclib "github.com/larsartmann/go-localsync/pkg/sync"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestWiring_EventLoggerReceivesEventLogLines proves the consumer-facing
// EventLogger seam end to end: a stack built with CQRSConfig.EventLogger,
// driven through the Syncer (the way consumers actually drive it), must
// route its per-event middleware lines ("event succeeded", event type,
// stream) to THAT logger — not to the global default. Only the default path
// (EventLogger nil → log.Default) was previously covered; this is the
// explicit-sink path across the sync→cqrs seam.
func TestWiring_EventLoggerReceivesEventLogLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	consumerLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	stack, err := cqrs.NewCQRSStack(cqrs.CQRSConfig{
		Backend:     "memory",
		EventLogger: consumerLogger,
	})
	testutil.MustNoError(t, err)
	defer func() { _ = stack.Close() }()

	items := []*provider.Item{{
		SourceID: id.NewSourceID("wiring-1"),
		Source:   id.NewProviderID("github"),
		Type:     id.NewEventTypeID("PushEvent"),
		Attributes: map[string]string{
			"actor_login": "testuser",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}}

	syncer := synclib.NewSyncer(&testutil.MockProvider{Items: items}, stack, log.Default())

	_, err = syncer.Sync(context.Background(), &synclib.SyncOptions{Source: "github"})
	testutil.MustNoError(t, err)

	wired := buf.String()

	if !strings.Contains(wired, "event succeeded") {
		t.Errorf("consumer EventLogger must receive the event middleware lines, got:\n%s", wired)
	}

	if !strings.Contains(wired, "sync_item.synced") {
		t.Errorf("event log lines must carry the event type, got:\n%s", wired)
	}

	if !strings.Contains(wired, "stream") && !strings.Contains(wired, "streamID") {
		t.Errorf("event log lines must carry the stream identity, got:\n%s", wired)
	}
}
