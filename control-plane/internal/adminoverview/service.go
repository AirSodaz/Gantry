// Package adminoverview owns scope-authorized aggregates for the Admin home.
package adminoverview

import (
	"context"
	"encoding/json"
	"time"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Scope struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Label       string `json:"label"`
}

type Metrics struct {
	AgentsTotal         int `json:"agents_total"`
	PublishedAgents     int `json:"published_agents"`
	DraftsNeedingReview int `json:"drafts_needing_review"`
	InvalidDrafts       int `json:"invalid_drafts"`
	ActiveRuns          int `json:"active_runs"`
	AwaitingApprovals   int `json:"awaiting_approvals"`
	FailedRuns24Hours   int `json:"failed_runs_24_hours"`
}

type AttentionItem struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Href        string `json:"href"`
	CreatedAt   string `json:"created_at"`
}

type Publication struct {
	AgentID      string `json:"agent_id"`
	AgentName    string `json:"agent_name"`
	WorkspaceID  string `json:"workspace_id"`
	RevisionHash string `json:"revision_hash"`
	PublishedAt  string `json:"published_at"`
}

type ActivityItem struct {
	ID        int64           `json:"id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"created_at"`
}

type Overview struct {
	Scope              Scope           `json:"scope"`
	GeneratedAt        string          `json:"generated_at"`
	Metrics            Metrics         `json:"metrics"`
	Attention          []AttentionItem `json:"attention"`
	RecentPublications []Publication   `json:"recent_publications"`
	RecentActivity     []ActivityItem  `json:"recent_activity"`
	UnavailableSignals []string        `json:"unavailable_signals"`
}

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, workspaceID string) (Overview, error) {
	if workspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, workspaceID); err != nil {
			return Overview{}, err
		}
	}

	overview := Overview{
		Scope:              Scope{WorkspaceID: workspaceID, Label: "All manageable workspaces"},
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		Attention:          make([]AttentionItem, 0),
		RecentPublications: make([]Publication, 0),
		RecentActivity:     make([]ActivityItem, 0),
		UnavailableSignals: []string{
			"Provider health is not available until platform provider management is implemented.",
			"Runner capacity is not available until runner pool operations are implemented.",
		},
	}
	if workspaceID != "" {
		overview.Scope.Label = "Selected workspace"
	}
	if err := s.loadMetrics(ctx, actor, workspaceID, &overview.Metrics); err != nil {
		return Overview{}, err
	}
	if err := s.loadAttention(ctx, actor, workspaceID, &overview.Attention); err != nil {
		return Overview{}, err
	}
	if err := s.loadPublications(ctx, actor, workspaceID, &overview.RecentPublications); err != nil {
		return Overview{}, err
	}
	if err := s.loadActivity(ctx, actor, workspaceID, &overview.RecentActivity); err != nil {
		return Overview{}, err
	}
	return overview, nil
}

const accessibleAgent = `
	a.organization_id=$1 AND ($2='' OR a.workspace_id=$2) AND (
		EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
		OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=a.workspace_id)
	)`

func (s *Service) loadMetrics(ctx context.Context, actor identity.Principal, workspaceID string, metrics *Metrics) error {
	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE EXISTS (SELECT 1 FROM gantry.agent_deployments p WHERE p.agent_id=a.id AND p.workspace_id=a.workspace_id AND p.environment_kind='production' AND p.status='active')),
			count(*) FILTER (WHERE EXISTS (SELECT 1 FROM gantry.agent_revision_reviews r WHERE r.agent_id=a.id AND r.status='pending')),
			count(*) FILTER (WHERE d.validation_status='invalid')
		FROM gantry.agents a
		JOIN gantry.agent_draft_workspaces d ON d.agent_id=a.id AND d.name='Main'
		WHERE `+accessibleAgent, actor.OrganizationID, workspaceID, actor.ID).Scan(
		&metrics.AgentsTotal, &metrics.PublishedAgents, &metrics.DraftsNeedingReview, &metrics.InvalidDrafts)
	if err != nil {
		return err
	}

	return s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE r.status IN ('queued', 'assigned', 'accepted', 'awaiting_approval', 'canceling')),
			count(*) FILTER (WHERE r.status='awaiting_approval'),
			count(*) FILTER (WHERE r.status='failed' AND r.completed_at >= now() - interval '24 hours')
		FROM gantry.runs r
		JOIN gantry.tasks t ON t.id=r.task_id
		JOIN gantry.agents a ON a.id=t.agent_id
		WHERE `+accessibleAgent, actor.OrganizationID, workspaceID, actor.ID).Scan(
		&metrics.ActiveRuns, &metrics.AwaitingApprovals, &metrics.FailedRuns24Hours)
}

