package agentlifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) ListWorkspaces(ctx context.Context, actor identity.Principal) ([]authorization.Workspace, error) {
	return s.authz.ListWorkspaces(ctx, actor)
}

type AgentListOptions struct {
	WorkspaceID string
	Search      string
	Status      string
}

func (s *Service) ListAgents(ctx context.Context, actor identity.Principal, options AgentListOptions) ([]Agent, error) {
	options.WorkspaceID = strings.TrimSpace(options.WorkspaceID)
	options.Search = strings.TrimSpace(options.Search)
	options.Status = strings.TrimSpace(options.Status)
	if options.WorkspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, options.WorkspaceID); err != nil {
			return nil, err
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.organization_id, a.workspace_id, a.slug, a.display_name, a.description, a.category,
			COALESCE(production.status, 'draft'), COALESCE(production.revision_hash, '')
		FROM gantry.agents a
		LEFT JOIN gantry.agent_deployments production ON production.agent_id=a.id AND production.environment_kind='production' AND production.status='active'
		WHERE a.organization_id=$1 AND ($2='' OR a.workspace_id=$2) AND
			($4='' OR a.display_name ILIKE '%' || $4 || '%' OR a.slug ILIKE '%' || $4 || '%' OR a.description ILIKE '%' || $4 || '%' OR COALESCE(production.revision_hash, '') ILIKE '%' || $4 || '%') AND
			($5='' OR COALESCE(production.status, 'draft')=$5) AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=a.workspace_id)
		)
		ORDER BY a.display_name, a.id`, actor.OrganizationID, options.WorkspaceID, actor.ID, options.Search, options.Status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Agent, 0)
	for rows.Next() {
		var item Agent
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Slug, &item.DisplayName, &item.Description, &item.Category, &item.LifecycleStatus, &item.CurrentProductionRevisionHash); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Create(ctx context.Context, actor identity.Principal, request CreateRequest) (Agent, error) {
	request.Slug = strings.TrimSpace(request.Slug)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.Description = strings.TrimSpace(request.Description)
	request.Category = strings.TrimSpace(request.Category)
	if request.WorkspaceID == "" || request.Slug == "" || request.DisplayName == "" || request.Description == "" || request.Category == "" || !validSlug(request.Slug) {
		return Agent{}, ErrInvalidInput
	}
	if err := s.authz.RequireWorkspace(ctx, actor, request.WorkspaceID); err != nil {
		return Agent{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Agent{}, err
	}
	defer tx.Rollback(ctx)
	agent := Agent{ID: newID("agt"), OrganizationID: actor.OrganizationID, WorkspaceID: request.WorkspaceID, Slug: request.Slug, DisplayName: request.DisplayName, Description: request.Description, Category: request.Category, LifecycleStatus: "draft"}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, agent.ID, agent.OrganizationID, agent.WorkspaceID, actor.ID, agent.Slug, agent.DisplayName, agent.Description, agent.Category); err != nil {
		return Agent{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO gantry.agent_draft_workspaces
		(id, agent_id, name, status, spec_json, working_copy_etag, validation_status, validation_findings, created_by_principal_id, updated_by_principal_id)
		VALUES ($1,$2,'Main','active',$3::jsonb,1,'valid','[]'::jsonb,$4,$4)`, newID("drf"), agent.ID, string(defaultSpec()), actor.ID); err != nil {
		return Agent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, agentID string) (Agent, error) {
	return s.getTargetAgent(ctx, actor, agentID)
}

func appendAudit(ctx context.Context, tx pgx.Tx, organizationID, actorID, resourceType, resourceID, eventType string, payload any) error {
	data := mustJSON(map[string]any{})
	if payload != nil {
		data = mustJSON(payload)
	}
	_, err := tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, organizationID, actorID, resourceType, resourceID, eventType, string(data))
	return err
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func (s *Service) getTargetAgent(ctx context.Context, actor identity.Principal, agentID string) (Agent, error) {
	var agent Agent
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.organization_id, a.workspace_id, a.slug, a.display_name, a.description, a.category,
			COALESCE(production.status, 'draft'), COALESCE(production.revision_hash, '')
		FROM gantry.agents a
		LEFT JOIN gantry.agent_deployments production ON production.agent_id=a.id AND production.environment_kind='production' AND production.status='active'
		WHERE a.id=$1`, agentID).Scan(&agent.ID, &agent.OrganizationID, &agent.WorkspaceID, &agent.Slug, &agent.DisplayName, &agent.Description, &agent.Category, &agent.LifecycleStatus, &agent.CurrentProductionRevisionHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, err
	}
	if err := s.authz.RequireWorkspace(ctx, actor, agent.WorkspaceID); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func validSlug(value string) bool {
	if len(value) > 64 {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || (character == '-' && index > 0 && index < len(value)-1) {
			continue
		}
		return false
	}
	return true
}
