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
	ErrIdempotency    = errors.New("approval idempotency key reused")
)

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type Request struct {
	ID            string    `json:"id"`
	RunID         string    `json:"run_id"`
	ActionID      string    `json:"action_id"`
	ActionDigest  string    `json:"action_digest"`
	ToolName      string    `json:"tool_name"`
	Operation     string    `json:"operation"`
	Target        string    `json:"target,omitempty"`
	Effect        string    `json:"effect"`
	ActionPreview any       `json:"action_preview"`
	RiskClass     string    `json:"risk_class"`
	Status        string    `json:"status"`
	RequestedBy   string    `json:"requested_by"`
	AssignedTo    string    `json:"assigned_to"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

type Resolution struct {
	ApprovalID   string
	RunID        string
	ActionID     string
	ActionDigest string
	Decision     string
	Reason       string
}

type DecisionInput struct {
	ID           string
	ActionDigest string
	Decision     string
	Reason       string
	Idempotency  string
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
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.actions (id, run_id, tool_name, operation, arguments_json, target, effect, credential_ref, credential_mode, policy_version, action_digest, state, revision, requested_by_principal_id) VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,$10,$11,$12,1,$13)`, actionID, canonical.RunID, canonical.ToolName, canonical.Operation, string(canonical.Arguments), canonical.Target, canonical.Effect, canonical.CredentialRef, canonical.CredentialMode, canonical.PolicyVersion, digest, actionState, canonical.RequestedBy); err != nil {
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
	return Request{ID: approvalID, RunID: canonical.RunID, ActionID: actionID, ActionDigest: digest, ToolName: canonical.ToolName, Operation: canonical.Operation, Target: canonical.Target, Effect: canonical.Effect, ActionPreview: previewMap(canonical), RiskClass: riskClass(canonical.Effect), Status: "pending", RequestedBy: canonical.RequestedBy, AssignedTo: canonical.RequestedBy, ExpiresAt: expiresAt}, evaluation, nil
}

func (s *Service) List(ctx context.Context, actor identity.Principal, limit int) ([]Request, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	rows, err := s.pool.Query(ctx, `SELECT ar.id, ar.run_id, ar.action_id, ar.action_digest, ar.action_preview, ar.risk_class, ar.status, ar.requested_by_principal_id, ar.assigned_principal_id, ar.expires_at, ar.created_at, a.tool_name, a.operation, a.target, a.effect FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id WHERE ar.status='pending' AND (ar.assigned_principal_id=$1 OR (ar.assigned_principal_id IS NULL AND ar.requested_by_principal_id=$1)) ORDER BY ar.created_at, ar.id LIMIT $2`, actor.ID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Request, 0)
	for rows.Next() {
		var item Request
		var previewJSON []byte
		if err := rows.Scan(&item.ID, &item.RunID, &item.ActionID, &item.ActionDigest, &previewJSON, &item.RiskClass, &item.Status, &item.RequestedBy, &item.AssignedTo, &item.ExpiresAt, &item.CreatedAt, &item.ToolName, &item.Operation, &item.Target, &item.Effect); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(previewJSON, &item.ActionPreview); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Decide(ctx context.Context, actor identity.Principal, input DecisionInput) (Resolution, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.ActionDigest = strings.TrimSpace(input.ActionDigest)
	input.Decision = strings.TrimSpace(input.Decision)
	input.Idempotency = strings.TrimSpace(input.Idempotency)
	if input.ID == "" || input.ActionDigest == "" || input.Idempotency == "" || (input.Decision != "approve" && input.Decision != "reject") {
		return Resolution{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Resolution{}, err
	}
	defer tx.Rollback(ctx)
	var resolution Resolution
	var status, requestedBy, assignedTo, actionState string
	var runID, actionID, digest string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `SELECT ar.status, ar.requested_by_principal_id, COALESCE(ar.assigned_principal_id,''), ar.run_id, ar.action_id, ar.action_digest, ar.expires_at, a.state FROM gantry.approval_requests ar JOIN gantry.actions a ON a.id=ar.action_id WHERE ar.id=$1 FOR UPDATE`, input.ID).Scan(&status, &requestedBy, &assignedTo, &runID, &actionID, &digest, &expiresAt, &actionState)
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
	var existingDecision, existingKey string
	err = tx.QueryRow(ctx, `SELECT decision, idempotency_key FROM gantry.approval_decisions WHERE approval_id=$1 AND principal_id=$2`, input.ID, actor.ID).Scan(&existingDecision, &existingKey)
	if err == nil {
		if existingKey != input.Idempotency {
			return Resolution{}, ErrAlreadyDecided
		}
		if existingDecision != input.Decision {
			return Resolution{}, ErrIdempotency
		}
		return Resolution{ApprovalID: input.ID, RunID: runID, ActionID: actionID, ActionDigest: digest, Decision: existingDecision, Reason: input.Reason}, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Resolution{}, err
	}
	if status != "pending" || actionState != "awaiting_approval" {
		return Resolution{}, ErrAlreadyDecided
	}
	if !expiresAt.After(time.Now().UTC()) {
		if _, err := tx.Exec(ctx, `UPDATE gantry.approval_requests SET status='expired', decided_at=now() WHERE id=$1 AND status='pending'`, input.ID); err != nil { return Resolution{}, err }
		return Resolution{}, ErrExpired
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.approval_decisions (approval_id, principal_id, decision, reason, action_digest, idempotency_key) VALUES ($1,$2,$3,$4,$5,$6)`, input.ID, actor.ID, input.Decision, strings.TrimSpace(input.Reason), digest, input.Idempotency); err != nil {
		return Resolution{}, err
	}
	newStatus, newActionState := "rejected", "rejected"
	if input.Decision == "approve" {
		newStatus, newActionState = "satisfied", "ready"
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.approval_requests SET status=$2, decided_at=now() WHERE id=$1`, input.ID, newStatus); err != nil {
		return Resolution{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.actions SET state=$2, revision=revision+1 WHERE id=$1 AND state='awaiting_approval'`, actionID, newActionState); err != nil {
		return Resolution{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.runs SET status='accepted' WHERE id=$1 AND status='awaiting_approval'`, runID); err != nil {
		return Resolution{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.tasks SET status='running' WHERE current_run_id=$1 AND status='awaiting_approval'`, runID); err != nil {
		return Resolution{}, err
	}
	eventType := "approval.rejected"
	if input.Decision == "approve" {
		eventType = "approval.satisfied"
	}
	if err := appendEvent(ctx, tx, runID, eventType, map[string]any{"approval_id": input.ID, "action_digest": digest, "principal_id": actor.ID}); err != nil {
		return Resolution{}, err
	}
	resolution = Resolution{ApprovalID: input.ID, RunID: runID, ActionID: actionID, ActionDigest: digest, Decision: input.Decision, Reason: strings.TrimSpace(input.Reason)}
	if err := tx.Commit(ctx); err != nil {
		return Resolution{}, err
	}
	return resolution, nil
}

func appendEvent(ctx context.Context, tx pgx.Tx, runID, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var sequence int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.runs SET event_sequence=event_sequence+1 WHERE id=$1 RETURNING event_sequence`, runID).Scan(&sequence); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.run_events (run_id, sequence, event_type, payload) VALUES ($1,$2,$3,$4::jsonb)`, runID, sequence, eventType, string(data))
	return err
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
