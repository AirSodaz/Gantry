package agentlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func (s *Service) ListAgents(ctx context.Context, actor identity.Principal, workspaceID string) ([]Agent, error) {
	if workspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, workspaceID); err != nil {
			return nil, err
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.organization_id, a.workspace_id, a.slug, a.display_name, a.description, a.category,
			COALESCE(current_publication.status, CASE WHEN retired_publication.agent_id IS NULL THEN 'draft' ELSE 'retired' END),
			COALESCE(current_publication.agent_version_id, '')
		FROM gantry.agents a
		LEFT JOIN gantry.agent_publications current_publication ON current_publication.agent_id=a.id AND current_publication.status='published'
		LEFT JOIN LATERAL (SELECT agent_id FROM gantry.agent_publications WHERE agent_id=a.id AND status='retired' LIMIT 1) retired_publication ON true
		WHERE a.organization_id=$1 AND ($2='' OR a.workspace_id=$2) AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=a.workspace_id)
		)
		ORDER BY a.display_name, a.id`, actor.OrganizationID, workspaceID, actor.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Agent, 0)
	for rows.Next() {
		var item Agent
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Slug, &item.DisplayName, &item.Description, &item.Category, &item.LifecycleStatus, &item.CurrentPublishedVersionID); err != nil {
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
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.agent_drafts (agent_id, revision, spec_json, validation_status, updated_by_principal_id) VALUES ($1, 1, $2::jsonb, 'valid', $3)`, agent.ID, string(defaultSpec()), actor.ID); err != nil {
		return Agent{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, agentID string) (Agent, error) {
	agent, err := s.loadAgent(ctx, s.pool, agentID)
	if err != nil {
		return Agent{}, err
	}
	if err := s.authz.RequireWorkspace(ctx, actor, agent.WorkspaceID); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func (s *Service) GetDraft(ctx context.Context, actor identity.Principal, agentID string) (Draft, error) {
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return Draft{}, err
	}
	return loadDraft(ctx, s.pool, agent.ID)
}

func (s *Service) UpdateDraft(ctx context.Context, actor identity.Principal, agentID string, expectedRevision int, spec json.RawMessage) (Draft, error) {
	if expectedRevision < 1 || len(spec) == 0 {
		return Draft{}, ErrInvalidInput
	}
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return Draft{}, err
	}
	canonical, findings := ValidateSpec(spec)
	stored := spec
	status := "invalid"
	if len(findings) == 0 {
		stored, status = canonical, "valid"
	}
	findingsJSON, err := json.Marshal(findings)
	if err != nil {
		return Draft{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Draft{}, err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE gantry.agent_drafts SET revision=revision+1, spec_json=$3::jsonb, validation_status=$4, validation_findings=$5::jsonb, updated_by_principal_id=$6, updated_at=now() WHERE agent_id=$1 AND revision=$2`, agent.ID, expectedRevision, string(stored), status, string(findingsJSON), actor.ID)
	if err != nil {
		return Draft{}, err
	}
	if command.RowsAffected() == 0 {
		return Draft{}, ErrRevisionConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agents SET updated_at=now() WHERE id=$1`, agent.ID); err != nil {
		return Draft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Draft{}, err
	}
	return s.GetDraft(ctx, actor, agent.ID)
}

func (s *Service) ListVersions(ctx context.Context, actor identity.Principal, agentID string) ([]Version, error) {
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, agent_id, version, source_draft_revision, spec_json, spec_digest FROM gantry.agent_versions WHERE agent_id=$1 ORDER BY version DESC`, agent.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		var item Version
		if err := rows.Scan(&item.ID, &item.AgentID, &item.Version, &item.SourceDraftRevision, &item.Spec, &item.SpecDigest); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Publish(ctx context.Context, actor identity.Principal, agentID string, expectedRevision int) (Version, bool, error) {
	if expectedRevision < 1 {
		return Version{}, false, ErrInvalidInput
	}
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return Version{}, false, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, false, err
	}
	defer tx.Rollback(ctx)
	draft, err := loadDraft(ctx, tx, agent.ID)
	if err != nil {
		return Version{}, false, err
	}
	if draft.Revision != expectedRevision {
		return Version{}, false, ErrRevisionConflict
	}
	if draft.ValidationStatus != "valid" {
		return Version{}, false, ErrInvalidState
	}
	if existing, found, err := loadVersionForDraft(ctx, tx, agent.ID, draft.Revision); err != nil || found {
		if err != nil {
			return Version{}, false, err
		}
		return existing, true, tx.Commit(ctx)
	}
	canonical, findings := ValidateSpec(draft.Spec)
	if len(findings) != 0 {
		return Version{}, false, ErrInvalidState
	}
	var nextVersion int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0)+1 FROM gantry.agent_versions WHERE agent_id=$1`, agent.ID).Scan(&nextVersion); err != nil {
		return Version{}, false, err
	}
	digest := sha256.Sum256(canonical)
	version := Version{ID: newID("agtv"), AgentID: agent.ID, Version: nextVersion, SourceDraftRevision: draft.Revision, Spec: canonical, SpecDigest: "sha256:" + hex.EncodeToString(digest[:])}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.agent_versions (id, agent_id, version, source_draft_revision, spec_json, spec_digest, created_by_principal_id) VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7)`, version.ID, version.AgentID, version.Version, version.SourceDraftRevision, string(version.Spec), version.SpecDigest, actor.ID); err != nil {
		return Version{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agent_publications SET status='retired', retired_at=now() WHERE agent_id=$1 AND workspace_id=$2 AND status='published'`, agent.ID, agent.WorkspaceID); err != nil {
		return Version{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status, published_by_principal_id) VALUES ($1,$2,$3,$4,'published',$5)`, newID("pub"), agent.ID, version.ID, agent.WorkspaceID, actor.ID); err != nil {
		return Version{}, false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agents SET updated_at=now() WHERE id=$1`, agent.ID); err != nil {
		return Version{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, false, err
	}
	return version, false, nil
}

