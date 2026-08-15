package tasks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/agentlifecycle"
	"github.com/AirSodaz/gantry/internal/policy"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ClaimNext(ctx context.Context, runnerID string) (Assignment, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Assignment{}, false, err
	}
	defer tx.Rollback(ctx)
	var runID string
	var spec json.RawMessage
	err = tx.QueryRow(ctx, `SELECT r.id, v.spec_json FROM gantry.runs r JOIN gantry.agent_versions v ON v.id=r.agent_version_id WHERE r.status='queued' ORDER BY r.created_at, r.id FOR UPDATE OF r SKIP LOCKED LIMIT 1`).Scan(&runID, &spec)
	if errors.Is(err, pgx.ErrNoRows) {
		return Assignment{}, false, nil
	}
	if err != nil {
		return Assignment{}, false, err
	}
	manifest, digest, err := executionManifest(spec)
	if err != nil {
		return Assignment{}, false, err
	}
	var epoch uint64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET status='assigned', runner_id=$2, lease_epoch=lease_epoch+1, started_at=COALESCE(started_at, now()) WHERE id=$1 RETURNING lease_epoch`, runID, runnerID).Scan(&epoch); err != nil {
		return Assignment{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='running' WHERE current_run_id=$1`, runID); err != nil {
		return Assignment{}, false, err
	}
	if err := appendEvent(ctx, tx, runID, "run.assigned"); err != nil {
		return Assignment{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Assignment{}, false, err
	}
	return Assignment{RunID: runID, LeaseEpoch: epoch, Manifest: manifest, ManifestDigest: digest}, true, nil
}

func (s *Service) Accept(ctx context.Context, runnerID, runID string, epoch uint64, digest string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var spec json.RawMessage
	err = tx.QueryRow(ctx, `SELECT v.spec_json FROM gantry.runs r JOIN gantry.agent_versions v ON v.id=r.agent_version_id WHERE r.id=$1 AND r.runner_id=$2 AND r.lease_epoch=$3 AND r.status='assigned' FOR UPDATE OF r`, runID, runnerID, epoch).Scan(&spec)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, expected, err := executionManifest(spec)
	if err != nil {
		return err
	}
	if digest != expected {
		return ErrInvalidInput
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='accepted' WHERE id=$1`, runID); err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, runID, "run.started"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) RecordEvents(ctx context.Context, runnerID, runID string, epoch uint64, events []RunnerEvent) (RecordEventsResult, error) {
	if len(events) == 0 {
		return RecordEventsResult{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RecordEventsResult{}, err
	}
	defer tx.Rollback(ctx)
	var status, requesterID string
	var current uint64
	err = tx.QueryRow(ctx, `SELECT r.status, r.runner_event_sequence, t.requester_principal_id FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.id=$1 AND r.runner_id=$2 AND r.lease_epoch=$3 FOR UPDATE`, runID, runnerID, epoch).Scan(&status, &current, &requesterID)
	if errors.Is(err, pgx.ErrNoRows) {
		return RecordEventsResult{}, ErrNotFound
	}
	if err != nil {
		return RecordEventsResult{}, err
	}
	if status != "accepted" && status != "canceling" {
		return RecordEventsResult{}, ErrInvalidInput
	}
	var grant *ExecutionGrant
	for _, event := range events {
		if err := validateRunnerEvent(event); err != nil {
			return RecordEventsResult{}, err
		}
		if event.ClientSequence != current+1 {
			return RecordEventsResult{}, ErrInvalidInput
		}
		current++
		if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET runner_event_sequence=$2 WHERE id=$1`, runID, current); err != nil {
			return RecordEventsResult{}, err
		}
		if err := appendEventPayload(ctx, tx, runID, event.Type, event.Payload); err != nil {
			return RecordEventsResult{}, err
		}
		switch event.Type {
		case "action.proposed":
			if s.approvals == nil {
				return RecordEventsResult{}, ErrInvalidInput
			}
			action, err := decodeObservedAction(event.Payload, runID, requesterID)
			if err != nil {
				return RecordEventsResult{}, err
			}
			request, evaluation, err := s.approvals.Propose(ctx, tx, action, time.Now().UTC().Add(15*time.Minute))
			if err != nil {
				return RecordEventsResult{}, err
			}
			if evaluation.Decision == policy.RequireApproval {
				if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='awaiting_approval' WHERE id=$1`, runID); err != nil {
					return RecordEventsResult{}, err
				}
				if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='awaiting_approval' WHERE current_run_id=$1`, runID); err != nil {
					return RecordEventsResult{}, err
				}
				payload, _ := json.Marshal(map[string]any{"approval_id": request.ID, "action_digest": request.ActionDigest})
				if err := appendEventPayload(ctx, tx, runID, "approval.requested", string(payload)); err != nil {
					return RecordEventsResult{}, err
				}
			}
		case "action.execution_requested":
			var request struct {
				ActionID string `json:"action_id"`
				CallID   string `json:"call_id"`
				PermitID string `json:"permit_id"`
			}
			if err := json.Unmarshal([]byte(event.Payload), &request); err != nil || strings.TrimSpace(request.ActionID) == "" || strings.TrimSpace(request.CallID) == "" || strings.TrimSpace(request.PermitID) == "" {
				return RecordEventsResult{}, ErrInvalidInput
			}
			var actionID, callID, permitID string
			var expiresAt time.Time
			err := tx.QueryRow(ctx, `SELECT id, runner_call_id, execution_permit_id, execution_permit_expires_at FROM gantry.actions WHERE id=$1 AND run_id=$2 AND runner_call_id=$3 AND execution_permit_id=$4 AND execution_permit_lease_epoch=$5 AND state='ready' AND execution_permit_expires_at > now() FOR UPDATE`, request.ActionID, runID, request.CallID, request.PermitID, epoch).Scan(&actionID, &callID, &permitID, &expiresAt)
			if errors.Is(err, pgx.ErrNoRows) {
				return RecordEventsResult{}, ErrInvalidInput
			}
			if err != nil {
				return RecordEventsResult{}, err
			}
			result, err := tx.Exec(ctx, `UPDATE gantry.actions SET state='executing', revision=revision+1, execution_claimed_at=now(), updated_at=now() WHERE id=$1 AND state='ready'`, actionID)
			if err != nil || result.RowsAffected() != 1 {
				if err != nil {
					return RecordEventsResult{}, err
				}
				return RecordEventsResult{}, ErrInvalidInput
			}
			grant = &ExecutionGrant{ActionID: actionID, CallID: callID, PermitID: permitID, ExpiresAt: expiresAt}
		case "tool.call.completed", "tool.call.failed":
			callID, err := decodeObservedCallID(event.Payload)
			if err != nil {
				return RecordEventsResult{}, err
			}
			terminalState := "succeeded"
			if event.Type == "tool.call.failed" {
				terminalState = "failed"
			}
			if err := transitionObservedAction(ctx, tx, runID, callID, terminalState); err != nil {
				return RecordEventsResult{}, err
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return RecordEventsResult{}, err
	}
	return RecordEventsResult{Sequence: current, Grant: grant}, nil
}

