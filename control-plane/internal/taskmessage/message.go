// Package taskmessage owns durable, employee-visible Task message records.
package taskmessage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Part is an OpenAPI TaskMessage part. Its constructors keep the persisted
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

// Append allocates the next task-wide sequence and commits an immutable
// requester, agent, or system summary message in the caller's transaction.
func Append(ctx context.Context, tx pgx.Tx, taskID, runID, role string, parts ...Part) error {
	taskID = strings.TrimSpace(taskID)
	runID = strings.TrimSpace(runID)
	if taskID == "" || runID == "" || (role != "requester" && role != "agent" && role != "system_summary") || len(parts) == 0 {
		return fmt.Errorf("invalid task message")
	}
	content, err := contentFor(parts)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(parts)
	if err != nil {
		return err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.tasks SET task_event_sequence=task_event_sequence+1 WHERE id=$1 RETURNING task_event_sequence`, taskID).Scan(&sequence); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.task_messages (id, task_id, run_id, task_sequence, role, parts, content) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)`, newID(), taskID, runID, sequence, role, string(payload), content)
	return err
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
