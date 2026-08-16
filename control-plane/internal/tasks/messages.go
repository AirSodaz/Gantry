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
func (s *Service) AppendMessage(ctx context.Context, actor identity.Principal, taskID, key string, expectedRevision int64, request AppendMessageRequest) (Task, bool, error) {
	key = strings.TrimSpace(key)
	message := strings.TrimSpace(request.Message)
	if key == "" || len(key) > 256 || message == "" {
		return Task{}, false, ErrInvalidInput
	}
	if expectedRevision < 1 {
		return Task{}, false, ErrPreconditionRequired
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
	var currentRevision int64
	err = tx.QueryRow(ctx, `SELECT r.agent_revision_id, COALESCE(r.deployment_id, ''), r.id, r.status, t.conversation_revision FROM gantry.tasks t JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2 FOR UPDATE`, taskID, actor.ID).Scan(&revisionID, &deploymentID, &oldRunID, &oldStatus, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, false, ErrNotFound
	}
	if err != nil {
		return Task{}, false, err
	}
	if currentRevision != expectedRevision {
		return Task{}, false, ErrConversationChanged
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
	var taskSequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.tasks SET task_event_sequence=task_event_sequence+1 WHERE id=$1 RETURNING task_event_sequence`, taskID).Scan(&taskSequence); err != nil {
		return Task{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.task_messages (id, task_id, run_id, task_sequence, role, parts, content) VALUES ($1,$2,$3,$4,'requester',jsonb_build_array(jsonb_build_object('type','text','text',$5)),$5)`, newID("msg"), taskID, runID, taskSequence, message); err != nil {
		return Task{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET current_run_id=$2, status='queued', conversation_revision=conversation_revision+1 WHERE id=$1`, taskID, runID); err != nil {
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