func decodeObservedAction(payload, runID, requesterID string) (policy.Action, error) {
	var action policy.Action
	if err := json.Unmarshal([]byte(payload), &action); err != nil {
		return policy.Action{}, ErrInvalidInput
	}
	action.RunID = strings.TrimSpace(runID)
	action.RequestedBy = strings.TrimSpace(requesterID)
	action.CallID = strings.TrimSpace(action.CallID)
	if action.CallID == "" || len(action.Arguments) == 0 || string(action.Arguments) == "null" {
		return policy.Action{}, ErrInvalidInput
	}
	var arguments map[string]json.RawMessage
	if err := json.Unmarshal(action.Arguments, &arguments); err != nil || arguments == nil {
		return policy.Action{}, ErrInvalidInput
	}
	return action, nil
}

func decodeObservedCallID(payload string) (string, error) {
	var event struct {
		CallID string `json:"call_id"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return "", ErrInvalidInput
	}
	event.CallID = strings.TrimSpace(event.CallID)
	if event.CallID == "" {
		return "", ErrInvalidInput
	}
	return event.CallID, nil
}

func transitionObservedAction(ctx context.Context, tx pgx.Tx, runID, callID, terminalState string) error {
	runID = strings.TrimSpace(runID)
	callID = strings.TrimSpace(callID)
	if runID == "" || callID == "" || (terminalState != "succeeded" && terminalState != "failed") {
		return ErrInvalidInput
	}
	var actionID, state string
	err := tx.QueryRow(ctx, `SELECT id, state FROM gantry.actions WHERE run_id=$1 AND runner_call_id=$2 FOR UPDATE`, runID, callID).Scan(&actionID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if state != "executing" {
		return ErrInvalidInput
	}
	result, err := tx.Exec(ctx, `UPDATE gantry.actions SET state=$2, revision=revision+1, updated_at=now() WHERE id=$1 AND state='executing'`, actionID, terminalState)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrInvalidInput
	}
	return nil
}

func validateRunnerEvent(event RunnerEvent) error {
	if event.ClientSequence == 0 || len(event.Type) == 0 || len(event.Type) > 128 {
		return ErrInvalidInput
	}
	if !strings.HasPrefix(event.Type, "agent.") &&
		!strings.HasPrefix(event.Type, "action.") &&
		!strings.HasPrefix(event.Type, "checkpoint.") &&
		!strings.HasPrefix(event.Type, "context.") &&
		!strings.HasPrefix(event.Type, "model.") &&
		!strings.HasPrefix(event.Type, "run.") &&
		!strings.HasPrefix(event.Type, "security.") &&
		!strings.HasPrefix(event.Type, "tool.") {
		return ErrInvalidInput
	}
	if event.Payload == "" {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		return ErrInvalidInput
	}
	return nil
}

// RecordControlEvent persists a control-plane-owned transition without
// consuming the runner's client event sequence.
func (s *Service) RecordControlEvent(ctx context.Context, runnerID, runID string, epoch uint64, eventType, payload string) error {
	if err := validateRunnerEvent(RunnerEvent{ClientSequence: 1, Type: eventType, Payload: payload}); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM gantry.runs WHERE id=$1 AND runner_id=$2 AND lease_epoch=$3 FOR UPDATE`, runID, runnerID, epoch).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if status != "accepted" && status != "canceling" {
		return ErrInvalidInput
	}
	if eventType == "model.delta" && s.store != nil {
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return s.appendModelDelta(ctx, runID, payload)
	}
	if err := appendEventPayload(ctx, tx, runID, eventType, payload); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Finish(ctx context.Context, runnerID, runID string, epoch uint64, terminal, reason string) error {
	if err := s.flushRunContent(ctx, runID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status, taskID string
	err = tx.QueryRow(ctx, `SELECT status, task_id FROM gantry.runs WHERE id=$1 AND runner_id=$2 AND lease_epoch=$3 FOR UPDATE`, runID, runnerID, epoch).Scan(&status, &taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "accepted" && status != "canceling" {
		return ErrInvalidInput
	}
	if (terminal == "completed" && status == "canceling") || (terminal == "canceled" && status != "canceling") {
		return ErrInvalidInput
	}
	if terminal != "completed" && terminal != "failed" && terminal != "canceled" {
		return ErrInvalidInput
	}
	if err := markExecutingActionsUnknown(ctx, tx, runID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status=$2, status_reason=$3, completed_at=now() WHERE id=$1`, runID, terminal, reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status=$2 WHERE id=$1`, taskID, terminal); err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, runID, "run."+terminal); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) FailActive(ctx context.Context, runnerID, runID, reason string) error {
	if err := s.flushRunContent(ctx, runID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var taskID string
	err = tx.QueryRow(ctx, `UPDATE gantry.runs SET status='failed', status_reason=$3, completed_at=now() WHERE id=$1 AND runner_id=$2 AND status IN ('assigned','accepted','canceling') RETURNING task_id`, runID, runnerID, reason).Scan(&taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := markExecutingActionsUnknown(ctx, tx, runID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='failed' WHERE id=$1`, taskID); err != nil {
		return err
	}
	if err := appendEvent(ctx, tx, runID, "run.failed"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FailInFlight records the explicit recovery contract for deterministic demo
// runs: an interrupted control plane does not resume execution.
func (s *Service) FailInFlight(ctx context.Context, reason string) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `UPDATE gantry.runs SET status='failed', status_reason=$1, completed_at=now() WHERE status IN ('assigned','accepted','canceling') RETURNING id, task_id`, reason)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type failedRun struct {
		id     string
		taskID string
	}
	failed := make([]failedRun, 0)
	for rows.Next() {
		var run failedRun
		if err := rows.Scan(&run.id, &run.taskID); err != nil {
			return 0, err
		}
		failed = append(failed, run)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, run := range failed {
		if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='failed' WHERE id=$1 AND current_run_id=$2`, run.taskID, run.id); err != nil {
			return 0, err
		}
		if err := appendEvent(ctx, tx, run.id, "run.failed"); err != nil {
			return 0, err
		}
		if err := markExecutingActionsUnknown(ctx, tx, run.id); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {

		return 0, err
	}
	return len(failed), nil
}
func markExecutingActionsUnknown(ctx context.Context, tx pgx.Tx, runID string) error {
	_, err := tx.Exec(ctx, `UPDATE gantry.actions SET state='unknown_outcome', revision=revision+1, updated_at=now() WHERE run_id=$1 AND state='executing'`, runID)
	return err
}

func executionManifest(spec json.RawMessage) ([]byte, string, error) {
	manifest, findings := agentlifecycle.ValidateSpec(spec)
	if len(findings) != 0 {
		return nil, "", ErrInvalidInput
	}
	sum := sha256.Sum256(manifest)
	return manifest, "sha256:" + hex.EncodeToString(sum[:]), nil
}