func (s *Service) Retire(ctx context.Context, actor identity.Principal, agentID string) error {
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	command, err := tx.Exec(ctx, `UPDATE gantry.agent_publications SET status='retired', retired_at=now() WHERE agent_id=$1 AND workspace_id=$2 AND status='published'`, agent.ID, agent.WorkspaceID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agents SET updated_at=now() WHERE id=$1`, agent.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) loadAgent(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string) (Agent, error) {
	var agent Agent
	err := querier.QueryRow(ctx, `
		SELECT a.id, a.organization_id, a.workspace_id, a.slug, a.display_name, a.description, a.category,
			COALESCE(current_publication.status, CASE WHEN retired_publication.agent_id IS NULL THEN 'draft' ELSE 'retired' END),
			COALESCE(current_publication.agent_version_id, '')
		FROM gantry.agents a
		LEFT JOIN gantry.agent_publications current_publication ON current_publication.agent_id=a.id AND current_publication.status='published'
		LEFT JOIN LATERAL (SELECT agent_id FROM gantry.agent_publications WHERE agent_id=a.id AND status='retired' LIMIT 1) retired_publication ON true
		WHERE a.id=$1`, agentID).Scan(&agent.ID, &agent.OrganizationID, &agent.WorkspaceID, &agent.Slug, &agent.DisplayName, &agent.Description, &agent.Category, &agent.LifecycleStatus, &agent.CurrentPublishedVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	return agent, err
}

func loadDraft(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string) (Draft, error) {
	var draft Draft
	var findings []byte
	err := querier.QueryRow(ctx, `SELECT agent_id, revision, spec_json, validation_status, validation_findings, updated_by_principal_id FROM gantry.agent_drafts WHERE agent_id=$1`, agentID).Scan(&draft.AgentID, &draft.Revision, &draft.Spec, &draft.ValidationStatus, &findings, &draft.UpdatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return Draft{}, ErrNotFound
	}
	if err != nil {
		return Draft{}, err
	}
	if err := json.Unmarshal(findings, &draft.ValidationFindings); err != nil {
		return Draft{}, err
	}
	return draft, nil
}

func loadVersionForDraft(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string, revision int) (Version, bool, error) {
	var version Version
	err := querier.QueryRow(ctx, `SELECT id, agent_id, version, source_draft_revision, spec_json, spec_digest FROM gantry.agent_versions WHERE agent_id=$1 AND source_draft_revision=$2`, agentID, revision).Scan(&version.ID, &version.AgentID, &version.Version, &version.SourceDraftRevision, &version.Spec, &version.SpecDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, false, nil
	}
	return version, err == nil, err
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
