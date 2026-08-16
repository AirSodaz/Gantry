package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/taskmessage"
	"github.com/jackc/pgx/v5"
)

const submitRoute = "POST /api/copilot/v1/tasks"

func (s *Service) Submit(ctx context.Context, actor identity.Principal, key string, request SubmitRequest) (Task, bool, error) {
	if key = strings.TrimSpace(key); key == "" || len(key) > 256 {
		return Task{}, false, ErrInvalidInput
	}
	input, err := normalizeInput(request)
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
	workspaceID, deploymentID, revisionID, displayName, err := resolveProductionAgent(ctx, tx, actor, request.AgentID)
	if err != nil {
		return Task{}, false, err
	}
	manifestDigest, err := manifestDigestForRevision(ctx, tx, revisionID)
	if err != nil {
		return Task{}, false, err
	}
	runID := newID("run")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.tasks (id, organization_id, workspace_id, requester_principal_id, agent_id, input_json, current_run_id, status) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,'queued')`, taskID, actor.OrganizationID, workspaceID, actor.ID, request.AgentID, input, runID); err != nil {
		return Task{}, false, err
	}
	if err := bindAttachments(ctx, tx, actor, taskID, workspaceID, request.AttachmentIDs); err != nil {
		return Task{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.runs (id, task_id, agent_revision_id, deployment_id, manifest_digest, attempt_number, status) VALUES ($1,$2,$3,$4,$5,1,'queued')`, runID, taskID, revisionID, deploymentID, manifestDigest); err != nil {
		return Task{}, false, err
	}
	if message := strings.TrimSpace(request.Message); message != "" {
		if err := taskmessage.Append(ctx, tx, taskID, runID, "requester", taskmessage.Text(message)); err != nil {
			return Task{}, false, err
		}
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
	return Task{ID: taskID, AgentID: request.AgentID, AgentDisplayName: displayName, Status: "queued", CurrentRun: Run{ID: runID, Status: "queued"}, ConversationRevision: 1, CreatedAt: time.Now().UTC()}, false, nil
}

func resolveProductionAgent(ctx context.Context, tx pgx.Tx, actor identity.Principal, agentID string) (workspaceID, deploymentID, revisionID, displayName string, err error) {
	err = tx.QueryRow(ctx, `SELECT a.workspace_id, d.id, d.revision_id, a.display_name FROM gantry.agents a JOIN gantry.agent_deployments d ON d.agent_id=a.id AND d.workspace_id=a.workspace_id AND d.environment_kind='production' AND d.status='active' JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1 WHERE a.id=$2 AND a.organization_id=$3`, actor.ID, agentID, actor.OrganizationID).Scan(&workspaceID, &deploymentID, &revisionID, &displayName)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return
}

func manifestDigestForRevision(ctx context.Context, tx pgx.Tx, revisionID string) (string, error) {
	var spec json.RawMessage
	if err := tx.QueryRow(ctx, `SELECT spec_json FROM gantry.agent_revisions WHERE id=$1`, revisionID).Scan(&spec); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	_, digest, err := executionManifest(spec)
	return digest, err
}

func normalizeInput(request SubmitRequest) (string, error) {
	message := strings.TrimSpace(request.Message)
	var structured any
	if len(request.StructuredInput) != 0 && string(request.StructuredInput) != "null" {
		if err := json.Unmarshal(request.StructuredInput, &structured); err != nil {
			return "", ErrInvalidInput
		}
	}
	if message == "" && structured == nil {
		return "", ErrInvalidInput
	}
	attachmentIDs, err := normalizedAttachmentIDs(request.AttachmentIDs)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{"message": message, "structured_input": structured, "attachment_ids": attachmentIDs})
	return string(payload), err
}

func normalizedAttachmentIDs(ids []string) ([]string, error) {
	if len(ids) > 10 {
		return nil, ErrInvalidInput
	}
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, ErrInvalidInput
		}
		if _, exists := seen[id]; exists {
			return nil, ErrInvalidInput
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func bindAttachments(ctx context.Context, tx pgx.Tx, actor identity.Principal, taskID, workspaceID string, attachmentIDs []string) error {
	attachmentIDs, err := normalizedAttachmentIDs(attachmentIDs)
	if err != nil {
		return err
	}
	for _, attachmentID := range attachmentIDs {
		result, err := tx.Exec(ctx, `UPDATE gantry.attachments SET bound_task_id=$1, workspace_id=$2 WHERE id=$3 AND organization_id=$4 AND requester_principal_id=$5 AND bound_task_id IS NULL AND state='available' AND scan_status='passed'`, taskID, workspaceID, attachmentID, actor.OrganizationID, actor.ID)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return fmt.Errorf("%w: attachment is not ready", ErrInvalidInput)
		}
	}
	return nil
}

func requestDigest(agentID, input string) string {
	sum := sha256.Sum256([]byte(agentID + "\n" + input))
	return "sha256:" + hex.EncodeToString(sum[:])
}
