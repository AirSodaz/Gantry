package copilotapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
	"github.com/coder/websocket"
)

const eventTicketLifetime = time.Minute

const (
	snapshotSchemaVersion = "gantry.copilot.snapshot/v1"
	eventSchemaVersion    = "gantry.copilot.event/v1"
)

type eventTicketClaims struct {
	TaskID string `json:"task_id"`
	Actor  string `json:"actor_id"`
	Expiry int64  `json:"expiry"`
}

type eventCursorClaims struct {
	TaskID string `json:"task_id"`
	Actor  string `json:"actor_id"`
	Seq    uint64 `json:"sequence"`
}

func loadEventTicketKey() []byte {
	key := strings.TrimSpace(os.Getenv("GANTRY_EVENT_TICKET_KEY"))
	if key == "" {
		key = "gantry-development-event-ticket-key"
	}
	return []byte(key)
}

func (h Handler) issueEventTicket(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request struct {
		LastCursor string `json:"last_cursor"`
	}
	if err := decodeJSON(w, r, &request); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	reader, ok := h.tasks.(eventReader)
	if !ok {
		writeInternal(w, errors.New("event service is unavailable"))
		return
	}
	if _, err := reader.Events(r.Context(), actor, r.PathValue("taskID"), 0, 1); err != nil {
		writeTaskError(w, err)
		return
	}
	if strings.TrimSpace(request.LastCursor) != "" {
		if _, ok := parseAfterCursor(h.eventKey, request.LastCursor, r.PathValue("taskID"), actor.ID); !ok {
			writeError(w, http.StatusUnprocessableEntity, "cursor_invalid", "The event cursor is no longer valid for this task.")
			return
		}
	}
	expiresAt := time.Now().UTC().Add(eventTicketLifetime)
	ticket := signPayload(h.eventKey, "evt", eventTicketClaims{TaskID: r.PathValue("taskID"), Actor: actor.ID, Expiry: expiresAt.Unix()})
	writeJSON(w, http.StatusOK, map[string]any{"ticket": ticket, "task_id": r.PathValue("taskID"), "websocket_url": eventWebSocketURL(r, r.PathValue("taskID")), "expires_at": expiresAt})
}

func (h Handler) events(w http.ResponseWriter, r *http.Request) {
	ticket, ok := verifyPayload[eventTicketClaims](h.eventKey, "evt", r.URL.Query().Get("ticket"))
	if !ok || ticket.Expiry <= time.Now().Unix() || ticket.TaskID != r.PathValue("taskID") {
		writeError(w, http.StatusUnauthorized, "invalid_event_ticket", "The event ticket is invalid or expired.")
		return
	}
	reader, ok := h.tasks.(eventReader)
	if !ok {
		writeInternal(w, errors.New("event service is unavailable"))
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	conn.SetReadLimit(1 << 20)
	ctx := r.Context()
	page, err := reader.Events(ctx, identity.Principal{ID: ticket.Actor}, ticket.TaskID, 0, 1)
	if err != nil {
		_ = writeEventFrame(ctx, conn, map[string]any{"type": "error", "code": "not_found"})
		return
	}
	lastSeq, ok := parseAfterCursor(h.eventKey, r.URL.Query().Get("after"), ticket.TaskID, ticket.Actor)
	if !ok {
		_ = writeEventFrame(ctx, conn, map[string]any{"type": "error", "code": "cursor_invalid"})
		return
	}
	if page.EarliestSeq != 0 && lastSeq+1 < page.EarliestSeq {
		_ = writeEventFrame(ctx, conn, cursorExpiredFrame(h.eventKey, page, ticket.Actor))
		return
	}
	lastSeq = page.CurrentSeq
	if err := writeEventFrame(ctx, conn, snapshotFrame(h.eventKey, ticket.TaskID, ticket.Actor, page, lastSeq)); err != nil {
		return
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()
	poll := time.NewTicker(250 * time.Millisecond)
	defer poll.Stop()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-readDone:
			return
		case <-heartbeat.C:
			if err := writeEventFrame(ctx, conn, map[string]any{"type": "heartbeat", "cursor": encodeCursor(h.eventKey, ticket.TaskID, ticket.Actor, lastSeq)}); err != nil {
				return
			}
		case <-poll.C:
			if ticket.Expiry <= time.Now().Unix() {
				_ = writeEventFrame(ctx, conn, map[string]any{"type": "error", "code": "event_ticket_expired"})
				return
			}
			page, err := reader.Events(ctx, identity.Principal{ID: ticket.Actor}, ticket.TaskID, lastSeq, 100)
			if err != nil {
				return
			}
			if page.EarliestSeq != 0 && lastSeq+1 < page.EarliestSeq {
				_ = writeEventFrame(ctx, conn, cursorExpiredFrame(h.eventKey, page, ticket.Actor))
				return
			}
			for _, event := range page.Events {
				lastSeq = event.Sequence
				frame, visible := taskEventFrame(h.eventKey, ticket.Actor, page, event)
				if !visible {
					continue
				}
				if err := writeEventFrame(ctx, conn, frame); err != nil {
					return
				}
			}
		}
	}
}

