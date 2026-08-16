// Package approvals owns approvals for concrete agent actions. Business
// workflow approvals remain owned by the calling tool or enterprise system.
package approvals

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/policy"
	"github.com/AirSodaz/gantry/internal/taskevents"
	"github.com/AirSodaz/gantry/internal/taskmessage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("approval not found")
	ErrInvalidInput   = errors.New("invalid approval input")
	ErrInvalidDigest  = errors.New("approval action digest is stale")
	ErrNotEligible    = errors.New("principal is not eligible to decide approval")
	ErrAlreadyDecided = errors.New("approval already decided")
	ErrExpired        = errors.New("approval has expired")
	ErrChanged        = errors.New("approval has changed")
	ErrIdempotency    = errors.New("approval idempotency key reused")
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type Request struct {
	ID               string    `json:"id"`
	RunID            string    `json:"run_id"`
	ActionID         string    `json:"action_id"`
	ActionDigest     string    `json:"action_digest"`
	Revision         int64     `json:"approval_revision"`
	ToolName         string    `json:"tool_name"`
	Operation        string    `json:"operation"`
	Target           string    `json:"target,omitempty"`
	Effect           string    `json:"effect"`
	ActionPreview    any       `json:"action_preview"`
	RiskClass        string    `json:"risk_class"`
	Status           string    `json:"status"`
	RequestedBy      string    `json:"requested_by"`
	AssignedTo       string    `json:"assigned_to"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	TaskID           string    `json:"task_id,omitempty"`
	AgentDisplayName string    `json:"agent_display_name"`
	PolicyVersion    string    `json:"policy_version,omitempty"`
	Decision         *Decision `json:"latest_decision,omitempty"`
}

type Decision struct {
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason,omitempty"`
	DecidedBy string    `json:"decided_by"`
	CreatedAt time.Time `json:"created_at"`
}

type Resolution struct {
	ApprovalID       string
	RunID            string
	ActionID         string
	CallID           string
	ActionDigest     string
	Decision         string
	Reason           string
	PermitID         string
	PermitLeaseEpoch uint64
	PermitExpiresAt  time.Time
}

