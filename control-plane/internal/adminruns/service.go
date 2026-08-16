package adminruns

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("admin run not found")
	ErrInvalidInput = errors.New("invalid admin run query")
)

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) List(ctx context.Context, actor identity.Principal, options ListOptions) ([]Run, error) {
	options = normalizeOptions(options)
	if options.WorkspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, options.WorkspaceID); err != nil {
			return nil, err
		}
	}
	if options.Status != "" && !validStatus(options.Status) {
		return nil, ErrInvalidInput
	}
	rows, err := s.pool.Query(ctx, runSelect+`
		WHERE `+accessibleRun+` AND ($4='' OR t.agent_id=$4) AND ($5='' OR revision.revision_hash=$5) AND ($6='' OR r.status=$6)
		ORDER BY CASE WHEN r.status IN ('queued', 'assigned', 'accepted', 'awaiting_approval', 'canceling') THEN 0 WHEN r.status='failed' THEN 1 ELSE 2 END,
			COALESCE(last_event.created_at, r.completed_at, r.started_at, r.created_at) DESC, r.id DESC
		LIMIT $7`, actor.OrganizationID, options.WorkspaceID, actor.ID, options.AgentID, options.RevisionHash, options.Status, options.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Run, 0)
	for rows.Next() {
		item, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, runID string) (Detail, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return Detail{}, ErrInvalidInput
	}
	item, err := scanRun(s.pool.QueryRow(ctx, runSelect+` WHERE r.id=$4 AND `+accessibleRun, actor.OrganizationID, "", actor.ID, runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, err
	}
	detail := Detail{Run: item, Events: make([]Event, 0), Actions: make([]Action, 0), Approvals: make([]Approval, 0), Artifacts: make([]Artifact, 0)}
	if err := s.loadEvents(ctx, runID, &detail.Events); err != nil {
		return Detail{}, err
	}
	if err := s.loadActions(ctx, runID, &detail.Actions); err != nil {
		return Detail{}, err
	}
	if err := s.loadApprovals(ctx, runID, &detail.Approvals); err != nil {
		return Detail{}, err
	}
	if err := s.loadArtifacts(ctx, runID, &detail.Artifacts); err != nil {
		return Detail{}, err
	}
	return detail, nil
}

const accessibleRun = `
	t.organization_id=$1 AND ($2='' OR t.workspace_id=$2) AND (
		EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
		OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=t.workspace_id)
	)`

const runSelect = `
	SELECT r.id, r.task_id, t.workspace_id, workspace.display_name, t.agent_id, agent.display_name, revision.revision_hash,
		COALESCE(deployment.id, ''), COALESCE(deployment.name, ''), requester.id, requester.display_name,
		r.status, r.status_reason, COALESCE(r.runner_id, ''), r.attempt_number, r.manifest_digest,
		(SELECT count(*) FROM gantry.actions action WHERE action.run_id=r.id),
		(SELECT count(*) FROM gantry.approval_requests approval WHERE approval.run_id=r.id),
		r.created_at, r.started_at, r.completed_at, last_event.created_at
	FROM gantry.runs r
	JOIN gantry.tasks t ON t.id=r.task_id
	JOIN gantry.workspaces workspace ON workspace.id=t.workspace_id
	JOIN gantry.agents agent ON agent.id=t.agent_id
	JOIN gantry.agent_revisions revision ON revision.id=r.agent_revision_id
	JOIN gantry.principals requester ON requester.id=t.requester_principal_id
	LEFT JOIN gantry.agent_deployments deployment ON deployment.id=r.deployment_id
	LEFT JOIN LATERAL (
		SELECT created_at FROM gantry.run_events event WHERE event.run_id=r.id ORDER BY sequence DESC LIMIT 1
	) last_event ON true`

func (s *Service) loadEvents(ctx context.Context, runID string, target *[]Event) error {
	rows, err := s.pool.Query(ctx, `SELECT sequence, event_type, payload, created_at FROM gantry.run_events WHERE run_id=$1 ORDER BY sequence`, runID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item Event
		var createdAt time.Time
		if err := rows.Scan(&item.Sequence, &item.Type, &item.Payload, &createdAt); err != nil {
			return err
		}
		item.CreatedAt = formatTime(createdAt)
		*target = append(*target, item)
	}
	return rows.Err()
}

