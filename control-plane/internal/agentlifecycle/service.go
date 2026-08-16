package agentlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

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
			COALESCE(production.status, 'draft'), COALESCE(production.revision_hash, '')
		FROM gantry.agents a
		LEFT JOIN gantry.agent_deployments production ON production.agent_id=a.id AND production.environment_kind='production' AND production.status='active'
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
	if len(findings) == 0 {
		assetFindings, err := validateAssetBindings(ctx, s.pool, agent.WorkspaceID, spec)
		if err != nil {
			return Draft{}, err
		}
		findings = append(findings, assetFindings...)
	}
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
	if _, err := tx.Exec(ctx, `UPDATE gantry.agent_reviews SET status='superseded', reviewed_at=now(), review_reason='draft revision changed' WHERE agent_id=$1 AND status IN ('pending','approved')`, agent.ID); err != nil {
		return Draft{}, err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent", agent.ID, "agent.draft_updated", map[string]any{"revision": expectedRevision + 1}); err != nil {
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
	rows, err := s.pool.Query(ctx, `
		SELECT v.id, v.agent_id, v.version, v.source_draft_revision, v.spec_json, v.spec_digest,
			v.created_at, COALESCE(creator.display_name, ''),
			(p.agent_version_id IS NOT NULL), p.created_at, v.prompt_snapshot_json
		FROM gantry.agent_versions v
		JOIN gantry.principals creator ON creator.id=v.created_by_principal_id
		LEFT JOIN gantry.agent_publications p ON p.agent_version_id=v.id AND p.workspace_id=$2 AND p.status='published'
		WHERE v.agent_id=$1 ORDER BY v.version DESC`, agent.ID, agent.WorkspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		var item Version
		var createdAt, publishedAt *time.Time
		var promptJSON []byte
		if err := rows.Scan(&item.ID, &item.AgentID, &item.Version, &item.SourceDraftRevision, &item.Spec, &item.SpecDigest, &createdAt, &item.CreatedBy, &item.Published, &publishedAt, &promptJSON); err != nil {
			return nil, err
		}
		if createdAt != nil {
			item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		}
		if publishedAt != nil {
			item.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
		}
		if len(promptJSON) > 0 && string(promptJSON) != "{}" {
			if err := json.Unmarshal(promptJSON, &item.PromptSnapshot); err != nil {
				return nil, err
			}
		}
		if err := populatePromptSnapshot(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetVersion(ctx context.Context, actor identity.Principal, agentID, versionID string) (Version, error) {
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return Version{}, err
	}
	var item Version
	var createdAt, publishedAt *time.Time
	var promptJSON []byte
	var published bool
	err = s.pool.QueryRow(ctx, `
		SELECT v.id, v.agent_id, v.version, v.source_draft_revision, v.spec_json, v.spec_digest,
			v.created_at, COALESCE(creator.display_name, ''),
			(p.agent_version_id IS NOT NULL), p.created_at, v.prompt_snapshot_json
		FROM gantry.agent_versions v
		JOIN gantry.principals creator ON creator.id=v.created_by_principal_id
		LEFT JOIN gantry.agent_publications p ON p.agent_version_id=v.id AND p.workspace_id=$2 AND p.status='published'
		WHERE v.id=$1 AND v.agent_id=$3`, versionID, agent.WorkspaceID, agent.ID).
		Scan(&item.ID, &item.AgentID, &item.Version, &item.SourceDraftRevision, &item.Spec, &item.SpecDigest, &createdAt, &item.CreatedBy, &published, &publishedAt, &promptJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, err
	}
	item.Published = published
	if createdAt != nil {
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	}
	if publishedAt != nil {
		item.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
	}
	if len(promptJSON) > 0 && string(promptJSON) != "{}" {
		if err := json.Unmarshal(promptJSON, &item.PromptSnapshot); err != nil {
			return Version{}, err
		}
	}
	if item.PromptSnapshot.ContentDigest == "" {
		if err := populatePromptSnapshot(&item); err != nil {
			return Version{}, err
		}
	}
	return item, nil
}

func (s *Service) GetOverview(ctx context.Context, actor identity.Principal, agentID string) (AgentOverview, error) {
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return AgentOverview{}, err
	}
	draft, err := loadDraft(ctx, s.pool, agent.ID)
	if err != nil {
		return AgentOverview{}, err
	}
	overview := AgentOverview{Agent: agent, Draft: draft, RecentActivity: make([]ActivityItem, 0)}
	if current, currentErr := loadCurrentVersion(ctx, s.pool, agent.ID); currentErr == nil {
		if detailed, detailErr := s.GetVersion(ctx, actor, agent.ID, current.ID); detailErr != nil {
			return AgentOverview{}, detailErr
		} else {
			overview.CurrentVersion = &detailed
		}
	} else if !errors.Is(currentErr, ErrNotFound) {
		return AgentOverview{}, currentErr
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM gantry.agent_versions WHERE agent_id=$1`, agent.ID).Scan(&overview.VersionCount); err != nil {
		return AgentOverview{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, event_type, payload, created_at FROM gantry.audit_events WHERE resource_type='agent' AND resource_id=$1 ORDER BY created_at DESC, id DESC LIMIT 12`, agent.ID)
	if err != nil {
		return AgentOverview{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ActivityItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.EventType, &item.Payload, &createdAt); err != nil {
			return AgentOverview{}, err
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		overview.RecentActivity = append(overview.RecentActivity, item)
	}
	return overview, rows.Err()
}

func (s *Service) GetReview(ctx context.Context, actor identity.Principal, agentID string) (Review, error) {
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return Review{}, err
	}
	draft, err := loadDraft(ctx, s.pool, agent.ID)
	if err != nil {
		return Review{}, err
	}
	canonical, digestValue, err := canonicalDigest(draft.Spec)
	if err != nil {
		canonical = draft.Spec
		digestValue = digest(draft.Spec)
	}
	base, err := loadCurrentVersion(ctx, s.pool, agent.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Review{}, err
	}
	var baseSpec json.RawMessage
	if err == nil {
		baseSpec = base.Spec
	}
	diff, summary, err := buildDiff(baseSpec, canonical)
	if err != nil {
		return Review{}, err
	}
	review, err := loadReview(ctx, s.pool, agent.ID, draft.Revision)
	if errors.Is(err, ErrNotFound) {
		return Review{AgentID: agent.ID, DraftRevision: draft.Revision, DraftDigest: digestValue, BaseVersionID: base.ID, BaseVersion: base.Version, Diff: diff, RiskSummary: summary, Status: "not_submitted"}, nil
	}
	if err != nil {
		return Review{}, err
	}
	review.DraftDigest = digestValue
	review.BaseVersionID, review.BaseVersion = base.ID, base.Version
	review.Diff, review.RiskSummary = diff, summary
	return review, nil
}

func (s *Service) SubmitReview(ctx context.Context, actor identity.Principal, agentID string, expectedRevision int, releaseNotes string) (Review, error) {
	if expectedRevision < 1 {
		return Review{}, ErrInvalidInput
	}
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return Review{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback(ctx)
	draft, err := loadDraft(ctx, tx, agent.ID)
	if err != nil {
		return Review{}, err
	}
	if draft.Revision != expectedRevision {
		return Review{}, ErrRevisionConflict
	}
	canonical, digestValue, err := canonicalDigest(draft.Spec)
	if err != nil {
		return Review{}, err
	}
	base, err := loadCurrentVersion(ctx, tx, agent.ID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Review{}, err
	}
	if errors.Is(err, ErrNotFound) {
		base = Version{}
	}
	var baseSpec json.RawMessage
	if base.ID != "" {
		baseSpec = base.Spec
	}
	diff, summary, err := buildDiff(baseSpec, canonical)
	if err != nil {
		return Review{}, err
	}
	var existingStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM gantry.agent_reviews WHERE agent_id=$1 AND draft_revision=$2`, agent.ID, draft.Revision).Scan(&existingStatus)
	if err == nil && existingStatus == "approved" {
		return Review{}, ErrInvalidState
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Review{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agent_reviews SET status='superseded', reviewed_at=now(), review_reason='new review submitted' WHERE agent_id=$1 AND status IN ('pending','approved')`, agent.ID); err != nil {
		return Review{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.agent_reviews (id, agent_id, draft_revision, draft_digest, base_version_id, release_notes, diff_json, risk_summary, status, submitted_by_principal_id) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7::jsonb,$8::jsonb,'pending',$9) ON CONFLICT (agent_id,draft_revision) DO UPDATE SET draft_digest=EXCLUDED.draft_digest, base_version_id=EXCLUDED.base_version_id, release_notes=EXCLUDED.release_notes, diff_json=EXCLUDED.diff_json, risk_summary=EXCLUDED.risk_summary, status='pending', submitted_by_principal_id=EXCLUDED.submitted_by_principal_id, reviewed_by_principal_id=NULL, review_reason='', submitted_at=now(), reviewed_at=NULL`, newID("rev"), agent.ID, draft.Revision, digestValue, base.ID, strings.TrimSpace(releaseNotes), string(mustJSON(diff)), string(mustJSON(summary)), actor.ID); err != nil {
		return Review{}, err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent", agent.ID, "agent.review_submitted", map[string]any{"draft_revision": draft.Revision, "draft_digest": digestValue}); err != nil {
		return Review{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Review{}, err
	}
	return s.GetReview(ctx, actor, agent.ID)
}

func (s *Service) DecideReview(ctx context.Context, actor identity.Principal, agentID, decision, reason string) (Review, error) {
	if decision != "approve" && decision != "reject" {
		return Review{}, ErrInvalidInput
	}
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return Review{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Review{}, err
	}
	defer tx.Rollback(ctx)
	draft, err := loadDraft(ctx, tx, agent.ID)
	if err != nil {
		return Review{}, err
	}
	var reviewID, status string
	err = tx.QueryRow(ctx, `SELECT id, status FROM gantry.agent_reviews WHERE agent_id=$1 AND draft_revision=$2 FOR UPDATE`, agent.ID, draft.Revision).Scan(&reviewID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrReviewRequired
	}
	if err != nil {
		return Review{}, err
	}
	if status != "pending" {
		return Review{}, ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agent_reviews SET status=$2, reviewed_by_principal_id=$3, review_reason=$4, reviewed_at=now() WHERE id=$1`, reviewID, decision+"d", actor.ID, strings.TrimSpace(reason)); err != nil {
		return Review{}, err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent_review", reviewID, "agent.review_"+decision+"d", map[string]any{"agent_id": agent.ID, "draft_revision": draft.Revision, "reason": strings.TrimSpace(reason)}); err != nil {
		return Review{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Review{}, err
	}
	return s.GetReview(ctx, actor, agent.ID)
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
	var reviewStatus, reviewDigest string
	err = tx.QueryRow(ctx, `SELECT status, draft_digest FROM gantry.agent_reviews WHERE agent_id=$1 AND draft_revision=$2`, agent.ID, draft.Revision).Scan(&reviewStatus, &reviewDigest)
	if errors.Is(err, pgx.ErrNoRows) || reviewStatus != "approved" {
		return Version{}, false, ErrReviewRequired
	}
	if err != nil {
		return Version{}, false, err
	}
	canonicalDraft, currentDigest, err := canonicalDigest(draft.Spec)
	if err != nil || currentDigest != reviewDigest {
		return Version{}, false, ErrReviewRequired
	}
	assetFindings, err := validateAssetBindings(ctx, s.pool, agent.WorkspaceID, canonicalDraft)
	if err != nil {
		return Version{}, false, err
	}
	if len(assetFindings) != 0 {
		return Version{}, false, ErrInvalidState
	}
	if existing, found, err := loadVersionForDraft(ctx, tx, agent.ID, draft.Revision); err != nil || found {
		if err != nil {
			return Version{}, false, err
		}
		return existing, true, tx.Commit(ctx)
	}
	canonical := canonicalDraft
	promptSnapshot, err := CompilePromptSnapshot(canonical)
	if err != nil {
		return Version{}, false, err
	}
	var nextVersion int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0)+1 FROM gantry.agent_versions WHERE agent_id=$1`, agent.ID).Scan(&nextVersion); err != nil {
		return Version{}, false, err
	}
	digest := sha256.Sum256(canonical)
	version := Version{ID: newID("agtv"), AgentID: agent.ID, Version: nextVersion, SourceDraftRevision: draft.Revision, Spec: canonical, SpecDigest: "sha256:" + hex.EncodeToString(digest[:])}
	promptJSON, err := json.Marshal(promptSnapshot)
	if err != nil {
		return Version{}, false, err
	}
	version.PromptSnapshot = promptSnapshot
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.agent_versions (id, agent_id, version, source_draft_revision, spec_json, spec_digest, created_by_principal_id, prompt_snapshot_json, prompt_snapshot_digest, prompt_compiler_version) VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8::jsonb,$9,$10)`, version.ID, version.AgentID, version.Version, version.SourceDraftRevision, string(version.Spec), version.SpecDigest, actor.ID, string(promptJSON), promptSnapshot.ContentDigest, promptSnapshot.CompilerVersion); err != nil {
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
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent", agent.ID, "agent.published", map[string]any{"version_id": version.ID, "draft_revision": draft.Revision}); err != nil {
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
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent", agent.ID, "agent.retired", nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Rollback(ctx context.Context, actor identity.Principal, agentID, versionID string) error {
	if strings.TrimSpace(versionID) == "" {
		return ErrInvalidInput
	}
	agent, err := s.Get(ctx, actor, agentID)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var workspaceID, currentVersionID string
	err = tx.QueryRow(ctx, `SELECT workspace_id, agent_version_id FROM gantry.agent_publications WHERE agent_id=$1 AND status='published' FOR UPDATE`, agent.ID).Scan(&workspaceID, &currentVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidState
	}
	if err != nil {
		return err
	}
	if currentVersionID == versionID {
		return ErrInvalidState
	}
	var targetAgentID string
	if err := tx.QueryRow(ctx, `SELECT agent_id FROM gantry.agent_versions WHERE id=$1`, versionID).Scan(&targetAgentID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if targetAgentID != agent.ID {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agent_publications SET status='retired', retired_at=now() WHERE agent_id=$1 AND workspace_id=$2 AND status='published'`, agent.ID, workspaceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status, published_by_principal_id) VALUES ($1,$2,$3,$4,'published',$5)`, newID("pub"), agent.ID, versionID, workspaceID, actor.ID); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent", agent.ID, "agent.rolled_back", map[string]any{"from_version_id": currentVersionID, "to_version_id": versionID}); err != nil {
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
		WHERE a.id=$1`, agentID).Scan(&agent.ID, &agent.OrganizationID, &agent.WorkspaceID, &agent.Slug, &agent.DisplayName, &agent.Description, &agent.Category, &agent.LifecycleStatus, &agent.CurrentProductionRevisionHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	return agent, err
}

func loadCurrentVersion(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string) (Version, error) {
	var version Version
	err := querier.QueryRow(ctx, `SELECT v.id, v.agent_id, v.version, v.source_draft_revision, v.spec_json, v.spec_digest FROM gantry.agent_publications p JOIN gantry.agent_versions v ON v.id=p.agent_version_id WHERE p.agent_id=$1 AND p.status='published'`, agentID).Scan(&version.ID, &version.AgentID, &version.Version, &version.SourceDraftRevision, &version.Spec, &version.SpecDigest)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	return version, err
}

func populatePromptSnapshot(version *Version) error {
	if version.PromptSnapshot.ContentDigest != "" {
		return nil
	}
	snapshot, err := CompilePromptSnapshot(version.Spec)
	if err != nil {
		return err
	}
	version.PromptSnapshot = snapshot
	return nil
}

func loadReview(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string, revision int) (Review, error) {
	var review Review
	var baseVersionID *string
	var diffJSON, summaryJSON []byte
	var submittedAt, reviewedAt *time.Time
	err := querier.QueryRow(ctx, `SELECT id, agent_id, draft_revision, draft_digest, base_version_id, release_notes, diff_json, risk_summary, status, submitted_by_principal_id, COALESCE(reviewed_by_principal_id,''), review_reason, submitted_at, reviewed_at FROM gantry.agent_reviews WHERE agent_id=$1 AND draft_revision=$2`, agentID, revision).Scan(&review.ID, &review.AgentID, &review.DraftRevision, &review.DraftDigest, &baseVersionID, &review.ReleaseNotes, &diffJSON, &summaryJSON, &review.Status, &review.SubmittedBy, &review.ReviewedBy, &review.ReviewReason, &submittedAt, &reviewedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	if err != nil {
		return Review{}, err
	}
	if baseVersionID != nil {
		review.BaseVersionID = *baseVersionID
	}
	if err := json.Unmarshal(diffJSON, &review.Diff); err != nil {
		return Review{}, err
	}
	if err := json.Unmarshal(summaryJSON, &review.RiskSummary); err != nil {
		return Review{}, err
	}
	if submittedAt != nil {
		review.SubmittedAt = submittedAt.UTC().Format(time.RFC3339)
	}
	if reviewedAt != nil {
		review.ReviewedAt = reviewedAt.UTC().Format(time.RFC3339)
	}
	return review, nil
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func appendAudit(ctx context.Context, tx pgx.Tx, organizationID, actorID, resourceType, resourceID, eventType string, payload any) error {
	data := mustJSON(map[string]any{})
	if payload != nil {
		data = mustJSON(payload)
	}
	_, err := tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, organizationID, actorID, resourceType, resourceID, eventType, string(data))
	return err
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