func (s *Service) loadAttention(ctx context.Context, actor identity.Principal, workspaceID string, target *[]AttentionItem) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, severity, title, description, href, created_at FROM (
			SELECT 'invalid-draft:' || a.id AS id, 'invalid_draft' AS kind, 'high' AS severity, a.display_name || ' has an invalid draft' AS title, 'Resolve validation findings before review or publication.' AS description, '/agents/' || a.id || '/design' AS href, d.updated_at AS created_at
			FROM gantry.agents a JOIN gantry.agent_draft_workspaces d ON d.agent_id=a.id AND d.name='Main'
			WHERE `+accessibleAgent+` AND d.validation_status='invalid'
			UNION ALL
			SELECT 'review:' || r.id AS id, 'review' AS kind, 'medium' AS severity, a.display_name || ' is awaiting review' AS title, 'An approved review is required before Production deployment.' AS description, '/agents/' || a.id || '/design' AS href, r.submitted_at AS created_at
			FROM gantry.agent_revision_reviews r JOIN gantry.agents a ON a.id=r.agent_id
			WHERE `+accessibleAgent+` AND r.status='pending'
			UNION ALL
			SELECT 'approval:' || ar.id AS id, 'approval' AS kind, 'high' AS severity, a.display_name || ' has a requester approval pending' AS title, 'The requester must decide the exact action before this run continues.' AS description, '/agents/' || a.id AS href, ar.created_at AS created_at
			FROM gantry.approval_requests ar JOIN gantry.runs run ON run.id=ar.run_id JOIN gantry.tasks t ON t.id=run.task_id JOIN gantry.agents a ON a.id=t.agent_id
			WHERE `+accessibleAgent+` AND ar.status='pending'
			UNION ALL
			SELECT 'failed-run:' || run.id AS id, 'failed_run' AS kind, 'high' AS severity, a.display_name || ' has a failed run' AS title, COALESCE(NULLIF(run.status_reason, ''), 'Inspect the run status before retrying.') AS description, '/agents/' || a.id AS href, COALESCE(run.completed_at, run.created_at) AS created_at
			FROM gantry.runs run JOIN gantry.tasks t ON t.id=run.task_id JOIN gantry.agents a ON a.id=t.agent_id
			WHERE `+accessibleAgent+` AND run.status='failed' AND run.completed_at >= now() - interval '24 hours'
		) attention
		ORDER BY CASE severity WHEN 'high' THEN 0 ELSE 1 END, created_at DESC
		LIMIT 12`, actor.OrganizationID, workspaceID, actor.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item AttentionItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Kind, &item.Severity, &item.Title, &item.Description, &item.Href, &createdAt); err != nil {
			return err
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		*target = append(*target, item)
	}
	return rows.Err()
}

func (s *Service) loadPublications(ctx context.Context, actor identity.Principal, workspaceID string, target *[]Publication) error {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.display_name, a.workspace_id, v.revision_hash, p.updated_at
		FROM gantry.agent_deployments p
		JOIN gantry.agents a ON a.id=p.agent_id
		JOIN gantry.agent_revisions v ON v.id=p.revision_id
		WHERE `+accessibleAgent+` AND p.environment_kind='production' AND p.status='active'
		ORDER BY p.updated_at DESC, p.id DESC LIMIT 8`, actor.OrganizationID, workspaceID, actor.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item Publication
		var publishedAt time.Time
		if err := rows.Scan(&item.AgentID, &item.AgentName, &item.WorkspaceID, &item.RevisionHash, &publishedAt); err != nil {
			return err
		}
		item.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
		*target = append(*target, item)
	}
	return rows.Err()
}

func (s *Service) loadActivity(ctx context.Context, actor identity.Principal, workspaceID string, target *[]ActivityItem) error {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.event_type, e.payload, e.created_at
		FROM gantry.audit_events e
		JOIN gantry.agents a ON a.id=e.resource_id
		WHERE e.resource_type='agent' AND `+accessibleAgent+`
		ORDER BY e.created_at DESC, e.id DESC LIMIT 12`, actor.OrganizationID, workspaceID, actor.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item ActivityItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.EventType, &item.Payload, &createdAt); err != nil {
			return err
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		*target = append(*target, item)
	}
	return rows.Err()
}
