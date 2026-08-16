package copilotapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AirSodaz/gantry/internal/tasks"
)

func TestEventCursorIsSignedAndBoundToTaskAndRun(t *testing.T) {
	key := []byte("test-event-key")
	cursor := encodeCursor(key, "tsk_1", "principal_1", 42)
	sequence, ok := parseAfterCursor(key, cursor, "tsk_1", "principal_1")
	if !ok || sequence != 42 {
		t.Fatalf("valid cursor parsed as sequence=%d ok=%t", sequence, ok)
	}
	if _, ok := parseAfterCursor(key, cursor, "tsk_2", "principal_1"); ok {
		t.Fatal("cursor was accepted for another task")
	}
	if _, ok := parseAfterCursor(key, cursor, "tsk_1", "principal_2"); ok {
		t.Fatal("cursor was accepted for another requester")
	}
	if _, ok := parseAfterCursor(key, cursor+"x", "tsk_1", "principal_1"); ok {
		t.Fatal("tampered cursor was accepted")
	}
}

func TestEventTicketRejectsExpiredClaims(t *testing.T) {
	key := []byte("test-event-key")
	ticket := signPayload(key, "evt", eventTicketClaims{TaskID: "tsk_1", Actor: "principal_1", Expiry: time.Now().Add(-time.Second).Unix()})
	claims, ok := verifyPayload[eventTicketClaims](key, "evt", ticket)
	if !ok || claims.Expiry > time.Now().Unix() {
		t.Fatalf("expired ticket was not decoded as expected: %#v, %t", claims, ok)
	}
}

func TestTaskEventFramePublishesOnlyTheTypedEmployeeProjection(t *testing.T) {
	key := []byte("test-event-key")
	page := tasks.EventPage{
		Task: tasks.Task{
			ID: "tsk_1",
			Messages: []tasks.Message{{
				ID:           "msg_1",
				TaskSequence: 4,
				Role:         "agent",
				Parts:        json.RawMessage(`[{"type":"text","text":"Hello"}]`),
			}},
		},
		Runs: []tasks.RunAttempt{{ID: "run_1", Attempt: 1, Status: "running"}},
	}

	content, visible := taskEventFrame(key, "principal_1", page, tasks.Event{
		RunID:       "run_1",
		Sequence:    5,
		RunSequence: 9,
		Type:        "model.delta",
		Payload:     json.RawMessage(`{"message_id":"msg_1","segment_index":2,"text":"Hello"}`),
	})
	if !visible {
		t.Fatal("content segment was suppressed")
	}
	if content["schema_version"] != eventSchemaVersion || content["task_sequence"] != uint64(5) || content["run_sequence"] != uint64(9) {
		t.Fatalf("frame metadata = %#v", content)
	}
	event := content["event"].(map[string]any)
	if event["type"] != "content_segment" || event["message_id"] != "msg_1" || event["text"] != "Hello" {
		t.Fatalf("content event = %#v", event)
	}

	message, visible := taskEventFrame(key, "principal_1", page, tasks.Event{RunID: "run_1", Sequence: 4, RunSequence: 8, Type: "message.committed", Payload: json.RawMessage(`{"message_id":"msg_1"}`)})
	if !visible || message["event"].(map[string]any)["type"] != "message_committed" {
		t.Fatalf("message event = %#v, visible=%t", message, visible)
	}

	if _, visible := taskEventFrame(key, "principal_1", page, tasks.Event{RunID: "run_1", Sequence: 6, RunSequence: 10, Type: "action.proposed", Payload: json.RawMessage(`{}`)}); visible {
		t.Fatal("internal action payload was exposed")
	}
}

func TestSnapshotFrameUsesTheCurrentProjectionWatermark(t *testing.T) {
	key := []byte("test-event-key")
	page := tasks.EventPage{Task: tasks.Task{ID: "tsk_1"}, CurrentSeq: 41}
	frame := snapshotFrame(key, "tsk_1", "principal_1", page, page.CurrentSeq)
	if frame["type"] != "snapshot" || frame["schema_version"] != snapshotSchemaVersion {
		t.Fatalf("snapshot = %#v", frame)
	}
	sequence, ok := parseAfterCursor(key, frame["cursor"].(string), "tsk_1", "principal_1")
	if !ok || sequence != 41 {
		t.Fatalf("snapshot cursor = %d, %t", sequence, ok)
	}
}

func TestEventWebSocketURLUsesForwardedHTTPS(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://control-plane.internal/api/copilot/v1/tasks/tsk_1/events:ticket", nil)
	request.Host = "copilot.example.test"
	request.Header.Set("X-Forwarded-Proto", "https")
	if got := eventWebSocketURL(request, "tsk_1"); got != "wss://copilot.example.test/api/copilot/v1/tasks/tsk_1/events" {
		t.Fatalf("websocket URL = %q", got)
	}
}
