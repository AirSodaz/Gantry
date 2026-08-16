package tasks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

const (
	cancelRoute = "POST /api/copilot/v1/tasks/{task_id}/runs/{run_id}:cancel"
	retryRoute  = "POST /api/copilot/v1/tasks/{task_id}:retry"
)

func (s *Service) Cancel(ctx context.Context, actor identity.Principal, taskID, runID, key string) (CancelResult, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 {
		return CancelResult{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CancelResult{}, err
	}
	defer tx.Rollback(ctx)
	duplicate, err := reserveTaskCommand(ctx, tx, actor.ID, cancelRoute, key, requestDigest(taskID, "cancel\n"+runID), taskID)
	if err != nil {
		return CancelResult{}, err
	}
	if duplicate {
		if err := tx.Commit(ctx); err != nil {
			return CancelResult{}, err
		}
		existing, err := s.GetRun(ctx, actor, runID)
		if err != nil {
			return CancelResult{}, err
		}
		if existing.TaskID != taskID {
			return CancelResult{}, ErrNotFound
		}
		return CancelResult{Run: existing.Run}, nil
	}
	var status string
	var epoch, acknowledgedSequence uint64
	err = tx.QueryRow(ctx, `SELECT r.status, r.lease_epoch, r.runner_event_sequence FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.id=$1 AND r.task_id=$2 AND t.requester_principal_id=$3 FOR UPDATE`, runID, taskID, actor.ID).Scan(&status, &epoch, &acknowledgedSequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return CancelResult{}, ErrNotFound
	}
	if err != nil {
		return CancelResult{}, err
	}
	result := CancelResult{Run: Run{ID: runID, Status: publicStatus(status), LeaseEpoch: epoch, AcknowledgedEventSequence: acknowledgedSequence}}
	switch status {
	case "queued":
		if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='canceled', status_reason='canceled before assignment', completed_at=now() WHERE id=$1`, runID); err != nil {
			return CancelResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='canceled', conversation_revision=conversation_revision+1 WHERE id=$1`, taskID); err != nil {
			return CancelResult{}, err
		}
		if err := appendEvent(ctx, tx, runID, "run.canceled"); err != nil {
			return CancelResult{}, err
		}
		result.Run.Status = "canceled"
	case "assigned", "accepted":
		if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='canceling' WHERE id=$1`, runID); err != nil {
			return CancelResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='canceling', conversation_revision=conversation_revision+1 WHERE id=$1`, taskID); err != nil {
			return CancelResult{}, err
		}
		if err := appendEvent(ctx, tx, runID, "run.cancel_requested"); err != nil {
			return CancelResult{}, err
		}
		result.Run.Status, result.Deliver = "canceling", true
	case "canceling", "completed", "failed", "canceled":
		result.Run.Status = publicStatus(status)
	default:
		return CancelResult{}, fmt.Errorf("unknown run status %q", status)
	}
	if err := tx.Commit(ctx); err != nil {
		return CancelResult{}, err
	}
	return result, nil
}

func (s *Service) Retry(ctx context.Context, actor identity.Principal, taskID string, useLatest bool, key string, expectedRevision int64) (Task, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 {
		return Task{}, ErrInvalidInput
	}
	if expectedRevision < 1 {
		return Task{}, ErrPreconditionRequired
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback(ctx)
	duplicate, err := reserveTaskCommand(ctx, tx, actor.ID, retryRoute, key, requestDigest(taskID, fmt.Sprintf("retry\n%t", useLatest)), taskID)
	if err != nil {
		return Task{}, err
	}
	if duplicate {
		if err := tx.Commit(ctx); err != nil {
			return Task{}, err
		}
		return s.Get(ctx, actor, taskID)
	}
	var agentID, workspaceID, revisionID, deploymentID, oldRunID, oldStatus string
	var currentRevision int64
	err = tx.QueryRow(ctx, `SELECT t.agent_id, t.workspace_id, r.agent_revision_id, COALESCE(r.deployment_id, ''), r.id, r.status, t.conversation_revision FROM gantry.tasks t JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2 FOR UPDATE`, taskID, actor.ID).Scan(&agentID, &workspaceID, &revisionID, &deploymentID, &oldRunID, &oldStatus, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	if currentRevision != expectedRevision {
		return Task{}, ErrConversationChanged
	}
	if oldStatus != "failed" && oldStatus != "canceled" {
		return Task{}, ErrInvalidState
	}
	if useLatest {
		err = tx.QueryRow(ctx, `SELECT d.id, d.revision_id FROM gantry.agents a JOIN gantry.agent_deployments d ON d.agent_id=a.id AND d.workspace_id=a.workspace_id AND d.environment_kind='production' AND d.status='active' JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1 WHERE a.id=$2 AND a.workspace_id=$3`, actor.ID, agentID, workspaceID).Scan(&deploymentID, &revisionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		if err != nil {
			return Task{}, err
		}
	}
	if deploymentID == "" {
		return Task{}, ErrInvalidState
	}
	manifestDigest, err := manifestDigestForRevision(ctx, tx, revisionID)
	if err != nil {
		return Task{}, err
	}
	var attempt int
	if err := tx.QueryRow(ctx, `SELECT attempt_number+1 FROM gantry.runs WHERE id=$1`, oldRunID).Scan(&attempt); err != nil {
		return Task{}, err
	}
	runID := newID("run")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.runs (id, task_id, agent_revision_id, deployment_id, manifest_digest, attempt_number, status) VALUES ($1,$2,$3,$4,$5,$6,'queued')`, runID, taskID, revisionID, deploymentID, manifestDigest, attempt); err != nil {
		return Task{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET current_run_id=$2, status='queued', conversation_revision=conversation_revision+1 WHERE id=$1`, taskID, runID); err != nil {
		return Task{}, err
	}
	if err := appendEvent(ctx, tx, runID, "run.queued"); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, err
	}
	return s.Get(ctx, actor, taskID)
}

func reserveTaskCommand(ctx context.Context, tx pgx.Tx, principalID, route, key, digest, taskID string) (bool, error) {
	var storedDigest, storedTaskID string
	err := tx.QueryRow(ctx, `INSERT INTO gantry.idempotency_tombstones (principal_id, route, idempotency_key, request_digest, task_id) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING request_digest, task_id`, principalID, route, key, digest, taskID).Scan(&storedDigest, &storedTaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT request_digest, task_id FROM gantry.idempotency_tombstones WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3 FOR UPDATE`, principalID, route, key).Scan(&storedDigest, &storedTaskID); err != nil {
			return false, err
		}
		if storedDigest != digest || storedTaskID != taskID {
			return false, ErrIdempotencyConflict
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}
