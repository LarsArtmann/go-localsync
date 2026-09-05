package sync

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"charm.land/log/v2"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/provider"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

// TestSyncer_StructuredLogFields capture-asserts the structured logging
// contract: every sync-completion log line carries the source field so logs
// are filterable per source without message parsing.
func TestSyncer_StructuredLogFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	store := &mockSyncStore{}
	syncer := NewSyncer(
		&testutil.MockProvider{Items: []*provider.Item{
			testSyncItem("lf-1", "PushEvent"),
			testSyncItem("lf-2", "IssueEvent"),
		}},
		store,
		log.New(&buf),
	)

	_, err := syncer.Sync(context.Background(), testSyncOpts())
	testutil.MustNoError(t, err)

	output := buf.String()

	completionLines := 0

	for line := range strings.SplitSeq(output, "\n") {
		if !strings.Contains(line, "Sync completed") || strings.Contains(line, "no valid items") {
			continue
		}

		completionLines++

		if !strings.Contains(line, "source=testuser") {
			t.Errorf("completion log line must carry source=testuser, got:\n%s", line)
		}
	}

	if completionLines == 0 {
		t.Fatalf("expected a 'Sync completed' log line, got:\n%s", output)
	}
}

// TestSyncer_InvalidItemLogCarriesSource asserts the invalid-item warning
// includes the source of the offending item.
func TestSyncer_InvalidItemLogCarriesSource(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	invalid := testSyncItem("lf-bad", "PushEvent")
	invalid.Type = id.NewEventTypeID("")

	syncer := NewSyncer(
		&testutil.MockProvider{Items: []*provider.Item{invalid}},
		&mockSyncStore{},
		log.New(&buf),
	)

	result, err := syncer.Sync(context.Background(), testSyncOpts())
	if !errors.Is(err, pkgerrors.ErrPartialSync) {
		t.Fatalf("expected ErrPartialSync for an all-invalid batch, got %v", err)
	}

	if result.Errors == 0 {
		t.Fatal("expected the invalid item to be counted as an error")
	}

	output := buf.String()

	if !strings.Contains(output, "Skipping invalid item") {
		t.Fatalf("expected a 'Skipping invalid item' warning, got:\n%s", output)
	}

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "Skipping invalid item") && !strings.Contains(line, "source=github") {
			t.Errorf("invalid-item warning must carry source=github, got:\n%s", line)
		}
	}
}