type DecisionInput struct {
	ID           string
	ActionDigest string
	Decision     string
	Reason       string
	Idempotency  string
	Revision     int64
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

type Page struct {
	Items   []Request
	HasMore bool
}

// Expire advances the current requester's elapsed approval requests into a
// terminal non-executing state. The caller delivers each returned resolution to
// an active runner, when one still owns the matching lease.
func (s *Service) Expire(ctx context.Context, actor identity.Principal) ([]Resolution, error) {
	return s.expire(ctx, actor.ID)
}

// ExpireAll reconciles every elapsed approval. It is reserved for the control
// plane's background worker; requester-facing reads use Expire to retain their
// narrower projection.
func (s *Service) ExpireAll(ctx context.Context) ([]Resolution, error) {
	return s.expire(ctx, "")
}

func (s *Service) expire(ctx context.Context, principalID string) ([]Resolution, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	query := `SELECT ar.id, ar.run_id, ar.action_id, a.runner_call_id, ar.action_digest FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id WHERE ar.status='pending' AND ar.expires_at <= now()`
	arguments := []any{}
	if principalID != "" {
		query += ` AND (ar.assigned_principal_id=$1 OR (ar.assigned_principal_id IS NULL AND ar.requested_by_principal_id=$1))`
		arguments = append(arguments, principalID)
	}
	query += ` FOR UPDATE OF ar, a`
	rows, err := tx.Query(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	deferred := make([]Resolution, 0)
	for rows.Next() {
		var resolution Resolution
		if err := rows.Scan(&resolution.ApprovalID, &resolution.RunID, &resolution.ActionID, &resolution.CallID, &resolution.ActionDigest); err != nil {
			rows.Close()
			return nil, err
		}
		resolution.Decision = "reject"
		resolution.Reason = "approval expired"
		deferred = append(deferred, resolution)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for _, resolution := range deferred {
		if _, err := tx.Exec(ctx, `UPDATE gantry.approval_requests SET status='expired', decided_at=now() WHERE id=$1 AND status='pending'`, resolution.ApprovalID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.actions SET state='rejected', revision=revision+1, updated_at=now() WHERE id=$1 AND state='awaiting_approval'`, resolution.ActionID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='accepted' WHERE id=$1 AND status='awaiting_approval'`, resolution.RunID); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='awaiting_requester_input', conversation_revision=conversation_revision+1 WHERE current_run_id=$1 AND status='awaiting_approval'`, resolution.RunID); err != nil {
			return nil, err
		}
		if err := appendActionSummary(ctx, tx, resolution.RunID, resolution.ActionID, "rejected"); err != nil {
			return nil, err
		}
		if err := appendApprovalStatus(ctx, tx, resolution.RunID, "approval.expired", "Action approval expired."); err != nil {
			return nil, err
		}
		if err := appendEvent(ctx, tx, resolution.RunID, "approval.expired", map[string]any{"approval_id": resolution.ApprovalID, "action_digest": resolution.ActionDigest}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return deferred, nil
}

// Propose evaluates and persists a concrete action in the caller's
// transaction. Only actions requiring a human decision create an approval.
func (s *Service) Propose(ctx context.Context, tx pgx.Tx, action policy.Action, expiresAt time.Time) (Request, policy.Evaluation, error) {
	canonical, _, digest, err := policy.Canonicalize(action)
	if err != nil {
		return Request{}, policy.Evaluation{}, err
	}
	evaluation, err := policy.Evaluate(canonical, false)
	if err != nil {
		return Request{}, policy.Evaluation{}, err
	}
	actionID := newID("act")
	actionState := "ready"
	if evaluation.Decision == policy.RequireApproval {
		actionState = "awaiting_approval"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.actions (id, run_id, runner_call_id, tool_name, operation, arguments_json, target, effect, credential_ref, credential_mode, policy_version, action_digest, state, revision, requested_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10,$11,$12,$13,1,$14)`, actionID, canonical.RunID, canonical.CallID, canonical.ToolName, canonical.Operation, string(canonical.Arguments), canonical.Target, canonical.Effect, canonical.CredentialRef, canonical.CredentialMode, canonical.PolicyVersion, digest, actionState, canonical.RequestedBy); err != nil {
		return Request{}, policy.Evaluation{}, err
	}
	if evaluation.Decision != policy.RequireApproval {
		return Request{ActionID: actionID, ActionDigest: digest, Status: "ready"}, evaluation, nil
	}
	if expiresAt.IsZero() {
		expiresAt = time.Now().UTC().Add(15 * time.Minute)
	}
	approvalID := newID("apr")
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.approval_requests (id, action_id, run_id, action_digest, action_preview, risk_class, status, requested_by_principal_id, assigned_principal_id, expires_at) VALUES ($1,$2,$3,$4,$5::jsonb,$6,'pending',$7,$7,$8)`, approvalID, actionID, canonical.RunID, digest, string(preview(canonical)), riskClass(canonical.Effect), canonical.RequestedBy, expiresAt); err != nil {
		return Request{}, policy.Evaluation{}, err
	}
	return Request{ID: approvalID, RunID: canonical.RunID, ActionID: actionID, ActionDigest: digest, Revision: 1, ToolName: canonical.ToolName, Operation: canonical.Operation, Target: canonical.Target, Effect: canonical.Effect, ActionPreview: previewMap(canonical), RiskClass: riskClass(canonical.Effect), Status: "pending", RequestedBy: canonical.RequestedBy, AssignedTo: canonical.RequestedBy, ExpiresAt: expiresAt}, evaluation, nil
}

func (s *Service) List(ctx context.Context, actor identity.Principal, state string, after *Cursor, limit int) (Page, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	state = strings.TrimSpace(state)
	if state == "" {
		state = "pending"
	}
	var afterCreatedAt *time.Time
	var afterID string
	if after != nil {
		afterCreatedAt, afterID = &after.CreatedAt, after.ID
	}
	rows, err := s.pool.Query(ctx, `SELECT ar.id, ar.run_id, ar.action_id, ar.action_digest, a.revision, ar.action_preview, ar.risk_class, ar.status, ar.requested_by_principal_id, ar.assigned_principal_id, ar.expires_at, ar.created_at, a.tool_name, a.operation, a.target, a.effect, t.id, a.policy_version, agent.display_name FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id JOIN gantry.runs r ON r.id=ar.run_id JOIN gantry.tasks t ON t.id=r.task_id JOIN gantry.agents agent ON agent.id=t.agent_id WHERE ar.status=$2 AND (ar.assigned_principal_id=$1 OR (ar.assigned_principal_id IS NULL AND ar.requested_by_principal_id=$1)) AND ($3::timestamptz IS NULL OR ar.created_at > $3 OR (ar.created_at = $3 AND ar.id > $4)) ORDER BY ar.created_at, ar.id LIMIT $5`, actor.ID, state, afterCreatedAt, afterID, limit+1)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := make([]Request, 0)
	for rows.Next() {
		var item Request
		var previewJSON []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.ActionID, &item.ActionDigest, &item.Revision, &previewJSON, &item.RiskClass, &item.Status, &item.RequestedBy, &item.AssignedTo, &item.ExpiresAt, &item.CreatedAt, &item.ToolName, &item.Operation, &item.Target, &item.Effect, &item.TaskID, &item.PolicyVersion, &item.AgentDisplayName); err != nil {
			return Page{}, err
		}
		if err := json.Unmarshal(previewJSON, &item.ActionPreview); err != nil {
			return Page{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	page := Page{Items: items, HasMore: len(items) > limit}
	if page.HasMore {
		page.Items = page.Items[:limit]
	}
	return page, nil
}

// ListTask returns the full requester-authorized approval history for one
// task. It is used only by that Task's event snapshot, not by the pending-work
// queue, which intentionally remains limited to pending approvals.
func (s *Service) ListTask(ctx context.Context, actor identity.Principal, taskID string, limit int) ([]Request, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	rows, err := s.pool.Query(ctx, `SELECT ar.id, ar.run_id, ar.action_id, ar.action_digest, a.revision, ar.action_preview, ar.risk_class, ar.status, ar.requested_by_principal_id, COALESCE(ar.assigned_principal_id,''), ar.expires_at, ar.created_at, a.tool_name, a.operation, a.target, a.effect, t.id, a.policy_version, agent.display_name FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id JOIN gantry.runs r ON r.id=ar.run_id JOIN gantry.tasks t ON t.id=r.task_id JOIN gantry.agents agent ON agent.id=t.agent_id WHERE t.id=$1 AND t.requester_principal_id=$2 ORDER BY ar.created_at, ar.id LIMIT $3`, taskID, actor.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Request, 0)
	for rows.Next() {
		var item Request
		var previewJSON []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.ActionID, &item.ActionDigest, &item.Revision, &previewJSON, &item.RiskClass, &item.Status, &item.RequestedBy, &item.AssignedTo, &item.ExpiresAt, &item.CreatedAt, &item.ToolName, &item.Operation, &item.Target, &item.Effect, &item.TaskID, &item.PolicyVersion, &item.AgentDisplayName); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(previewJSON, &item.ActionPreview); err != nil {
			return nil, err
		}
		var decision Decision
		err := s.pool.QueryRow(ctx, `SELECT decision, reason, principal_id, created_at FROM gantry.approval_decisions WHERE approval_id=$1 ORDER BY created_at DESC LIMIT 1`, item.ID).Scan(&decision.Decision, &decision.Reason, &decision.DecidedBy, &decision.CreatedAt)
		if err == nil {
			item.Decision = &decision
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Get returns the requester-authorized approval projection, including the
// server-winning decision evidence when the request is no longer pending.
func (s *Service) Get(ctx context.Context, actor identity.Principal, approvalID string) (Request, error) {
	var item Request
	var previewJSON []byte
	err := s.pool.QueryRow(ctx, `SELECT ar.id, ar.run_id, ar.action_id, ar.action_digest, a.revision, ar.action_preview, ar.risk_class, ar.status, ar.requested_by_principal_id, COALESCE(ar.assigned_principal_id,''), ar.expires_at, ar.created_at, a.tool_name, a.operation, a.target, a.effect, t.id, a.policy_version, agent.display_name FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id JOIN gantry.runs r ON r.id=ar.run_id JOIN gantry.tasks t ON t.id=r.task_id JOIN gantry.agents agent ON agent.id=t.agent_id WHERE ar.id=$1 AND (ar.assigned_principal_id=$2 OR (ar.assigned_principal_id IS NULL AND ar.requested_by_principal_id=$2))`, approvalID, actor.ID).Scan(&item.ID, &item.RunID, &item.ActionID, &item.ActionDigest, &item.Revision, &previewJSON, &item.RiskClass, &item.Status, &item.RequestedBy, &item.AssignedTo, &item.ExpiresAt, &item.CreatedAt, &item.ToolName, &item.Operation, &item.Target, &item.Effect, &item.TaskID, &item.PolicyVersion, &item.AgentDisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	if err != nil {
		return Request{}, err
	}
	if err := json.Unmarshal(previewJSON, &item.ActionPreview); err != nil {
		return Request{}, err
	}
	var decision Decision
	err = s.pool.QueryRow(ctx, `SELECT decision, reason, principal_id, created_at FROM gantry.approval_decisions WHERE approval_id=$1 ORDER BY created_at DESC LIMIT 1`, approvalID).Scan(&decision.Decision, &decision.Reason, &decision.DecidedBy, &decision.CreatedAt)
	if err == nil {
		item.Decision = &decision
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Request{}, err
	}
	return item, nil
}

func (s *Service) Decide(ctx context.Context, actor identity.Principal, input DecisionInput) (Resolution, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.ActionDigest = strings.TrimSpace(input.ActionDigest)
	input.Decision = strings.TrimSpace(input.Decision)
	input.Idempotency = strings.TrimSpace(input.Idempotency)
	if input.ID == "" || input.ActionDigest == "" || input.Idempotency == "" || input.Revision < 1 || (input.Decision != "approve" && input.Decision != "reject") {
		return Resolution{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Resolution{}, err
	}
	var status, requestedBy, assignedTo, actionState string
	var runID, actionID, callID, digest, permitID string
	var expiresAt time.Time
	var permitExpiresAt *time.Time
	var leaseEpoch, permitLeaseEpoch uint64
	var revision int64
	err = tx.QueryRow(ctx, `SELECT ar.status, ar.requested_by_principal_id, COALESCE(ar.assigned_principal_id,''), ar.run_id, ar.action_id, a.runner_call_id, ar.action_digest, ar.expires_at, a.state, a.revision, COALESCE(a.execution_permit_id,''), a.execution_permit_expires_at, a.execution_permit_lease_epoch, r.lease_epoch FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id JOIN gantry.runs r ON r.id=ar.run_id WHERE ar.id=$1 FOR UPDATE`, input.ID).Scan(&status, &requestedBy, &assignedTo, &runID, &actionID, &callID, &digest, &expiresAt, &actionState, &revision, &permitID, &permitExpiresAt, &permitLeaseEpoch, &leaseEpoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resolution{}, ErrNotFound
	}
	if err != nil {
		return Resolution{}, err
	}
	if assignedTo != "" && assignedTo != actor.ID {
		return Resolution{}, ErrNotEligible
	}
	if assignedTo == "" && requestedBy != actor.ID {
		return Resolution{}, ErrNotEligible
	}
	if digest != input.ActionDigest {
		return Resolution{}, ErrInvalidDigest
	}
	if revision != input.Revision {
		return Resolution{}, ErrChanged
	}
	var existingDecision, existingKey string
	err = tx.QueryRow(ctx, `SELECT decision, idempotency_key FROM gantry.approval_decisions WHERE approval_id=$1 AND principal_id=$2`, input.ID, actor.ID).Scan(&existingDecision, &existingKey)
	if err == nil {
		if existingKey != input.Idempotency {
			return Resolution{}, ErrAlreadyDecided
		}
		if existingDecision != input.Decision {
			return Resolution{}, ErrIdempotency
		}
		var existingPermitExpiry time.Time
		if permitExpiresAt != nil {
			existingPermitExpiry = *permitExpiresAt
		}
		return Resolution{ApprovalID: input.ID, RunID: runID, ActionID: actionID, CallID: callID, ActionDigest: digest, Decision: existingDecision, Reason: input.Reason, PermitID: permitID, PermitLeaseEpoch: permitLeaseEpoch, PermitExpiresAt: existingPermitExpiry}, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Resolution{}, err
	}
	if status == "expired" {
		return Resolution{}, ErrExpired
	}
	if status != "pending" || actionState != "awaiting_approval" {
		return Resolution{}, ErrAlreadyDecided
	}
	if !expiresAt.After(time.Now().UTC()) {
		if _, err := tx.Exec(ctx, `UPDATE gantry.approval_requests SET status='expired', decided_at=now() WHERE id=$1 AND status='pending'`, input.ID); err != nil {
			return Resolution{}, err
		}
		return Resolution{}, ErrExpired
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.approval_decisions (approval_id, principal_id, decision, reason, action_digest, idempotency_key) VALUES ($1,$2,$3,$4,$5,$6)`, input.ID, actor.ID, input.Decision, strings.TrimSpace(input.Reason), digest, input.Idempotency); err != nil {
		return Resolution{}, err
	}
	newStatus, newActionState := "rejected", "rejected"
	if input.Decision == "approve" {
		newStatus, newActionState = "satisfied", "ready"
		expiry := time.Now().UTC().Add(2 * time.Minute)
		permitID = newID("permit")
		permitExpiresAt = &expiry
		permitLeaseEpoch = leaseEpoch
	} else {
		permitID = ""
		permitExpiresAt = nil
		permitLeaseEpoch = 0
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.approval_requests SET status=$2, decided_at=now() WHERE id=$1`, input.ID, newStatus); err != nil {
		return Resolution{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.actions SET state=$2, revision=revision+1, execution_permit_id=$3, execution_permit_lease_epoch=$4, execution_permit_expires_at=$5 WHERE id=$1 AND state='awaiting_approval'`, actionID, newActionState, permitID, permitLeaseEpoch, permitExpiresAt); err != nil {
		return Resolution{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='accepted' WHERE id=$1 AND status='awaiting_approval'`, runID); err != nil {
		return Resolution{}, err
	}
	taskStatus := "awaiting_requester_input"
	if input.Decision == "approve" {
		taskStatus = "running"
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status=$2, conversation_revision=conversation_revision+1 WHERE current_run_id=$1 AND status='awaiting_approval'`, runID, taskStatus); err != nil {
		return Resolution{}, err
	}
	if err := appendActionSummary(ctx, tx, runID, actionID, newActionState); err != nil {
		return Resolution{}, err
	}
	approvalMessage := "Action approval rejected."
	if input.Decision == "approve" {
		approvalMessage = "Action approval approved."
	}
	if err := appendApprovalStatus(ctx, tx, runID, "approval."+newStatus, approvalMessage); err != nil {
		return Resolution{}, err
	}
	eventType := "approval.rejected"
	if input.Decision == "approve" {
		eventType = "approval.satisfied"
	}
	if err := appendEvent(ctx, tx, runID, eventType, map[string]any{"approval_id": input.ID, "action_digest": digest, "principal_id": actor.ID}); err != nil {
		return Resolution{}, err
	}
	if err := appendEvent(ctx, tx, runID, "run.resumed", map[string]any{"approval_id": input.ID}); err != nil {
		return Resolution{}, err
	}
	resolution := Resolution{ApprovalID: input.ID, RunID: runID, ActionID: actionID, CallID: callID, ActionDigest: digest, Decision: input.Decision, Reason: strings.TrimSpace(input.Reason), PermitID: permitID, PermitLeaseEpoch: permitLeaseEpoch}
	if permitExpiresAt != nil {
		resolution.PermitExpiresAt = *permitExpiresAt
	}
	if err := tx.Commit(ctx); err != nil {
		return Resolution{}, err
	}
	return resolution, nil
}

func appendActionSummary(ctx context.Context, tx pgx.Tx, runID, actionID, state string) error {
	var taskID, toolName, operation, target string
	if err := tx.QueryRow(ctx, `SELECT r.task_id, a.tool_name, a.operation, a.target FROM gantry.actions a JOIN gantry.runs r ON r.id=a.run_id WHERE a.id=$1 AND a.run_id=$2`, actionID, runID).Scan(&taskID, &toolName, &operation, &target); err != nil {
		return err
	}
	summary := strings.TrimSpace(toolName + " " + operation)
	if target = strings.TrimSpace(target); target != "" {
		summary += " for " + target
	}
	return taskmessage.Append(ctx, tx, taskID, runID, "system_summary", taskmessage.ActionSummary(actionID, summary, state))
}

func appendApprovalStatus(ctx context.Context, tx pgx.Tx, runID, code, message string) error {
	var taskID string
	if err := tx.QueryRow(ctx, `SELECT task_id FROM gantry.runs WHERE id=$1`, runID).Scan(&taskID); err != nil {
		return err
	}
	return taskmessage.Append(ctx, tx, taskID, runID, "system_summary", taskmessage.Status(code, message))
}

func appendEvent(ctx context.Context, tx pgx.Tx, runID, eventType string, payload any) error {
	return taskevents.Append(ctx, tx, runID, eventType, payload)
}

func preview(action policy.Action) []byte {
	data, _ := json.Marshal(previewMap(action))
	return data
}
func previewMap(action policy.Action) map[string]any {
	return map[string]any{"tool_name": action.ToolName, "operation": action.Operation, "target": action.Target, "effect": action.Effect, "credential_mode": action.CredentialMode}
}
func riskClass(effect string) string {
	if effect == "destructive" {
		return "high"
	}
	return "write"
}
func newID(prefix string) string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}