func cursorExpiredFrame(key []byte, page tasks.EventPage, actorID string) map[string]any {
	seq := uint64(0)
	if page.EarliestSeq > 0 {
		seq = page.EarliestSeq - 1
	}
	return map[string]any{"type": "cursor_expired", "code": "cursor_expired", "earliest_cursor": encodeCursor(key, page.Task.ID, actorID, seq), "snapshot": snapshotFrame(key, page.Task.ID, actorID, page, page.CurrentSeq)}
}

func snapshotFrame(key []byte, taskID, actorID string, page tasks.EventPage, sequence uint64) map[string]any {
	return map[string]any{
		"type":           "snapshot",
		"schema_version": snapshotSchemaVersion,
		"task":           page.Task,
		"runs":           page.Runs,
		"approvals":      page.Approvals,
		"cursor":         encodeCursor(key, taskID, actorID, sequence),
	}
}

func taskEventFrame(key []byte, actorID string, page tasks.EventPage, item tasks.Event) (map[string]any, bool) {
	publicEvent, ok := projectTaskEvent(page, item)
	if !ok {
		return nil, false
	}
	return map[string]any{
		"schema_version": eventSchemaVersion,
		"task_id":        page.Task.ID,
		"run_id":         item.RunID,
		"task_sequence":  item.Sequence,
		"run_sequence":   item.RunSequence,
		"cursor":         encodeCursor(key, page.Task.ID, actorID, item.Sequence),
		"event":          publicEvent,
	}, true
}

func projectTaskEvent(page tasks.EventPage, item tasks.Event) (map[string]any, bool) {
	switch item.Type {
	case "message.committed":
		messageID := eventPayloadString(item.Payload, "message_id")
		for _, message := range page.Task.Messages {
			if message.ID == messageID {
				return map[string]any{"type": "message_committed", "message": message}, true
			}
		}
	case "model.delta":
		messageID := eventPayloadString(item.Payload, "message_id")
		text := eventPayloadString(item.Payload, "text")
		if messageID != "" && text != "" {
			return map[string]any{
				"type":          "content_segment",
				"message_id":    messageID,
				"segment_index": eventPayloadInt(item.Payload, "segment_index"),
				"text":          text,
			}, true
		}
	case "approval.requested", "approval.satisfied", "approval.rejected", "approval.expired":
		approvalID := eventPayloadString(item.Payload, "approval_id")
		for _, approval := range page.Approvals {
			if approval.ID == approvalID {
				return map[string]any{"type": "approval_changed", "approval": approval}, true
			}
		}
	case "artifact.uploaded":
		artifactID := eventPayloadString(item.Payload, "artifact_id")
		for _, artifact := range page.Task.Artifacts {
			if artifact.ID == artifactID {
				return map[string]any{"type": "artifact_changed", "artifact": artifact}, true
			}
		}
	default:
		status, changed := runStatusForEvent(item.Type)
		if !changed {
			return nil, false
		}
		for _, run := range page.Runs {
			if run.ID == item.RunID {
				run.Status = status
				return map[string]any{"type": "run_state_changed", "run": run}, true
			}
		}
	}
	return nil, false
}

func runStatusForEvent(eventType string) (string, bool) {
	switch eventType {
	case "run.queued":
		return "queued", true
	case "run.assigned", "run.started", "run.resumed":
		return "running", true
	case "run.awaiting_approval":
		return "awaiting_approval", true
	case "run.cancel_requested":
		return "canceling", true
	case "run.completed", "run.failed", "run.canceled":
		return strings.TrimPrefix(eventType, "run."), true
	default:
		return "", false
	}
}

func eventPayloadString(payload json.RawMessage, key string) string {
	var value map[string]any
	if json.Unmarshal(payload, &value) != nil {
		return ""
	}
	text, _ := value[key].(string)
	return text
}

func eventPayloadInt(payload json.RawMessage, key string) int64 {
	var value map[string]any
	if json.Unmarshal(payload, &value) != nil {
		return 0
	}
	number, _ := value[key].(float64)
	if number < 0 {
		return 0
	}
	return int64(number)
}

func parseAfterCursor(key []byte, raw, taskID, actorID string) (uint64, bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, true
	}
	claims, ok := verifyPayload[eventCursorClaims](key, "cur", raw)
	return claims.Seq, ok && claims.TaskID == taskID && claims.Actor == actorID
}

func encodeCursor(key []byte, taskID, actorID string, seq uint64) string {
	return signPayload(key, "cur", eventCursorClaims{TaskID: taskID, Actor: actorID, Seq: seq})
}

func eventWebSocketURL(r *http.Request, taskID string) string {
	scheme := "ws"
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		scheme = "wss"
	}
	return scheme + "://" + r.Host + "/api/copilot/v1/tasks/" + url.PathEscape(taskID) + "/events"
}

func signPayload[T any](key []byte, prefix string, value T) string {
	data, _ := json.Marshal(value)
	encoded := base64.RawURLEncoding.EncodeToString(data)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(prefix + "." + encoded))
	return prefix + "." + encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyPayload[T any](key []byte, prefix, token string) (T, bool) {
	var zero T
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != prefix {
		return zero, false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(provided, mac.Sum(nil)) {
		return zero, false
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return zero, false
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return zero, false
	}
	return value, true
}

func writeEventFrame(ctx context.Context, conn *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}
