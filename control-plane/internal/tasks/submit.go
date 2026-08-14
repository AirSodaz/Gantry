package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

const submitRoute = "POST /api/copilot/v1/tasks"

func (s *Service) Submit(ctx context.Context, actor identity.Principal, key string, request SubmitRequest) (Task, bool, error) {
	if key = strings.TrimSpace(key); key == "" || len(key) > 256 {
		return Task{}, false, ErrInvalidInput
	}
	input, mode, err := normalizeInput(request)
	if err != nil {
		return Task{}, false, err
	}
	digest := requestDigest(request.AgentID, input)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Task{}, false, err
	}
	defer tx.Rollback(ctx)
	taskID := newID("tsk")
	var storedDigest, storedTaskID string
	err = tx.QueryRow(ctx, `INSERT INTO gantry.idempotency_tombstones (principal_id, route, idempotency_key, request_digest, task_id) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING request_digest, task_id`, actor.ID, submitRoute, key, digest, taskID).Scan(&storedDigest, &storedTaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT request_digest, task_id FROM gantry.idempotency_tombstones WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3 FOR UPDATE`, actor.ID, submitRoute, key).Scan(&storedDigest, &storedTaskID)
		if err != nil {
			return Task{}, false, err
		}
		if storedDigest != digest {
			return Task{}, false, ErrIdempotencyConflict
		}
		task, err := loadTask(ctx, tx, actor, storedTaskID)
		return task, true, err
	}
	if err != nil {
		return Task{}, false, err
	}
	workspaceID, versionID, displayName, err := resolvePublishedAgent(ctx, tx, actor, request.AgentID)
	if err != nil {
		return Task{}, false, err
	}
	runID := newID("run")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.tasks (id, organization_id, workspace_id, requester_principal_id, agent_id, input_json, current_run_id, status) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,'queued')`, taskID, actor.OrganizationID, workspaceID, actor.ID, request.AgentID, input, runID); err != nil {
		return Task{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.runs (id, task_id, agent_version_id, attempt_number, demo_mode, status) VALUES ($1,$2,$3,1,$4,'queued')`, runID, taskID, versionID, mode); err != nil {
		return Task{}, false, err
	}
	if err := appendEvent(ctx, tx, runID, "task.accepted"); err != nil {
		return Task{}, false, err
	}
	if err := appendEvent(ctx, tx, runID, "run.queued"); err != nil {
		return Task{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, false, err
	}
	return Task{ID: taskID, AgentID: request.AgentID, AgentDisplayName: displayName, Status: "queued", CurrentRun: Run{ID: runID, Status: "queued"}, CreatedAt: time.Now().UTC()}, false, nil
}

func resolvePublishedAgent(ctx context.Context, tx pgx.Tx, actor identity.Principal, agentID string) (workspaceID, versionID, displayName string, err error) {
	err = tx.QueryRow(ctx, `SELECT a.workspace_id, p.agent_version_id, a.display_name FROM gantry.agents a JOIN gantry.agent_publications p ON p.agent_id=a.id AND p.workspace_id=a.workspace_id AND p.status='published' JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1 WHERE a.id=$2 AND a.organization_id=$3`, actor.ID, agentID, actor.OrganizationID).Scan(&workspaceID, &versionID, &displayName)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

func normalizeInput(request SubmitRequest) (string, string, error) {
	if len(request.AttachmentIDs) != 0 {
		return "", "", ErrInvalidInput
	}
	message := strings.TrimSpace(request.Message)
	var structured any
	if len(request.StructuredInput) != 0 && string(request.StructuredInput) != "null" {
		if err := json.Unmarshal(request.StructuredInput, &structured); err != nil {
			return "", "", ErrInvalidInput
		}
	}
	if message == "" && structured == nil {
		return "", "", ErrInvalidInput
	}
	mode := "complete"
	if object, ok := structured.(map[string]any); ok {
		if candidate, ok := object["mode"].(string); ok {
			mode = candidate
		}
	}
	if mode != "complete" && mode != "await_cancel" {
		return "", "", ErrInvalidInput
	}
	payload, err := json.Marshal(map[string]any{"message": message, "structured_input": structured})
	return string(payload), mode, err
}

func requestDigest(agentID, input string) string {
	sum := sha256.Sum256([]byte(agentID + "\n" + input))
	return "sha256:" + hex.EncodeToString(sum[:])
}
