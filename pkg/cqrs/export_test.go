package cqrs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-localsync/pkg/data/model"
	"github.com/larsartmann/go-localsync/pkg/id"
	"github.com/larsartmann/go-localsync/pkg/testutil"
)

func TestExportEvents_JSONLines(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	syncTestItem(t, stack, ctx, "exp-1", "PushEvent")
	waitForCount(t, stack, ctx, 1)

	testutil.MustNoError(t, stack.TombstoneItem(ctx, "github", id.NewSourceID("exp-1"), model.ReasonUserHidden))

	// Give the synchronous bus a beat to deliver the tombstone before reading
	// the journal for export.
	time.Sleep(50 * time.Millisecond)

	var buf bytes.Buffer
	testutil.MustNoError(t, stack.ExportEvents(ctx, &buf))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 exported events, got %d", len(lines))
	}

	var first ExportedEvent
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("export line is not JSON: %v\nline: %s", err, lines[0])
	}

	if first.EventType != string(EventItemSynced) {
		t.Errorf("first exported event type = %q, want %q", first.EventType, EventItemSynced)
	}

	if first.PayloadBase64 == "" {
		t.Error("payload must be exported base64-encoded")
	}

	if _, err := base64.StdEncoding.DecodeString(first.PayloadBase64); err != nil {
		t.Errorf("payload is not valid base64: %v", err)
	}

	var tombstone ExportedEvent
	found := false

	for _, line := range lines[1:] {
		var evt ExportedEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("bad export line: %v", err)
		}
		if evt.EventType == string(EventItemTombstoned) {
			tombstone = evt
			found = true
		}
	}

	if !found {
		t.Fatal("expected the tombstone event in the export")
	}

	if tombstone.Causation == nil || tombstone.Causation.CommandType != "sync_item.tombstone" {
		t.Errorf("tombstone export must carry causation, got %+v", tombstone.Causation)
	}
}

func TestExportEvents_CSV(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	ctx := context.Background()
	syncTestItem(t, stack, ctx, "exp-csv", "PushEvent")
	waitForCount(t, stack, ctx, 1)

	var buf bytes.Buffer
	testutil.MustNoError(t, stack.ExportEventsCSV(ctx, &buf))

	records, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("export is not valid CSV: %v", err)
	}

	if len(records) < 2 {
		t.Fatalf("expected header + at least 1 row, got %d records", len(records))
	}

	header := records[0]
	for _, want := range []string{"event_id", "event_type", "payload_base64", "causation_command_type"} {
		found := false

		for _, h := range header {
			if h == want {
				found = true
			}
		}

		if !found {
			t.Errorf("CSV header missing column %q: %v", want, header)
		}
	}

	sawSynced := false

	for _, row := range records[1:] {
		typeIdx := indexOf(header, "event_type")
		if row[typeIdx] == string(EventItemSynced) {
			sawSynced = true
		}
	}

	if !sawSynced {
		t.Error("expected an ItemSynced row in the CSV export")
	}
}

func indexOf(list []string, target string) int {
	for i, v := range list {
		if v == target {
			return i
		}
	}

	return -1
}

// TestExportEvents_EmptyJournal exports cleanly from a fresh stack (zero
// events, valid empty output).
func TestExportEvents_EmptyJournal(t *testing.T) {
	t.Parallel()

	stack := newMemoryStack(t)
	defer func() { _ = stack.Close() }()

	var json bytes.Buffer
	testutil.MustNoError(t, stack.ExportEvents(context.Background(), &json))
	if json.Len() != 0 {
		t.Errorf("empty journal must export nothing, got %q", json.String())
	}

	var csvOut bytes.Buffer
	testutil.MustNoError(t, stack.ExportEventsCSV(context.Background(), &csvOut))
	if !strings.Contains(csvOut.String(), "event_id") {
		t.Errorf("CSV export must still carry the header, got %q", csvOut.String())
	}
}
