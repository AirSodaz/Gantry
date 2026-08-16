package copilotapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
	"github.com/coder/websocket"
)

const eventTicketLifetime = time.Minute

type eventTicketClaims struct {
	TaskID string `json:"task_id"`
	Actor  string `json:"actor_id"`
	Expiry int64  `json:"expiry"`
}

type eventCursorClaims struct {
	TaskID string `json:"task_id"`
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
	reader, ok := h.tasks.(eventReader)
	if !ok {
		writeInternal(w, errors.New("event service is unavailable"))
		return
	}
	if _, err := reader.Events(r.Context(), actor, r.PathValue("taskID"), 0, 1); err != nil {
		writeTaskError(w, err)
		return
	}
	expiresAt := time.Now().UTC().Add(eventTicketLifetime)
	ticket := signPayload(h.eventKey, "evt", eventTicketClaims{TaskID: r.PathValue("taskID"), Actor: actor.ID, Expiry: expiresAt.Unix()})
	writeJSON(w, http.StatusOK, map[string]any{"ticket": ticket, "task_id": r.PathValue("taskID"), "expires_at": expiresAt})
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
	lastSeq, ok := parseAfterCursor(h.eventKey, r.URL.Query().Get("after"), ticket.TaskID)
	if !ok {
		_ = writeEventFrame(ctx, conn, cursorExpiredFrame(h.eventKey, page))
		return
	}
	if page.EarliestSeq != 0 && lastSeq+1 < page.EarliestSeq {
		_ = writeEventFrame(ctx, conn, cursorExpiredFrame(h.eventKey, page))
		return
	}
	if err := writeEventFrame(ctx, conn, map[string]any{"type": "snapshot", "task": page.Task, "run": page.Task.CurrentRun, "cursor": encodeCursor(h.eventKey, ticket.TaskID, lastSeq)}); err != nil {
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
			if err := writeEventFrame(ctx, conn, map[string]any{"type": "heartbeat", "cursor": encodeCursor(h.eventKey, ticket.TaskID, lastSeq)}); err != nil {
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
				_ = writeEventFrame(ctx, conn, cursorExpiredFrame(h.eventKey, page))
				return
			}
			for _, event := range page.Events {
				lastSeq = event.Sequence
				if err := writeEventFrame(ctx, conn, map[string]any{"type": "event", "cursor": encodeCursor(h.eventKey, ticket.TaskID, lastSeq), "event": event, "provisional": false}); err != nil {
					return
				}
			}
		}
	}
}

func cursorExpiredFrame(key []byte, page tasks.EventPage) map[string]any {
	seq := uint64(0)
	if page.EarliestSeq > 0 {
		seq = page.EarliestSeq - 1
	}
	return map[string]any{"type": "cursor_expired", "code": "cursor_expired", "earliest_cursor": encodeCursor(key, page.Task.ID, seq), "snapshot": map[string]any{"task": page.Task, "run": page.Task.CurrentRun}}
}

func parseAfterCursor(key []byte, raw, taskID string) (uint64, bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, true
	}
	claims, ok := verifyPayload[eventCursorClaims](key, "cur", raw)
	return claims.Seq, ok && claims.TaskID == taskID
}

func encodeCursor(key []byte, taskID string, seq uint64) string {
	return signPayload(key, "cur", eventCursorClaims{TaskID: taskID, Seq: seq})
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