func (s *Service) loadActions(ctx context.Context, runID string, target *[]Action) error {
	rows, err := s.pool.Query(ctx, `SELECT id, tool_name, operation, target, effect, state, action_digest, created_at, updated_at FROM gantry.actions WHERE run_id=$1 ORDER BY created_at, id`, runID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item Action
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.ToolName, &item.Operation, &item.Target, &item.Effect, &item.State, &item.ActionDigest, &createdAt, &updatedAt); err != nil {
			return err
		}
		item.CreatedAt, item.UpdatedAt = formatTime(createdAt), formatTime(updatedAt)
		*target = append(*target, item)
	}
	return rows.Err()
}

func (s *Service) loadApprovals(ctx context.Context, runID string, target *[]Approval) error {
	rows, err := s.pool.Query(ctx, `SELECT id, action_id, action_digest, risk_class, status, requested_by_principal_id, expires_at, created_at, decided_at FROM gantry.approval_requests WHERE run_id=$1 ORDER BY created_at, id`, runID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item Approval
		var expiresAt, createdAt time.Time
		var decidedAt *time.Time
		if err := rows.Scan(&item.ID, &item.ActionID, &item.ActionDigest, &item.RiskClass, &item.Status, &item.RequestedBy, &expiresAt, &createdAt, &decidedAt); err != nil {
			return err
		}
		item.ExpiresAt, item.CreatedAt = formatTime(expiresAt), formatTime(createdAt)
		if decidedAt != nil {
			item.DecidedAt = formatTime(*decidedAt)
		}
		*target = append(*target, item)
	}
	return rows.Err()
}

func (s *Service) loadArtifacts(ctx context.Context, runID string, target *[]Artifact) error {
	rows, err := s.pool.Query(ctx, `SELECT id, filename, media_type, size_bytes, digest, classification, scan_status, state, created_at FROM gantry.artifacts WHERE run_id=$1 ORDER BY created_at, id`, runID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item Artifact
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Filename, &item.MediaType, &item.SizeBytes, &item.Digest, &item.Classification, &item.ScanStatus, &item.State, &createdAt); err != nil {
			return err
		}
		item.CreatedAt = formatTime(createdAt)
		*target = append(*target, item)
	}
	return rows.Err()
}

type rowScanner interface{ Scan(...any) error }

func scanRun(row rowScanner) (Run, error) {
	var item Run
	var createdAt time.Time
	var startedAt, completedAt, lastEventAt *time.Time
	err := row.Scan(&item.ID, &item.TaskID, &item.WorkspaceID, &item.WorkspaceName, &item.AgentID, &item.AgentName, &item.RevisionHash,
		&item.DeploymentID, &item.DeploymentName, &item.RequesterID, &item.RequesterName, &item.Status, &item.StatusReason, &item.RunnerID,
		&item.AttemptNumber, &item.ManifestDigest, &item.ActionCount, &item.ApprovalCount, &createdAt, &startedAt, &completedAt, &lastEventAt)
	if err != nil {
		return Run{}, err
	}
	item.CreatedAt = formatTime(createdAt)
	if startedAt != nil {
		item.StartedAt = formatTime(*startedAt)
	}
	if completedAt != nil {
		item.CompletedAt = formatTime(*completedAt)
	}
	if lastEventAt != nil {
		item.LastEventAt = formatTime(*lastEventAt)
	}
	return item, nil
}

func normalizeOptions(options ListOptions) ListOptions {
	options.WorkspaceID = strings.TrimSpace(options.WorkspaceID)
	options.AgentID = strings.TrimSpace(options.AgentID)
	options.RevisionHash = strings.TrimSpace(options.RevisionHash)
	options.Status = strings.TrimSpace(options.Status)
	if options.Limit < 1 {
		options.Limit = 50
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	return options
}

func validStatus(value string) bool {
	for _, status := range []string{"queued", "assigned", "accepted", "awaiting_approval", "canceling", "completed", "failed", "canceled"} {
		if value == status {
			return true
		}
	}
	return false
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339) }
