package tasks

import (
	"context"
	"errors"
	"fmt"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

func (s *Service) Cancel(ctx context.Context, actor identity.Principal, taskID, runID string) (CancelResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CancelResult{}, err
	}
	defer tx.Rollback(ctx)
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
		if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='canceled' WHERE id=$1`, taskID); err != nil {
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
		if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='canceling' WHERE id=$1`, taskID); err != nil {
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

func (s *Service) Retry(ctx context.Context, actor identity.Principal, taskID string, useLatest bool) (Task, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback(ctx)
	var agentID, workspaceID, versionID, oldRunID, oldStatus string
	err = tx.QueryRow(ctx, `SELECT t.agent_id, t.workspace_id, r.agent_version_id, r.id, r.status FROM gantry.tasks t JOIN gantry.runs r ON r.id=t.current_run_id WHERE t.id=$1 AND t.requester_principal_id=$2 FOR UPDATE`, taskID, actor.ID).Scan(&agentID, &workspaceID, &versionID, &oldRunID, &oldStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, err
	}
	if oldStatus != "failed" && oldStatus != "canceled" {
		return Task{}, ErrInvalidState
	}
	if useLatest {
		err = tx.QueryRow(ctx, `SELECT p.agent_version_id FROM gantry.agents a JOIN gantry.agent_publications p ON p.agent_id=a.id AND p.workspace_id=a.workspace_id AND p.status='published' JOIN gantry.workspace_memberships m ON m.workspace_id=a.workspace_id AND m.principal_id=$1 WHERE a.id=$2 AND a.workspace_id=$3`, actor.ID, agentID, workspaceID).Scan(&versionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Task{}, ErrNotFound
		}
		if err != nil {
			return Task{}, err
		}
	}
	var mode string
	var attempt int
	if err := tx.QueryRow(ctx, `SELECT demo_mode, attempt_number+1 FROM gantry.runs WHERE id=$1`, oldRunID).Scan(&mode, &attempt); err != nil {
		return Task{}, err
	}
	runID := newID("run")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.runs (id, task_id, agent_version_id, attempt_number, demo_mode, status) VALUES ($1,$2,$3,$4,$5,'queued')`, runID, taskID, versionID, attempt, mode); err != nil {
		return Task{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET current_run_id=$2, status='queued' WHERE id=$1`, taskID, runID); err != nil {
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
