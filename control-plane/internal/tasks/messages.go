package tasks

import (
	"context"
	"errors"
	"strings"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

const appendMessageRoute = "POST /api/copilot/v1/tasks/{task_id}/messages"

// AppendMessage records a new requester instruction only after a rejected or
// expired action leaves the task awaiting further direction. It creates a new
// run attempt and preserves the prior run's evidence.
func (s *Service) AppendMessage(ctx context.Context, actor identity.Principal, taskID, key string, request AppendMessageRequest) (Task, bool, error) {
	key = strings.TrimSpace(key)
	message := strings.TrimSpace(request.Message)
	if key == "" || len(key) > 256 || message == "" {
		return Task{}, false, ErrInvalidInput
	}
	digest := requestDigest(taskID, message)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Task{}, false, err
	}
	defer tx.Rollback(ctx)

	var storedDigest, storedTaskID string
	err = tx.QueryRow(ctx, `INSERT INTO gantry.idempotency_tombstones (principal_id, route, idempotency_key, request_digest, task_id) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING request_digest, task_id`, actor.ID, appendMessageRoute, key, digest, taskID).Scan(&storedDigest, &storedTaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT request_digest, task_id FROM gantry.idempotency_tombstones WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3 FOR UPDATE`, actor.ID, appendMessageRoute, key).Scan(&storedDigest, &storedTaskID); err != nil {
			return Task{}, false, err
		}
		if storedDigest != digest || storedTaskID != taskID {
			return Task{}, false, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Task{}, false, err
		}
		task, err := s.Get(ctx, actor, taskID)
		return task, true, err
	}
	if err != nil {
		return Task{}, false, err
	}

	var revisionID, deploymentID, oldRunID, oldStatus string
	err = tx.QueryRow(ctx, `SELECT r.agent_revision_id, COALESCE(r.deployment_id, ''), r.id, r.status FROM gantry.tasks t JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2 FOR UPDATE`, taskID, actor.ID).Scan(&revisionID, &deploymentID, &oldRunID, &oldStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, false, ErrNotFound
	}
	if err != nil {
		return Task{}, false, err
	}
	if oldStatus != "failed" {
		return Task{}, false, ErrInvalidState
	}
	var taskStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM gantry.tasks WHERE id=$1`, taskID).Scan(&taskStatus); err != nil {
		return Task{}, false, err
	}
	if taskStatus != "awaiting_requester_input" || deploymentID == "" {
		return Task{}, false, ErrInvalidState
	}
	manifestDigest, err := manifestDigestForRevision(ctx, tx, revisionID)
	if err != nil {
		return Task{}, false, err
	}
	var attempt int
	if err := tx.QueryRow(ctx, `SELECT attempt_number+1 FROM gantry.runs WHERE id=$1`, oldRunID).Scan(&attempt); err != nil {
		return Task{}, false, err
	}
	runID := newID("run")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.runs (id, task_id, agent_revision_id, deployment_id, manifest_digest, attempt_number, status) VALUES ($1,$2,$3,$4,$5,$6,'queued')`, runID, taskID, revisionID, deploymentID, manifestDigest, attempt); err != nil {
		return Task{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.task_messages (id, task_id, run_id, role, content) VALUES ($1,$2,$3,'requester',$4)`, newID("msg"), taskID, runID, message); err != nil {
		return Task{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET current_run_id=$2, status='queued' WHERE id=$1`, taskID, runID); err != nil {
		return Task{}, false, err
	}
	if err := appendEvent(ctx, tx, runID, "task.message"); err != nil {
		return Task{}, false, err
	}
	if err := appendEvent(ctx, tx, runID, "run.queued"); err != nil {
		return Task{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, false, err
	}
	task, err := s.Get(ctx, actor, taskID)
	return task, false, err
}
