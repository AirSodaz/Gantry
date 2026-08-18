// Package sessionmessage owns durable, employee-visible Session message records.
package sessionmessage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AirSodaz/gantry/internal/sessionevents"
	"github.com/jackc/pgx/v5"
)

// Part is an OpenAPI SessionMessage part. Its constructors keep the persisted
// projection aligned with the public discriminator contract.
type Part map[string]string

func Text(text string) Part {
	return Part{"type": "text", "text": text}
}

func Artifact(artifactID, label string) Part {
	return Part{"type": "artifact", "artifact_id": strings.TrimSpace(artifactID), "label": strings.TrimSpace(label)}
}

func ActionSummary(actionID, summary, state string) Part {
	return Part{"type": "action_summary", "action_id": strings.TrimSpace(actionID), "summary": strings.TrimSpace(summary), "state": strings.TrimSpace(state)}
}

func Status(code, message string) Part {
	return Part{"type": "status", "code": strings.TrimSpace(code), "message": strings.TrimSpace(message)}
}

// Append allocates the next session-wide sequence and commits an immutable
// requester, agent, or system summary message in the caller's transaction.
func Append(ctx context.Context, tx pgx.Tx, sessionID, runID, role string, parts ...Part) error {
	_, err := AppendWithID(ctx, tx, sessionID, runID, role, parts...)
	return err
}

// AppendWithID returns the immutable message identifier for a dependent
// projection, including a content segment that references its committed text.
func AppendWithID(ctx context.Context, tx pgx.Tx, sessionID, runID, role string, parts ...Part) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	runID = strings.TrimSpace(runID)
	if sessionID == "" || runID == "" || (role != "requester" && role != "agent" && role != "system_summary") || len(parts) == 0 {
		return "", fmt.Errorf("invalid session message")
	}
	content, err := contentFor(parts)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(parts)
	if err != nil {
		return "", err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.sessions SET session_event_sequence=session_event_sequence+1 WHERE id=$1 RETURNING session_event_sequence`, sessionID).Scan(&sequence); err != nil {
		return "", err
	}
	messageID := newID()
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.session_messages (id, session_id, run_id, session_sequence, role, parts, content) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)`, messageID, sessionID, runID, sequence, role, string(payload), content); err != nil {
		return "", err
	}
	if err := sessionevents.AppendAtSessionSequence(ctx, tx, runID, sequence, "message.committed", map[string]string{"message_id": messageID}); err != nil {
		return "", err
	}
	return messageID, nil
}

func contentFor(parts []Part) (string, error) {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part["type"] {
		case "text":
			if strings.TrimSpace(part["text"]) == "" {
				return "", fmt.Errorf("text message part is empty")
			}
			values = append(values, part["text"])
		case "artifact":
			if part["artifact_id"] == "" || part["label"] == "" {
				return "", fmt.Errorf("artifact message part is incomplete")
			}
			values = append(values, part["label"])
		case "action_summary":
			if part["action_id"] == "" || part["summary"] == "" || part["state"] == "" {
				return "", fmt.Errorf("action summary message part is incomplete")
			}
			values = append(values, part["summary"])
		case "status":
			if part["code"] == "" || part["message"] == "" {
				return "", fmt.Errorf("status message part is incomplete")
			}
			values = append(values, part["message"])
		default:
			return "", fmt.Errorf("unsupported message part")
		}
	}
	return strings.Join(values, "\n"), nil
}

func newID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return "msg_" + hex.EncodeToString(value)
}
