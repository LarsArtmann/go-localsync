package cqrs

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	pkgerrors "github.com/larsartmann/go-localsync/pkg/errors"
)

// ExportedEvent is the wire shape of one exported stored event: identity,
// positioning, schema version, raw payload (base64), correlation/causation
// metadata, and timing. Payload stays raw so exports never lose fidelity to
// what was persisted.
type ExportedEvent struct {
	EventID       string            `json:"eventId"`
	EventType     string            `json:"eventType"`
	StreamID      string            `json:"streamId"`
	StreamType    string            `json:"streamType"`
	Version       uint64            `json:"version"`
	SchemaVersion int               `json:"schemaVersion"`
	OccurredAt    time.Time         `json:"occurredAt"`
	PayloadBase64 string            `json:"payloadBase64"`
	Encoding      string            `json:"encoding"`
	CorrelationID string            `json:"correlationId,omitempty"`
	Causation     *event.Causation  `json:"causation,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// ExportEvents writes every stored event (across all streams, in journal
// order — which is OccurredAt order) to w as newline-delimited JSON. This is
// the analysis/audit path: one JSON object per line, loadable by jq, pandas,
// or event-replay tooling.
func (s *CQRSStack) ExportEvents(ctx context.Context, w io.Writer) error {
	journal, err := s.journal()
	if err != nil {
		return err
	}

	return exportEvents(ctx, journal, w)
}

func (s *CQRSStack) journal() (event.Journal, error) {
	if j, ok := s.Store.(event.Journal); ok {
		return j, nil
	}

	return nil, pkgerrors.Wrap(pkgerrors.ErrDBNil, "event store does not support journal reads")
}

func exportEvents(ctx context.Context, journal event.Journal, w io.Writer) error {
	events, err := journal.ReadAll(ctx)
	if err != nil {
		return pkgerrors.Wrap(err, "export events: read journal")
	}

	encoder := json.NewEncoder(w)

	for _, evt := range events {
		if err := encoder.Encode(exportedEventFrom(evt)); err != nil {
			return pkgerrors.Wrap(err, "export events: encode event")
		}
	}

	return nil
}

// ExportEventsCSV writes every stored event as CSV with a header row. The
// payload is base64-encoded (binary-safe); metadata columns cover the common
// correlation/causation fields.
func (s *CQRSStack) ExportEventsCSV(ctx context.Context, w io.Writer) error {
	journal, err := s.journal()
	if err != nil {
		return err
	}

	return exportEventsCSV(ctx, journal, w)
}

func exportEventsCSV(ctx context.Context, journal event.Journal, w io.Writer) error {
	events, err := journal.ReadAll(ctx)
	if err != nil {
		return pkgerrors.Wrap(err, "export events csv: read journal")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	header := []string{
		"event_id", "event_type", "stream_id", "stream_type", "version",
		"schema_version", "occurred_at", "encoding", "payload_base64",
		"correlation_id", "causation_command_type", "causation_command_id",
	}

	if err := writer.Write(header); err != nil {
		return pkgerrors.Wrap(err, "export events csv: write header")
	}

	for _, evt := range events {
		exported := exportedEventFrom(evt)

		causationType, causationID := "", ""
		if exported.Causation != nil {
			causationType = exported.Causation.CommandType
			causationID = exported.Causation.CommandID.String()
		}

		row := []string{
			exported.EventID,
			exported.EventType,
			exported.StreamID,
			exported.StreamType,
			strconv.FormatUint(exported.Version, 10),
			strconv.Itoa(exported.SchemaVersion),
			exported.OccurredAt.Format(time.RFC3339Nano),
			exported.Encoding,
			exported.PayloadBase64,
			exported.CorrelationID,
			causationType,
			causationID,
		}

		if err := writer.Write(row); err != nil {
			return pkgerrors.Wrap(err, "export events csv: write row")
		}
	}

	return nil
}

func exportedEventFrom(evt event.Event) ExportedEvent {
	meta := evt.Metadata()

	metadata := make(map[string]string, len(meta.Custom))
	for k, v := range meta.Custom {
		metadata[string(k)] = v
	}

	return ExportedEvent{
		EventID:       evt.ID().String(),
		EventType:     evt.Type().String(),
		StreamID:      evt.StreamID().String(),
		StreamType:    evt.StreamType().String(),
		Version:       uint64(evt.Version()),
		SchemaVersion: int(evt.SchemaVersion()),
		OccurredAt:    evt.OccurredAt(),
		PayloadBase64: base64.StdEncoding.EncodeToString(evt.Payload()),
		Encoding:      string(evt.Encoding()),
		CorrelationID: meta.CorrelationID.String(),
		Causation:     meta.Causation,
		Metadata:      metadata,
	}
}
