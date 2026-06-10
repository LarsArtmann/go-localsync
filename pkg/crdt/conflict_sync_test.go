package crdt

import (
	"encoding/json"
	"testing"
	"time"
)

func assertNodeID(t *testing.T, got NodeID, want string, context string) {
	t.Helper()

	if got.String() != want {
		t.Errorf("%s node ID mismatch: got %q, want %q", context, got.String(), want)
	}
}

func TestSyncMessage_JSON(t *testing.T) {
	t.Parallel()

	msg := SyncMessage{
		Type:    SyncMessageTypeRequest,
		Payload: json.RawMessage(`{"test":true}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded SyncMessage
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Type != SyncMessageTypeRequest {
		t.Errorf("type: got %q, want %q", decoded.Type, SyncMessageTypeRequest)
	}
}

func TestSyncRequest_JSON(t *testing.T) {
	t.Parallel()

	since := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	req := SyncRequest{
		SyncContext: SyncContext{
			NodeID: MustParseNodeID("node-1"),
			Clock:  VectorClock{NodeID("node-1"): 5},
		},
		Since: since,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded SyncRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	assertNodeID(t, decoded.NodeID, "node-1", "first message")

	if decoded.Clock.Get(NodeID("node-1")) != 5 {
		t.Errorf("clock: got %d, want 5", decoded.Clock.Get(NodeID("node-1")))
	}
}

func TestSyncResponse_JSON(t *testing.T) {
	t.Parallel()

	resp := SyncResponse[testItem]{
		SyncContext: SyncContext{
			NodeID: MustParseNodeID("node-2"),
			Clock:  VectorClock{NodeID("node-1"): 5, NodeID("node-2"): 3},
		},
		Operations: []*Operation[testItem]{
			MustNewOperation(
				OperationID("op-1"),
				OpCreate,
				MustParseNodeID("node-2"),
				testItem{Name: "item1", UpdatedAt: time.Now()},
			),
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded SyncResponse[testItem]
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	assertNodeID(t, decoded.NodeID, "node-2", "second message")

	if len(decoded.Operations) != 1 {
		t.Fatalf("operations: got %d, want 1", len(decoded.Operations))
	}

	if decoded.Operations[0].Payload.Name != "item1" {
		t.Errorf("payload name: got %q, want %q", decoded.Operations[0].Payload.Name, "item1")
	}
}

func TestNewLWWResolver_NilTimestampFunc_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := NewLWWResolver[testItem](nil)
	if err == nil {
		t.Fatal("expected error when TimestampFunc is nil")
	}
}

func TestNewSyncContext(t *testing.T) {
	t.Parallel()

	nodeID := MustParseNodeID("node-1")
	clock := VectorClock{nodeID: 5}

	ctx := NewSyncContext(nodeID, clock)

	if ctx.NodeID != nodeID {
		t.Errorf("NodeID = %q, want %q", ctx.NodeID, nodeID)
	}

	if ctx.Clock.Get(nodeID) != 5 {
		t.Errorf("Clock[nodeID] = %d, want 5", ctx.Clock.Get(nodeID))
	}
}
