package copilotapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AirSodaz/gantry/internal/sessions"
)

func TestEventCursorIsSignedAndBoundToSessionAndRun(t *testing.T) {
	key := []byte("test-event-key")
	cursor := encodeCursor(key, "ses_1", "principal_1", 42)
	sequence, ok := parseAfterCursor(key, cursor, "ses_1", "principal_1")
	if !ok || sequence != 42 {
		t.Fatalf("valid cursor parsed as sequence=%d ok=%t", sequence, ok)
	}
	if _, ok := parseAfterCursor(key, cursor, "ses_2", "principal_1"); ok {
		t.Fatal("cursor was accepted for another Session")
	}
	if _, ok := parseAfterCursor(key, cursor, "ses_1", "principal_2"); ok {
		t.Fatal("cursor was accepted for another requester")
	}
	if _, ok := parseAfterCursor(key, cursor+"x", "ses_1", "principal_1"); ok {
		t.Fatal("tampered cursor was accepted")
	}
}

func TestEventTicketRejectsExpiredClaims(t *testing.T) {
	key := []byte("test-event-key")
	ticket := signPayload(key, "evt", eventTicketClaims{SessionID: "ses_1", Actor: "principal_1", Expiry: time.Now().Add(-time.Second).Unix()})
	claims, ok := verifyPayload[eventTicketClaims](key, "evt", ticket)
	if !ok || claims.Expiry > time.Now().Unix() {
		t.Fatalf("expired ticket was not decoded as expected: %#v, %t", claims, ok)
	}
}

func TestSessionEventFramePublishesOnlyTheTypedEmployeeProjection(t *testing.T) {
	key := []byte("test-event-key")
	page := sessions.EventPage{
		Session: sessions.Session{
			ID: "ses_1",
			Messages: []sessions.Message{{
				ID:              "msg_1",
				SessionSequence: 4,
				AuthorKind:      "agent",
				Parts:           json.RawMessage(`[{"type":"text","text":"Hello"}]`),
			}},
		},
		Runs: []sessions.Run{{ID: "run_1", SessionSequence: 1, State: "running"}},
	}

	content, disposition := sessionEventFrame(key, "principal_1", page, sessions.Event{
		RunID:       eventString("run_1"),
		Sequence:    5,
		RunSequence: eventSequence(9),
		Type:        "model.delta",
		Payload:     json.RawMessage(`{"message_id":"msg_1","segment_index":2,"text":"Hello"}`),
	})
	if disposition != eventProjectionVisible {
		t.Fatal("content segment was suppressed")
	}
	if content["schema_version"] != eventSchemaVersion || content["session_sequence"] != uint64(5) || content["run_sequence"] != uint64(9) {
		t.Fatalf("frame metadata = %#v", content)
	}
	event := content["event"].(map[string]any)
	if event["type"] != "content_segment" || event["message_id"] != "msg_1" || event["text"] != "Hello" {
		t.Fatalf("content event = %#v", event)
	}

	message, disposition := sessionEventFrame(key, "principal_1", page, sessions.Event{RunID: eventString("run_1"), Sequence: 4, RunSequence: eventSequence(8), Type: "message.committed", Payload: json.RawMessage(`{"message_id":"msg_1"}`)})
	if disposition != eventProjectionVisible || message["event"].(map[string]any)["type"] != "message_committed" {
		t.Fatalf("message event = %#v, disposition=%d", message, disposition)
	}

	if _, disposition := sessionEventFrame(key, "principal_1", page, sessions.Event{RunID: eventString("run_1"), Sequence: 6, RunSequence: eventSequence(10), Type: "action.proposed", Payload: json.RawMessage(`{}`)}); disposition != eventProjectionSkipped {
		t.Fatal("internal action payload was exposed")
	}
}

func TestSessionEventFrameBlocksCursorUntilProjectionExists(t *testing.T) {
	key := []byte("test-event-key")
	page := sessions.EventPage{Session: sessions.Session{ID: "ses_1"}}

	_, disposition := sessionEventFrame(key, "principal_1", page, sessions.Event{RunID: eventString("run_1"), Sequence: 5, Type: "message.committed", Payload: json.RawMessage(`{"message_id":"msg_1"}`)})
	if disposition != eventProjectionBlocked {
		t.Fatalf("message disposition = %d", disposition)
	}
	_, disposition = sessionEventFrame(key, "principal_1", page, sessions.Event{RunID: eventString("run_1"), Sequence: 6, Type: "run.completed"})
	if disposition != eventProjectionBlocked {
		t.Fatalf("run disposition = %d", disposition)
	}
}

func TestSessionOwnerTransferEventHasNoRunAndProjectsCurrentMembership(t *testing.T) {
	key := []byte("test-event-key")
	page := sessions.EventPage{Session: sessions.Session{
		ID:                   "ses_1",
		State:                "active",
		Mode:                 "shared",
		ConversationRevision: 12,
		Members: []sessions.SessionMember{
			{PrincipalID: "principal_previous", Role: "contributor"},
			{PrincipalID: "principal_new", Role: "owner"},
		},
	}}

	frame, disposition := sessionEventFrame(key, "principal_new", page, sessions.Event{Sequence: 13, Type: "session.owner_transferred"})
	if disposition != eventProjectionVisible {
		t.Fatalf("disposition=%d", disposition)
	}
	if frame["run_id"] != nil || frame["run_sequence"] != nil {
		t.Fatalf("Session event must not claim a Run: %#v", frame)
	}
	event := frame["event"].(map[string]any)
	if event["type"] != "session_changed" || event["conversation_revision"] != int64(12) {
		t.Fatalf("event=%#v", event)
	}
	members := event["members"].([]sessions.SessionMember)
	if members[0].Role != "contributor" || members[1].Role != "owner" {
		t.Fatalf("membership projection=%#v", members)
	}
}

func eventString(value string) *string   { return &value }
func eventSequence(value uint64) *uint64 { return &value }

func TestSnapshotFrameUsesTheCurrentProjectionWatermark(t *testing.T) {
	key := []byte("test-event-key")
	page := sessions.EventPage{Session: sessions.Session{ID: "ses_1"}, CurrentSeq: 41}
	frame := snapshotFrame(key, "ses_1", "principal_1", page, page.CurrentSeq)
	if frame["type"] != "snapshot" || frame["schema_version"] != snapshotSchemaVersion {
		t.Fatalf("snapshot = %#v", frame)
	}
	session, ok := frame["session"].(sessions.Session)
	if !ok || session.ID != "ses_1" {
		t.Fatalf("snapshot session projection = %#v", frame)
	}
	sequence, ok := parseAfterCursor(key, frame["cursor"].(string), "ses_1", "principal_1")
	if !ok || sequence != 41 {
		t.Fatalf("snapshot cursor = %d, %t", sequence, ok)
	}
}

func TestEventWebSocketURLUsesForwardedHTTPS(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://control-plane.internal/api/copilot/v1/sessions/ses_1/events:ticket", nil)
	request.Host = "copilot.example.test"
	request.Header.Set("X-Forwarded-Proto", "https")
	if got := eventWebSocketURL(request, "ses_1"); got != "wss://copilot.example.test/api/copilot/v1/sessions/ses_1/events" {
		t.Fatalf("websocket URL = %q", got)
	}
}
