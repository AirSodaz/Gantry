package sessions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/sessionmessage"
	"github.com/jackc/pgx/v5"
)

const (
	cancelRoute = "POST /api/copilot/v1/sessions/{session_id}/runs/{run_id}:cancel"
	retryRoute  = "POST /api/copilot/v1/sessions/{session_id}/runs/{run_id}:retry"
)

func (s *Service) Cancel(ctx context.Context, actor identity.Principal, sessionID, runID, key string) (CancelResult, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 {
		return CancelResult{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CancelResult{}, err
	}
	defer tx.Rollback(ctx)
	duplicate, err := reserveSessionCommand(ctx, tx, actor.ID, cancelRoute, key, requestDigest(sessionID, "cancel\n"+runID), sessionID)
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
		if existing.SessionID != sessionID {
			return CancelResult{}, ErrNotFound
		}
		return CancelResult{Run: existing.Run}, nil
	}
	var status string
	var epoch, acknowledgedSequence uint64
	err = tx.QueryRow(ctx, `SELECT r.status, r.lease_epoch, r.runner_event_sequence FROM gantry.runs r JOIN gantry.sessions s ON s.id=r.session_id WHERE r.id=$1 AND r.session_id=$2 AND (r.requester_principal_id=$3 OR s.owner_principal_id=$3) FOR UPDATE`, runID, sessionID, actor.ID).Scan(&status, &epoch, &acknowledgedSequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return CancelResult{}, ErrNotFound
	}
	if err != nil {
		return CancelResult{}, err
	}
	result := CancelResult{Run: Run{ID: runID, Status: publicStatus(status), LeaseEpoch: epoch, AcknowledgedEventSequence: acknowledgedSequence}}
	switch status {
	case "queued":
		if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='canceled', outcome='canceled', status_reason='canceled before assignment', completed_at=now() WHERE id=$1`, runID); err != nil {
			return CancelResult{}, err
		}
		if err := sessionmessage.Append(ctx, tx, sessionID, runID, "system_summary", sessionmessage.Status("run.canceled", "Run canceled before assignment.")); err != nil {
			return CancelResult{}, err
		}
		if err := appendEvent(ctx, tx, runID, "run.canceled"); err != nil {
			return CancelResult{}, err
		}
		result.Run.Status = "canceled"
	case "assigned", "accepted", "awaiting_approval", "suspended":
		if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='canceling' WHERE id=$1`, runID); err != nil {
			return CancelResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.approval_requests SET status='superseded', decided_at=now() WHERE run_id=$1 AND status='pending'`, runID); err != nil {
			return CancelResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.actions SET state='rejected', revision=revision+1, updated_at=now() WHERE run_id=$1 AND state='awaiting_approval'`, runID); err != nil {
			return CancelResult{}, err
		}
		if err := sessionmessage.Append(ctx, tx, sessionID, runID, "system_summary", sessionmessage.Status("run.cancel_requested", "Cancellation requested.")); err != nil {
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

func (s *Service) Retry(ctx context.Context, actor identity.Principal, sessionID string, useLatest bool, key string, expectedRevision int64) (RetryResult, error) {
	return s.retry(ctx, actor, sessionID, "", useLatest, key, expectedRevision)
}

func (s *Service) RetryRun(ctx context.Context, actor identity.Principal, sessionID, runID string, useLatest bool, key string, expectedRevision int64) (RetryResult, error) {
	return s.retry(ctx, actor, sessionID, strings.TrimSpace(runID), useLatest, key, expectedRevision)
}

func (s *Service) retry(ctx context.Context, actor identity.Principal, sessionID, sourceRunID string, useLatest bool, key string, expectedRevision int64) (RetryResult, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 {
		return RetryResult{}, ErrInvalidInput
	}
	if expectedRevision < 1 {
		return RetryResult{}, ErrPreconditionRequired
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RetryResult{}, err
	}
	defer tx.Rollback(ctx)
	if sourceRunID == "" && sessionID == "" {
		return RetryResult{}, ErrInvalidInput
	}
	execution, err := lockExecutableSession(ctx, tx, actor, sessionID)
	if err != nil {
		return RetryResult{}, err
	}
	if execution.ConversationRevision != expectedRevision {
		return RetryResult{}, ErrConversationChanged
	}
	if execution.State != "active" {
		return RetryResult{}, ErrInvalidState
	}
	duplicate, err := reserveSessionCommand(ctx, tx, actor.ID, retryRoute, key, requestDigest(sessionID, fmt.Sprintf("retry\n%s\n%t", sourceRunID, useLatest)), sessionID)
	if err != nil {
		return RetryResult{}, err
	}
	if duplicate {
		var receiptRunID *string
		if err := tx.QueryRow(ctx, `SELECT run_id FROM gantry.idempotency_tombstones WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3`, actor.ID, retryRoute, key).Scan(&receiptRunID); err != nil {
			return RetryResult{}, err
		}
		if receiptRunID == nil || *receiptRunID == "" {
			return RetryResult{}, ErrInvalidState
		}
		if err := tx.Commit(ctx); err != nil {
			return RetryResult{}, err
		}
		existing, err := s.GetRun(ctx, actor, *receiptRunID)
		if err != nil || existing.SessionID != sessionID {
			if err == nil {
				err = ErrNotFound
			}
			return RetryResult{}, err
		}
		return RetryResult{Run: existing.Run, Duplicate: true}, nil
	}
	var revisionID, deploymentID, oldRunID, oldStatus string
	query := `SELECT r.agent_revision_id,COALESCE(r.deployment_id,''),r.id,r.status FROM gantry.runs r `
	args := []any{sessionID, actor.ID}
	if sourceRunID != "" {
		query += `WHERE r.session_id=$1 AND r.id=$2 FOR UPDATE`
		args = []any{sessionID, sourceRunID}
	} else {
		query += `WHERE r.session_id=$1 ORDER BY r.session_sequence DESC LIMIT 1 FOR UPDATE`
		args = []any{sessionID}
	}
	err = tx.QueryRow(ctx, query, args...).Scan(&revisionID, &deploymentID, &oldRunID, &oldStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return RetryResult{}, ErrNotFound
	}
	if err != nil {
		return RetryResult{}, err
	}
	if oldStatus != "failed" && oldStatus != "canceled" {
		return RetryResult{}, ErrInvalidState
	}
	if useLatest {
		deploymentID, revisionID = execution.DeploymentID, execution.RevisionID
	}
	if deploymentID == "" {
		return RetryResult{}, ErrInvalidState
	}
	manifestDigest, err := manifestDigestForRevision(ctx, tx, revisionID)
	if err != nil {
		return RetryResult{}, err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(session_sequence),0)+1 FROM gantry.runs WHERE session_id=$1`, sessionID).Scan(&sequence); err != nil {
		return RetryResult{}, err
	}
	runID := newID("run")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.runs (id, session_id, requester_principal_id, agent_revision_id, deployment_id, manifest_digest, session_sequence, status, retry_of_run_id) VALUES ($1,$2,$3,$4,$5,$6,$7,'queued',$8)`, runID, sessionID, actor.ID, revisionID, deploymentID, manifestDigest, sequence, oldRunID); err != nil {
		return RetryResult{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.idempotency_tombstones SET run_id=$4 WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3`, actor.ID, retryRoute, key, runID); err != nil {
		return RetryResult{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.sessions SET conversation_revision=conversation_revision+1, updated_at=now() WHERE id=$1`, sessionID); err != nil {
		return RetryResult{}, err
	}
	if err := appendEvent(ctx, tx, runID, "run.queued"); err != nil {
		return RetryResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RetryResult{}, err
	}
	created, err := s.GetRun(ctx, actor, runID)
	if err != nil || created.SessionID != sessionID {
		if err == nil {
			err = ErrNotFound
		}
		return RetryResult{}, err
	}
	return RetryResult{Run: created.Run}, nil
}

type executionContext struct {
	WorkspaceID          string
	AgentID              string
	DeploymentID         string
	RevisionID           string
	ConversationRevision int64
	State                string
}

// lockExecutableSession rechecks every authority that can admit a new Run.
// Locking the mutable facts prevents membership, ACL, or deployment changes
// from racing a successful queue insertion.
func lockExecutableSession(ctx context.Context, tx pgx.Tx, actor identity.Principal, sessionID string) (executionContext, error) {
	var result executionContext
	err := tx.QueryRow(ctx, `SELECT s.workspace_id,s.agent_id,d.id,d.revision_id,s.conversation_revision,s.state FROM gantry.sessions s JOIN gantry.session_members sm ON sm.session_id=s.id AND sm.principal_id=$2 AND sm.role IN ('owner','contributor') JOIN gantry.workspace_memberships wm ON wm.workspace_id=s.workspace_id AND wm.principal_id=$2 JOIN gantry.agents a ON a.id=s.agent_id AND a.organization_id=$3 JOIN gantry.agent_deployments d ON d.agent_id=a.id AND d.workspace_id=s.workspace_id AND d.environment_kind='production' AND d.status='active' JOIN gantry.agent_access_grants g ON g.agent_id=a.id AND g.subject_type='principal' AND g.subject_id=$2 AND g.state='active' AND g.valid_from<=now() AND (g.expires_at IS NULL OR g.expires_at>now()) JOIN gantry.agent_access_grant_capabilities c ON c.grant_id=g.id AND c.capability='execute' WHERE s.id=$1 FOR UPDATE OF s FOR SHARE OF sm,wm,d,g,c`, sessionID, actor.ID, actor.OrganizationID).Scan(&result.WorkspaceID, &result.AgentID, &result.DeploymentID, &result.RevisionID, &result.ConversationRevision, &result.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return executionContext{}, ErrNotFound
	}
	return result, err
}

func reserveSessionCommand(ctx context.Context, tx pgx.Tx, principalID, route, key, digest, sessionID string) (bool, error) {
	var storedDigest, storedSessionID string
	err := tx.QueryRow(ctx, `INSERT INTO gantry.idempotency_tombstones (principal_id, route, idempotency_key, request_digest, session_id) VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING RETURNING request_digest, session_id`, principalID, route, key, digest, sessionID).Scan(&storedDigest, &storedSessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT request_digest, session_id FROM gantry.idempotency_tombstones WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3 FOR UPDATE`, principalID, route, key).Scan(&storedDigest, &storedSessionID); err != nil {
			return false, err
		}
		if storedDigest != digest || storedSessionID != sessionID {
			return false, ErrIdempotencyConflict
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}
